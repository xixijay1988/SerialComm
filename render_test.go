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
	s := a.activeSession()
	s.name = "调试串口"
	s.portSel = a.ports[0].label()
	s.opened = true
	s.devName = "/dev/cu.wchusbserial-1410"
	s.cts, s.dsr = true, true
	s.status = "已连接 wchusbserial-1410 @ 115200 bps"
	s.statusKind = 1
	s.showTS = true
	now := time.Now()
	lines := []struct {
		dir  int
		data string
	}{
		{dirSYS, "串口已打开 /dev/cu.wchusbserial-1410 @ 115200 bps, 8/None/1"},
		{dirRX, "dram_bist_test start\r\n"},
		{dirRX, "[stage 4] sdram_init() returned\r\n"},
		{dirRX, "[stage 5] external DDR software memory test @0x03000000 (16 MiB)...\r\n"},
		{dirRX, "[stage 5] memory test done, total errors=0\r\n"},
		{dirRX, "[stage 6] restoring DDR image from DLM backup...\r\n"},
		{dirTX, "AT+CGSN"},
		{dirRX, "862419050012345\r\n"},
		{dirRX, "[stage 7] resumed in DDR code, main=0x0290045c\r\n"},
		{dirRX, "[stage 8] PASS: total errors=0\r\n"},
		{dirRX, "==== ALL DONE stage=8 errors=0 done=0x900d900d ====\r\n"},
		{dirTX, "dram_bist_test"},
	}
	for i, l := range lines {
		s.rxRecords = append(s.rxRecords, rxRecord{
			id: i + 1, ts: now.Add(time.Duration(i) * 80 * time.Millisecond),
			dir: l.dir, data: []byte(l.data),
		})
	}
	s.rxCount = 1024
	s.txCount = 22
	s.sendText = "sysinfo"

	s2 := a.newSession()
	s2.name = "Modem"
	s2.portSel = a.ports[2].label()
	a.active = 0

	for _, dark := range []bool{true, false} {
		a.dark = dark
		img := ui.Render(a.view, 1280, 800, 2)
		name := "docs/screenshot.png"
		if !dark {
			name = "docs/screenshot-light.png"
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
