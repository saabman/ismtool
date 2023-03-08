package main

import (
	"errors"
	"fmt"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/data/binding"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/roffe/ismtool/ui"
	"go.bug.st/serial/enumerator"
)

type mainWindow struct {
	log        *widget.List
	logEntries binding.StringList

	port         string
	rescanButton *widget.Button
	portList     *widget.Select

	statusbar *widget.Label
	fyne.Window
}

func newMainWindow(app fyne.App) *mainWindow {
	mw := &mainWindow{
		Window:     app.NewWindow("ISM Tool"),
		statusbar:  &widget.Label{Text: "Welcome to ISM tool", Alignment: fyne.TextAlignLeading},
		logEntries: binding.NewStringList(),
	}

	mw.log = widget.NewListWithData(
		mw.logEntries,
		func() fyne.CanvasObject {
			return &widget.Label{
				TextStyle: fyne.TextStyle{Monospace: true},
			}
		},
		func(item binding.DataItem, obj fyne.CanvasObject) {
			i := item.(binding.String)
			txt, err := i.Get()
			if err != nil {
				panic(err)
			}
			if v, ok := obj.(*widget.Label); ok {
				v.SetText(txt)
			}
		},
	)

	message, ports, err := ListPorts()
	if err != nil {
		mw.output(err.Error())
	}
	if message != "" {
		mw.output(message)
	}

	mw.rescanButton = widget.NewButtonWithIcon("Refresh ports", theme.ViewRefreshIcon(), func() {
		message, ports, err := ListPorts()
		if err != nil {
			mw.output(err.Error())
			return
		}
		mw.portList.Options = ports
		mw.output(message)
	})

	mw.portList = &widget.Select{
		PlaceHolder: mw.port,
		Alignment:   fyne.TextAlignCenter,
		Options:     ports,
		OnChanged: func(s string) {
			mw.port = s
		},
	}

	mw.Resize(fyne.NewSize(800, 600))
	mw.SetContent(mw.layout())
	mw.Show()
	return mw
}

func (mw *mainWindow) layout() fyne.CanvasObject {
	cg := widget.NewCheckGroup([]string{
		"PG3L, Page3 Lock (OTP)",
		"PWP1, Page Write Protected 1 (OTP)",
		"PWP0, Page Write Protected 0 (OTP)",
		"ENC, Enable Crypto Mode",
		"MS1, Read Only Mode Select",
		"MS0, Read Only Mode Select",
		"Data Coding (0=MC, 1=BF)",
	}, func(s []string) {

	})

	split := &container.Split{
		Horizontal: true,
		Offset:     0.9,
		Leading: &container.Split{
			Offset: 0.8,
			Leading: container.NewGridWithColumns(2,
				container.NewVBox(
					widget.NewForm(
						widget.NewFormItem("", widget.NewLabel("")),
						widget.NewFormItem("P0", widget.NewEntry()),
						widget.NewFormItem("P1", widget.NewEntry()),
						widget.NewFormItem("P2", widget.NewEntry()),
						widget.NewFormItem("P3", widget.NewEntry()),
						widget.NewFormItem("P4", widget.NewEntry()),
						widget.NewFormItem("P5", widget.NewEntry()),
						widget.NewFormItem("P6", widget.NewEntry()),
						widget.NewFormItem("P7", widget.NewEntry()),
					),
				),
				container.NewVBox(
					widget.NewLabel("TMCF (P3)"),
					cg,
					//widget.NewLabel("PG3L, Page3 Lock (OTP)"),
					//widget.NewLabel("PWP1, Page Write Protected 1 (OTP)"),
					//widget.NewLabel("PWP0, Page Write Protected 0 (OTP)"),
					//widget.NewLabel("ENC, Enable Crypto Mode"),
					//widget.NewLabel("MS1, Read Only Mode Select"),
					//widget.NewLabel("MS0, Read Only Mode Select"),
					//widget.NewLabel("Data Coding (0=MC, 1=BF)"),
				),
			),
			Trailing: container.NewVScroll(mw.log),
		},
		Trailing: container.NewVBox(
			mw.rescanButton,
			mw.portList,
			widget.NewButton("Connect", func() {}),
			widget.NewButton("Read key", func() {}),
			layout.NewSpacer(),
		),
	}
	return container.NewBorder(nil, mw.statusbar, nil, nil, split)
}
func (mw *mainWindow) output(str string) {
	lines := strings.Split(str, "\n")
	for _, line := range lines {
		mw.logEntries.Append(line)
	}
	mw.log.ScrollToBottom()
}

func main() {
	ismtool := app.NewWithID("com.ismtool")
	ismtool.Settings().SetTheme(&ui.Theme{})
	w := newMainWindow(ismtool)
	ismtool.Lifecycle().SetOnStarted(func() {
	})
	w.ShowAndRun()
}

func ListPorts() (string, []string, error) {
	var portsList []string
	ports, err := enumerator.GetDetailedPortsList()
	if err != nil {
		return "", nil, err
	}
	if len(ports) == 0 {
		return "", nil, errors.New("no serial ports found")
	}
	var output strings.Builder

	output.WriteString("detected ports:\n")
	for i, port := range ports {
		prefix := " "
		jun := "┗"
		if len(ports) > 1 && i+1 < len(ports) {
			prefix = "┃"
			jun = "┣"
		}
		output.WriteString(fmt.Sprintf("  %s %s\n", jun, port.Name))
		if port.IsUSB {
			output.WriteString(fmt.Sprintf("  %s  ┣ USB ID: %s:%s\n", prefix, port.VID, port.PID))
			output.WriteString(fmt.Sprintf("  %s  ┗ USB serial: %s\n", prefix, port.SerialNumber))
			portsList = append(portsList, port.Name)
		}
	}
	return output.String(), portsList, nil
}
