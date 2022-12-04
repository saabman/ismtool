package main

import (
	"bytes"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/roffe/ismtool/ism"
)

var (
	portName string
)

func init() {
	log.SetFlags(log.Lshortfile | log.LstdFlags)
	//log.SetFlags(0)
	flag.StringVar(&portName, "port", "COM6", "Port name")
	flag.Parse()
}

func main() {
	quit := make(chan os.Signal, 2)
	signal.Notify(quit, syscall.SIGTERM, syscall.SIGINT)

	log.Println("Starting on", portName)

	c, err := ism.New(portName)
	if err != nil {
		log.Fatal(err)
	}
	defer c.Close()

	//go c.Debug(10)

	go handleISMStateChange(c)

	<-quit
	log.Println("exit")

}

func handleISMStateChange(c *ism.Client) {
	var lastData []byte
	keyInserted := false
	for msg := range c.K.Subscribe(14) {
		if len(lastData) == 0 {
			lastData = msg.Data()
		}
		if !bytes.Equal(lastData, msg.Data()) {
			//log.Printf("state change: %X %08b", msg.Data(), msg.Data())
			if bytes.Equal(msg.Data(), []byte{0x99, 0x60, 0x6B}) {
				log.Println("Key inserted")
				if !keyInserted {
					c.ReleaseKey()
					keyInserted = true
				}
			}
			if bytes.Equal(msg.Data(), []byte{0x91, 0x69, 0x2B}) {
				log.Println("Key removed")
				if keyInserted {
					c.LockKey()
					keyInserted = false
				}
			}
			if bytes.Equal(msg.Data(), []byte{0xB1, 0x48, 0x6B}) {
				log.Println("Key in ON position")
			}

			if bytes.Equal(msg.Data(), []byte{0xF1, 0x08, 0x6B}) {
				log.Println("Key in START Position")
			}

		}
		lastData = msg.Data()
	}
}
