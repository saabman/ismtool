package main

import (
	"bytes"
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

type Result struct {
	Flag1    bool
	KeyPos1  uint8
	Flag2    bool
	Flag3    bool
	Flag4    bool
	Flag5    bool
	Flag6    bool
	KeyPos2  uint8
	Num1     uint8
	Unknown1 bool
	Flag7    bool
	Flag8    bool
	Num2     uint8
	Unknown2 bool
	Unknown3 bool
	Unknown4 bool
}

func (r *Result) Strfing() string {
	return fmt.Sprintf("f1: %t, k1: %d, f2: %t, f3: %t, f4: %t, f5: %t, f6: %t, k2: %d, n1: %d, u1: %t, f7: %t, f8: %t, n2: %d, u2: %t, u3: %t, u4: %t",
		r.Flag1, r.KeyPos1, r.Flag2, r.Flag3, r.Flag4, r.Flag5, r.Flag6, r.KeyPos2, r.Num1, r.Unknown1, r.Flag7, r.Flag8, r.Num2, r.Unknown2, r.Unknown3, r.Unknown4)
}

func (r *Result) String() string {
	return fmt.Sprintf("%t, %d, %t, %t, %t, %t, %t, %d, %d, %t, %t, %t, %d, %t, %t, %t",
		r.Flag1, r.KeyPos1, r.Flag2, r.Flag3, r.Flag4, r.Flag5, r.Flag6, r.KeyPos2, r.Num1, r.Unknown1, r.Flag7, r.Flag8, r.Num2, r.Unknown2, r.Unknown3, r.Unknown4)
}

func boolToByte(b bool) byte {
	if b {
		return 1
	}
	return 0
}

func (r Result) MarshalBinary() ([]byte, error) {
	output := make([]byte, 3)
	output[0] = byte((boolToByte(r.Flag1) << 7) | (r.KeyPos1 << 3) | (boolToByte(r.Flag2) << 2) | (boolToByte(r.Flag3) << 1) | boolToByte(r.Flag4))
	output[1] = byte((boolToByte(r.Flag5) << 7) | (r.KeyPos2 << 3) | (r.Num1 << 1) | boolToByte(r.Unknown1))
	output[2] = byte((boolToByte(r.Flag7) << 7) | (boolToByte(r.Flag8) << 6) | (r.Num2 << 3) | (boolToByte(r.Unknown2) << 2) | (boolToByte(r.Unknown3) << 1) | boolToByte(r.Unknown4))
	return output[:], nil
}

func (r *Result) UnmarshalBinary(input []byte) error {
	r.Flag1 = input[0]&0x80 != 0
	r.KeyPos1 = (input[0] & 0x78) >> 3
	r.Flag2 = input[0]&0x04 != 0
	r.Flag3 = input[0]&0x02 != 0
	r.Flag4 = input[0]&0x01 != 0
	r.Flag5 = input[1]&0x80 != 0
	r.KeyPos2 = (input[1] & 0x78) >> 3
	r.Num1 = (input[1] & 0x06) >> 1
	r.Unknown1 = input[1]&0x01 != 0
	r.Flag7 = input[2]&0x80 != 0
	r.Flag8 = input[2]&0x40 != 0
	r.Num2 = (input[2] & 0x38) >> 3
	r.Unknown2 = input[2]&0x04 != 0
	r.Unknown3 = input[2]&0x02 != 0
	r.Unknown4 = input[2]&0x01 != 0
	return nil
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

	client, err := ism.New(portName)
	if err != nil {
		log.Fatal(err)
	}
	defer client.Close()

	client.OnError = func(err error) {
		ui.WriteMessagef("Error: %v", err)
	}

	sc, err := os.Create("statechange.log")
	if err != nil {
		log.Fatal(err)
	}
	defer sc.Close()

	keyInserted := false
	var lastKey *ism.KeyInfo

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

		res := &Result{}
		if err := res.UnmarshalBinary(data); err != nil {
			log.Fatal(err)
		}

		if c == ism.KeyInserted && !keyInserted {
			lastKey, err = client.ReadKeyIDE()
			if err == nil {
				if bytes.Equal(lastKey.P0, []byte{0xF5, 0x9A, 0x8A, 0x21}) {
					keyInserted = true
					client.ReleaseKey()
					client.SetLedBrightness(31)
				}
			} else {
				ui.WriteMessage(err.Error())
				return
			}
		}

		if c == ism.KeyNotInserted {
			keyInserted = false
			client.SetLedBrightness(0)
			client.LockKey()
		}

		if keyInserted {
			ui.WriteMessagef("Key %s %X [%s]", c.String(), lastKey.P0, res.String())
			return
		}

		ui.WriteMessagef("Key %s [%s]", c.String(), res.String())
	}

	f, err := os.Create("communication.log")
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()

	client.K.OnIncoming = func(msg message.Message) {
		f.Write([]byte(fmt.Sprintf(" IN: %-12s %d %d %X\n", time.Now().Format("15:04:05.999"), msg.ID(), len(msg.Data()), msg.Data())))
		switch msg.ID() {
		case 0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 11, 12, 13, 15:
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

				k, err := client.ReadKeyIDE()
				if err != nil {
					ui.WriteMessage(err.Error())
					return
				}
				ui.WriteMessagef("read: %X", k.P0)
				/*
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
				*/
			}()
		},
	}

	g.SetCurrentView("command")

	if err := ui.Run(); err != nil && err != gocui.ErrQuit {
		log.Fatal(err)
	}
}

/*
func getBit(b byte, p int) uint8 {
	return b & (1 << p) >> p
}
*/
