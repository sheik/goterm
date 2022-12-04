package main

import (
	"bufio"
	"bytes"
	"fmt"
	"image"
	"io"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/sheik/freetype-go/freetype/truetype"
	"github.com/sheik/xgb/xproto"
	"github.com/sheik/xgbutil"
	"github.com/sheik/xgbutil/ewmh"
	"github.com/sheik/xgbutil/keybind"
	"github.com/sheik/xgbutil/xevent"
	"github.com/sheik/xgbutil/xgraphics"
	"github.com/sheik/xgbutil/xwindow"
)

var (
	// The background color of the canvas.
	bg = xgraphics.BGRA{B: 0xdd, G: 0xff, R: 0xff, A: 0xff}

	// The path to the font used to draw text.
	fontPath     = "/usr/share/fonts/truetype/firacode/FiraCode-Regular.ttf"
	fontPathBold = "/usr/share/fonts/truetype/firacode/FiraCode-SemiBold.ttf"

	// The color of the text.
	fg = xgraphics.BGRA{B: 0x22, G: 0x22, R: 0x22, A: 0xff}

	// The size of the text.
	size = 13.0
)

type Glyph struct {
	X       int
	Y       int
	fg      xgraphics.BGRA
	bg      xgraphics.BGRA
	literal []byte
}

type Terminal struct {
	cursor Cursor
	width  int
	height int
	pty    io.ReadWriter
	input  string

	glyphs    [][]*Glyph
	dirtyRows map[int]bool

	ui UI

	// top and bottom pointers (cursor Y values)
	top int
	bot int
}

type Cursor struct {
	X      int
	Y      int
	width  int
	height int
}

var redraw = false
var needsDraw = true

