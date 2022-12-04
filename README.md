# goterm
Goterm is a terminal emulator / ssh client written in pure Go -- it should produce a static binary with no dependencies. It does not link to GTK or QT to produce a GUI.
Instead it uses xgb (Go bindings for [XCB](https://xcb.freedesktop.org/))

## Install

```bash
go install github.com/sheik/goterm/cmd/goterm@latest
```
