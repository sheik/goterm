package main

import (
	"image"
	"log"
	"os"
	"strings"

	"github.com/sheik/freetype-go/freetype/truetype"
	"github.com/sheik/xgb/xproto"
	"github.com/sheik/xgbutil"
	"github.com/sheik/xgbutil/ewmh"
	"github.com/sheik/xgbutil/keybind"
	"github.com/sheik/xgbutil/xevent"
	"github.com/sheik/xgbutil/xgraphics"
	"github.com/sheik/xgbutil/xwindow"
)

type XGBGui struct {
	X           *xgbutil.XUtil
	font        *truetype.Font
	fontRegular *truetype.Font
	fontBold    *truetype.Font
	img         *xgraphics.Image
	window      *xwindow.Window
}

func (x *XGBGui) KeyPressCallback(term *Terminal) func(*xgbutil.XUtil, xevent.KeyPressEvent) {
	return func(X *xgbutil.XUtil, e xevent.KeyPressEvent) {
		modStr := keybind.ModifierString(e.State)
		keyStr := keybind.LookupString(X, e.State, e.Detail)

		if keybind.KeyMatch(X, "Backspace", e.State, e.Detail) {
			term.pty.Write([]byte{0x08})
			return
		}

		if keybind.KeyMatch(X, "Escape", e.State, e.Detail) {
			if e.State&xproto.ModMaskControl > 0 {
				log.Println("Control-Escape detected. Quitting...")
				xevent.Quit(X)
			}
		}

		if keybind.KeyMatch(X, "Return", e.State, e.Detail) {
			term.pty.Write([]byte{'\n'})
			return
		}

		if keybind.KeyMatch(X, "Escape", e.State, e.Detail) {
			term.pty.Write([]byte{27})
			return
		}

		if keybind.KeyMatch(X, "Tab", e.State, e.Detail) {
			term.pty.Write([]byte{'\t'})
			return
		}

		if len(modStr) > 0 {
			if strings.Contains(modStr, "shift") {
				reply, _ := xproto.GetKeyboardMapping(x.X.Conn(), e.Detail, 1).Reply()
				chr := string(reply.Keysyms[1])
				term.pty.Write([]byte(chr))
			}
			if strings.Contains(modStr, "control") {
				switch keyStr {
				case "a":
					term.pty.Write([]byte{0x01})
				case "c":
					term.pty.Write([]byte{0x03})
				case "d":
					term.pty.Write([]byte{0x04})
				case "l":
					term.pty.Write([]byte{0x0C})
				case "p":
					term.pty.Write([]byte{0x10})
				case "r":
					term.pty.Write([]byte{0x12})
				}
			}
		} else {
			switch keyStr {
			case "Left":
				term.pty.Write([]byte("\033[D"))
			case "Up":
				term.pty.Write([]byte("\033[A"))
			case "Right":
				term.pty.Write([]byte("\033[C"))
			case "Down":
				term.pty.Write([]byte("\033[B"))
			}
			if len(keyStr) == 1 {
				term.pty.Write([]byte(keyStr))
			}
		}
	}
}

func (x *XGBGui) Main(term *Terminal) {
	xevent.Main(x.X)
}

func (x *XGBGui) CreateWindow(term *Terminal) (err error) {
	x.X, err = xgbutil.NewConn()
	if err != nil {
		return err
	}
	keybind.Initialize(x.X)

	fontReader, err := os.Open(fontPath)
	if err != nil {
		log.Fatal(err)
	}

	// Now parse the font.
	x.fontRegular, err = xgraphics.ParseFont(fontReader)
	if err != nil {
		log.Fatal(err)
	}
	fontReader.Close()

	fontReader, err = os.Open(fontPathBold)
	if err != nil {
		log.Fatal(err)
	}
	x.font = x.fontRegular

	// Now parse the font.
	x.fontBold, err = xgraphics.ParseFont(fontReader)
	if err != nil {
		log.Fatal(err)
	}
	fontReader.Close()

	// set term width/height to full block
	term.cursor.width, term.cursor.height = term.ui.GetCursorSize()

	// Create some canvas.
	x.img = xgraphics.New(x.X, image.Rect(0, 0, term.width*term.cursor.width, term.height*term.cursor.height))
	x.img.For(func(x, y int) xgraphics.BGRA {
		return *bg
	})

	// Now show the image in its own window.
	x.window = x.img.XShowExtra("goterm", true)

	x.window.Listen(xproto.EventMaskKeyPress, xproto.EventMaskKeyRelease)

	xevent.KeyPressFun(x.KeyPressCallback(term)).Connect(x.X, x.window.Id)

	return nil
}

