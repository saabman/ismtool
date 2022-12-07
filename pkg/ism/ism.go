package ism

import (
	"log"
	"math"
	"time"

	"github.com/roffe/ismtool/pkg/kline"
	"github.com/roffe/ismtool/pkg/message"
)

type Client struct {
	K *kline.Engine // K-line client

	ismState [2]byte
	ledBrightness uint8
	keyReleased   bool

	transmittPacket10 bool
	quit chan struct{}
}

func New(portName string) (*Client, error) {
	k, err := kline.New(portName)
	if err != nil {
		return nil, err
	}

	// Init
	if err := k.Send(message.New(0, []byte{})); err != nil {
		return nil, err
	}

	c := &Client{
		K:    k,
		quit: make(chan struct{}),
		transmittPacket10: true,
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

func (c *Client) SetLedBrightness(brightness int) {
	if brightness < 0 {
		brightness = 0 
	}
	if brightness > 31 {
		brightness = 31
	}
	c.ledBrightness = uint8(brightness)
}

func (c *Client) LedBrightnessInc() error {
	if c.ledBrightness < 31 {
		c.ledBrightness++
	}
	return nil
}

func (c *Client) LedBrightnessDec() error {
	if c.ledBrightness > 0 {
		c.ledBrightness--
	}
	return nil
}

func (c *Client) ReleaseKey() error {
	c.keyReleased = true
	c.ledBrightness = 31
	return nil
}

func (c *Client) LockKey() error {
	c.keyReleased = false
	c.ledBrightness = 0
	return nil
}

func (c *Client) run() {
	t := time.NewTicker(100 * time.Millisecond)
	defer t.Stop()
	for {
		select {
		case <-t.C:
			if c.transmittPacket10 {
				if err := c.K.Send(generatePacket10()); err != nil {
					log.Println(err)
				}
				if err := c.setState(); err != nil {
					log.Println(err)
				}
			}
		case <-c.quit:
			return
		}
	}
}

func (c *Client) Start10() {
	c.transmittPacket10 = true
}

func (c *Client) Stop10() {
	c.transmittPacket10 = false
}

// var dir = 1
func (c *Client) setState() error {
	//	if dir == 1 {
	//		c.LightInc()
	//	} else {
	//		c.LightDec()
	//	}
	//	if c.ledBrightness == 31 {
	//		dir = 0
	//	}
	//	if c.ledBrightness == 0 {
	//		dir = 1
	//	}

	c.ismState[0] &= 0x03
	if c.keyReleased {
		c.ismState[0] |= 0x80
	}

	c.ledBrightness = uint8(math.Max(0, math.Min(float64(c.ledBrightness), 31)))
	c.ismState[0] |= 0x7C & (c.ledBrightness << 2)

	// ledBrightness := (c.ismState[0] << 1) >> 3

	msg := message.New(14, c.ismState[:])

	//c.g.Update(func(g *gocui.Gui) error {
	//	if v, err := g.View("messages"); err == nil {
	//		fmt.Fprintf(v, "Set state: %X\n", msg.Data())
	//	}
	//	return nil
	//})

	return c.K.Send(msg)
}
