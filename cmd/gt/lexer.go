package main

import (
	"bufio"
	"io"
	"log"
)

type Lexer struct {
	reader    *bufio.Reader
	tokenChan chan *Token
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
	RESET_CURSOR TokenType = "RESET_CURSOR"
	CLEAR        TokenType = "CLEAR"
)

type State string

const (
	INITIAL         State = "INITIAL"
	ESCAPE_SEQUENCE State = "ESCAPE_SEQUENCE"
	ANSI_SEQUENCE   State = "ANSI_SEQUENCE"
	EPSON_SEQUENCE  State = "EPSON_SEQUENCE"
	IN_TEXT         State = "IN_TEXT"
	IN_NEWLINE      State = "IN_NEWLINE"
)

var state = INITIAL
var prevState = INITIAL

func (lexer *Lexer) Token() {
	literal := ""
	for {
		r, err := lexer.reader.ReadByte()
		if err != nil {
			if err == io.EOF {
				close(lexer.tokenChan)
			}
			log.Fatal(err)
		}

		literal += string(r)

		switch state {
		case INITIAL:
			if r == '\033' {
				prevState = state
				state = ESCAPE_SEQUENCE
			} else {
				prevState = state
				state = IN_TEXT
			}
		case ESCAPE_SEQUENCE:
			if r == '(' {
				prevState = state
				state = EPSON_SEQUENCE
			}
			if r == '[' {
				prevState = state
				state = ANSI_SEQUENCE
			}
		case ANSI_SEQUENCE:
			if r == 'H' {
				prevState = state
				state = IN_TEXT
				lexer.tokenChan <- &Token{Type: RESET_CURSOR, Literal: literal}
				literal = ""
			}
			if r == 'J' {
				prevState = state
				state = IN_TEXT
				lexer.tokenChan <- &Token{Type: CLEAR, Literal: literal}
				literal = ""
			}
			if r == 'm' || r == 'l' || r == 'h' || r == 'K' {
				prevState = state
				state = IN_TEXT
				lexer.tokenChan <- &Token{Type: COLOR_CODE, Literal: literal}
				literal = ""
			}
		case EPSON_SEQUENCE:
			if r == 'B' {
				prevState = state
				state = IN_TEXT
				lexer.tokenChan <- &Token{Type: BAR, Literal: literal}
				literal = ""
			}
		case IN_TEXT:
			if r == '\r' {
				prevState = state
				state = IN_NEWLINE
			} else if r == '\033' {
				prevState = state
				state = ESCAPE_SEQUENCE
			} else {
				lexer.tokenChan <- &Token{Type: TEXT, Literal: literal}
				literal = ""
			}
		case IN_NEWLINE:
			if r == '\n' {
				state = prevState
				lexer.tokenChan <- &Token{Type: CRLF, Literal: literal}
				literal = ""
			} else {
				if r == '\033' {
					state = ESCAPE_SEQUENCE
				} else {
					state = IN_TEXT
				}
				lexer.tokenChan <- &Token{Type: CRLF, Literal: "\r"}
				literal = literal[1:]
			}
		}
	}
}
