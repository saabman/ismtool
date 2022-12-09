package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/fatih/color"
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

var defaultTimeout = 200 * time.Millisecond

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

	ismClient, err := ism.New(portName, mw)
	if err != nil {
		log.Fatal(err)
	}
	defer ismClient.Close()

	go debugPrinter(ismClient, mw, 1, 2, 3, 4, 5, 6, 7, 8, 9, 11, 12, 13, 15) // enter message id(s) after g if you want to filter
	go monitorISMStateChange(ismClient, mw)

	if err := g.SetKeybinding("", gocui.KeyCtrlC, gocui.ModNone, quit); err != nil {
		log.Fatal(err)
	}

	if err := g.SetKeybinding("", gocui.KeyArrowUp, gocui.ModNone, func(g *gocui.Gui, v *gocui.View) error {
		pre := ismClient.GetLedBrightness()
		ismClient.LedBrightnessInc()
		if ismClient.GetLedBrightness() != pre {
			mw.WriteMessage(fmt.Sprintf("Brightness %d", ismClient.GetLedBrightness()))
		}
		return nil
	}); err != nil {
		log.Fatal(err)
	}

	if err := g.SetKeybinding("", gocui.KeyArrowDown, gocui.ModNone, func(g *gocui.Gui, v *gocui.View) error {
		pre := ismClient.GetLedBrightness()
		ismClient.LedBrightnessDec()
		if ismClient.GetLedBrightness() != pre {
			mw.WriteMessage(fmt.Sprintf("Brightness %d", ismClient.GetLedBrightness()))
		}
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

	if err := g.SetKeybinding("", gocui.KeyTab, gocui.ModNone, func(g *gocui.Gui, v *gocui.View) error {
		s := ismClient.Toggle10()
		if s {
			mw.WriteMessage("Start10")
		} else {
			mw.WriteMessage("Stop10")
		}
		return nil
	}); err != nil {
		log.Fatal(err)
	}

	k := ismClient.K
	mw.SetCommandMap(
		map[string]func(){
			"quit": mw.Close,
			"q":    mw.Close,
			"lock": func() {
				mw.WriteMessage("Lock key")
				if err := ismClient.LockKey(); err != nil {
					mw.WriteMessage(err.Error())
					return
				}
			},
			"release": func() {
				mw.WriteMessage("Release key")
				if err := ismClient.ReleaseKey(); err != nil {
					mw.WriteMessage(err.Error())
					return
				}
			},
			"open": func() { // open (o)
				msg, err := k.SendAndRecv(defaultTimeout, message.New(2, []byte{0x03, 0x1f}), 2)
				if err != nil {
					mw.WriteMessage(err.Error())
					return
				}
				mw.WriteMessage("open: " + msg.String())
			},
			"rid": func() { //request IDE (i050C)
				msg, err := k.SendAndRecv(defaultTimeout, message.New(2, []byte{0x04}), 2)
				if err != nil {
					mw.WriteMessage(err.Error())
					return
				}
				mw.WriteMessage("request IDE (i050C): " + msg.String())
			},
			"rs": func() { // read status
				msg, err := k.SendAndRecv(defaultTimeout, message.New(2, []byte{0x02, 0x06}), 2)
				if err != nil {
					mw.WriteMessage(err.Error())
					return
				}
				mw.WriteMessage("read status: " + msg.String())
			},
			"off": func() { // off (f)
				msg, err := k.SendAndRecv(defaultTimeout, message.New(2, []byte{0x01}), 2)
				if err != nil {
					mw.WriteMessage(err.Error())
					return
				}
				mw.WriteMessage("off: " + msg.String())
			},
			"read": func() {
				msg, err := k.SendAndRecv(defaultTimeout, message.New(2, []byte{0x03, 0x1f}), 2)
				if err != nil {
					mw.WriteMessage(err.Error())
					return
				}
				mw.WriteMessage("open: " + msg.String())

				msg, err = k.SendAndRecv(defaultTimeout, message.New(2, []byte{0x04}), 2)
				if err != nil {
					mw.WriteMessage(err.Error())
					return
				}
				mw.WriteMessage("request IDE (i050C): " + msg.String())

				msg, err = k.SendAndRecv(defaultTimeout, message.New(2, []byte{0x02, 0x06}), 2)
				if err != nil {
					mw.WriteMessage(err.Error())
					return
				}
				mw.WriteMessage("read status: " + msg.String())

				msg, err = k.SendAndRecv(defaultTimeout, message.New(2, []byte{0x01}), 2)
				if err != nil {
					mw.WriteMessage(err.Error())
					return
				}
				mw.WriteMessage("off: " + msg.String())
			},
		},
	)

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
			gui.WriteDebug("Subscription closed")
			break
		}
		gui.WriteDebug(" " + message.PrettyPrint(msg))
	}
}

var (
	red   = color.New(color.FgRed).SprintFunc()
	green = color.New(color.FgGreen).SprintFunc()
)

func monitorISMStateChange(ismClient *ism.Client, gui *gui.Gui) {
	var lastData []byte
	sub := ismClient.K.Subscribe(context.TODO(), 14)
	for msg := range sub.Chan() {
		if !bytes.Equal(lastData, msg.Data()) {
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

			gui.WriteState(fmt.Sprintf(" %X %s", msg.Data(), byteView.String()))

			var keyStatus string
			if bytes.Equal(msg.Data()[:2], []byte{0x91, 0x69}) {
				keyStatus = "No key inserted"
			}

			if bytes.Equal(msg.Data()[:2], []byte{0x91, 0x68}) {
				keyStatus = "Key half inserted"
			}

			if bytes.Equal(msg.Data()[:2], []byte{0x19, 0xE0}) {
				keyStatus = "Key blocked"
			}

			if bytes.Equal(msg.Data()[:2], []byte{0x99, 0x60}) {
				keyStatus = "Key inserted"
			}

			if bytes.Equal(msg.Data()[:2], []byte{0xB1, 0x48}) {
				keyStatus = "Key in ON position"
			}

			if bytes.Equal(msg.Data()[:2], []byte{0xF1, 0x08}) {
				keyStatus = "Key in START Position"
			}

			if keyStatus != "" {
				gui.WriteMessage(keyStatus)
				gui.SetKeyPosition(keyStatus)
			}

		}

		lastData = msg.Data()
	}
}
