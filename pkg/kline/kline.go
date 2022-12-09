package kline

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"time"

	"github.com/roffe/ismtool/pkg/gui"
	"github.com/roffe/ismtool/pkg/message"
	"github.com/smallnest/ringbuffer"
	"go.bug.st/serial"
)

type Engine struct {
	mw   *gui.Gui
	port serial.Port

	in  chan message.Message
	out chan message.Message

	register   chan *Subscriber
	unregister chan *Subscriber

	listeners map[*Subscriber]bool

	nextWriteAllowed time.Time

	rb *ringbuffer.RingBuffer

	quit chan struct{}
}

func New(portName string, mw *gui.Gui) (*Engine, error) {
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

	if err := sr.ResetInputBuffer(); err != nil {
		return nil, err
	}
	if err := sr.ResetOutputBuffer(); err != nil {
		return nil, err
	}

	if err := sr.SetReadTimeout(1 * time.Millisecond); err != nil {
		log.Fatal(err)
	}

	e := &Engine{
		mw:   mw,
		port: sr,

		in:  make(chan message.Message, 100),
		out: make(chan message.Message, 100),

		register:   make(chan *Subscriber, 10),
		unregister: make(chan *Subscriber, 10),

		listeners: map[*Subscriber]bool{},

		rb:   ringbuffer.New(1024),
		quit: make(chan struct{}),
	}

	go e.handler()
	go e.serialReader() // Start serial port reader
	go e.serialWriter() // Start serial port writer

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

func (e *Engine) SendAndRecv(timeout time.Duration, msg message.Message, identifiers ...uint8) (message.Message, error) {
	ctx, cancel := context.WithTimeout(context.TODO(), timeout)
	defer cancel()
	sub := e.Subscribe(ctx, identifiers...)
	defer sub.Close()
	if err := e.Send(msg); err != nil {
		return nil, err
	}
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case msg := <-sub.Chan():
		return msg, nil
	}
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
				ids := l.GetFilters()
				if len(ids) == 0 {
					select {
					case l.callback <- msg:
					default:
						l.errcount++
					}
					continue
				}
				for _, idf := range ids {
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
	rb := ringbuffer.New(128)
	serialBuff := make([]byte, 1)
	for {
		var packetLen int
		firstBytes := true
		for {
			n, err := e.port.Read(serialBuff)
			if err != nil {
				if err == io.EOF {
					e.mw.WriteMessage("got EOF on serial read")
					return
				}
				log.Fatal(err)
			}
			if n == 0 {
				continue
			}

			if firstBytes {
				packetLen = int(2 + (serialBuff[0] & 0x0f))
				firstBytes = false
			}

			n1, err := rb.Write(serialBuff[:n])
			if err != nil {
				e.mw.WriteMessage("failed to write to ringbuffer: " + err.Error())
			}

			if n1 != n {
				e.mw.WriteMessage("read serial bytes and written to ringbuffer does not match")
			}

			if rb.Length() >= packetLen {
				frameData := make([]byte, packetLen)
				n2, err := rb.Read(frameData)

				if err != nil {
					e.mw.WriteMessage("failed to read from ringbuffer: " + err.Error())
					break
				}

				if n2 != packetLen {
					e.mw.WriteMessage("ringbuffer length and packet length does not match")
					break
				}
				e.processMessage(frameData)
				firstBytes = true
				break
			}
		}
	}
}

func (e *Engine) processMessage(packet []byte) {
	msg, err := message.NewFromBytes(packet)
	if err != nil {
		e.mw.WriteMessage(err.Error())
		return
	}
	if msg.ID() == 0 {
		return
	}
	last := make([]byte, len(packet))
	rbr, err := e.rb.TryRead(last)
	if err != nil {
		if err != ringbuffer.ErrIsEmpty {
			e.mw.WriteMessage(err.Error())
		}
	} else {
		if rbr == len(packet) {
			if bytes.Equal(last, packet) {
				return
			}
		}
	}
	select {
	case e.in <- msg:
	default:
		e.mw.WriteMessage(fmt.Sprintf("incomming buffer full, discarded: %s", msg.String()))
	}
}

func (e *Engine) serialWriter() {
	defer e.port.Close()
	for msg := range e.out {
		if msg == nil {
			e.mw.WriteDebug("got nil msg")
			break
		}
		if d := time.Until(e.nextWriteAllowed); d > 0 {
			time.Sleep(d)
		}
		swn, err := e.port.Write(msg.Bytes())
		if err != nil {
			e.mw.WriteMessage("error writing to serial: " + err.Error())
		}

		rwn, err := e.rb.Write(msg.Bytes())
		if err != nil {
			e.mw.WriteMessage("error writing to ringbuffer: " + err.Error())
		}

		if swn != rwn {
			e.mw.WriteMessage("not same number of bytes written to serial port and ringbuffer")
		}
		e.nextWriteAllowed = time.Now().Add(
			40 * time.Millisecond,
		)
	}
}
