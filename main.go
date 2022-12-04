package main

import (
	"bufio"
	"bytes"
	"flag"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"go.bug.st/serial"
)

var portName string

func init() {
	log.SetFlags(0)
	for idx, def := range codeDefinition {
		for _, b := range def.data {
			for i := 0; i < def.repetitions; i++ {
				codes[idx] = append(codes[idx], b)
			}
		}
	}

	flag.StringVar(&portName, "port", "COM6", "Port name")
	flag.Parse()
}

type codeDef struct {
	repetitions int
	data        []byte
}

var codeDefinition = []codeDef{
	{1, []byte{0x30, 0x60, 0x03, 0x0c, 0x0a}},
	{5, []byte{0x0a, 0x13, 0x23, 0x45, 0x6d, 0x7c, 0xb4, 0xef}},
	{5, []byte{0xf5, 0xec, 0xdc, 0xba, 0x92, 0x83, 0x4b, 0x10}},
	{5, []byte{0xb1, 0x69, 0xe8, 0xd9, 0x98, 0x20, 0x60, 0x88}},
	{5, []byte{0x4e, 0x96, 0x17, 0x26, 0x67, 0xdf, 0x9f, 0x77}},
}

var codeIndexResetValues = []int{
	-1,
	-3,
	-3,
	-2,
	-2,
}

var codeIndex = []int{
	0,
	0,
	0,
	0,
	0,
}

var codes = make([][]byte, 5)

func main() {
	quit := make(chan os.Signal, 2)
	signal.Notify(quit, syscall.SIGTERM, syscall.SIGINT)

	mode := &serial.Mode{
		BaudRate: 9600,
		DataBits: 8,
		Parity:   serial.OddParity,
		StopBits: serial.OneStopBit,
	}

	sr, err := serial.Open(portName, mode)
	if err != nil {
		log.Fatal(err)
	}
	defer sr.Close()

	sr.SetReadTimeout(40 * time.Millisecond)

	//var packetBuffer []byte
	packetBuffer := make([]byte, 32)
	packetBufferPos := 0
	go func() {
		for {
			buff := make([]byte, 16)

			n, err := sr.Read(buff)
			if err != nil {
				log.Fatal(err)
			}
			if n == 0 {
				continue
			}
			for _, b := range buff[:n] {
				packetBuffer[packetBufferPos] = b
				packetBufferPos++
			}
			packetLen := getPacketBytesLen(packetBuffer[:packetBufferPos])
			if packetBufferPos >= int(packetLen) {
				if packetLen == 2 {
					parsePackage(getId(packetBuffer[:packetLen]), []byte{})
				} else {
					parsePackage(getId(packetBuffer[:packetLen]), packetBuffer[1:packetLen-1])
				}
				if packetBufferPos > int(packetLen) {
					copy(packetBuffer, packetBuffer[packetLen:])
				}
				packetBufferPos -= int(packetLen)
			}
		}
	}()

	writeSerial(sr, createPacket(0, []byte{}))

	time.Sleep(20 * time.Millisecond)

	go func() {
		copy(codeIndex, codeIndexResetValues)
		for {
			//writeSerial(sr, createPacket(14, []byte{0x80, 0x0C}))
			writeSerial(sr, sendPacket10())
			time.Sleep(100 * time.Millisecond)
		}
	}()

	reader := bufio.NewReader(os.Stdin)
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		cmd := scanner.Text()
		switch cmd {
		case "unlock":
			writeSerial(sr, createPacket(14, []byte{0x80, 0x0C}))
		case "lock":
			writeSerial(sr, createPacket(14, []byte{0x00, 0x0C}))
		}
	}

	<-quit

}

func sendPacket10() []byte {
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
	return createPacket(10, data)
}

func parsePackage(id uint8, data []byte) {
	switch id {
	case 0:
		log.Println("init")
	case 10:
		// log.Printf("<< 10 %X %08b", data, data)
	case 12:
		// log.Printf("<< 12 %X %08b", data, data)
	case 14:
		//handlePacket14(data)
	default:
		log.Printf("<< %d: %X %08b", id, data, data)
	}

}

func handlePacket14(data []byte) {
	switch len(data) {
	case 2:
		// local value, noop
	case 3:
		fallthrough
	default:
		log.Printf("<< 14 %X %08b", data, data)
	}
}

func getId(data []byte) uint8 {
	return data[0] >> 4
}

func getPacketBytesLen(buffer []byte) byte {
	if len(buffer) == 0 {
		return 0
	}
	return 2 + (buffer[0] & 0x0f)
}

var writeLock sync.Mutex

func writeSerial(sr serial.Port, data []byte) {
	writeLock.Lock()
	defer writeLock.Unlock()
	noBytesWritten, err := sr.Write(data)
	if err != nil {
		log.Println(err)
	}
	if noBytesWritten != len(data) {
		log.Println("/!\\ did not write all bytes")
	}
	//	log.Printf(">> %d: %X", noBytesWritten, data)
}

func createPacket(id uint8, data []byte) []byte {
	var out bytes.Buffer

	var firstByte byte
	firstByte = id << 4
	firstByte += byte(len(data))

	out.WriteByte(firstByte)
	out.Write(data)

	var crc byte
	for _, b := range out.Bytes() {
		crc += b
	}

	out.WriteByte(crc)

	return out.Bytes()
}
