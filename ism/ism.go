package ism

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"math"
	"time"

	"github.com/jroimartin/gocui"
	"github.com/roffe/ismtool/kline"
	"github.com/roffe/ismtool/message"
)

type Client struct {
	K *kline.Engine // K-line client

	g *gocui.Gui

	ismState [2]byte

	ledBrightness uint8
	keyReleased   bool

	quit chan struct{}
}

func New(g *gocui.Gui, portName string) (*Client, error) {
	k, err := kline.New(g, portName)
	if err != nil {
		return nil, err
	}

	// Init
	if err := k.Send(message.New(0, []byte{})); err != nil {
		return nil, err
	}

	c := &Client{
		K:    k,
		g:    g,
		quit: make(chan struct{}),
	}

	go c.keepAlive()
	go c.handleISMStateChange()

	return c, nil
}

func (c *Client) Close() error {
	close(c.quit)
	return c.K.Close()
}

func (c *Client) Debug(g *gocui.Gui, identifiers ...uint8) {
	sub := c.K.Subscribe(context.TODO(), identifiers...)
	for msg := range sub.Chan() {
		if msg == nil {
			log.Println("Subscription closed")
			break
		}

		var updateFunc = func(g *gocui.Gui) error {
			v, err := g.View("debug")
			if err != nil {
				return err
			}
			fmt.Fprintf(v, "%s\n", message.PrettyPrint(msg))
			return nil
		}
		g.Update(updateFunc)

	}
}

func (c *Client) LightInc() error {
	if c.ledBrightness < 31 {
		c.ledBrightness++
	}
	return nil
}

func (c *Client) LightDec() error {
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

func (c *Client) keepAlive() {
	t := time.NewTicker(100 * time.Millisecond)
	defer t.Stop()
	for {
		select {
		case <-t.C:
			if err := c.K.Send(generatePacket10()); err != nil {
				log.Println(err)
			}
			if err := c.setState(); err != nil {
				log.Println(err)
			}
		case <-c.quit:
			return
		}
	}
}

func (c *Client) handleISMStateChange() {
	var lastData []byte
	sub := c.K.Subscribe(context.TODO(), 14)

	for msg := range sub.Chan() {
		if len(lastData) == 0 {
			lastData = msg.Data()
		}

		var message string
		if !bytes.Equal(lastData, msg.Data()) {
			//log.Printf("state change: %X %08b", msg.Data(), msg.Data())
			if bytes.Equal(msg.Data()[:2], []byte{0x99, 0x60}) {
				message = "Key inserted, Releasing key lock"
				if !c.keyReleased {
					c.ReleaseKey()
				}
			}
			if bytes.Equal(msg.Data()[:2], []byte{0x91, 0x69}) {
				message = "Key removed, Locking key position"
				if c.keyReleased {
					c.LockKey()
				}
			}
			if bytes.Equal(msg.Data()[:2], []byte{0xB1, 0x48}) {
				message = "Key in ON position"
			}

			if bytes.Equal(msg.Data()[:2], []byte{0xF1, 0x08}) {
				message = "Key in START Position"
			}

		}
		if message != "" {
			var updateFunc = func(g *gocui.Gui) error {
				v, err := g.View("messages")
				if err != nil {
					return err
				}
				fmt.Fprintf(v, "%s\n", message)
				return nil
			}
			c.g.Update(updateFunc)
		}

		lastData = msg.Data()
	}
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
