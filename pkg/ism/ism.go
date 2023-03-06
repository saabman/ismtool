package ism

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"log"
	"time"

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

	rfStatus bool
}

func New(portName string) (*Client, error) {
	k, err := kline.New(portName)
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
	t := time.NewTicker(200 * time.Millisecond)
	defer t.Stop()
	for {
		select {
		case <-t.C:
			if c.transmitState && time.Since(lastState) > 100*time.Millisecond {
				c.transmitState = false
				if err := c.setState(); err != nil {
					c.OnError(fmt.Errorf("failed to set state: %w", err))
				}
				lastState = time.Now()
				continue
			}
			if c.transmitPacket10 {
				if err := c.K.Send(message.New(10, []byte{0x00, 0x00, 0x00, 0x00, 0x00})); err != nil {
					//if err := c.K.Send(generatePacket10()); err != nil {
					c.OnError(fmt.Errorf("failed to send packet 10: %w", err))
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

type KeyInfo struct {
	P0 []byte
	P1 []byte
	P2 []byte
	P3 []byte
	P4 []byte
	P5 []byte
	P6 []byte
	P7 []byte
}

func (c *Client) ReadKeyIDE() (*KeyInfo, error) {
	if err := c.rfON(); err != nil {
		return nil, err
	}
	result := &KeyInfo{}
	ide, err := c.readIDE()
	if err != nil {
		return nil, err
	}

	if bytes.Equal(ide, []byte{0x1f, 0x40}) {
		return nil, fmt.Errorf("failed to read key IDE")
	}

	result.P0 = ide[1:]

	if err := c.rfOFF(); err != nil {
		c.OnError(err)
	}
	return result, nil

}

func (c *Client) rfON() error {
	msg, err := c.K.SendAndRecv(2000*time.Millisecond, message.New(2, []byte{0x03, 0x1f}), 2)
	if err != nil {
		return fmt.Errorf("RFON: %w", err)
	}
	if !bytes.Equal(msg.Data(), []byte{0x03, 0x13}) {
		return fmt.Errorf("RFON: invalid response: %x", msg.Data())
	}

	c.rfStatus = true
	return nil
}

func (c *Client) rfOFF() error {
	if _, err := c.K.SendAndRecv(2000*time.Millisecond, message.New(2, []byte{0x01}), 2); err != nil {
		return fmt.Errorf("RFOFF: %w", err)
	}
	c.rfStatus = false
	return nil
}

func (c *Client) readIDE() ([]byte, error) {
	resp, err := c.K.SendAndRecv(250*time.Millisecond, message.New(2, []byte{0x04}), 2)
	if err != nil {
		return nil, fmt.Errorf("ReadP0: %w", err)
	}
	return resp.Data(), nil
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
