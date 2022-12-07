package ism

import (
	"github.com/roffe/ismtool/pkg/message"
)

type codeDef struct {
	repetitions int
	data        []byte
}

var (
	codeIndex = []int{-1, -3, -3, -2, -2}
	codes     = make([][]byte, 5)
)

func generatePacket10() message.Message {
	data := make([]byte, 5)
	for idx := 0; idx < 5; idx++ {
		if codeIndex[idx] < 0 {
			data[idx] = 0
			codeIndex[idx]++
		} else {
			data[idx] = codes[idx][codeIndex[idx]]
			codeIndex[idx]++
			if codeIndex[idx] == len(codes[idx]) {
				codeIndex[idx] = 0
			}
		}
	}
	return message.New(10, data)
}

func init() {
	codeDefinition := []codeDef{
		{1, []byte{0x30, 0x60, 0x03, 0x0c, 0x0a}},
		{5, []byte{0x0a, 0x13, 0x23, 0x45, 0x6d, 0x7c, 0xb4, 0xef}},
		{5, []byte{0xf5, 0xec, 0xdc, 0xba, 0x92, 0x83, 0x4b, 0x10}},
		{5, []byte{0xb1, 0x69, 0xe8, 0xd9, 0x98, 0x20, 0x60, 0x88}},
		{5, []byte{0x4e, 0x96, 0x17, 0x26, 0x67, 0xdf, 0x9f, 0x77}},
	}
	for idx, def := range codeDefinition {
		for _, b := range def.data {
			for i := 0; i < def.repetitions; i++ {
				codes[idx] = append(codes[idx], b)
			}
		}
	}
}
