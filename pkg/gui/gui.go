package gui

import (
	"fmt"
	"log"
	"strings"

	"github.com/jroimartin/gocui"
)

type CmdMap map[string]func()

type Gui struct {
	g          *gocui.Gui
	CommandMap map[string]func()
}

func New(g *gocui.Gui) (*Gui, error) {
	ui := &Gui{g: g}
	g.SetManagerFunc(ui.layout)
	return ui, nil
}

func (ui *Gui) layout(g *gocui.Gui) error {
	maxX, maxY := g.Size()
	_ = maxX
	if v, err := g.SetView("debug", 0, 0, 73, maxY-4); err != nil {
		if err != gocui.ErrUnknownView {
			return err
		}
		v.Autoscroll = true
		v.FgColor = gocui.ColorWhite
		v.Title = "Debug"
		v.Wrap = true
	}

	if v, err := g.SetView("state", 74, 0, 120, maxY-4); err != nil {
		if err != gocui.ErrUnknownView {
			return err
		}
		v.Title = "State changes"
		v.Wrap = true
		v.Autoscroll = true
	}

	if v, err := g.SetView("messages", 121, 0, maxX-35, maxY-4); err != nil {
		if err != gocui.ErrUnknownView {
			return err
		}
		v.Title = "Messages"
		v.Autoscroll = true
		v.Wrap = true
		//fmt.Fprintln(v, "Welcome to ISM tool 0.0.1")
	}

	if v, err := g.SetView("help", maxX-34, 0, maxX-1, maxY-4); err != nil {
		if err != gocui.ErrUnknownView {
			return err
		}
		v.Title = "Commands"
		v.Wrap = true

		commands := []string{
			"quit,q - exit ism tool",
			"release - release ism lock",
			"clear - clear output",
			"lock - lock ism",
			"open - radio open",
			"rid - read id",
			"rs - read status",
			"off - radio off",
			"read - open, read close",
		}
		fmt.Fprintln(v, strings.Join(commands, "\n"))
	}

	if v, err := g.SetView("key_position", 0, maxY-3, 17, maxY-1); err != nil {
		if err != gocui.ErrUnknownView {
			return err
		}
		v.Title = "Key Position"
		v.Overwrite = true
	}

	if v, err := g.SetView("led", 18, maxY-3, 25, maxY-1); err != nil {
		if err != gocui.ErrUnknownView {
			return err
		}
		fmt.Fprint(v, " 00")
		v.Title = "LED"
		v.Overwrite = true
	}

	input := &Input{
		ui:        ui,
		Name:      "input",
		Title:     "Command",
		X:         26,
		Y:         maxY - 3,
		W:         40,
		MaxLength: 45,
	}

	if err := input.Layout(g); err != nil {
		return err
	}

	if _, err := g.SetCurrentView("input"); err != nil {
		log.Fatal(err)
	}
	return nil
}

func (ui *Gui) Close() {
	ui.g.Update(func(g *gocui.Gui) error {
		return gocui.ErrQuit
	})
}

func (ui *Gui) Run() error {
	return ui.g.MainLoop()
}

func (ui *Gui) Write(view string, str string) {
	ui.g.Update(func(g *gocui.Gui) error {
		if v, err := g.View(view); err == nil {
			fmt.Fprintf(v, "%s\n", str)
		}
		return nil
	})
}

func (ui *Gui) Writef(view string, format string, values ...interface{}) {
	ui.Write(view, fmt.Sprintf(format, values...))
}

func (ui *Gui) WriteMessage(str string) {
	ui.g.Update(func(g *gocui.Gui) error {
		if v, err := g.View("messages"); err == nil {
			fmt.Fprintf(v, "%s\n", str)
		}
		return nil
	})
}

func (ui *Gui) WriteMessagef(format string, values ...interface{}) {
	ui.WriteMessage(fmt.Sprintf(format, values...))
}

func (ui *Gui) WriteDebug(str string) {
	ui.g.Update(func(g *gocui.Gui) error {
		if v, err := g.View("debug"); err == nil {
			fmt.Fprintf(v, "%s\n", str)
		}
		return nil
	})
}

func (ui *Gui) WriteDebugf(format string, values ...interface{}) {
	ui.WriteDebug(fmt.Sprintf(format, values...))
}

func (ui *Gui) WriteState(str string) {
	ui.g.Update(func(g *gocui.Gui) error {
		if v, err := g.View("state"); err == nil {
			fmt.Fprintf(v, "%s\n", str)
		}
		return nil
	})
}

func (ui *Gui) WriteStatef(format string, values ...interface{}) {
	ui.WriteState(fmt.Sprintf(format, values...))
}

func (ui *Gui) SetKeyPosition(str string) {
	ui.g.Update(func(g *gocui.Gui) error {
		if v, err := g.View("key_position"); err == nil {
			v.Clear()
			fmt.Fprintf(v, "%s", str)
		}
		return nil
	})
}

func (ui *Gui) SetLED(value uint8) {
	ui.g.Update(func(g *gocui.Gui) error {
		if v, err := g.View("led"); err == nil {
			v.Clear()
			fmt.Fprintf(v, " %02d", value)
		}
		return nil
	})
}
