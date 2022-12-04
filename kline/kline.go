package kline

import (
	"bytes"
	"fmt"
	"log"
	"time"

	"go.bug.st/serial"
)

type Engine struct {
	port serial.Port

	in  chan KLineMsg
	out chan KLineMsg

	sent chan KLineMsg

	register   chan *Listener
	unregister chan *Listener

	listeners map[*Listener]bool

	quit chan struct{}
}

func New(portName string) (*Engine, error) {
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
		port: sr,

		in:   make(chan KLineMsg, 100),
		out:  make(chan KLineMsg, 100),
		sent: make(chan KLineMsg, 100),

		register:   make(chan *Listener, 10),
		unregister: make(chan *Listener, 10),

		listeners: map[*Listener]bool{},

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

func (e *Engine) Send(msg KLineMsg) error {
	select {
	case e.out <- msg:
	default:
		return fmt.Errorf("send buffer full")
	}
	return nil
}

func (e *Engine) Subscribe(identifiers ...uint8) <-chan KLineMsg {
	cb := make(chan KLineMsg, 10)
	e.register <- &Listener{
		callback:    cb,
		identifiers: identifiers,
	}
	return cb
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

var lastData time.Time

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

		lastData = time.Now()

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
			} else {
				log.Printf("CRC error %X %d %d", packetBuffer[:packetLen], crc, packetBuffer[packetLen-1])
				copy(packetBuffer, packetBuffer[1:])
				packetBufferPos -= 1
				continue
			}
			if packetBufferPos > packetLen {
				copy(packetBuffer, packetBuffer[packetLen:])
			}
			packetBufferPos -= packetLen
		}
	}
}

func (e *Engine) serialWriter() {
	defer e.port.Close()
	for msg := range e.out {
		if msg == nil {
			log.Println("nil msg")
			break
		}
		//for time.Since(lastData) < 30*time.Millisecond {
		//	time.Sleep(1 * time.Millisecond)
		//}
		_, err := e.port.Write(msg.Byte())
		if err != nil {
			log.Println(err)
		}
		e.sent <- msg
	}
}

func (e *Engine) handlePacket(id uint8, data []byte) {
	if id == 0 {
		log.Println("init")
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
	case e.in <- NewMsg(id, payload):
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
