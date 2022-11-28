package main

import (
	"bufio"
	"bytes"
	"fmt"
	"github.com/creack/pty"
	"github.com/sheik/freetype-go/freetype/truetype"
	"github.com/sheik/xgb/glx"
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
	"regexp"
	"strings"
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
	err = glx.Init(terminal.X.Conn())
	if err != nil {
		return nil, err
	}

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
		X:      10,
		Y:      10,
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
		for {
			fmt.Println("syncing")
			terminal.img.XDraw()
			terminal.img.XPaint(terminal.window.Id)
			terminal.X.Sync()
			buf := make([]byte, 1024)
			n, err := reader.Read(buf)
			if n == 0 {
				continue
			}
			if err != nil {
				if err == io.EOF {
					return
				}
				os.Exit(0)
			}
			buf = buf[:n]

			r := string(buf)
			if strings.Contains(r, "\033[2J") || strings.Contains(r, "\033[H") {
				rect := image.Rect(0, 0, terminal.width*terminal.cursor.width, terminal.height*terminal.cursor.height)
				box := terminal.img.SubImage(rect).(*xgraphics.Image)
				box.For(func(x, y int) xgraphics.BGRA {
					return bg
				})
				box.XDraw()
				terminal.img.XDraw()
				terminal.img.XPaint(terminal.window.Id)
				terminal.X.Sync()
				terminal.cursor.X = 10
				terminal.cursor.Y = 10

				fmt.Println("clear screen!")
			}
			re := regexp.MustCompile(`\033\(B`)
			buf = re.ReplaceAll(buf, []byte{})
			re = regexp.MustCompile(`\033\[.[\(\)0-9;mhHJKABCDEF#l]*`)
			buf = re.ReplaceAll(buf, []byte{})
			r = string(buf)

			if bytes.Contains(buf, []byte{0x08}) {
				_, _, err = terminal.img.Text(terminal.cursor.X, terminal.cursor.Y, bg, size, terminal.font, "\u2588")
				if err != nil {
					log.Fatal(err)
				}

				// Now repaint on the region that we drew text on. Then update the screen.
				err := terminal.img.XDrawChecked()
				if err != nil {
					log.Println(err)
				}
				terminal.img.XPaint(terminal.window.Id)
				terminal.X.Sync()
				continue
			}

			lines := strings.Split(r, "\n")
			for n, line := range lines {
				fmt.Println([]byte(line))
				_, _, err = terminal.img.Text(terminal.cursor.X, terminal.cursor.Y, fg, size, terminal.font, line)
				if err != nil {
					log.Fatal(err)
				}

				w, h := xgraphics.Extents(terminal.font, size, line)

				bounds := image.Rect(terminal.cursor.X, terminal.cursor.Y, terminal.cursor.X+w, terminal.cursor.Y+h)
				subimg := terminal.img.SubImage(bounds)
				if subimg != nil {
					//					subimg.(*xgraphics.Image).XDraw()
					fmt.Println("calling draw")
					terminal.img.XDraw()
					terminal.img.XPaint(terminal.window.Id)
					terminal.X.Sync()
				}
				if len(lines) != 0 && strings.Contains(r, "\n") && n != len(lines)-1 {
					fmt.Println("resetting line")
					terminal.cursor.X = 10
					terminal.cursor.Y += terminal.cursor.height
				} else {
					terminal.cursor.X += w
				}
			}
			terminal.X.Sync()
		}
	}()
	return terminal, nil
}

func (term *Terminal) KeyPressCallback(X *xgbutil.XUtil, e xevent.KeyPressEvent) {
	modStr := keybind.ModifierString(e.State)
	keyStr := keybind.LookupString(X, e.State, e.Detail)

	if keybind.KeyMatch(X, "Backspace", e.State, e.Detail) {
		term.pty.Write([]byte{0x08})
		term.cursor.X -= term.cursor.width
		return
	}

	if keybind.KeyMatch(X, "Escape", e.State, e.Detail) {
		if e.State&xproto.ModMaskControl > 0 {
			log.Println("Control-Escape detected. Quitting...")
			xevent.Quit(X)
		}
	}

	if keybind.KeyMatch(X, "Return", e.State, e.Detail) {
		//term.cursor.Y += term.cursor.height
		//term.cursor.X = 10
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
