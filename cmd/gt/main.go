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
	"time"
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
	fontPath = "/usr/share/fonts/truetype/firacode/FiraCode-Regular.ttf"

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
	terminal = &Terminal{width: 120, height: 34}

	os.Setenv("TERM", "xterm-256color")
	os.Setenv("PS1", "bash$ ")
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

	terminal.cursor = Cursor{
		X:      0,
		Y:      0,
		width:  0,
		height: 0,
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
				rect := image.Rect(terminal.cursor.X, terminal.cursor.Y, terminal.width*terminal.cursor.width, terminal.cursor.Y+terminal.cursor.height)
				box := terminal.img.SubImage(rect).(*xgraphics.Image)
				box.For(func(x, y int) xgraphics.BGRA {
					return bg
				})
				box.XDraw()
				continue
			case CRLF:
				terminal.cursor.X = 0
				terminal.cursor.Y += terminal.cursor.height
				continue
			case BACKSPACE:
				terminal.cursor.X -= terminal.cursor.width
				rect := image.Rect(terminal.cursor.X, terminal.cursor.Y, terminal.cursor.X+terminal.cursor.width, terminal.cursor.Y+terminal.cursor.height)
				box, ok := terminal.img.SubImage(rect).(*xgraphics.Image)
				if ok {
					box.For(func(x, y int) xgraphics.BGRA {
						return bg
					})
					box.XDraw()
				}
				continue
			case COLOR_CODE:
				continue
			case SET_TITLE:
				title := token.Literal[4 : len(token.Literal)-1]
				ewmh.WmNameSet(terminal.X, terminal.window.Id, title)
				continue
			case RESET_CURSOR:
				terminal.cursor.X = 0
				terminal.cursor.Y = 0
			case TEXT:
				rect := image.Rect(terminal.cursor.X, terminal.cursor.Y, terminal.cursor.X+terminal.cursor.width, terminal.cursor.Y+terminal.cursor.height)
				box, ok := terminal.img.SubImage(rect).(*xgraphics.Image)
				if ok {
					box.For(func(x, y int) xgraphics.BGRA {
						return bg
					})
					box.XDraw()
				}
				_, _, err = terminal.img.Text(terminal.cursor.X, terminal.cursor.Y, fg, size, terminal.font, token.Literal)
				if err != nil {
					log.Fatal(err)
				}
				terminal.cursor.X += terminal.cursor.width
			case CLEAR:
				rect := image.Rect(0, 0, terminal.width*terminal.cursor.width, terminal.height*terminal.cursor.height)
				box := terminal.img.SubImage(rect).(*xgraphics.Image)
				box.For(func(x, y int) xgraphics.BGRA {
					return bg
				})

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
		log.Printf("Key: %s-%s\n", modStr, keyStr)
	} else {
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