type XGBGui struct {
	X           *xgbutil.XUtil
	font        *truetype.Font
	fontRegular *truetype.Font
	fontBold    *truetype.Font
	img         *xgraphics.Image
	window      *xwindow.Window
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
		return bg
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

func NewTerminal(inPty io.ReadWriter, ui UI, width, height int) (term *Terminal, err error) {
	term = &Terminal{width: width, height: height, top: 0, bot: height - 1, pty: inPty}

	term.ui = ui

	term.cursor = Cursor{
		width:  0,
		height: 0,
		X:      0,
		Y:      0,
	}

	term.dirtyRows = make(map[int]bool)

	term.glyphs = make([][]*Glyph, term.height+1)
	for i := range term.glyphs {
		term.glyphs[i] = make([]*Glyph, term.width+1)
	}

	fmt.Println(term.cursor.width, term.cursor.height)

	term.ui.CreateWindow(term)

	reader := bufio.NewReader(term.pty)

	// Goroutine that reads from pty
	go func() {
		tokenChan := make(chan Token, 2000)
		NewLexer(reader, tokenChan)
		i := 5000
		for {
			var token Token
			select {
			case <-time.After(time.Duration(i) * time.Microsecond):
				if needsDraw || redraw {
					term.ui.Draw(term)
				}
				continue
			case token = <-tokenChan:
				needsDraw = true
			}

			if *debug {
				if token.Type == TEXT {
					fmt.Printf("%s %v \"%s\"\n", token.Type, []byte(token.Literal), token.Literal)
				} else {
					fmt.Printf("%s %v \"%s\"\n", token.Type, []byte(token.Literal), token.Literal[1:])
				}
			}

			switch token.Type {
			case BAR:
				continue
			case RESET_INITIAL_STATE:
				// reset title
				term.ui.SetWindowTitle("none")
				// reset cursor
				term.cursor.X = 0
				term.cursor.Y = 0
				term.top = 0
				term.bot = term.height - 1
				// reset cols
			case CR:
				term.ui.EraseCursor(term)
				term.cursor.X = 0
			case LF:
				term.ui.EraseCursor(term)
				term.IncreaseY()
				continue
			case BACKSPACE:
				term.ui.EraseCursor(term)
				term.cursor.X -= 1
				if term.cursor.X < 0 {
					term.cursor.X = 0
				}
				continue
			case INSERT_LINE:
				term.ScrollUp()
			case MOVE_TO_COL:
				n := 1
				if len(token.Literal) > 3 {
					n, err = strconv.Atoi(string(token.Literal[2 : len(token.Literal)-1]))
					if err != nil {
						fmt.Println("could not determine n for col move")
					}
				}
				term.cursor.X = n - 1
			case CLEAR_LINE:
				n := 0
				if len(token.Literal) > 3 {
					n, err = strconv.Atoi(string(token.Literal[2 : len(token.Literal)-1]))
					if err != nil {
						fmt.Println("unable to determine n for delete chars")
						continue
					}
				}

				switch n {
				case 0:
					term.ClearRegion(term.cursor.X, term.cursor.Y, term.width-1, term.cursor.Y)
				case 1:
					term.ClearRegion(0, term.cursor.Y, term.cursor.X, term.cursor.Y)
				case 2:
					term.ClearRegion(0, term.cursor.Y, term.width-1, term.cursor.Y)
				}

				redraw = true
				term.dirtyRows[term.cursor.Y] = true
			case COLOR_CODE:

				// cursor <n> forward
				if token.Literal[len(token.Literal)-1] == 'C' {
					n := 1
					if len(token.Literal) > 3 {
						n, err = strconv.Atoi(string(token.Literal[2 : len(token.Literal)-1]))
						if err != nil {
							fmt.Println("could not determine n for cursor move")
						}
					}
					term.cursor.X += n
					if term.cursor.X >= term.width {
						term.cursor.X = term.width - 1
					}
				}

				if token.Literal[len(token.Literal)-1] == '@' {
					n, err := strconv.Atoi(string(token.Literal[2 : len(token.Literal)-1]))
					if err != nil {
						fmt.Println("Could not convert to number:", token.Literal[1:])
					}

					// Move characters after the cursor to the right
					for i := term.width - 1; i > term.cursor.X; i-- {
						if (i - n) < 0 {
							break
						}
						term.glyphs[term.cursor.Y][i] = term.glyphs[term.cursor.Y][i-n]
					}

					// Fill n characters after cursor with blanks
					for i := 0; i < n; i++ {
						term.ui.WriteText(term, (term.cursor.X*term.cursor.width)+i*term.cursor.width, term.cursor.Y*term.cursor.height, "\u2588")
					}

					redraw = true
					term.dirtyRows[term.cursor.Y] = true
				}

				if token.Literal[len(token.Literal)-1] == 'r' {
					// TODO set scroll region
					top, err := strconv.Atoi(strings.Split(string(token.Literal[2:len(token.Literal)-1]), ";")[0])
					if err != nil {
						fmt.Println("could not convert top:", string(token.Literal[1:]))
						top = 0
					}
					bot, err := strconv.Atoi(strings.Split(string(token.Literal[2:len(token.Literal)-1]), ";")[1])
					if err != nil {
						fmt.Println("could not convert bot")
						bot = 0
					}
					term.top = top - 1
					term.bot = bot - 1
					term.cursor.Y = term.top
				}

				/*
					if token.Literal[len(token.Literal)-1] == 'h' {
						// TODO implement set term mode
					}
					if token.Literal[len(token.Literal)-1] == 'l' {
						// TODO implement unset term mode
					}
				*/
				// color codes
				if token.Literal[len(token.Literal)-1] == 'm' {
					args := strings.Split(string(token.Literal[2:len(token.Literal)-1]), ";")

					if len(args) > 0 {
						switch args[0] {
						case "":
							term.ui.SetFont("regular")
							fg = xgraphics.BGRA{B: 0x22, G: 0x22, R: 0x22, A: 0xff}
							bg = xgraphics.BGRA{B: 0xdd, G: 0xff, R: 0xff, A: 0xff}
						case "0":
							term.ui.SetFont("regular")
							fg = xgraphics.BGRA{B: 0x22, G: 0x22, R: 0x22, A: 0xff}
							bg = xgraphics.BGRA{B: 0xdd, G: 0xff, R: 0xff, A: 0xff}
						case "00":
							term.ui.SetFont("regular")
							fg = xgraphics.BGRA{B: 0x22, G: 0x22, R: 0x22, A: 0xff}
							bg = xgraphics.BGRA{B: 0xdd, G: 0xff, R: 0xff, A: 0xff}
						case "1":
							term.ui.SetFont("bold")
						case "01":
							term.ui.SetFont("bold")
						case "7":
							fg = xgraphics.BGRA{B: 0xdd, G: 0xff, R: 0xff, A: 0xff}
							bg = xgraphics.BGRA{B: 0x22, G: 0x22, R: 0x22, A: 0xff}
						case "27":
							fg = xgraphics.BGRA{B: 0x22, G: 0x22, R: 0x22, A: 0xff}
							bg = xgraphics.BGRA{B: 0xdd, G: 0xff, R: 0xff, A: 0xff}
						case "32":
							fg = xgraphics.BGRA{B: 0x00, G: 0xff, R: 0x00, A: 0xff}
						case "34":
							fg = xgraphics.BGRA{B: 0xff, G: 0x00, R: 0x00, A: 0xff}
						case "39":
							fg = xgraphics.BGRA{B: 0x22, G: 0x22, R: 0x22, A: 0xff}
						case "42":
							bg = xgraphics.BGRA{B: 0x00, G: 0xff, R: 0x00, A: 0xff}
						}
					}
					if len(args) > 1 {
						switch args[1] {
						case "32":
							fg = xgraphics.BGRA{B: 0x00, G: 0xff, R: 0x00, A: 0xff}
						case "34":
							fg = xgraphics.BGRA{B: 0xff, G: 0x00, R: 0x00, A: 0xff}
						case "39":
							fg = xgraphics.BGRA{B: 0x22, G: 0x22, R: 0x22, A: 0xff}
						case "42":
							bg = xgraphics.BGRA{B: 0x00, G: 0xff, R: 0x00, A: 0xff}
						}

					}
					if len(args) > 2 {
						fmt.Println("SET BACKGROUND:", args[2])
						switch args[2] {
						case "32":
							fg = xgraphics.BGRA{B: 0x00, G: 0xff, R: 0x00, A: 0xff}
						case "34":
							fg = xgraphics.BGRA{B: 0xff, G: 0x00, R: 0x00, A: 0xff}
						case "39":
							fg = xgraphics.BGRA{B: 0xdd, G: 0xff, R: 0xff, A: 0xff}
						}
					}
				}

				continue
			case OSC:
				parts := strings.Split(string(token.Literal[2:len(token.Literal)-1]), ";")
				switch parts[0] {
				case "0":
					term.ui.SetWindowTitle(parts[1])
				case "10":
					if parts[1] == "?" {
						// SEND CODES
					}
				case "11":
				case "12":
				}
				continue
			case CURSOR_POSITION_REQUEST:
				//				position := fmt.Sprintf("\033%d;%dR", term.cursor.Y, term.cursor.X)
				//				term.pty.WriteString(position)
			case RESET_CURSOR:
				term.ui.EraseCursor(term)
				x := 1
				y := 1
				if bytes.Contains(token.Literal, []byte(";")) {
					y, err = strconv.Atoi(strings.Split(string(token.Literal[2:len(token.Literal)-1]), ";")[0])
					if err != nil {
						fmt.Println("unable to convert y coordinate", []byte(token.Literal), string(token.Literal[1:]))
						continue
					}
					x, err = strconv.Atoi(strings.Split(string(token.Literal[2:len(token.Literal)-1]), ";")[1])
					if err != nil {
						fmt.Println("unable to convert x coordinate")
						continue
					}
				}
				if x < 0 {
					x = 0
				}
				if y < 0 {
					y = 0
				}

				term.cursor.X = x - 1
				term.cursor.Y = term.top + y - 1
			case DELETE_LINES:
				term.ScrollUp()
			case DELETE_CHARS:
				// TODO this function needs to be rewritten!
				n := 1
				n, err := strconv.Atoi(string(token.Literal[2 : len(token.Literal)-1]))
				if err != nil {
					fmt.Println("unable to determine n for delete chars")
					continue
				}
				for i := term.cursor.X; i < term.width-1; i++ {
					if i+n > term.width-1 {
						term.glyphs[term.cursor.Y][i] = nil
						continue
					}
					term.glyphs[term.cursor.Y][i] = term.glyphs[term.cursor.Y][i+n]
				}
				for i := term.width - n; i < term.width; i++ {
					term.glyphs[term.cursor.Y][i] = nil
				}
				redraw = true
				term.dirtyRows[term.cursor.Y] = true
			case CURSOR_ROW:
				y := 1
				y, err := strconv.Atoi(string(token.Literal[2 : len(token.Literal)-1]))
				if err != nil {
					fmt.Println("unable to convert y coordinate for CURSOR_ROW")
				}
				y -= 1
				if y < 0 {
					term.cursor.Y = 0
					term.ScrollUp()
				} else {
					term.cursor.Y = y
				}
			case TEXT:
				if token.Literal[0] == '\t' {
					token.Literal = []byte("    ")
				}

				if token.Literal[0] > 128 || token.Literal[0] == 0x07 {
					continue
				}

				// TODO is wrapping a term mode?
				if term.cursor.X >= term.width {
					term.cursor.X = 0
					term.IncreaseY()
					continue
				}

				if term.cursor.Y >= term.height {
					term.cursor.Y = term.height - 1
				}

				if term.cursor.X >= term.width {
					term.cursor.X = term.width - 1
				}

				term.glyphs[term.cursor.Y][term.cursor.X] = &Glyph{
					X:       term.cursor.X,
					Y:       term.cursor.Y,
					fg:      fg,
					bg:      bg,
					literal: token.Literal,
				}

				term.ui.WriteText(term, term.cursor.X, term.cursor.Y, string(token.Literal))

				term.cursor.X += len(token.Literal)
				//				redraw = true
				//				term.dirtyRows[term.cursor.Y] = true
			case CLEAR:
				term.ui.Clear(term)
				for i := 0; i < term.height; i++ {
					for j := 0; j < term.width; j++ {
						term.glyphs[i][j] = nil
					}
				}

			}
		}
	}()
	return term, nil
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

func (term *Terminal) ScrollUp() {
	for i := term.bot; i > term.top; i-- {
		term.glyphs[i] = term.glyphs[i-1]
	}
	term.glyphs[term.top] = make([]*Glyph, term.width)
	for i := 0; i < term.height; i++ {
		term.dirtyRows[i] = true
	}
	redraw = true
}

func (term *Terminal) Scroll() {
	for i := term.top; i < term.bot; i++ {
		term.glyphs[i] = term.glyphs[i+1]
	}
	term.glyphs[term.bot] = make([]*Glyph, term.width)
	for i := 0; i < term.height; i++ {
		term.dirtyRows[i] = true
	}
	redraw = true
}

func (term *Terminal) IncreaseY() {
	if term.top+term.cursor.Y+1 > term.bot {
		term.Scroll()
	} else {
		term.cursor.Y += 1
	}
}

func (term *Terminal) ClearRegion(x1, y1, x2, y2 int) {
	y1 = y1 + term.top
	y2 = y2 + term.top

	if x1 > x2 {
		x1, x2 = x2, x1
	}
	if y1 > y2 {
		y1, y2 = y2, y1
	}
	if x2 > term.width-1 {
		x2 = term.width - 1
	}

	for i := y1; i <= y2; i++ {
		for j := x1; j <= x2; j++ {
			term.glyphs[i][j] = &Glyph{
				X:       j,
				Y:       i,
				fg:      fg,
				bg:      bg,
				literal: []byte(" "),
			}
		}
	}
	redraw = true
	for i := y1; i < y2; i++ {
		term.dirtyRows[i] = true
	}
}

type UI interface {
	SetWindowTitle(string)
	CreateWindow(term *Terminal) error
	GetCursorSize() (int, int)
	DrawCursor(*Terminal)
	EraseCursor(*Terminal)
	Draw(term *Terminal)
	WriteText(*Terminal, int, int, string)
	Clear(*Terminal)
	SetFont(string)
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
			return bg
		})
		box.XDraw()
	}
}

