package main

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

// newTestApp returns an app with the same defaults main() uses.
func newTestApp() *app {
	a := &app{
		showSide:   true,
		showPanel:  true,
		dark:       true,
		cmdGroup:   cmdGroupNames[0],
		customCmds: map[string][]quickCmd{},
	}
	a.newSession()
	return a
}

// The view runs without a window in tests.
func TestView(t *testing.T) {
	a := newTestApp()
	s := a.activeSession()
	s.rxRecords = []rxRecord{{id: 1, dir: dirRX, data: []byte("hello")}}
	tt := ui.NewTester(a.view, 1280, 800)
	for _, want := range []string{"SerialComm", "PRO", "工作区", "活动会话",
		"会话 1", "当前设备", "串口监视器", "已断开", "hello",
		"多条发送", "快捷命令组", "连接设备", "◐ 跟随系统"} {
		if !tt.HasText(want) {
			t.Errorf("expected %q, texts %q", want, tt.Texts())
		}
	}
}

func TestDirTag(t *testing.T) {
	a := newTestApp()
	if tag, _ := a.dirTag(dirRX); tag != "RX" {
		t.Fatalf("dirTag rx = %q", tag)
	}
	if tag, _ := a.dirTag(dirTX); tag != "TX" {
		t.Fatalf("dirTag tx = %q", tag)
	}
	if tag, _ := a.dirTag(dirSYS); tag != "SYS" {
		t.Fatalf("dirTag sys = %q", tag)
	}
}
