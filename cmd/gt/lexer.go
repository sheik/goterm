package main

import (
	"bufio"
	"fmt"
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
	TEXT                    TokenType = "TEXT"
	CLEAR_SCREEN            TokenType = "CLEAR_SCREEN"
	BAR                     TokenType = "BAR"
	COLOR_CODE              TokenType = "COLOR_CODE"
	CRLF                    TokenType = "CRLF"
	CR                      TokenType = "CR"
	LF                      TokenType = "LF"
	RESET_CURSOR            TokenType = "RESET_CURSOR"
	CLEAR                   TokenType = "CLEAR"
	BACKSPACE               TokenType = "BACKSPACE"
	OSC                     TokenType = "OSC"
	CURSOR_ROW              TokenType = "CURSOR_ROW"
	CURSOR_POSITION_REQUEST TokenType = "CURSOR_POSITION_REQUEST"
	DEVICE_CONTROL_STRING   TokenType = "DEVICE_CONTROL_STRING"
	RESET_INITIAL_STATE     TokenType = "RESET_INITIAL_STATE"
)

type State string

const (
	INITIAL                  State = "INITIAL"
	ESCAPE_SEQUENCE          State = "ESCAPE_SEQUENCE"
	ANSI_SEQUENCE            State = "ANSI_SEQUENCE"
	EPSON_SEQUENCE           State = "EPSON_SEQUENCE"
	IN_TEXT                  State = "IN_TEXT"
	IN_NEWLINE               State = "IN_NEWLINE"
	OPERATING_SYSTEM_COMMAND State = "OPERATING_SYSTEM_COMMAND"
	DCS                      State = "DCS"
	DCS_TERMINATE            State = "DCS_TERMINATE"
)

var state = INITIAL

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
				state = ESCAPE_SEQUENCE
			} else {
				state = INITIAL
				lexer.tokenChan <- &Token{Type: TEXT, Literal: literal}
				literal = ""
			}
		case DCS:
			if lexer.char == '\033' {
				state = DCS_TERMINATE
			}
		case DCS_TERMINATE:
			if lexer.char == '\\' {
				fmt.Println("IN DCS TERMINATE")
				state = IN_TEXT
				lexer.tokenChan <- &Token{Type: DEVICE_CONTROL_STRING, Literal: literal}
				literal = ""
			}
		case ESCAPE_SEQUENCE:
			if lexer.char == '(' {
				state = EPSON_SEQUENCE
			}
			if lexer.char == '[' {
				state = ANSI_SEQUENCE
			}
			if lexer.char == ']' {
				state = OPERATING_SYSTEM_COMMAND
			}
			if lexer.char == '=' || lexer.char == '>' {
				literal = ""
				state = INITIAL
			}

			if lexer.char == 'P' {
				state = DCS // device control string
			}

		case ANSI_SEQUENCE:
			if lexer.char == 'H' {
				state = IN_TEXT
				lexer.tokenChan <- &Token{Type: RESET_CURSOR, Literal: literal}
				literal = ""
			}
			if lexer.char == 'J' {
				state = IN_TEXT
				lexer.tokenChan <- &Token{Type: CLEAR, Literal: literal}
				literal = ""
			}

			// move to row
			if lexer.char == 'd' {
				state = IN_TEXT
				lexer.tokenChan <- &Token{Type: CURSOR_ROW, Literal: literal}
				literal = ""
			}

			if lexer.char == 'n' {
				state = IN_TEXT
				lexer.tokenChan <- &Token{Type: CURSOR_POSITION_REQUEST, Literal: literal}
				literal = ""
			}

			if lexer.char == 'c' {
				state = IN_TEXT
				lexer.tokenChan <- &Token{Type: RESET_INITIAL_STATE, Literal: literal}
				literal = ""
			}

			if lexer.char == 'm' || lexer.char == 'l' || lexer.char == 'h' || lexer.char == 'K' || lexer.char == 'f' || lexer.char == '@' || lexer.char == 'C' || lexer.char == 't' || lexer.char == 'r' || lexer.char == 'G' {
				state = IN_TEXT
				lexer.tokenChan <- &Token{Type: COLOR_CODE, Literal: literal}
				literal = ""
			}
		case EPSON_SEQUENCE:
			if lexer.char == 'B' {
				state = IN_TEXT
				lexer.tokenChan <- &Token{Type: BAR, Literal: literal}
				literal = ""
			}
		case OPERATING_SYSTEM_COMMAND:
			if lexer.char == '\a' {
				state = IN_TEXT
				lexer.tokenChan <- &Token{Type: OSC, Literal: literal}
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
