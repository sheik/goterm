package main

import (
	"gioui.org/io/key"
	"strings"
)

var registered_keys = "(Shift)-(Ctrl)-A|" +
	"(Shift)-(Ctrl)-B|" +
	"(Shift)-(Ctrl)-C|" +
	"(Shift)-(Ctrl)-D|" +
	"(Shift)-(Ctrl)-E|" +
	"(Shift)-(Ctrl)-F|" +
	"(Shift)-(Ctrl)-G|" +
	"(Shift)-(Ctrl)-H|" +
	"(Shift)-(Ctrl)-I|" +
	"(Shift)-(Ctrl)-J|" +
	"(Shift)-(Ctrl)-K|" +
	"(Shift)-(Ctrl)-L|" +
	"(Shift)-(Ctrl)-M|" +
	"(Shift)-(Ctrl)-N|" +
	"(Shift)-(Ctrl)-O|" +
	"(Shift)-(Ctrl)-P|" +
	"(Shift)-(Ctrl)-Q|" +
	"(Shift)-(Ctrl)-R|" +
	"(Shift)-(Ctrl)-S|" +
	"(Shift)-(Ctrl)-T|" +
	"(Shift)-(Ctrl)-U|" +
	"(Shift)-(Ctrl)-V|" +
	"(Shift)-(Ctrl)-W|" +
	"(Shift)-(Ctrl)-X|" +
	"(Shift)-(Ctrl)-Y|" +
	"(Shift)-(Ctrl)-Z|" +
	"(Shift)-!|" +
	"(Shift)-1|" +
	"1|" +
	"(Shift)--|" +
	"Ctrl--|" +
	"-|" +
	"Shift-_|" +
	"(Shift)-(Ctrl)-.|" +
	key.NameEscape + "|" +
	"(Shift)-:|;|:|(Shift)-;|Space|" +
	strings.Join([]string{key.NameReturn, key.NameDeleteBackward}, "|")
