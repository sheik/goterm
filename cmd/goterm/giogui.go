package main

import (
	"fmt"
	"image/color"
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
						if e.Modifiers.String() == "Ctrl" {
							fmt.Println(e.Name)
							switch e.Name {
							case "A":
								char = "\x01"
							case "C":
								char = "\x03"
							case "D":
								char = "\x04"
							case "L":
								char = "\x0C"
							case "P":
								char = "\x10"
							case "R":
								char = "\x12"
							}
						}
						if e.Name == "Space" {
							char = " "
						}
						if e.Name == key.NameDeleteBackward {
							char = "\x08"
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
				Top:    0,
				Bottom: 0,
				Left:   0,
				Right:  0,
			}
			h := layout.Flex{}

			var visList = layout.List{
				Axis: layout.Vertical,
			}

			margins.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return visList.Layout(gtx, len(term.glyphs), func(gtx layout.Context, index int) layout.Dimensions {
					l := []layout.FlexChild{}
					for i := 0; i < term.width; i++ {
						if term.glyphs[index][i] == nil {
							term.glyphs[index][i] = &Glyph{
								X:       i,
								Y:       index,
								literal: []byte{' '},
							}
						}
						th.Bg = color.NRGBA{
							R: 0x00,
							G: 0x00,
							B: 0,
							A: 0xff,
						}
						th.Fg = color.NRGBA{
							R: term.glyphs[index][i].fg.R,
							G: term.glyphs[index][i].fg.G,
							B: term.glyphs[index][i].fg.B,
							A: term.glyphs[index][i].fg.A,
						}
						label := material.Label(th, unit.Sp(12), string(term.glyphs[index][i].literal))
						label.Font = gofont.Collection()[6].Font
						l = append(l, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return label.Layout(gtx)
						}))
					}
					return h.Layout(gtx, l...)
				})
			})

			gui.text = []string{}
			key.FocusOp{
				Tag: gui.window, // Use the window as the event routing tag. This means we can call gtx.Events(w) and get these events.
			}.Add(gtx.Ops)

			// Specify keys for key.Event
			// Other keys are caught as key.EditEvent
			key.InputOp{
				Keys: key.Set("A|B|C|D|E|F|G|H|I|J|K|L|M|N|O|P|Q|R|S|T|U|V|W|X|Y|Z|-|Space|Ctrl-D|Ctrl-L|Ctrl-A|Ctrl-C|Ctrl-P|Ctrl-R|" + strings.Join([]string{key.NameReturn, key.NameDeleteBackward}, "|")),
				Tag:  gui.window, // Use the window as the event routing tag. This means we can call gtx.Events(w) and get these events.
			}.Add(gtx.Ops)

			e.Frame(gtx.Ops)
		}
	}
}

func (gui *GioGUI) Main(term *Terminal) {
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
	if gui.window != nil {
		gui.window.Invalidate()
	}
}

func (gui *GioGUI) Clear(term *Terminal) {
	// panic("not implemented") // TODO: Implement
}

func (gui *GioGUI) SetFont(weight string) {
	// panic("not implemented") // TODO: Implement
}
