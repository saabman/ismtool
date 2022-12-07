package gui

import (
	"fmt"
	"log"

	"github.com/jroimartin/gocui"
)

type Gui struct {
	g *gocui.Gui
}

func New(g *gocui.Gui) (*Gui, error) {
	mw := &Gui{g: g}
	g.SetManagerFunc(mw.layout)
	return mw, nil
}



func (mw *Gui) layout(g *gocui.Gui) error {
	maxX, maxY := g.Size()
	_ = maxX
	if v, err := g.SetView("debug", 0, 1, 70, maxY-3); err != nil {
		if err != gocui.ErrUnknownView {
			return err
		}
		v.Autoscroll = true
		v.FgColor = gocui.ColorWhite
		v.Title = "Debug"
	}

	if v, err := g.SetView("messages", 71, 1, maxX-1, maxY-3); err != nil {
		if err != gocui.ErrUnknownView {
			return err
		}
		v.Title = "Messages"
		v.Autoscroll = true
		fmt.Fprintln(v, "Welcome to ISM tool 0.0.1")
	}

	input := &Input{
		mw: mw,
		Name:      "input",
		Title:     "Command",
		X:         0,
		Y:         maxY-3,
		W:         maxX-1,
		MaxLength: maxX-1,
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
	mw.g.Close()
}

func (mw *Gui) Run() error {
	return mw.g.MainLoop()
}

func (mw *Gui) WriteMessage(str string) {
	mw.g.Update(func(g *gocui.Gui) error {
		if v, err := g.View("messages"); err == nil {
			fmt.Fprintf(v, "%s\n",str)
		}
		return nil
	})
}

func (mw *Gui) WriteDebug(str string) {
	mw.g.Update(func(g *gocui.Gui) error {
		if v, err := g.View("debug"); err == nil {
			fmt.Fprintf(v, "%s\n",str)
		}
		return nil
	})
}

