package kline

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"time"

	"github.com/jroimartin/gocui"
	"github.com/roffe/ismtool/message"
	"go.bug.st/serial"
)

type Engine struct {
	g    *gocui.Gui
	port serial.Port

	in  chan message.Message
	out chan message.Message

	sent chan message.Message

	register   chan *Subscriber
	unregister chan *Subscriber

	listeners map[*Subscriber]bool

	quit chan struct{}
}

func New(g *gocui.Gui, portName string) (*Engine, error) {
	mode := &serial.Mode{
		BaudRate: 9600,
		DataBits: 8,
		Parity:   serial.OddParity,
		StopBits: serial.OneStopBit,
	}

	sr, err := serial.Open(portName, mode)
	if err != nil {
		return nil, err
	}

	sr.ResetInputBuffer()
	sr.ResetOutputBuffer()

	if err := sr.SetReadTimeout(2 * time.Millisecond); err != nil {
		log.Fatal(err)
	}

	e := &Engine{
		g:    g,
		port: sr,

		in:   make(chan message.Message, 100),
		out:  make(chan message.Message, 100),
		sent: make(chan message.Message, 100),

		register:   make(chan *Subscriber, 10),
		unregister: make(chan *Subscriber, 10),

		listeners: map[*Subscriber]bool{},

		quit: make(chan struct{}),
	}

	e.run()

	return e, nil
}

func (e *Engine) Close() error {
	close(e.out)
	close(e.quit)
	return e.port.Close()
}

func (e *Engine) Send(msg message.Message) error {
	select {
	case e.out <- msg:
	default:
		return fmt.Errorf("send buffer full")
	}
	return nil
}

func (e *Engine) SendAndRecv(ctx context.Context, msg message.Message, identifiers ...uint8) (message.Message, error) {
	sub := e.Subscribe(ctx, identifiers...)
	defer sub.Close()
	if err := e.Send(msg); err != nil {
		return nil, err
	}

	select {
	case <-ctx.Done():
	case <-sub.Chan():
	}

	return nil, nil
}

func (e *Engine) Subscribe(ctx context.Context, identifiers ...uint8) *Subscriber {
	cb := make(chan message.Message, 10)
	sub := &Subscriber{
		e:           e,
		ctx:         ctx,
		callback:    cb,
		identifiers: identifiers,
	}
	select {
	case e.register <- sub:
	default:
		panic("could not register subscriber")
	}
	return sub
}

func (e *Engine) run() {
	go e.handler()
	go e.serialReader() // Start serial port reader
	go e.serialWriter() // Start serial port writer
}

func (e *Engine) handler() {
	for {
		select {
		case <-e.quit:
			return
		case r := <-e.register:
			e.listeners[r] = true
		case r := <-e.unregister:
			delete(e.listeners, r)
		case msg := <-e.in:
			for l := range e.listeners {
				select {
				case <-l.ctx.Done():
					e.unregister <- l
					continue
				default:
				}

				if len(l.identifiers) == 0 {
					select {
					case l.callback <- msg:
					default:
						l.errcount++
					}
					continue
				}
				for _, idf := range l.identifiers {
					if l.errcount > 10 {
						select {
						case e.unregister <- l:
						default:
							panic("could not queue unregister")
						}
					}
					if idf == msg.ID() {
						select {
						case l.callback <- msg:
						default:
							l.errcount++
						}
						break
					}
				}
			}
		}
	}
}

func (e *Engine) serialReader() {
	packetBuffer := make([]byte, 32)
	var packetBufferPos uint8 = 0
	for {
		buff := make([]byte, 16)
		n, err := e.port.Read(buff)
		if err != nil {
			log.Fatal(err)
		}
		if n == 0 {
			continue
		}

		for idx := range buff[:n] {
			packetBuffer[packetBufferPos] = buff[idx]
			packetBufferPos++
		}

		packetLen := getPacketBytesLen(packetBuffer[:packetBufferPos])
		if packetBufferPos >= packetLen {
			var crc byte
			for _, b := range packetBuffer[:packetLen-1] {
				crc += b
			}

			if crc == packetBuffer[packetLen-1] {
				e.handlePacket(getId(packetBuffer[:packetLen]), packetBuffer[1:packetLen-1])
				if packetBufferPos > packetLen {
					copy(packetBuffer, packetBuffer[packetLen:])
				}
				packetBufferPos -= packetLen
				continue
			}

			e.g.Update(func(g *gocui.Gui) error {
				message := fmt.Sprintf("CRC error %X %d %d", packetBuffer[:packetLen], crc, packetBuffer[packetLen-1])
				if v, err := g.View("messages"); err == nil {
					fmt.Fprintln(v, message)
				}

				return nil
			})

			copy(packetBuffer, packetBuffer[1:])
			packetBufferPos -= 1
		}
	}
}

var nextWriteAllowed time.Time

func (e *Engine) serialWriter() {
	defer e.port.Close()
	for msg := range e.out {
		if msg == nil {
			log.Println("nil msg")
			break
		}

		if d := time.Until(nextWriteAllowed); d > 0 {
			time.Sleep(d)
		}
		_, err := e.port.Write(msg.Bytes())
		if err != nil {
			log.Println(err)
		}
		nextWriteAllowed = time.Now().Add(
			20 * time.Millisecond,
		)
		e.sent <- msg
	}
}

func (e *Engine) handlePacket(id uint8, data []byte) {
	if id == 0 {
		//		log.Println("got init message response")
		return
	}
	payload := make([]byte, len(data))
	copy(payload, data)

	select {
	case ls := <-e.sent:
		if ls.ID() == id && bytes.Equal(ls.Data(), payload) {
			return
		}
	default:
	}

	select {
	case e.in <- message.New(id, payload):
	default:
		log.Printf("incomming buffer full, discarded: %d: %x", id, payload)
	}
}

func getId(data []byte) uint8 {
	return data[0] >> 4
}

func getPacketBytesLen(buffer []byte) byte {
	if len(buffer) == 0 {
		return 0
	}
	return 2 + (buffer[0] & 0x0f)
}

/*
func (e *Engine) serialReader() {
	packetBuffer := make([]byte, 32)
	packetBufferPos := 0
	for {
		buff := make([]byte, 16)

		n, err := e.port.Read(buff)
		if err != nil {
			log.Fatal(err)
		}
		if n == 0 {
			continue
		}
		for _, b := range buff[:n] {
			packetBuffer[packetBufferPos] = b
			packetBufferPos++
		}
		packetLen := getPacketBytesLen(packetBuffer[:packetBufferPos])
		if packetBufferPos >= int(packetLen) {
			var crc byte
			for _, b := range packetBuffer[:packetLen-1] {
				crc += b
			}
			if crc == packetBuffer[packetLen-1] {
				if packetLen == 2 {
					e.handlePacket(getId(packetBuffer[:packetLen]), []byte{})
				} else {
					e.handlePacket(getId(packetBuffer[:packetLen]), packetBuffer[1:packetLen-1])
				}
			} else {
				log.Printf("CRC error %X %d %d", packetBuffer[:packetLen], crc, packetBuffer[packetLen-1])
				copy(packetBuffer, packetBuffer[1:])
				packetBufferPos -= 1
				continue
			}
			if packetBufferPos > int(packetLen) {
				copy(packetBuffer, packetBuffer[packetLen:])
			}
			packetBufferPos -= int(packetLen)
		}
	}
}
*/
