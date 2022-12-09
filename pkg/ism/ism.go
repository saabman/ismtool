package ism

import (
	"math"
	"time"

	"github.com/roffe/ismtool/pkg/gui"
	"github.com/roffe/ismtool/pkg/kline"
	"github.com/roffe/ismtool/pkg/message"
)

type Client struct {
	K  *kline.Engine // K-line client
	mw *gui.Gui

	ismState      [2]byte
	ledBrightness uint8
	keyReleased   bool

	transmitPacket10 bool
	transmitState    bool
	quit             chan struct{}
}

func New(portName string, mw *gui.Gui) (*Client, error) {
	k, err := kline.New(portName, mw)
	if err != nil {
		return nil, err
	}

	if err := k.Send(message.New(0, []byte{})); err != nil {
		return nil, err
	}

	c := &Client{
		K:                k,
		mw:               mw,
		quit:             make(chan struct{}),
		transmitPacket10: false,
	}

	go c.run()

	return c, nil
}

func (c *Client) Close() error {
	close(c.quit)
	return c.K.Close()
}

func (c *Client) KeyReleased() bool {
	return c.keyReleased
}

func (c *Client) GetLedBrightness() uint8 {
	return c.ledBrightness
}

func (c *Client) SetLedBrightness(brightness int) {
	if brightness < 0 {
		brightness = 0
	}
	if brightness > 31 {
		brightness = 31
	}
	if uint8(c.ledBrightness) == c.ledBrightness {
		return
	}
	c.ledBrightness = uint8(brightness)
	c.transmitState = true
}

func (c *Client) LedBrightnessInc() error {
	if c.ledBrightness < 31 {
		c.ledBrightness++
	} else {
		return nil
	}
	c.transmitState = true
	return nil
}

func (c *Client) LedBrightnessDec() error {
	if c.ledBrightness > 0 {
		c.ledBrightness--
	} else {
		return nil
	}
	c.transmitState = true
	return nil
}

func (c *Client) ReleaseKey() error {
	c.keyReleased = true
	c.ledBrightness = 31
	c.transmitState = true
	return nil
}

func (c *Client) LockKey() error {
	c.keyReleased = false
	c.ledBrightness = 0
	c.transmitState = true
	return nil
}

func (c *Client) Start10() {
	c.transmitPacket10 = true
}

func (c *Client) Stop10() {
	c.transmitPacket10 = false
}

func (c *Client) Toggle10() bool {
	c.transmitPacket10 = !c.transmitPacket10
	return c.transmitPacket10

}

func (c *Client) run() {
	t := time.NewTicker(80 * time.Millisecond)
	defer t.Stop()
	for {
		select {
		case <-t.C:
			if c.transmitState {
				c.transmitState = false
				if err := c.setState(); err != nil {
					c.mw.WriteMessage(err.Error())
				}
				continue
			}
			if c.transmitPacket10 {
				if err := c.K.Send(generatePacket10()); err != nil {
					c.mw.WriteMessage(err.Error())
				}
			}
		case <-c.quit:
			return
		}
	}
}

func (c *Client) setState() error {
	c.ismState[0] &= 0x03
	if c.keyReleased {
		c.ismState[0] |= 0x80
	}
	c.ledBrightness = uint8(math.Max(0, math.Min(float64(c.ledBrightness), 31)))
	c.ismState[0] |= 0x7C & (c.ledBrightness << 2)
	msg := message.New(14, c.ismState[:])
	return c.K.Send(msg)
}
