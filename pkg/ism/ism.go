package ism

import (
	"encoding/binary"
	"log"
	"time"

	"github.com/roffe/ismtool/pkg/gui"
	"github.com/roffe/ismtool/pkg/kline"
	"github.com/roffe/ismtool/pkg/message"
)

type Client struct {
	K *kline.Engine // K-line client

	internalState [2]byte
	state         [3]byte

	ledBrightness uint8
	keyReleased   bool

	transmitPacket10 bool
	transmitState    bool

	stateSubscriptions map[*kline.Subscriber]bool

	quit chan struct{}

	OnStateChange func(state [3]byte)
	OnError       func(err error)
	Log           func(str string)
}

func New(portName string, mw *gui.Gui) (*Client, error) {
	k, err := kline.New(portName, mw)
	if err != nil {
		return nil, err
	}

	if err := k.Send(message.New(0, []byte{})); err != nil {
		return nil, err
	}

	client := &Client{
		K:                  k,
		quit:               make(chan struct{}),
		transmitPacket10:   true,
		stateSubscriptions: make(map[*kline.Subscriber]bool),
		OnError: func(err error) {
			log.Println(err)
		},
	}
	go client.run()
	go client.handleStateChange()

	return client, nil
}

func (c *Client) run() {
	lastState := time.Now()
	t := time.NewTicker(75 * time.Millisecond)
	defer t.Stop()
	for {
		select {
		case <-t.C:
			if c.transmitState && time.Since(lastState) > 100*time.Millisecond {
				c.transmitState = false
				if err := c.setState(); err != nil {
					c.OnError(err)
				}
				lastState = time.Now()
				continue
			}
			if c.transmitPacket10 {
				if _, err := c.K.SendAndRecv(100*time.Millisecond, generatePacket10(), 2, 10, 12, 14); err != nil {
					c.OnError(err)
				}
			}
		case <-c.quit:
			return
		}
	}
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

func (c *Client) SetLedBrightness(brightness uint8) {
	if brightness == c.ledBrightness {
		return
	}
	if brightness > 31 {
		brightness = 31
	}
	c.ledBrightness = brightness
	c.transmitState = true
}

func (c *Client) LedBrightnessInc() {
	if c.ledBrightness < 31 {
		c.ledBrightness++
	}
	c.transmitState = true
}

func (c *Client) LedBrightnessDec() {
	if c.ledBrightness > 0 {
		c.ledBrightness--
	}
	c.transmitState = true
}

func (c *Client) ReleaseKey() {
	c.keyReleased = true
	c.transmitState = true
}

func (c *Client) LockKey() {
	c.keyReleased = false
	c.transmitState = true
}

func (c *Client) Start10() {
	c.transmitPacket10 = true
}

func (c *Client) Stop10() {
	c.transmitPacket10 = false
}

func (c *Client) Toggle10() bool {
	if !c.transmitPacket10 {
		codeIndex = []int{-1, -3, -3, -2, -2}
		if err := c.K.Send(message.New(0, []byte{})); err != nil {
			c.OnError(err)
		}

	}
	c.transmitPacket10 = !c.transmitPacket10
	return c.transmitPacket10

}

const (
	keyNotInsertedMask  = 0b100100010110100100101011
	keyHalfInsertedMask = 0b100100010110100001101011
	keyBlockedMask      = 0b000110011110000001101011
	keyInsertedMask     = 0b100110010110000001101011
	keyONMask           = 0b101100010100100001101011
	keySTARTMask        = 0b111100010000100001101011
)

func (c *Client) GetKeyPosition() (KeyStatus, []byte) {

	// Convert the 3-byte slice to a 4-byte slice and
	// convert it to an integer in big-endian byte order.
	state := binary.BigEndian.Uint32(append([]byte{0x00}, c.state[:]...))

	// Use a switch statement to check if the state matches
	// any of the bitmasks for the key positions.
	switch {
	case state&keyNotInsertedMask == keyNotInsertedMask:
		return KeyNotInserted, c.state[:]

	case state&keyHalfInsertedMask == keyHalfInsertedMask:
		return KeyHalfInserted, c.state[:]

	case state&keyBlockedMask == keyBlockedMask:
		return KeyBlocked, c.state[:]

	case state&keyInsertedMask == keyInsertedMask:
		return KeyInserted, c.state[:]

	case state&keyONMask == keyONMask:
		return KeyON, c.state[:]

	case state&keySTARTMask == keySTARTMask:
		return KeySTART, c.state[:]

	// If the state doesn't match any of the bitmasks,
	// return the KeyUnknown value.
	default:
		return KeyUnknown, c.state[:]
	}
}

func (c *Client) GetKeyPositionasd() KeyStatus {

	switch {
	case c.state[0]&^0b10010001 == 0 &&
		c.state[1]&^0b01101001 == 0 &&
		c.state[2]&^0b00101011 == 0:
		return KeyNotInserted

	case c.state[0]&^0b10010001 == 0 &&
		c.state[1]&^0b01101000 == 0 &&
		c.state[2]&^0b01101011 == 0:
		return KeyHalfInserted

	case c.state[0]&^0b00011001 == 0 &&
		c.state[1]&^0b11100000 == 0 &&
		c.state[2]&^0b01101011 == 0:
		return KeyBlocked

	case c.state[0]&^0b10011001 == 0 &&
		c.state[1] == 0b01100000 &&
		c.state[2] == 0b01101011:
		return KeyInserted

	case c.state[0]&^0b10110001 == 0 &&
		c.state[1]&^0b01001000 == 0 &&
		c.state[2]&^0b01101011 == 0:
		return KeyON

	case c.state[0]&^0b11110001 == 0 &&
		c.state[1]&^0b00001000 == 0 &&
		c.state[2]&^0b01101011 == 0:
		return KeySTART

	default:
		return KeyUnknown
	}
}
