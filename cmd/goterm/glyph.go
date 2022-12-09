package main

import (
	"gioui.org/font/gofont"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget/material"
	"image"
	"image/color"
)

type TerminalChar struct {
	th      *material.Theme
	literal string
	font    text.Font
	fg      color.NRGBA
	bg      color.NRGBA
	label   material.LabelStyle
}

func NewTerminalChar(th *material.Theme, text string) *TerminalChar {
	if text == "" {
		text = " "
	}
	tc := &TerminalChar{literal: text}
	tc.fg = color.NRGBA{
		R: th.Fg.R,
		G: th.Fg.G,
		B: th.Fg.B,
		A: 0xff,
	}
	tc.bg = color.NRGBA{
		R: th.Bg.R,
		G: th.Bg.G,
		B: th.Bg.B,
		A: th.Bg.A,
	}
	tc.th = th
	tc.label = material.Label(tc.th, unit.Sp(12), tc.literal)
	tc.label.Font = gofont.Collection()[6].Font
	return tc
}

func (tc *TerminalChar) Layout(gtx layout.Context) layout.Dimensions {
	//	th := material.NewTheme(gofont.Collection())
	//	th.Fg = tc.fg
	//	th.Bg = tc.bg
	return layout.Stack{}.Layout(gtx,
		layout.Stacked(func(gtx layout.Context) layout.Dimensions {
			return drawSquare(gtx, gtx.Ops, tc.bg, tc.label)
		}),
		layout.Stacked(func(gtx layout.Context) layout.Dimensions {
			return tc.label.Layout(gtx)
		}),
	)
}

func drawSquare(gtx layout.Context, ops *op.Ops, color color.NRGBA, l material.LabelStyle) layout.Dimensions {
	dimensions := l.Layout(gtx)
	defer clip.Rect{Max: image.Pt(dimensions.Size.X, dimensions.Size.Y)}.Push(ops).Pop()
	paint.ColorOp{Color: color}.Add(ops)
	paint.PaintOp{}.Add(ops)
	return layout.Dimensions{Size: image.Pt(dimensions.Size.X, dimensions.Size.Y)}
}
