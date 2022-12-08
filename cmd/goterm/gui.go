package main

// UI is an interface that allows
// for building different UIs for
// a terminal
type UI interface {
	SetWindowTitle(string)
	CreateWindow(*Terminal) error
	GetCursorSize() (int, int)
	DrawCursor(*Terminal)
	DrawRect(*Terminal, bool, interface{}, int, int, int, int)
	EraseCursor(*Terminal)
	UpdateDisplay(*Terminal)
	WriteText(*Terminal, int, int, interface{}, interface{}, string)
	Clear(*Terminal)
	SetFont(string)
	Main(term *Terminal)
}
