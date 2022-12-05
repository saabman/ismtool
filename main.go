package main

import (
	"flag"
	"fmt"
	"log"

	"github.com/jroimartin/gocui"
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
	mw, err := newGUI()
	if err != nil {
		log.Fatal(err)
	}
	defer mw.Close()

	mw.c, err = ism.New(mw.g, portName)
	if err != nil {
		log.Fatal(err)
	}

	if err := mw.g.SetKeybinding("", gocui.KeyCtrlC, gocui.ModNone, quit); err != nil {
		log.Fatal(err)
	}

	if err := mw.g.SetKeybinding("", gocui.KeyArrowUp, gocui.ModNone, func(g *gocui.Gui, v *gocui.View) error {
		g.Update(func(g *gocui.Gui) error {
			if v, err := g.View("messages"); err == nil {
				fmt.Fprintln(v, "Brightness up")
				mw.c.LightInc()
			}
			return nil
		})
		return nil
	}); err != nil {
		log.Fatal(err)
	}

	if err := mw.g.SetKeybinding("", gocui.KeyArrowDown, gocui.ModNone, func(g *gocui.Gui, v *gocui.View) error {
		g.Update(func(g *gocui.Gui) error {
			if v, err := g.View("messages"); err == nil {
				fmt.Fprintln(v, "Brightness down")
				mw.c.LightDec()
			}
			return nil
		})

		return nil
	}); err != nil {
		log.Fatal(err)
	}

	if err := mw.g.SetKeybinding("", gocui.KeyHome, gocui.ModNone, func(g *gocui.Gui, v *gocui.View) error {
		g.Update(func(g *gocui.Gui) error {
			if v, err := g.View("messages"); err == nil {
				fmt.Fprintln(v, "Release Key")
				mw.c.ReleaseKey()
			}
			return nil
		})

		return nil
	}); err != nil {
		log.Fatal(err)
	}

	if err := mw.g.SetKeybinding("", gocui.KeyEnd, gocui.ModNone, func(g *gocui.Gui, v *gocui.View) error {
		g.Update(func(g *gocui.Gui) error {
			if v, err := g.View("messages"); err == nil {
				fmt.Fprintln(v, "Lock key")
				mw.c.LockKey()
			}
			return nil
		})

		return nil
	}); err != nil {
		log.Fatal(err)
	}

	//defer c.Close()

	if err := mw.Run(); err != nil && err != gocui.ErrQuit {
		log.Fatal(err)
	}
}

type App struct {
	g *gocui.Gui
	c *ism.Client
}

func newGUI() (*App, error) {
	g, err := gocui.NewGui(gocui.Output256)
	if err != nil {
		return nil, err
	}
	mw := &App{g: g}

	g.SetManagerFunc(mw.layout)
	return mw, nil
}

func (mw *App) layout(g *gocui.Gui) error {
	maxX, maxY := g.Size()
	_ = maxX
	if v, err := g.SetView("debug", 0, 1, 70, maxY-2); err != nil {
		if err != gocui.ErrUnknownView {
			return err
		}
		v.Autoscroll = true
		v.FgColor = gocui.ColorWhite
		v.Title = "Debug"
		go mw.c.Debug(g, 14)
	}

	if v, err := g.SetView("messages", 71, 1, maxX-1, maxY-2); err != nil {
		if err != gocui.ErrUnknownView {
			return err
		}
		v.Title = "Messages"
		v.Autoscroll = true
		fmt.Fprintln(v, "Welcome to ISM tool 0.0.1")
	}

	return nil
}

func (mw *App) Close() {
	mw.g.Close()
}

func (mw *App) Run() error {
	return mw.g.MainLoop()
}

func quit(g *gocui.Gui, v *gocui.View) error {
	return gocui.ErrQuit
}

/*
	quitChan := make(chan os.Signal, 2)
	signal.Notify(quitChan, syscall.SIGTERM, syscall.SIGINT)

	log.Println("Starting on", portName)

	c, err := ism.New(portName)
	if err != nil {
		log.Fatal(err)
	}
	defer c.Close()

	go c.Debug() // Print all incomming messages

	<-quitChan
	log.Println("exit")
*/
