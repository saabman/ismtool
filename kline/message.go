package kline

import (
	"bytes"
	"fmt"
)

type KLineMsg interface {
	ID() uint8
	Data() []byte
	Byte() []byte
	String() string
	CRC() byte
}

type Msg struct {
	id   uint8
	data []byte
}

func NewMsg(id uint8, data []byte) *Msg {
	if id > 15 {
		panic("id cannot be higher than 15")
	}
	if len(data) > 15 {
		panic("data length cannot exceed 15 byte")
	}
	return &Msg{
		id:   id,
		data: data,
	}
}

func (msg *Msg) ID() uint8 {
	return msg.id
}

func (msg *Msg) Data() []byte {
	return msg.data
}

// Byte returns the byte representation of the message. the first half of byte 0 is id, second half is size. last byte is simple crc
func (msg *Msg) Byte() []byte {
	var out bytes.Buffer
	var firstByte byte
	var crc byte

	firstByte = msg.id << 4
	firstByte += byte(len(msg.data))

	out.WriteByte(firstByte)
	out.Write(msg.data)

	for _, b := range out.Bytes() {
		crc += b
	}

	out.WriteByte(crc)

	return out.Bytes()
}

func (msg *Msg) String() string {
	return fmt.Sprintf("%02d:%02X %08b", msg.id, msg.data, msg.data)
}

func (msg *Msg) CRC() (crc byte) {
	for _, b := range msg.data {
		crc += b
	}
	return
}
