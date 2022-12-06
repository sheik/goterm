package main

import (
	"image"
	"image/color"
	"log"
	"os"

	"gioui.org/app"
	"gioui.org/font/gofont"
	"gioui.org/io/system"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget/material"
)

type GioGUI struct {
	window *app.Window
	gtx    *layout.Context
	text   []string
}

func (gui *GioGUI) SetWindowTitle(title string) {
	gui.window.Option(app.Title(title))
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

	return nil
}

func (gui *GioGUI) run(term *Terminal) error {
	w := gui.window
	th := material.NewTheme(gofont.Collection())
	var ops op.Ops
	for {
		e := <-w.Events()
		switch e := e.(type) {
		case system.DestroyEvent:
			return e.Err
		case system.FrameEvent:
			gtx := layout.NewContext(&ops, e)
			if gui.gtx != nil {
				gtx = *gui.gtx
			}

			title := material.H1(th, "Hello, Gio")
			maroon := color.NRGBA{R: 127, G: 0, B: 0, A: 255}
			title.Color = maroon
			title.Alignment = text.Middle
			title.Layout(gtx)

			h := layout.Flex{}
			l := []layout.FlexChild{}
			for _, text := range gui.text {
				op.Offset(image.Pt(100, 0))
				label := material.Label(th, unit.Sp(12), text)
				l = append(l, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return label.Layout(gtx)
				}))
			}
			h.Layout(gtx, l...)

			gui.text = []string{}

			e.Frame(gtx.Ops)
		}
	}
}

func (gui *GioGUI) Main() {
	app.Main()
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
