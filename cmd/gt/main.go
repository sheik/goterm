package main

import (
	"bufio"
	"github.com/creack/pty"
	"github.com/sheik/freetype-go/freetype/truetype"
	"github.com/sheik/xgb/xproto"
	"github.com/sheik/xgbutil"
	"github.com/sheik/xgbutil/ewmh"
	"github.com/sheik/xgbutil/keybind"
	"github.com/sheik/xgbutil/xevent"
	"github.com/sheik/xgbutil/xgraphics"
	"github.com/sheik/xgbutil/xwindow"
	"image"
	"log"
	"os"
	"os/exec"
	"strings"
	"time"
)

const (
	MaxBufferSize = 128
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
	size    float64
	font    *truetype.Font
	literal string
}

type Terminal struct {
	cursor Cursor
	width  int
	height int
	pty    *os.File
	input  string

	glyphs [][]*Glyph

	X           *xgbutil.XUtil
	font        *truetype.Font
	fontRegular *truetype.Font
	fontBold    *truetype.Font
	img         *xgraphics.Image
	window      *xwindow.Window
}

type Cursor struct {
	RealX  int
	RealY  int
	X      int
	Y      int
	width  int
	height int
}

var redraw = false

func NewTerminal() (terminal *Terminal, err error) {
	terminal = &Terminal{width: 120, height: 34}

	terminal.glyphs = make([][]*Glyph, terminal.height)
	for i := range terminal.glyphs {
		terminal.glyphs[i] = make([]*Glyph, terminal.width)
	}

	os.Setenv("TERM", "xterm-256color")
	c := exec.Command("/bin/bash")

	terminal.pty, err = pty.Start(c)
	if err != nil {
		return
	}

	pty.Setsize(terminal.pty, &pty.Winsize{
		Rows: 34,
		Cols: 120,
		X:    0,
		Y:    0,
	})

	reader := bufio.NewReader(terminal.pty)

	terminal.X, err = xgbutil.NewConn()
	if err != nil {
		return nil, err
	}

	keybind.Initialize(terminal.X)

	// Load some font. You may need to change the path depending upon your
	// system configuration.
	fontReader, err := os.Open(fontPath)
	if err != nil {
		log.Fatal(err)
	}

	// Now parse the font.
	terminal.fontRegular, err = xgraphics.ParseFont(fontReader)
	if err != nil {
		log.Fatal(err)
	}
	fontReader.Close()

	fontReader, err = os.Open(fontPathBold)
	if err != nil {
		log.Fatal(err)
	}
	terminal.font = terminal.fontRegular

	// Now parse the font.
	terminal.fontBold, err = xgraphics.ParseFont(fontReader)
	if err != nil {
		log.Fatal(err)
	}
	fontReader.Close()

	terminal.cursor = Cursor{
		RealX:  0,
		RealY:  0,
		width:  0,
		height: 0,
		X:      0,
		Y:      0,
	}

	// set terminal width/height to full block
	terminal.cursor.width, terminal.cursor.height = xgraphics.Extents(terminal.font, size, "\u2588")

	// Create some canvas.
	terminal.img = xgraphics.New(terminal.X, image.Rect(0, 0, terminal.width*terminal.cursor.width, terminal.height*terminal.cursor.height))
	terminal.img.For(func(x, y int) xgraphics.BGRA {
		return bg
	})

	// Now show the image in its own window.
	terminal.window = terminal.img.XShowExtra("gt", true)

	terminal.window.Listen(xproto.EventMaskKeyPress, xproto.EventMaskKeyRelease)

	xevent.KeyPressFun(terminal.KeyPressCallback).Connect(terminal.X, terminal.window.Id)

	// Goroutine that reads from pty
	go func() {
		tokenChan := make(chan *Token)
		NewLexer(reader, tokenChan)
		needsDraw := true
		for {
			var token *Token
			select {
			case token = <-tokenChan:
				needsDraw = true
			case <-time.After(10 * time.Millisecond):
				if redraw {
					rect := image.Rect(0, 0, terminal.width*terminal.cursor.width, terminal.height*terminal.cursor.height)
					box := terminal.img.SubImage(rect).(*xgraphics.Image)
					box.For(func(x, y int) xgraphics.BGRA {
						return bg
					})
					box.XDraw()
					for i := 0; i < terminal.height; i++ {
						for j := 0; j < terminal.width; j++ {
							g := terminal.glyphs[i][j]
							if g != nil {
								_, _, err := terminal.img.Text(j*terminal.cursor.width, i*terminal.cursor.height, g.fg, g.size, g.font, g.literal)
								if err != nil {
									log.Fatal(err)
								}
							}
						}
					}
					redraw = false
				}
				if needsDraw {
					terminal.img.XDraw()
					terminal.img.XPaint(terminal.window.Id)
					terminal.X.Sync()
					needsDraw = false
				}
				continue
			}

			switch token.Type {
			case BAR:
				rect := image.Rect(terminal.cursor.RealX, terminal.cursor.RealY, terminal.width*terminal.cursor.width, terminal.cursor.RealY+terminal.cursor.height)
				box, ok := terminal.img.SubImage(rect).(*xgraphics.Image)
				if ok {
					box.For(func(x, y int) xgraphics.BGRA {
						return bg
					})
					box.XDraw()
				}
				continue
			case CRLF:
				terminal.cursor.X = 0
				terminal.cursor.RealX = 0
				terminal.IncreaseY()
				continue
			case BACKSPACE:
				terminal.cursor.X -= 1
				terminal.cursor.RealX -= terminal.cursor.width
				rect := image.Rect(terminal.cursor.RealX, terminal.cursor.RealY, terminal.cursor.RealX+terminal.cursor.width, terminal.cursor.RealY+terminal.cursor.height)
				box, ok := terminal.img.SubImage(rect).(*xgraphics.Image)
				if ok {
					box.For(func(x, y int) xgraphics.BGRA {
						return bg
					})
					box.XDraw()
				}
				continue
			case COLOR_CODE:
				if token.Literal[len(token.Literal)-1] == 'm' {
					colorString := strings.Split(token.Literal[2:len(token.Literal)-1], ";")
					for _, color := range colorString {
						switch color {
						case "0":
							fg = xgraphics.BGRA{B: 0x22, G: 0x22, R: 0x22, A: 0xff}
							terminal.font = terminal.fontRegular
						case "00":
							fg = xgraphics.BGRA{B: 0x22, G: 0x22, R: 0x22, A: 0xff}
							terminal.font = terminal.fontRegular
						case "1":
							terminal.font = terminal.fontBold
						case "01":
							terminal.font = terminal.fontBold
						case "32":
							fg = xgraphics.BGRA{B: 0x00, G: 0xff, R: 0x00, A: 0xff}
						case "34":
							fg = xgraphics.BGRA{B: 0xff, G: 0x00, R: 0x00, A: 0xff}
						case "39":
							fg = xgraphics.BGRA{B: 0x22, G: 0x22, R: 0x22, A: 0xff}
							terminal.font = terminal.fontRegular
						}
					}
				}

				continue
			case SET_TITLE:
				title := token.Literal[4 : len(token.Literal)-1]
				ewmh.WmNameSet(terminal.X, terminal.window.Id, title)
				continue
			case RESET_CURSOR:
				terminal.cursor.RealX = 0
				terminal.cursor.RealY = 0
				terminal.cursor.X = 0
				terminal.cursor.Y = 0
			case TEXT:
				if terminal.cursor.X >= terminal.width {
					terminal.cursor.X = 0
					terminal.cursor.RealX = 0
					terminal.IncreaseY()
					continue
				}
				rect := image.Rect(terminal.cursor.RealX, terminal.cursor.RealY, terminal.cursor.RealX+terminal.cursor.width, terminal.cursor.RealY+terminal.cursor.height)
				box, ok := terminal.img.SubImage(rect).(*xgraphics.Image)
				if ok {
					box.For(func(x, y int) xgraphics.BGRA {
						return bg
					})
					box.XDraw()
				}

				terminal.glyphs[terminal.cursor.Y][terminal.cursor.X] = &Glyph{
					X:       terminal.cursor.X,
					Y:       terminal.cursor.Y,
					fg:      fg,
					size:    size,
					font:    terminal.font,
					literal: token.Literal,
				}

				_, _, err = terminal.img.Text(terminal.cursor.RealX, terminal.cursor.RealY, fg, size, terminal.font, token.Literal)

				if err != nil {
					log.Fatal(err)
				}
				terminal.cursor.RealX += terminal.cursor.width
				terminal.cursor.X += 1
			case CLEAR:
				rect := image.Rect(0, 0, terminal.width*terminal.cursor.width, terminal.height*terminal.cursor.height)
				box := terminal.img.SubImage(rect).(*xgraphics.Image)
				box.For(func(x, y int) xgraphics.BGRA {
					return bg
				})
				box.XDraw()
				for i := 0; i < terminal.height; i++ {
					for j := 0; j < terminal.width; j++ {
						terminal.glyphs[i][j] = nil
					}
				}

			}
		}
	}()
	return terminal, nil
}

