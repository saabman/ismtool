package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/jroimartin/gocui"
	"github.com/roffe/ismtool/pkg/gui"
	"github.com/roffe/ismtool/pkg/ism"
	"github.com/roffe/ismtool/pkg/message"
)

var (
	defaultTimeout = 200 * time.Millisecond

	portName string

	red   = color.New(color.FgRed).SprintFunc()
	green = color.New(color.FgGreen).SprintFunc()
	blue  = color.New(color.FgCyan).SprintfFunc()
)

func init() {
	log.SetFlags(log.Lshortfile | log.LstdFlags)
	flag.StringVar(&portName, "port", "COM6", "Port name")
	flag.Parse()
}

func shiftAndXor(b []byte) byte {
	// Shift right by three and AND with 0x0f
	b0 := (b[0] >> 3) & 0x0f
	b1 := (b[1] >> 3) & 0x0f

	// Perform a NOT on one of the results
	b1 = ^b1

	// XOR the two results and return the value
	return b0 ^ b1
}

func main() {
	start := time.Now()

	g, err := gocui.NewGui(gocui.Output256)
	if err != nil {
		log.Fatal(err)
	}

	ui, err := gui.New(g)
	if err != nil {
		log.Fatal(err)
	}
	defer ui.Close()

	client, err := ism.New(portName, ui)
	if err != nil {
		log.Fatal(err)
	}
	defer client.Close()

	client.OnError = func(err error) {
		ui.WriteMessagef("Error: %v", err)
	}

	client.OnStateChange = func(state [3]byte) {
		var byteView strings.Builder
		for _, by := range state {
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
		ui.WriteStatef("%08d %s %s", time.Since(start).Milliseconds(), blue("%X", state), byteView.String())

		c, data := client.GetKeyPosition()
		ui.SetKeyPosition(" " + c.String())

		first := data[0] & 0x78 // 0b01111000
		second := shiftAndXor(data[:])
		third := data[0] & 0x80  // 0b10000000
		fourth := data[0] & 0x02 // 0b00000010
		fifth := data[0] & 0x01  // 0b00000001
		// It might be that (byte[1] & 0x06) is a packed number. ((byte[1] >> 1) & 0x03)
		sixth := data[0] & 0x04        // 0b00000100
		seventh := data[2] & 0x40      // 0b01000000
		eight := data[2] & 0x80        // 0b10000000
		ninth := (data[2] >> 3) & 0x07 // 0b00000111
		another := ((data[1] >> 3) & 0x0f) | (((data[2] >> 6) & 0x01) << 4)

		ui.WriteMessagef("Key %s first: %08b, second: %08b, third: %08b, fourth: %08b, fifth: %08b, sixth: %08b sevent: %08b,\neight: %08b, ninth: %08b, another: %08b", c.String(), first, second, third, fourth, fifth, sixth, seventh, eight, ninth, another)
	}

	f, err := os.Create("communication.log")
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()

	client.K.OnIncoming = func(msg message.Message) {
		f.Write([]byte(fmt.Sprintf(" IN: %-12s %d %d %X\n", time.Now().Format("15:04:05.999"), msg.ID(), len(msg.Data()), msg.Data())))
		switch msg.ID() {
		case 0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 15:
			ui.WriteDebugf("%08d %s", time.Since(start).Milliseconds(), message.PrettyPrint(msg))
		}
	}

	client.K.OnOutgoing = func(msg message.Message) {
		f.Write([]byte(fmt.Sprintf("OUT: %-12s %d %d %X\n", time.Now().Format("15:04:05.999"), msg.ID(), len(msg.Data()), msg.Data())))
		//ui.WriteMessagef("%08d %s", time.Since(start).Milliseconds(), message.PrettyPrint(msg))
	}

	client.K.OnError = func(err error) {
		ui.WriteMessage("K> " + err.Error())
	}

	client.Log = func(str string) {
		ui.WriteMessage(str)
	}

	if err := g.SetKeybinding("", gocui.KeyCtrlC, gocui.ModNone, func(g *gocui.Gui, v *gocui.View) error {
		return gocui.ErrQuit
	}); err != nil {
		log.Fatal(err)
	}

	if err := g.SetKeybinding("", gocui.KeyArrowUp, gocui.ModNone, func(g *gocui.Gui, v *gocui.View) error {
		client.LedBrightnessInc()
		ui.SetLED(client.GetLedBrightness())
		return nil
	}); err != nil {
		log.Fatal(err)
	}

	if err := g.SetKeybinding("", gocui.KeyArrowDown, gocui.ModNone, func(g *gocui.Gui, v *gocui.View) error {
		client.LedBrightnessDec()
		ui.SetLED(client.GetLedBrightness())
		return nil
	}); err != nil {
		log.Fatal(err)
	}

	if err := g.SetKeybinding("", gocui.KeyHome, gocui.ModNone, func(g *gocui.Gui, v *gocui.View) error {
		ui.WriteMessage("Release key")
		client.ReleaseKey()
		client.SetLedBrightness(31)
		ui.SetLED(client.GetLedBrightness())
		return nil
	}); err != nil {
		log.Fatal(err)
	}

	if err := g.SetKeybinding("", gocui.KeyEnd, gocui.ModNone, func(g *gocui.Gui, v *gocui.View) error {
		ui.WriteMessage("Lock key")
		client.LockKey()
		client.SetLedBrightness(0)
		ui.SetLED(client.GetLedBrightness())
		return nil
	}); err != nil {
		log.Fatal(err)
	}

	if err := g.SetKeybinding("", gocui.KeyTab, gocui.ModNone, func(g *gocui.Gui, v *gocui.View) error {
		client.Toggle10()
		return nil
	}); err != nil {
		log.Fatal(err)
	}

	k := client.K
	ui.CommandMap = map[string]func(){
		"quit": ui.Close,
		"q":    ui.Close,
		"clear": func() {
			g.Update(func(g *gocui.Gui) error {
				if v, err := g.View("debug"); err == nil {
					v.Clear()
					v.SetOrigin(0, 0)
				}
				if v, err := g.View("state"); err == nil {
					v.Clear()
					v.SetOrigin(0, 0)

				}
				if v, err := g.View("messages"); err == nil {
					v.Clear()
					v.SetOrigin(0, 0)

				}
				return nil
			})
		},
		"lock": func() {
			ui.WriteMessage("Lock key")
			client.LockKey()
		},
		"release": func() {
			ui.WriteMessage("Release key")
			client.ReleaseKey()
		},
		"open": func() { // open (o)
			msg, err := k.SendAndRecv(defaultTimeout, message.New(2, []byte{0x03, 0x1f}), 2)
			if err != nil {
				ui.WriteMessage(err.Error())
				return
			}
			ui.WriteMessage("open: " + msg.String())
		},
		"rid": func() { //request IDE (i050C)
			msg, err := k.SendAndRecv(defaultTimeout, message.New(2, []byte{0x04}), 2)
			if err != nil {
				ui.WriteMessage(err.Error())
				return
			}
			ui.WriteMessage("request IDE (i050C): " + msg.String())
		},
		"rs": func() { // read status
			msg, err := k.SendAndRecv(defaultTimeout, message.New(2, []byte{0x02, 0x06}), 2)
			if err != nil {
				ui.WriteMessage(err.Error())
				return
			}
			ui.WriteMessage("read status: " + msg.String())
		},
		"off": func() { // off (f)
			msg, err := k.SendAndRecv(defaultTimeout, message.New(2, []byte{0x01}), 2)
			if err != nil {
				ui.WriteMessage(err.Error())
				return
			}
			ui.WriteMessage("off: " + msg.String())
		},
		"read": func() {
			go func() {

				msg, err := k.SendAndRecv(defaultTimeout, message.New(2, []byte{0x03, 0x1f}), 2)
				if err != nil {
					ui.WriteMessage(err.Error())
					return
				}
				ui.WriteMessage("open: " + msg.String())

				msg, err = k.SendAndRecv(defaultTimeout, message.New(2, []byte{0x04}), 2)
				if err != nil {
					ui.WriteMessage(err.Error())
					return
				}
				ui.WriteMessage("request IDE (i050C): " + msg.String())

				msg, err = k.SendAndRecv(defaultTimeout, message.New(2, []byte{0x02, 0x06}), 2)
				if err != nil {
					ui.WriteMessage(err.Error())
					return
				}
				ui.WriteMessage("read status: " + msg.String())

				msg, err = k.SendAndRecv(defaultTimeout, message.New(2, []byte{0x01}), 2)
				if err != nil {
					ui.WriteMessage(err.Error())
					return
				}
				ui.WriteMessage("off: " + msg.String())
			}()
		},
	}

	if err := ui.Run(); err != nil && err != gocui.ErrQuit {
		log.Fatal(err)
	}
}

/*
func getBit(b byte, p int) uint8 {
	return b & (1 << p) >> p
}
*/
