package kline

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"
	"unsafe"

	"github.com/roffe/gocan/adapter/passthru"
	"github.com/roffe/ismtool/pkg/message"
)

type Engine struct {
	h *passthru.PassThru

	channelID, deviceID, flags, protocol uint32

	incoming chan message.Message
	outgoing chan message.Message

	register   chan *Subscriber
	unregister chan *Subscriber

	listeners map[*Subscriber]bool

	//loopback *ringbuffer.RingBuffer

	OnError    func(err error)
	OnIncoming func(msg message.Message)
	OnOutgoing func(msg message.Message)

	quit chan struct{}
}

func New(portName string) (*Engine, error) {
	e := &Engine{
		incoming: make(chan message.Message, 10),
		outgoing: make(chan message.Message, 10),

		register:   make(chan *Subscriber, 10),
		unregister: make(chan *Subscriber, 10),

		listeners: map[*Subscriber]bool{},

		quit: make(chan struct{}),

		OnError: func(err error) {
			log.Println(err)
		},

		channelID: 1,
		deviceID:  1,
		protocol:  passthru.ISO9141,
	}

	pt, err := passthru.NewJ2534(`C:\Program Files (x86)\Drew Technologies, Inc\J2534\MongoosePro GM II\monpa432.dll`)
	if err != nil {
		return nil, err
	}

	if err := pt.PassThruOpen("", &e.deviceID); err != nil {
		str, err2 := pt.PassThruGetLastError()
		if err2 != nil {
			e.OnError(fmt.Errorf("PassThruOpenGetLastError: %w", err))
		} else {

			log.Println("PassThruOpen: " + str)
		}
		return nil, fmt.Errorf("PassThruOpen: %w", err)
	}

	if err := pt.PassThruConnect(e.deviceID, e.protocol, 0x00001000, 9600, &e.channelID); err != nil {
		return nil, fmt.Errorf("PassThruConnect: %w", err)
	}

	opts := &passthru.SCONFIG_LIST{
		NumOfParams: 4,
		Params: []passthru.SCONFIG{
			{
				Parameter: passthru.LOOPBACK,
				Value:     0,
			},
			{
				Parameter: passthru.PARITY,
				Value:     1,
			},
			{
				Parameter: passthru.DATA_BITS,
				Value:     0,
			},
			{
				Parameter: passthru.DATA_RATE,
				Value:     9600,
			},
		},
	}
	if err := pt.PassThruIoctl(e.channelID, passthru.SET_CONFIG, opts, nil); err != nil {
		return nil, fmt.Errorf("PassThruIoctl set options: %w", err)
	}

	e.h = pt

	e.allowAll()

	go e.handler()
	go e.reader() // Start serial port reader
	go e.writer() // Start serial port writer

	return e, nil
}

func (e *Engine) allowAll() {
	filterID := uint32(0)
	maskMsg := &passthru.PassThruMsg{
		ProtocolID: e.protocol,
		DataSize:   1,
		Data:       [4128]byte{0x00},
	}
	patternMsg := &passthru.PassThruMsg{
		ProtocolID: e.protocol,
		DataSize:   1,
		Data:       [4128]byte{0x00},
	}
	if err := e.h.PassThruStartMsgFilter(e.channelID, passthru.PASS_FILTER, maskMsg, patternMsg, nil, &filterID); err != nil {
		e.OnError(fmt.Errorf("PassThruStartMsgFilter: %w", err))
	}
}

func (e *Engine) Close() error {
	close(e.outgoing)
	close(e.quit)
	time.Sleep(200 * time.Millisecond)
	e.h.PassThruIoctl(e.channelID, passthru.CLEAR_MSG_FILTERS, nil, nil)
	e.h.PassThruDisconnect(e.channelID)
	e.h.PassThruClose(e.deviceID)
	return e.h.Close()
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

func (e *Engine) reader() {
	for {
		select {
		case <-e.quit:
			return
		default:
		}
		msg, err := e.readMsg()
		if err != nil {
			e.OnError(err)
			continue
		}
		if msg == nil {
			continue
		}
		if msg.DataSize == 0 {

			//e.OnError(fmt.Errorf("empty message received: %08X", msg.RxStatus))
			continue
		}

		m, err := message.NewFromBytes(msg.Data[:msg.DataSize])
		if err != nil {
			e.OnError(err)
			continue
		}
		e.incoming <- m
	}
}

func (e *Engine) readMsg() (*passthru.PassThruMsg, error) {
	msg := &passthru.PassThruMsg{
		ProtocolID: e.protocol,
	}
	if err := e.h.PassThruReadMsgs(e.channelID, uintptr(unsafe.Pointer(msg)), 1, 0); err != nil {
		if errors.Is(err, passthru.ErrBufferEmpty) {
			return nil, nil
		}
		if errors.Is(err, passthru.ErrDeviceNotConnected) {
			return nil, fmt.Errorf("device not connected: %w", err)
		}
		return nil, fmt.Errorf("read error: %w", err)
	}
	return msg, nil
}

func (e *Engine) writer() {
	for msg := range e.outgoing {
		if msg == nil {
			e.OnError(errors.New("got nil message, closing writer"))
			break
		}
		msg.Bytes()
		fmsg := &passthru.PassThruMsg{
			ProtocolID: e.protocol,
			DataSize:   uint32(len(msg.Bytes())),
			TxFlags:    0,
		}
		copy(fmsg.Data[:], msg.Bytes())
		if err := e.sendMsg(fmsg); err != nil {
			e.OnError(err)
		}

		if e.OnOutgoing != nil {
			go e.OnOutgoing(msg)
		}

	}
}

func (e *Engine) sendMsg(msg *passthru.PassThruMsg) error {
	if err := e.h.PassThruWriteMsgs(e.channelID, uintptr(unsafe.Pointer(msg)), 1, 0); err != nil {
		if errStr, err2 := e.h.PassThruGetLastError(); err2 == nil {
			return fmt.Errorf("%w: %s", err, errStr)
		}
		return err
	}
	return nil
}

func getPacketSize(b byte) int {
	return int(1 + (b & 0x0f))
}

/*
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
*/
