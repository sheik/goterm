package main

import (
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"gioui.org/app"
	"gioui.org/font/gofont"
	"gioui.org/io/key"
	"gioui.org/io/system"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
	"gioui.org/widget/material"
)

type GioGUI struct {
	window *app.Window
	gtx    *layout.Context
	text   []string
}

func (gui *GioGUI) SetWindowTitle(title string) {
	// gui.window.Option(app.Title(title))
}

func (gui *GioGUI) CreateWindow(term *Terminal) error {
	go func() {
		gui.window = app.NewWindow(app.Title("goterm"), app.Size(unit.Dp(800), unit.Dp(500)))
		err := gui.run(term)
		if err != nil {
			log.Fatal(err)
		}
		os.Exit(0)
	}()
	go app.Main()
	return nil
}

func (gui *GioGUI) run(term *Terminal) error {
	th := material.NewTheme(gofont.Collection())
	var ops op.Ops
	for {
		e := <-gui.window.Events()
		switch e := e.(type) {
		case system.DestroyEvent:
			return e.Err
		case system.FrameEvent:
			ops.Reset()

			gtx := layout.NewContext(&ops, e)

			for _, gtxEvent := range gtx.Events(gui.window) {
				switch e := gtxEvent.(type) {
				case key.Event:
					if e.State.String() == "Press" {
						char := strings.ToLower(e.Name)
						if e.Name == "Space" {
							char = " "
						}
						if e.Name == key.NameReturn {
							char = "\n"
						}
						fmt.Println(char)
						term.pty.Write([]byte(char))
					}
				}
			}

			if gui.gtx != nil {
				gtx = *gui.gtx
			}

			margins := layout.Inset{
				Top:    1,
				Bottom: 1,
				Left:   1,
				Right:  1,
			}
			h := layout.Flex{}
			l := []layout.FlexChild{}

			for i := 0; i < term.height; i++ {
				for j := 0; j < term.width; j++ {
					if term.glyphs[i][j] != nil {
						label := material.Label(th, unit.Sp(12), string(term.glyphs[i][j].literal))
						label.Font = gofont.Collection()[6].Font
						l = append(l, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return label.Layout(gtx)
						}))
					}
				}
			}
			margins.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return h.Layout(gtx, l...)
			})

			gui.text = []string{}
			key.FocusOp{
				Tag: gui.window, // Use the window as the event routing tag. This means we can call gtx.Events(w) and get these events.
			}.Add(gtx.Ops)

			// Specify keys for key.Event
			// Other keys are caught as key.EditEvent
			key.InputOp{
				Keys: key.Set("A|B|C|D|E|F|G|H|I|J|K|L|M|N|O|P|Q|R|S|T|U|V|W|X|Y|Z|-|Space|" + key.NameReturn),
				Tag:  gui.window, // Use the window as the event routing tag. This means we can call gtx.Events(w) and get these events.
			}.Add(gtx.Ops)

			e.Frame(gtx.Ops)
		}
	}
}

func (gui *GioGUI) Main() {
	for {
		time.Sleep(10 * time.Second)
	}
}

func (gui *GioGUI) GetCursorSize() (int, int) {
	return 5, 10
	// panic("not implemented") // TODO: Implement
}

func (gui *GioGUI) DrawCursor(term *Terminal) {
	// panic("not implemented") // TODO: Implement
}

func (gui *GioGUI) DrawRect(term *Terminal, _ bool, _ interface{}, _ int, _ int, _ int, _ int) {
	// panic("not implemented") // TODO: Implement
}

func (gui *GioGUI) EraseCursor(term *Terminal) {
	// panic("not implemented") // TODO: Implement
}

func (gui *GioGUI) UpdateDisplay(term *Terminal) {
	// panic("not implemented") // TODO: Implement
}

func (gui *GioGUI) WriteText(term *Terminal, _ int, _ int, _ interface{}, _ interface{}, text string) {
	gui.text = append(gui.text, text)
}

func (gui *GioGUI) Clear(term *Terminal) {
	// panic("not implemented") // TODO: Implement
}

func (gui *GioGUI) SetFont(weight string) {
	// panic("not implemented") // TODO: Implement
}
