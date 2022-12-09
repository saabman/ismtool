package gui

import (
	"fmt"
	"log"
	"strings"

	"github.com/jroimartin/gocui"
)

type CmdMap map[string]func()

type Gui struct {
	g      *gocui.Gui
	cmdMap map[string]func()
}

func New(g *gocui.Gui) (*Gui, error) {
	mw := &Gui{g: g}
	g.SetManagerFunc(mw.layout)
	return mw, nil
}

func (mw *Gui) SetCommandMap(cmdMap map[string]func()) {
	mw.cmdMap = cmdMap
}

func (mw *Gui) layout(g *gocui.Gui) error {
	maxX, maxY := g.Size()
	_ = maxX
	if v, err := g.SetView("debug", 0, 0, 70, maxY-4); err != nil {
		if err != gocui.ErrUnknownView {
			return err
		}
		v.Autoscroll = true
		v.FgColor = gocui.ColorWhite
		v.Title = "Debug"
		v.Wrap = true
	}

	if v, err := g.SetView("state", 71, 0, 110, maxY-4); err != nil {
		if err != gocui.ErrUnknownView {
			return err
		}
		v.Title = "State changes"
		v.Wrap = true
		v.Autoscroll = true
	}

	if v, err := g.SetView("messages", 111, 0, maxX-35, maxY-4); err != nil {
		if err != gocui.ErrUnknownView {
			return err
		}
		v.Title = "Messages"
		v.Autoscroll = true
		v.Wrap = true
		fmt.Fprintln(v, "Welcome to ISM tool 0.0.1")
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
			"lock - lock ism",
			"open - radio open",
			"rid - read id",
			"rs - read status",
			"off - radio off",
			"read - open, read close",
		}
		fmt.Fprintln(v, strings.Join(commands, "\n"))
	}

	if v, err := g.SetView("key_position", maxX-80, 0, maxX-50, 2); err != nil {
		if err != gocui.ErrUnknownView {
			return err
		}
		v.Title = "Key Position"
	}

	input := &Input{
		mw:        mw,
		Name:      "input",
		Title:     "Command",
		X:         0,
		Y:         maxY - 3,
		W:         maxX - 1,
		MaxLength: maxX - 1,
	}

	if err := input.Layout(g); err != nil {
		return err
	}

	if _, err := g.SetCurrentView("input"); err != nil {
		log.Fatal(err)
	}
	return nil
}

func (mw *Gui) Close() {
	mw.g.Update(func(g *gocui.Gui) error {
		return gocui.ErrQuit
	})
}

func (mw *Gui) Run() error {
	return mw.g.MainLoop()
}

func (mw *Gui) WriteMessage(str string) {
	mw.g.Update(func(g *gocui.Gui) error {
		if v, err := g.View("messages"); err == nil {
			fmt.Fprintf(v, "%s\n", str)
		}
		return nil
	})
}

func (mw *Gui) WriteDebug(str string) {
	mw.g.Update(func(g *gocui.Gui) error {
		if v, err := g.View("debug"); err == nil {
			fmt.Fprintf(v, "%s\n", str)
		}
		return nil
	})
}

func (mw *Gui) WriteState(str string) {
	mw.g.Update(func(g *gocui.Gui) error {
		if v, err := g.View("state"); err == nil {
			fmt.Fprintf(v, "%s\n", str)
		}
		return nil
	})
}

func (mw *Gui) SetKeyPosition(str string) {
	mw.g.Update(func(g *gocui.Gui) error {
		if v, err := g.View("key_position"); err == nil {
			v.Clear()
			fmt.Fprintf(v, "%s", str)
		}
		return nil
	})
}