func (term *Terminal) KeyPressCallback(X *xgbutil.XUtil, e xevent.KeyPressEvent) {
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

	if len(modStr) > 0 {
		if strings.Contains(modStr, "shift") {
			reply, _ := xproto.GetKeyboardMapping(term.X.Conn(), e.Detail, 1).Reply()
			chr := string(reply.Keysyms[1])
			term.pty.WriteString(chr)
		}
	} else {
		// only write it if it's a character (i.e. not Shift_L, etc)
		if len(keyStr) == 1 {
			term.pty.WriteString(keyStr)
		}
	}
}

func (terminal *Terminal) Scroll() {

	for i := 0; i < terminal.height-1; i++ {
		terminal.glyphs[i] = terminal.glyphs[i+1]
	}
	terminal.glyphs[terminal.height-1] = make([]*Glyph, terminal.width)
	redraw = true

}

func (terminal *Terminal) IncreaseY() {
	if terminal.cursor.Y+1 >= terminal.height {
		// shift screen up
		terminal.Scroll()
	} else {
		terminal.cursor.Y += 1
		terminal.cursor.RealY += terminal.cursor.height
	}
}

func main() {
	term, err := NewTerminal()
	if err != nil {
		log.Fatal(err)
	}
	// All we really need to do is block, which could be achieved using
	// 'select{}'. Invoking the main event loop however, will emit error
	// message if anything went seriously wrong above.
	xevent.Main(term.X)
}