func (x *XGBGui) GetCursorSize() (width, height int) {
	return xgraphics.Extents(x.font, size, "\u2588")
}

func (x *XGBGui) SetFont(weight string) {
	switch weight {
	case "regular":
		x.font = x.fontRegular
	case "bold":
		x.font = x.fontBold
	}
}

func (x *XGBGui) Clear(term *Terminal) {
	rect := image.Rect(0, 0, term.width*term.cursor.width, term.height*term.cursor.height)
	box, ok := x.img.SubImage(rect).(*xgraphics.Image)
	if ok {
		box.For(func(x, y int) xgraphics.BGRA {
			return *bg
		})
		box.XDraw()
	}
}

func (gui *XGBGui) WriteText(term *Terminal, x int, y int, fg, bg interface{}, text string) {
	rect := image.Rect(x*term.cursor.width, y*term.cursor.height, x*term.cursor.width+term.cursor.width, y*term.cursor.height+term.cursor.height)
	box, ok := gui.img.SubImage(rect).(*xgraphics.Image)
	if ok {
		box.For(func(x, y int) xgraphics.BGRA {
			return bg.(xgraphics.BGRA)
		})
		box.XDraw()
	}

	_, _, err := gui.img.Text(x*term.cursor.width, y*term.cursor.height, fg.(xgraphics.BGRA), size, gui.font, text)

	if err != nil {
		log.Fatal(err)
	}
}

func (x *XGBGui) SetWindowTitle(title string) {
	ewmh.WmNameSet(x.X, x.window.Id, title)
}

func (x *XGBGui) DrawCursor(term *Terminal) {
	x.DrawRect(term,
		false,
		fg,
		term.cursor.X*term.cursor.width,
		term.cursor.Y*term.cursor.height,
		(term.cursor.X*term.cursor.width)+term.cursor.width,
		(term.cursor.Y*term.cursor.height)+term.cursor.height,
	)

	if term.cursor.Y > term.height-1 {
		return
	}

	if term.cursor.X > term.width-1 {
		return
	}

	g := term.glyphs[term.cursor.Y][term.cursor.X]
	if g != nil {
		x.WriteText(term, term.cursor.X*term.cursor.width, term.cursor.Y*term.cursor.height, g.fg, g.bg, string(g.literal))
	}
}

func (x *XGBGui) EraseCursor(term *Terminal) {
	rect := image.Rect(term.cursor.X*term.cursor.width, term.cursor.Y*term.cursor.height, (term.cursor.X*term.cursor.width)+term.cursor.width, (term.cursor.Y*term.cursor.height)+term.cursor.height)
	box, ok := x.img.SubImage(rect).(*xgraphics.Image)

	if ok {
		box.For(func(x, y int) xgraphics.BGRA {
			x = x / term.cursor.width
			y = y / term.cursor.height
			if term.glyphs[y][x] != nil {
				return *(term.glyphs[y][x].bg)
			} else {
				return *bg
			}
		})
		box.XDraw()
	}
	if term.cursor.Y > term.height-1 {
		return
	}
	if term.cursor.X > term.width-1 {
		return
	}
	g := term.glyphs[term.cursor.Y][term.cursor.X]
	if g != nil {
		_, _, err := x.img.Text(term.cursor.X*term.cursor.width, term.cursor.Y*term.cursor.height, g.fg, size, x.font, string(g.literal))
		if err != nil {
			panic(err)
		}
	}
	needsDraw = true
}

func (gui *XGBGui) DrawRect(term *Terminal, redraw bool, color interface{}, x0, y0, x1, y1 int) {
	rect := image.Rect(x0, y0, x1, y1)
	box, ok := gui.img.SubImage(rect).(*xgraphics.Image)
	if ok {
		box.For(func(x, y int) xgraphics.BGRA {
			if redraw {
				x = x / term.cursor.width
				y = y / term.cursor.height
				if term.glyphs[y][x] != nil {
					return *(term.glyphs[y][x].bg)
				} else {
					return *bg
				}
			} else {
				return color.(xgraphics.BGRA)
			}
		})
		box.XDraw()
	}
}

func (x *XGBGui) UpdateDisplay(term *Terminal) {
	if needsDraw {
		term.ui.DrawCursor(term)
		x.img.XDraw()
		x.img.XPaint(x.window.Id)
		needsDraw = false
	}
}
