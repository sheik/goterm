package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/sheik/xgbutil/xgraphics"
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
					term.Draw()
					term.ui.UpdateDisplay(term)
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
						term.ui.DrawRect(term,
							false,
							bg,
							term.cursor.X*term.cursor.width+i*term.cursor.width,
							term.cursor.Y*term.cursor.height,
							term.cursor.X*term.cursor.width+i*term.cursor.width+term.cursor.width,
							term.cursor.Y*term.cursor.height+term.cursor.height,
						)
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

				term.ui.WriteText(term, term.cursor.X, term.cursor.Y, fg, bg, string(token.Literal))

				term.cursor.X += len(token.Literal)
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
			term.glyphs[i][j] = nil
		}
	}
	redraw = true
	for i := y1; i < y2; i++ {
		term.dirtyRows[i] = true
	}
}

func (term *Terminal) Draw() {
	if redraw {
		if len(term.dirtyRows) == 0 {
			for i := 0; i < term.height; i++ {
				term.dirtyRows[i] = true
			}
		}

		for i := range term.dirtyRows {
			term.ui.DrawRect(term, true, bg, 0, i*term.cursor.height, term.width*term.cursor.width, i*term.cursor.height+term.cursor.height)
			for j := 0; j < term.width; j++ {
				g := term.glyphs[i][j]
				if g != nil {
					term.ui.WriteText(term, j, i, g.fg, g.bg, string(g.literal))
				} else {
					term.ui.DrawRect(term, false, bg, j*term.cursor.width, i*term.cursor.height, j*term.cursor.width+term.cursor.width, i*term.cursor.height+term.cursor.height)
				}
			}
		}
		term.dirtyRows = make(map[int]bool)
		redraw = false
	}
}
