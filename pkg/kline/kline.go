package kline

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"time"

	"github.com/roffe/ismtool/pkg/message"
	"github.com/smallnest/ringbuffer"
	"go.bug.st/serial"
)

type Engine struct {
	port serial.Port

	incoming chan message.Message
	outgoing chan message.Message

	register   chan *Subscriber
	unregister chan *Subscriber

	listeners map[*Subscriber]bool

	loopback *ringbuffer.RingBuffer

	OnError    func(err error)
	OnIncoming func(msg message.Message)
	OnOutgoing func(msg message.Message)

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

		port: sr,

		incoming: make(chan message.Message, 100),
		outgoing: make(chan message.Message, 100),

		register:   make(chan *Subscriber, 10),
		unregister: make(chan *Subscriber, 10),

		listeners: map[*Subscriber]bool{},

		loopback: ringbuffer.New(1024),
		quit:     make(chan struct{}),

		OnError: func(err error) {
			log.Println(err)
		},
	}

	go e.handler()
	go e.serialReader() // Start serial port reader
	go e.serialWriter() // Start serial port writer

	return e, nil
}

func (e *Engine) Close() error {
	close(e.outgoing)
	close(e.quit)
	return e.port.Close()
}

func (e *Engine) Send(msg message.Message) error {
	t := time.NewTimer(1 * time.Second)
	defer t.Stop()
	select {
	case e.outgoing <- msg:
	case <-t.C:
		return fmt.Errorf("send buffer full")
	}
	return nil
}

func (e *Engine) SendAndRecv(timeout time.Duration, msg message.Message, identifiers ...uint8) (message.Message, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	sub, err := e.Subscribe(ctx, identifiers...)
	if err != nil {
		return nil, err
	}
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
		case msg := <-e.incoming:
			if e.OnIncoming != nil {
				go e.OnIncoming(msg)
			}
			e.fanout(msg)
		}
	}
}

func (e *Engine) fanout(msg message.Message) {
	for l := range e.listeners {
		select {
		case <-l.ctx.Done():
			e.unregister <- l
			continue
		default:
		}
		ids := l.GetIDFilter()
		if len(ids) == 0 {
			select {
			case l.callback <- msg:
			default:
				l.errcount++
			}
			continue
		}
		for _, id := range ids {
			if l.errcount > 10 {
				select {
				case e.unregister <- l:
				default:
					panic("could not queue unregister")
				}
				break
			}
			if id == msg.ID() {
				select {
				case l.callback <- msg:
				default:
					l.errcount++
				}
			}
		}
	}
}

//var sendMutex = make(chan struct{}, 1)

func (e *Engine) serialReader() {
	packetBuffer := make([]byte, 128)
	packetBufferSize := 0
	serialBuffer := make([]byte, 16)
	for {
		var packetLen int = -1
		for {
			n, err := e.port.Read(serialBuffer)
			if err != nil {
				if err == io.EOF {
					e.OnError(errors.New("got EOF on serial read"))
					return
				}
				log.Fatal(err)
			}
			if n == 0 {
				continue
			}

			if packetLen == -1 {
				if packetBufferSize == 0 {
					packetLen = getPacketSize(serialBuffer[0])
				} else if packetBufferSize > 0 {
					packetLen = getPacketSize(packetBuffer[0])
				}
			}

			for _, b := range serialBuffer[:n] {
				packetBuffer[packetBufferSize] = b
				packetBufferSize++
			}

			if packetBufferSize >= packetLen {
				e.processMessage(packetBuffer[:packetLen])
				if packetBufferSize-packetLen > 0 {
					copy(packetBuffer, packetBuffer[packetLen:packetBufferSize])
				}
				packetBufferSize -= packetLen
				packetLen = -1
				break
			}
		}
	}
}

func (e *Engine) serialWriter() {
	nextWriteAllowed := time.Now()
	defer e.port.Close()
	for msg := range e.outgoing {
		if msg == nil {
			e.OnError(errors.New("got nil message, closing serial writer"))
			break
		}
		if d := time.Until(nextWriteAllowed); d > 0 {
			time.Sleep(d)
		}
		swn, err := e.port.Write(msg.Bytes())
		if err != nil {
			e.OnError(fmt.Errorf("error writing to serial: %w", err))
		}

		rwn, err := e.loopback.Write(msg.Bytes())
		if err != nil {
			e.OnError(fmt.Errorf("error writing to ringbuffer: %w", err))
		}

		if swn != rwn {
			e.OnError(errors.New("not same number of bytes written to serial port and ringbuffer"))
		}
		nextWriteAllowed = time.Now().Add(30 * time.Millisecond)
		//if msg.ID() != 0 {
		//	sendMutex <- struct{}{}
		//}

		if e.OnOutgoing != nil {
			go e.OnOutgoing(msg)
		}

	}
}

func getPacketSize(b byte) int {
	return int(2 + (b & 0x0f))
}

func (e *Engine) processMessage(data []byte) {
	packet := make([]byte, len(data))
	copy(packet, data)

	msg, err := message.NewFromBytes(packet)
	if err != nil {
		e.OnError(err)
		return
	}
	last := make([]byte, len(packet))
	if _, err := e.loopback.TryRead(last); err != nil {
		if err != ringbuffer.ErrIsEmpty {
			e.OnError(err)
		}
	}

	if bytes.Equal(last, packet) {
		return
	}

	//select {
	//case <-sendMutex:
	//default:
	//}

	select {
	case e.incoming <- msg:
	default:
		e.OnError(fmt.Errorf("incomming buffer full, discarded: %s", msg.String()))
	}
}