func (gui *XGBGui) WriteText(term *Terminal, x int, y int, text string) {
	rect := image.Rect(x*term.cursor.width, y*term.cursor.height, x*term.cursor.width+term.cursor.width, y*term.cursor.height+term.cursor.height)
	box, ok := gui.img.SubImage(rect).(*xgraphics.Image)
	if ok {
		box.For(func(x, y int) xgraphics.BGRA {
			return bg
		})
		box.XDraw()
	}

	_, _, err := gui.img.Text(x*term.cursor.width, y*term.cursor.height, fg, size, gui.font, text)

	if err != nil {
		log.Fatal(err)
	}
}

func (x *XGBGui) SetWindowTitle(title string) {
	ewmh.WmNameSet(x.X, x.window.Id, title)
}

func (x *XGBGui) DrawCursor(term *Terminal) {
	rect := image.Rect(term.cursor.X*term.cursor.width, term.cursor.Y*term.cursor.height, (term.cursor.X*term.cursor.width)+term.cursor.width, (term.cursor.Y*term.cursor.height)+term.cursor.height)
	box, ok := x.img.SubImage(rect).(*xgraphics.Image)
	if ok {
		box.For(func(x, y int) xgraphics.BGRA {
			x = x / term.cursor.width
			y = y / term.cursor.height
			if term.glyphs[y][x] != nil {
				return term.glyphs[y][x].fg
			} else {
				return fg
			}
		})
		box.XDraw()
		needsDraw = true
	}
	if term.cursor.Y > term.height-1 {
		return
	}

	if term.cursor.X > term.width-1 {
		return
	}

	g := term.glyphs[term.cursor.Y][term.cursor.X]
	if g != nil {
		_, _, err := x.img.Text(term.cursor.X*term.cursor.width, term.cursor.Y*term.cursor.height, g.bg, size, x.font, string(g.literal))
		if err != nil {
			panic(err)
		}
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
				return term.glyphs[y][x].bg
			} else {
				return bg
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

func (x *XGBGui) Draw(term *Terminal) {
	if redraw {
		if len(term.dirtyRows) == 0 {
			for i := 0; i < term.height; i++ {
				term.dirtyRows[i] = true
			}
		}

		for i, _ := range term.dirtyRows {
			rect := image.Rect(0, i*term.cursor.height, term.width*term.cursor.width, i*term.cursor.height+term.cursor.height)
			box, ok := x.img.SubImage(rect).(*xgraphics.Image)
			if ok {
				box.For(func(x, y int) xgraphics.BGRA {
					x = x / term.cursor.width
					y = y / term.cursor.height
					if term.glyphs[y][x] != nil {
						return term.glyphs[y][x].bg
					} else {
						return bg
					}
				})
				box.XDraw()
			}
			for j := 0; j < term.width; j++ {
				g := term.glyphs[i][j]
				if g != nil {
					_, _, err := x.img.Text(j*term.cursor.width, i*term.cursor.height, g.fg, size, x.font, string(g.literal))
					if err != nil {
						log.Fatal(err)
					}
				} else {
					_, _, err := x.img.Text(j*term.cursor.width, i*term.cursor.height, bg, size, x.font, "\u2588")
					if err != nil {
						log.Fatal(err)
					}
				}
			}
		}
		term.dirtyRows = make(map[int]bool)
		redraw = false
	}
	if needsDraw {
		term.ui.DrawCursor(term)
		x.img.XDraw()
		x.img.XDraw()
		x.img.XPaint(x.window.Id)
		needsDraw = false
	}
}
