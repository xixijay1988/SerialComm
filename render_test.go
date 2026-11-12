package main

import (
	"image/png"
	"os"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

func TestRenderScreenshot(t *testing.T) {
	a := &app{
		baud: "115200", dataBits: "8", parity: "None",
		stopBits: "1", suffix: "None", periodMs: 1000,
		status: "Select a port", dtr: true, rts: true,
	}
	a.ports = []portInfo{
		{Name: "/dev/cu.wchusbserial-1410", Base: "wchusbserial-1410", Chip: "WCH CH34x", VID: "1A86", PID: "7523"},
		{Name: "/dev/cu.usbserial-FT001", Base: "usbserial-FT001", Chip: "FTDI", VID: "0403", PID: "6001"},
		{Name: "/dev/cu.usbmodem1101", Base: "usbmodem1101", Chip: "CDC-ACM"},
	}
	a.portSel = a.ports[0].label()
	a.rxRecords = []rxRecord{
		{id: 1, ts: time.Now(), data: []byte("Hello from MCU\r\n")},
		{id: 2, ts: time.Now(), data: []byte{0xAA, 0x55, 0x03, 0xFF, 0x10, 0x20, 0x30}},
		{id: 3, ts: time.Now(), data: []byte("temp=25.3 humi=60\n")},
	}
	a.rxCount = 42
	a.txCount = 8
	a.sendText = "AT+GMR"
	img := ui.Render(a.view, 920, 640, 2)
	f, err := os.Create("/tmp/serialcomm_render.png")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		t.Fatal(err)
	}
}
