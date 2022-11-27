package main

// #cgo LDFLAGS: -lX11
// #include <X11/Xlib.h>
// #include <stdio.h>
// #include <stdlib.h>
// #include <string.h>
import "C"

import (
	"fmt"
	"unsafe"
)

func UnpackKeypress(event C.XEvent) {

}

func UnpackEvent(event C.XEvent) XEvent {
	_type := *(*uint32)(unsafe.Pointer(&event[0]))
	return XEvent{Type: _type}
}

type XEvent struct {
	Type uint32
}

func main() {
	var display = C.XOpenDisplay(nil)
	if display == nil {
		panic("Can't open display")
	}

	var screen = C.XDefaultScreen(display)

	var window = C.XCreateSimpleWindow(display, C.XRootWindow(display, screen), 10, 10, 200, 200,
		1, C.XBlackPixel(display, screen), C.XWhitePixel(display, screen))

	C.XSelectInput(display, window, C.ExposureMask|C.KeyPressMask)

	C.XMapWindow(display, window)

	msg := "test"
	var event C.XEvent

	for {
		C.XNextEvent(display, &event)
		e := *(*uint32)(unsafe.Pointer(&event[0]))
		if e == C.KeyPress {
			UnpackKeypress(event)
		}
		//		fmt.Println(e)
		//		fmt.Println(C.KeyPress)
		C.XDrawString(display, window, C.XDefaultGC(display, screen), 50, 50, C.CString(msg), C.int(C.strlen(C.CString(msg))))
	}

	fmt.Printf("%dx%d\n", C.XDisplayWidth(display, 0), C.XDisplayHeight(display, 0))
}
