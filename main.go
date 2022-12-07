package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"log"

	"github.com/jroimartin/gocui"
	"github.com/roffe/ismtool/pkg/gui"
	"github.com/roffe/ismtool/pkg/ism"
	"github.com/roffe/ismtool/pkg/message"
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
	g, err := gocui.NewGui(gocui.Output256)
	if err != nil {
		log.Fatal(err)
	}

	mw, err := gui.New(g)
	if err != nil {
		log.Fatal(err)
	}
	defer mw.Close()


	ismClient, err := ism.New(portName)
	if err != nil {
		log.Fatal(err)
	}

	go debugPrinter(ismClient, mw) // enter message id(s) after g if you want to filter
	go monitorISMStateChange(ismClient, mw)


	if err := g.SetKeybinding("", gocui.KeyCtrlC, gocui.ModNone, quit); err != nil {
		log.Fatal(err)
	}

	if err := g.SetKeybinding("", gocui.KeyArrowUp, gocui.ModNone, func(g *gocui.Gui, v *gocui.View) error {
		mw.WriteMessage("Brightness up")
		ismClient.LedBrightnessInc()
		return nil
	}); err != nil {
		log.Fatal(err)
	}

	if err := g.SetKeybinding("", gocui.KeyArrowDown, gocui.ModNone, func(g *gocui.Gui, v *gocui.View) error {
		mw.WriteMessage("Brightness down")
		ismClient.LedBrightnessDec()
		return nil
	}); err != nil {
		log.Fatal(err)
	}

	if err := g.SetKeybinding("", gocui.KeyHome, gocui.ModNone, func(g *gocui.Gui, v *gocui.View) error {
		mw.WriteMessage("Release key")
		ismClient.ReleaseKey()
		return nil
	}); err != nil {
		log.Fatal(err)
	}

	if err := g.SetKeybinding("", gocui.KeyEnd, gocui.ModNone, func(g *gocui.Gui, v *gocui.View) error {
		mw.WriteMessage("Lock key")
		ismClient.LockKey()
		return nil
	}); err != nil {
		log.Fatal(err)
	}

	sending := true
	if err := g.SetKeybinding("", gocui.KeyTab, gocui.ModNone, func(g *gocui.Gui, v *gocui.View) error {
		if sending {
			mw.WriteMessage("Stop 10")
			ismClient.Stop10()
			sending = false
			return nil
		}
		
		mw.WriteMessage("Start 10")
		ismClient.Start10()
		sending = true
		return nil
	}); err != nil {
		log.Fatal(err)
	}

	if err := mw.Run(); err != nil && err != gocui.ErrQuit {
		log.Fatal(err)
	}
}

func quit(g *gocui.Gui, v *gocui.View) error {
	return gocui.ErrQuit
}

func debugPrinter(c *ism.Client, gui *gui.Gui, identifiers ...uint8) {
	sub := c.K.Subscribe(context.TODO(), identifiers...)
	for msg := range sub.Chan() {
		if msg == nil {
			log.Println("Subscription closed")
			break
		}
		gui.WriteDebug(message.PrettyPrint(msg))
	}
}


func monitorISMStateChange(ismClient *ism.Client,gui *gui.Gui)  {
	var lastData []byte
	sub := ismClient.K.Subscribe(context.TODO(), 14)
	for msg := range sub.Chan() {
		if len(lastData) == 0 {
			lastData = msg.Data()
		}
		if !bytes.Equal(lastData, msg.Data()) {
			gui.WriteState(fmt.Sprintf("%X %08b", msg.Data(), msg.Data()))
		
			if bytes.Equal(msg.Data()[:2], []byte{0x99, 0x60}) {
				gui.WriteMessage("Key inserted, Releasing key lock")
				if !ismClient.KeyReleased() {
					ismClient.ReleaseKey()
				}
			}
			if bytes.Equal(msg.Data()[:2], []byte{0x91, 0x69}) {
				gui.WriteMessage("Key removed, Locking key position")
				if ismClient.KeyReleased() {
					ismClient.LockKey()
				}
			}
			if bytes.Equal(msg.Data()[:2], []byte{0xB1, 0x48}) {
				gui.WriteMessage("Key in ON position")
			}

			if bytes.Equal(msg.Data()[:2], []byte{0xF1, 0x08}) {
				gui.WriteMessage("Key in START Position")
			}
		}

		lastData = msg.Data()
	}
}