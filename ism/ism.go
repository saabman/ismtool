package ism

import (
	"log"
	"time"

	"github.com/roffe/ismtool/kline"
)

type Client struct {
	K *kline.Engine // K-line client

	quit chan struct{}
}

func New(portName string) (*Client, error) {
	k, err := kline.New(portName)
	if err != nil {
		return nil, err
	}

	// Init
	if err := k.Send(kline.NewMsg(0, []byte{})); err != nil {
		return nil, err
	}

	c := &Client{
		K:    k,
		quit: make(chan struct{}),
	}

	go c.keepAlive()

	return c, nil
}

func (c *Client) Close() error {
	close(c.quit)
	return c.K.Close()
}

func (c *Client) Debug(identifiers ...uint8) {
	for msg := range c.K.Subscribe(identifiers...) {
		log.Println("DEBUG:", msg.String())
	}
}

func (c *Client) ReleaseKey() error {
	log.Println("Releasing key position")
	return c.K.Send(kline.NewMsg(14, []byte{0x80, 0x8C}))
}

func (c *Client) LockKey() error {
	log.Println("Locking key position")
	return c.K.Send(kline.NewMsg(14, []byte{0x00, 0x8C}))
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
		case <-c.quit:
			return
		}
	}
}
