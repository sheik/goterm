package main

import (
	"bufio"
	"io"
	"log"
)

type Lexer struct {
	reader    *bufio.Reader
	tokenChan chan *Token
	char      byte
	peek      byte
}

func NewLexer(reader *bufio.Reader, tokenChan chan *Token) *Lexer {
	lexer := &Lexer{reader: reader, tokenChan: tokenChan}
	go lexer.Token()
	return lexer
}

type Token struct {
	Type    TokenType
	Literal string
}

type TokenType string

const (
	TEXT         TokenType = "TEXT"
	CLEAR_SCREEN TokenType = "CLEAR_SCREEN"
	BAR          TokenType = "BAR"
	COLOR_CODE   TokenType = "COLOR_CODE"
	CRLF         TokenType = "CRLF"
	CR           TokenType = "CR"
	LF           TokenType = "LF"
	RESET_CURSOR TokenType = "RESET_CURSOR"
	CLEAR        TokenType = "CLEAR"
	BACKSPACE    TokenType = "BACKSPACE"
	SET_TITLE    TokenType = "SET_TITLE"
	CURSOR_ROW   TokenType = "CURSOR_ROW"
)

type State string

const (
	INITIAL         State = "INITIAL"
	ESCAPE_SEQUENCE State = "ESCAPE_SEQUENCE"
	ANSI_SEQUENCE   State = "ANSI_SEQUENCE"
	EPSON_SEQUENCE  State = "EPSON_SEQUENCE"
	IN_TEXT         State = "IN_TEXT"
	IN_NEWLINE      State = "IN_NEWLINE"
	TITLE_SET       State = "TITLE_SET"
)

var state = INITIAL
var prevState = INITIAL

func (lexer *Lexer) ReadChar() {
	r, err := lexer.reader.ReadByte()
	if err != nil {
		if err == io.EOF {
			close(lexer.tokenChan)
		}
		log.Fatal(err)
	}
	lexer.char = r
}

func (lexer *Lexer) Token() {
	literal := ""
	i := 0
	for {
		lexer.ReadChar()
		i += 1
		literal += string(lexer.char)

		/*
			if lexer.char != '\033' && lexer.char != 0x08 && lexer.char != '[' {
				fmt.Println(string(lexer.char))
			}

		*/

		switch state {
		case INITIAL:
			if lexer.char == '\033' {
				prevState = state
				state = ESCAPE_SEQUENCE
			} else {
				prevState = state
				state = INITIAL
				lexer.tokenChan <- &Token{Type: TEXT, Literal: literal}
				literal = ""
			}
		case ESCAPE_SEQUENCE:
			if lexer.char == '(' {
				prevState = state
				state = EPSON_SEQUENCE
			}
			if lexer.char == '[' {
				prevState = state
				state = ANSI_SEQUENCE
			}
			if lexer.char == ']' {
				prevState = state
				state = TITLE_SET
			}
			if lexer.char == '=' || lexer.char == '>' {
				literal = ""
				state = INITIAL
			}
		case ANSI_SEQUENCE:
			if lexer.char == 'H' {
				prevState = state
				state = IN_TEXT
				lexer.tokenChan <- &Token{Type: RESET_CURSOR, Literal: literal}
				literal = ""
			}
			if lexer.char == 'J' {
				prevState = state
				state = IN_TEXT
				lexer.tokenChan <- &Token{Type: CLEAR, Literal: literal}
				literal = ""
			}

			// move to row
			if lexer.char == 'd' {
				prevState = state
				state = IN_TEXT
				lexer.tokenChan <- &Token{Type: CURSOR_ROW, Literal: literal}
				literal = ""
			}

			if lexer.char == 'm' || lexer.char == 'l' || lexer.char == 'h' || lexer.char == 'K' || lexer.char == 'f' || lexer.char == '@' || lexer.char == 'C' || lexer.char == 't' || lexer.char == 'r' || lexer.char == 'G' {
				prevState = state
				state = IN_TEXT
				lexer.tokenChan <- &Token{Type: COLOR_CODE, Literal: literal}
				literal = ""
			}
		case EPSON_SEQUENCE:
			if lexer.char == 'B' {
				prevState = state
				state = IN_TEXT
				lexer.tokenChan <- &Token{Type: BAR, Literal: literal}
				literal = ""
			}
		case TITLE_SET:
			if lexer.char == '\a' {
				prevState = state
				state = IN_TEXT
				lexer.tokenChan <- &Token{Type: SET_TITLE, Literal: literal}
				literal = ""
			}
		case IN_TEXT:
			if lexer.char == '\r' {
				lexer.tokenChan <- &Token{Type: CR, Literal: string(lexer.char)}
				literal = ""
			} else if lexer.char == '\n' {
				lexer.tokenChan <- &Token{Type: LF, Literal: string(lexer.char)}
				literal = ""
			} else if lexer.char == '\033' {
				prevState = state
				state = ESCAPE_SEQUENCE
			} else if lexer.char == 0x08 {
				lexer.tokenChan <- &Token{Type: BACKSPACE, Literal: literal}
				literal = ""
			} else {
				lexer.tokenChan <- &Token{Type: TEXT, Literal: literal}
				literal = ""
			}
		}
	}
}
