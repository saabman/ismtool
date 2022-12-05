package kline

import (
	"context"
	"errors"

	"github.com/roffe/ismtool/message"
)

type Subscriber struct {
	e           *Engine
	ctx         context.Context
	errcount    uint8
	identifiers []uint8
	callback    chan message.Message
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
