package main

import (
	"bufio"
	"github.com/sheik/freetype-go/freetype/truetype"
	"github.com/sheik/xgb/xproto"
	"github.com/sheik/xgbutil"
	"github.com/sheik/xgbutil/keybind"
	"github.com/sheik/xgbutil/xevent"
	"github.com/sheik/xgbutil/xgraphics"
	"github.com/sheik/xgbutil/xwindow"
	"image"
	"io"
	"log"
	"os"
	"os/exec"

	"github.com/creack/pty"
)

const (
	MaxBufferSize = 128
)

var (
	// The geometry of the canvas to draw text on.
	canvasWidth, canvasHeight = 800, 600

	// The background color of the canvas.
	bg = xgraphics.BGRA{B: 0xdd, G: 0xff, R: 0xff, A: 0xff}

	// The path to the font used to draw text.
	fontPath = "/home/jeff/.fonts/FiraCode-Regular.ttf"

	// The color of the text.
	fg = xgraphics.BGRA{B: 0x22, G: 0x22, R: 0x22, A: 0xff}

	// The size of the text.
	size = 14.0

	// The text to draw.
	msg = "user@host$ "
)

type Terminal struct {
	cursor Cursor
	width  int
	height int
	pty    *os.File
	input  string

	X      *xgbutil.XUtil
	font   *truetype.Font
	img    *xgraphics.Image
	window *xwindow.Window
}

type Cursor struct {
	X      int
	Y      int
	width  int
	height int
}

func NewTerminal() (terminal *Terminal, err error) {
	terminal = &Terminal{width: 800, height: 600}

	os.Setenv("TERM", "dumb")
	c := exec.Command("/bin/bash")
	terminal.pty, err = pty.Start(c)
	if err != nil {
		return
	}

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
	terminal.font, err = xgraphics.ParseFont(fontReader)
	if err != nil {
		log.Fatal(err)
	}

	// Create some canvas.
	terminal.img = xgraphics.New(terminal.X, image.Rect(0, 0, terminal.width, terminal.height))
	terminal.img.For(func(x, y int) xgraphics.BGRA {
		return bg
	})

	terminal.cursor = Cursor{
		X:      10,
		Y:      10,
		width:  0,
		height: 0,
	}

	// set terminal width/height to full block
	terminal.cursor.width, terminal.cursor.height = xgraphics.Extents(terminal.font, size, "\u2588")

	// Now show the image in its own window.
	terminal.window = terminal.img.XShowExtra("gt", true)

	terminal.window.Listen(xproto.EventMaskKeyPress, xproto.EventMaskKeyRelease)

	xevent.KeyPressFun(terminal.KeyPressCallback).Connect(terminal.X, terminal.window.Id)

	// Goroutine that reads from pty
	go func() {
		for {
			r, _, err := reader.ReadRune()

			if err != nil {
				if err == io.EOF {
					return
				}
				os.Exit(0)
			}
			if r == '\r' {
				continue
			}
			if r == '\n' {
				terminal.cursor.X = 10
				terminal.cursor.Y += terminal.cursor.height
				continue
			}
			_, _, err = terminal.img.Text(terminal.cursor.X, terminal.cursor.Y, fg, size, terminal.font, string(r))
			if err != nil {
				log.Fatal(err)
			}

			bounds := image.Rect(terminal.cursor.X, terminal.cursor.Y, terminal.cursor.X+terminal.cursor.width, terminal.cursor.Y+terminal.cursor.height)
			subimg := terminal.img.SubImage(bounds)
			if subimg != nil {
				subimg.(*xgraphics.Image).XDraw()
				terminal.img.XPaint(terminal.window.Id)
			}

			terminal.cursor.X += terminal.cursor.width
		}
	}()
	return terminal, nil
}

func (term *Terminal) KeyPressCallback(X *xgbutil.XUtil, e xevent.KeyPressEvent) {
	var err error
	modStr := keybind.ModifierString(e.State)
	keyStr := keybind.LookupString(X, e.State, e.Detail)

	if keybind.KeyMatch(X, "Backspace", e.State, e.Detail) {
		_, _, err = term.img.Text(term.cursor.X-term.cursor.width, 10, bg, size, term.font, "\u2588")
		if err != nil {
			log.Fatal(err)
		}

		// Now repaint on the region that we drew text on. Then update the screen.
		term.img.XDraw()
		term.img.XPaint(term.window.Id)
		term.cursor.X -= term.cursor.width
		if len(term.input) >= 1 {
			term.input = term.input[:len(term.input)-1]
		}
		return
	}

	if keybind.KeyMatch(X, "Escape", e.State, e.Detail) {
		if e.State&xproto.ModMaskControl > 0 {
			log.Println("Control-Escape detected. Quitting...")
			xevent.Quit(X)
		}
	}

	if keybind.KeyMatch(X, "Return", e.State, e.Detail) {
		//		term.cursor.Y += 10
		//		term.cursor.X = 10
		term.pty.Write([]byte{'\n'})
		return
	}

	if len(modStr) > 0 {
		log.Printf("Key: %s-%s\n", modStr, keyStr)
	} else {
		term.input += keyStr
		term.pty.WriteString(keyStr)
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
