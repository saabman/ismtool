package ism

import (
	"bytes"
	"context"
	"log"
	"math"

	"github.com/roffe/ismtool/pkg/message"
)

func (c *Client) handleStateChange() {
	sub, err := c.K.Subscribe(context.TODO(), 14)
	if err != nil {
		log.Fatal("failed to subscribe to state change", err)
	}
	for msg := range sub.Chan() {
		data := msg.Data()
		if !bytes.Equal(c.state[:], data) {
			copy(c.state[:], data)
			if c.OnStateChange != nil {
				go c.OnStateChange(c.state)
			}
		}
	}

}

func (c *Client) setState() error {
	c.internalState[0] &= 0x03
	if c.keyReleased {
		c.internalState[0] |= 0x80
	}
	c.ledBrightness = uint8(math.Max(0, math.Min(float64(c.ledBrightness), 31)))
	c.internalState[0] |= 0x7C & (c.ledBrightness << 2)
	msg := message.New(14, c.internalState[:])
	return c.K.Send(msg)
}

type KeyStatus int

func (k KeyStatus) String() string {
	switch k {
	case KeyNotInserted:
		return "Not Inserted"
	case KeyHalfInserted:
		return "Half Inserted"
	case KeyBlocked:
		return "Blocked"
	case KeyInserted:
		return "Inserted"
	case KeyON:
		return "ON"
	case KeySTART:
		return "START"
	case KeyUnknown:
		fallthrough
	default:
		return "Unknown"
	}
}

const (
	KeyUnknown KeyStatus = iota
	KeyNotInserted
	KeyHalfInserted
	KeyBlocked
	KeyInserted
	KeyON
	KeySTART
)
