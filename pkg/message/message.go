package message

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/fatih/color"
)

type Message interface {
	ID() uint8
	Data() []byte
	Bytes() []byte
	String() string
	CRC() byte
}

type Msg struct {
	id   uint8
	data []byte
}

func New(id uint8, data []byte) *Msg {
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

func NewFromBytes(data []byte) (Message, error) {
	crc := calculateMessageCRC(data)
	if crc != data[len(data)-1] {
		return nil, fmt.Errorf("CRC error %X %02X %02X", data, crc, data[len(data)-1])
	}

	id := getId(data)

	messageLen := int(2 + (data[0] & 0x0f))

	if len(data) != messageLen {
		return nil, fmt.Errorf("invalid length: %d expected %d", len(data)-1, messageLen)
	}
	if id == 0 {
		return &Msg{
			id:   id,
			data: []byte{0},
		}, nil
	}
	return &Msg{
		id:   id,
		data: data[1 : len(data)-1],
	}, nil
}

func calculateMessageCRC(data []byte) byte {
	var crc byte
	for _, b := range data[:len(data)-1] {
		crc += b
	}
	return crc
}

func getId(data []byte) uint8 {
	return data[0] >> 4
}

func (msg *Msg) ID() uint8 {
	return msg.id
}

func (msg *Msg) Data() []byte {
	return msg.data
}

// Byte returns the byte representation of the message. the first half of byte 0 is id, second half is size. last byte is simple crc
func (msg *Msg) Bytes() []byte {
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

func Equal(msg1, msg2 Message) bool {
	if msg1.ID() != msg2.ID() {
		return false
	}
	return bytes.Equal(msg1.Data(), msg2.Data())
}

var (
	red   = color.New(color.FgRed).SprintFunc()
	green = color.New(color.FgGreen).SprintFunc()
)

func PrettyPrint(msg Message) string {
	var byteView strings.Builder
	for _, by := range msg.Data() {
		bs := fmt.Sprintf("%08b", by)
		for _, b := range bs {
			if b == '0' {
				byteView.WriteString(red("0"))
				continue
			}
			byteView.WriteString(green("1"))
		}
		byteView.WriteString(" ")
	}

	return fmt.Sprintf("%02d:%02X || %s", msg.ID(), msg.Data(), byteView.String())
}
