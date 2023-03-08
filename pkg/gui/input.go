package gui

import (
	"strings"

	"github.com/jroimartin/gocui"
)

type Input struct {
	ui        *Gui
	Name      string
	Title     string
	X, Y      int
	W         int
	MaxLength int
}

func NewInput(ui *Gui, name string, x, y, w, maxLength int) *Input {
	return &Input{ui: ui, Name: name, X: x, Y: y, W: w, MaxLength: maxLength}
}

func (i *Input) Layout(g *gocui.Gui) error {
	v, err := g.SetView(i.Name, i.X, i.Y, i.X+i.W, i.Y+2)
	if err != nil {
		if err != gocui.ErrUnknownView {
			return err
		}
		v.Title = i.Title
		v.Overwrite = true
		v.Editor = i
		v.Editable = true
	}
	return nil
}

func (i *Input) Edit(v *gocui.View, key gocui.Key, ch rune, mod gocui.Modifier) {
	cx, _ := v.Cursor()
	ox, _ := v.Origin()
	limit := ox+cx+1 > i.MaxLength
	switch {
	case key == gocui.KeyEnter:
		i.process(
			strings.TrimSpace(v.ViewBuffer()),
		)
		v.Clear()
		v.SetOrigin(0, 0)
	case ch != 0 && mod == 0 && !limit:
		v.EditWrite(ch)
	case key == gocui.KeySpace && !limit:
		v.EditWrite(' ')
	case key == gocui.KeyBackspace || key == gocui.KeyBackspace2:
		v.EditDelete(true)
	}
}

func (i *Input) process(str string) {
	if i.ui.CommandMap != nil {
		if cmd, found := i.ui.CommandMap[str]; found {
			cmd()
		}

	}
}

type Box struct {
	Name string
}
