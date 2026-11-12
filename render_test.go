package main

import (
	"image/png"
	"os"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

func TestRenderScreenshot(t *testing.T) {
	a := newTestApp()
	a.ports = []portInfo{
		{Name: "/dev/cu.wchusbserial-1410", Base: "wchusbserial-1410", Chip: "WCH CH34x", VID: "1A86", PID: "7523"},
		{Name: "/dev/cu.usbserial-FT001", Base: "usbserial-FT001", Chip: "FTDI", VID: "0403", PID: "6001"},
		{Name: "/dev/cu.usbmodem1101", Base: "usbmodem1101", Chip: "CDC-ACM"},
	}
	a.portSel = a.ports[0].label()
	a.rxRecords = []rxRecord{
		{id: 1, ts: time.Now(), dir: dirRX, data: []byte("Hello from MCU\r\n")},
		{id: 2, ts: time.Now(), dir: dirTX, data: []byte("AT+GMR")},
		{id: 3, ts: time.Now(), dir: dirRX, data: []byte{0xAA, 0x55, 0x03, 0xFF, 0x10, 0x20, 0x30}},
		{id: 4, ts: time.Now(), dir: dirSYS, data: []byte("串口已打开 /dev/cu.wchusbserial-1410 @ 115200 bps")},
		{id: 5, ts: time.Now(), dir: dirRX, data: []byte("temp=25.3 humi=60\n")},
	}
	a.showTS = true
	a.rxCount = 42
	a.txCount = 8
	a.sendText = "AT+GMR"
	for _, dark := range []bool{true, false} {
		a.dark = dark
		img := ui.Render(a.view, 1180, 760, 2)
		name := "/tmp/serialcomm_render_dark.png"
		if !dark {
			name = "/tmp/serialcomm_render_light.png"
		}
		f, err := os.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if err := png.Encode(f, img); err != nil {
			t.Fatal(err)
		}
		f.Close()
	}
}
