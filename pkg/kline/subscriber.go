package kline

import (
	"context"
	"errors"
	"sync"

	"github.com/roffe/ismtool/pkg/message"
)

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

type Subscriber struct {
	e           *Engine
	ctx         context.Context
	errcount    uint8
	identifiers []uint8
	callback    chan message.Message
	mu          sync.Mutex
}

func (s *Subscriber) Close() error {
	select {
	case s.e.unregister <- s:
		return nil
	default:
		return errors.New("failed to unregister subscriber, queue full")
	}
}

func (s *Subscriber) Chan() chan message.Message {
	return s.callback
}

func (s *Subscriber) SetFilter(identifiers ...uint8) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.identifiers = identifiers
}

func (s *Subscriber) GetFilters() []uint8 {
	s.mu.Lock()
	defer s.mu.Unlock()
	ids := make([]uint8, len(s.identifiers))
	copy(ids, s.identifiers)
	return ids
}
