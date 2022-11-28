package main

import (
	"bufio"
	"io"
	"log"
)

type Lexer struct {
	reader *bufio.Reader
}

func NewLexer(reader *bufio.Reader) *Lexer {
	return &Lexer{reader: reader}
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
)

type State string

const (
	INITIAL        State = "INITIAL"
	ANSI_SEQUENCE  State = "ANSI_SEQUENCE"
	EPSON_SEQUENCE State = "EPSON_SEQUENCE"
)

var state = INITIAL

func (lexer *Lexer) Token() *Token {
	literal := ""
	for {
		r, err := lexer.reader.ReadByte()
		if err != nil {
			if err == io.EOF {
				return nil
			}
			log.Fatal(err)
		}

		switch state {
		case INITIAL:
			if r == '\033' {
				state = ANSI_SEQUENCE
			}
		case ANSI_SEQUENCE:
			if r == '(' {
				state = EPSON_SEQUENCE
			}
		case EPSON_SEQUENCE:
			if r == 'B' {
				return &Token{Type: BAR, Literal: literal}
			}
		}
		literal += string(r)
	}
}
