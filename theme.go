package main

import "github.com/egoist/mygo/ui"

// The SerialComm Pro palette, from docs/design/SerialComm_Pro_UI_Prototype.html.
type palette struct {
	bg, surface, surface2, surface3, line ui.Color
	text, muted, faint                    ui.Color
	accent, accent2                       ui.Color
	green, red, gold                      ui.Color
	consoleBg                             ui.Color
	rx, tx, sys, logTime, logMsg          ui.Color
}

var proDark = palette{
	bg: ui.Hex("#0d111b"), surface: ui.Hex("#131b29"), surface2: ui.Hex("#172231"),
	surface3: ui.Hex("#202b3b"), line: ui.Hex("#2a3545"),
	text: ui.Hex("#dfebfa"), muted: ui.Hex("#8e9cb1"), faint: ui.Hex("#687991"),
	accent: ui.Hex("#438cff"), accent2: ui.Hex("#2e70db"),
	green: ui.Hex("#3fd2a0"), red: ui.Hex("#ff6679"), gold: ui.Hex("#efbd70"),
	consoleBg: ui.Hex("#0e1724"),
	rx: ui.Hex("#45d4ad"), tx: ui.Hex("#5fa5ff"), sys: ui.Hex("#d7ac6c"),
	logTime: ui.Hex("#59708c"), logMsg: ui.Hex("#b6c8da"),
}

var proLight = palette{
	bg: ui.Hex("#eef3fb"), surface: ui.Hex("#ffffff"), surface2: ui.Hex("#eff3fa"),
	surface3: ui.Hex("#e6ecf6"), line: ui.Hex("#d4ddec"),
	text: ui.Hex("#22364c"), muted: ui.Hex("#657f9e"), faint: ui.Hex("#8a9db8"),
	accent: ui.Hex("#2c72e0"), accent2: ui.Hex("#2c72e0"),
	green: ui.Hex("#047d57"), red: ui.Hex("#c2334a"), gold: ui.Hex("#8d5b0d"),
	consoleBg: ui.Hex("#f6f9ff"),
	rx: ui.Hex("#008e61"), tx: ui.Hex("#206fdb"), sys: ui.Hex("#8d5b0d"),
	logTime: ui.Hex("#7b8ca4"), logMsg: ui.Hex("#344f6c"),
}

// proTheme builds a MyGo theme (dark or light) with the Pro palette.
func proTheme(dark bool) *ui.Theme {
	p := proLight
	t := ui.LightTheme()
	if dark {
		p = proDark
		t = ui.DarkTheme()
	}
	t.Background = p.bg
	t.Surface = p.surface2
	t.SurfaceHover = p.surface3
	t.SurfacePressed = p.surface3
	t.Border = p.line
	t.Text = p.text
	t.TextMuted = p.muted
	t.Accent = p.accent
	t.AccentHover = ui.Hex("#5c9dff")
	t.AccentPressed = p.accent2
	t.AccentText = ui.Hex("#ffffff")
	t.Danger = p.red
	t.Warning = p.gold
	t.Success = p.green
	t.Radius = 6
	return t
}

func (a *app) pal() palette {
	if a.dark {
		return proDark
	}
	return proLight
}
