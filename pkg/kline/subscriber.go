package kline

import (
	"context"
	"errors"
	"sync/atomic"
	"time"

	"github.com/roffe/ismtool/pkg/message"
)

var (
	ErrFailedToUnregister = errors.New("failed to unregister subscriber")
	ErrFailedToSubscribe  = errors.New("failed to subscribe")
)

func (e *Engine) Subscribe(ctx context.Context, identifiers ...uint8) (*Subscriber, error) {
	cb := make(chan message.Message, 10)
	sub := &Subscriber{
		e:        e,
		ctx:      ctx,
		callback: cb,
	}
	sub.identifiers.Store(identifiers)

	select {
	case e.register <- sub:
	case <-time.After(1 * time.Second):
		return nil, ErrFailedToSubscribe
	}
	return sub, nil
}

type Subscriber struct {
	e           *Engine
	ctx         context.Context
	errcount    uint8
	identifiers atomic.Value
	callback    chan message.Message
}

func (s *Subscriber) Close() error {
	select {
	case s.e.unregister <- s:
		return nil
	default:
		return ErrFailedToUnregister
	}
}

func (s *Subscriber) Chan() chan message.Message {
	return s.callback
}

func (s *Subscriber) SetIDFilter(identifiers ...uint8) {
	s.identifiers.Store(identifiers)
}

func (s *Subscriber) GetIDFilter() []uint8 {
	ids, _ := s.identifiers.Load().([]uint8)
	return ids
}
