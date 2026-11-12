# SerialComm

A serial port tool for macOS (SSCOM-style), written in Go with
[MyGo](https://github.com/egoist/mygo) native UI and
[go.bug.st/serial](https://go.bug.st/serial).

## Features

- Port enumeration of `/dev/cu.*` devices with chip detection
  (WCH CH34x/CH347, FTDI, CP210x, PL2303, CDC-ACM, ESP32, STM32…) plus
  USB VID:PID via IOKit (`ioreg`), auto-refreshed on plug/unplug
- Baud rate (editable combo box, incl. non-standard rates), data bits
  5–8, parity None/Odd/Even/Mark/Space, stop bits 1/1.5/2 — re-applied
  live while the port is open
- Receive view: Text/HEX display, timestamps, pause, clear, save to
  file, auto-scroll to the newest data, RX/TX byte counters
- Send: text or HEX input, line-ending suffix (None/\r/\n/\r\n/\n\r),
  periodic send with configurable interval
- DTR/RTS control lines

## Run

```sh
go run .            # build and run
go tool mygo dev    # develop with live reload
```

## Test

```sh
CGO_ENABLED=0 go test ./...
go tool mygo vet .
```

## Package

```sh
go tool mygo build  # -> build/darwin-arm64/SerialComm.app + .dmg
```

## Layout

- `main.go` — app state, view, read/send goroutines
- `serial.go` — port enumeration (chip detection, ioreg VID/PID),
  serial mode parsing
- `format.go` — hex dump/parse, display sanitizing
- `main_test.go` — unit + view tests (`ui.NewTester`)
- `render_test.go` — renders the view to `/tmp/serialcomm_render.png`
