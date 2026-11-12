package main

import (
	"fmt"
	"strings"

	"github.com/egoist/mygo/ui"
)

// dot renders a small status dot.
func dot(c *ui.Context, col ui.Color) ui.Element {
	return ui.Column(c).Width(8).Height(8).Radius(4).Background(col)
}

// vdiv renders a thin vertical divider inside a row.
func vdiv(c *ui.Context, p palette) ui.Element {
	return ui.Column(c).Width(1).Height(18).Background(p.line)
}

// hdiv renders a thin horizontal divider inside a column.
func hdiv(c *ui.Context, p palette) ui.Element {
	return ui.Row(c).Height(1).FillWidth().Background(p.line)
}

// pill renders a small labeled chip.
func pill(c *ui.Context, p palette, label string, fg, bg ui.Color) ui.Element {
	return ui.Text(c, label).FontSize(9).Bold().TextColor(fg).
		Background(bg).Padding(2, 7).Radius(4)
}

// labeledField renders a label above a control, like the prototype's
// .field blocks.
func labeledField(c *ui.Context, p palette, label string, build func()) ui.Element {
	return ui.Column(c).Gap(4).AlignItems(ui.Start).Children(func() {
		ui.Text(c, label).FontSize(9.5).TextColor(p.muted)
		build()
	})
}

func (a *app) statusColor() ui.Color {
	p := a.pal()
	switch a.statusKind {
	case 1:
		return p.green
	case 2:
		return p.red
	}
	return p.muted
}

func (a *app) view(c *ui.Context) {
	p := a.pal()
	c.OnShortcut(ui.Cmd, ui.KeyEnter, func() {
		if a.opened && a.composeTab == 0 {
			a.sendNow()
		}
	})
	c.OnShortcut(ui.Cmd, ui.KeyK, func() { a.wantFocus = true })

	ui.Themed(c, proTheme(a.dark)).Children(func() {
		ui.Column(c).Fill().Background(p.bg).FontSize(12).Children(func() {
			a.viewTopBar(c)
			hdiv(c, p)
			ui.Row(c).Grow(1).Children(func() {
				ui.Column(c).Grow(1).MinWidth(0).FillHeight().Children(func() {
					a.viewConfig(c)
					hdiv(c, p)
					a.viewConsoleHead(c)
					hdiv(c, p)
					a.viewConsole(c)
					hdiv(c, p)
					a.viewConsoleBottom(c)
					hdiv(c, p)
					a.viewComposer(c)
				})
				if a.showPanel {
					ui.Column(c).Width(1).FillHeight().Background(p.line)
					a.viewPanel(c)
				}
			})
			hdiv(c, p)
			a.viewStatusBar(c)
		})
	})
}

// viewTopBar renders the brand row with the session tab and theme toggle.
func (a *app) viewTopBar(c *ui.Context) {
	p := a.pal()
	ui.Row(c).Height(44).Padding(0, 14).Gap(10).AlignItems(ui.Center).
		Background(p.surface).Children(func() {
		ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
			ui.Text(c, "SerialComm").FontSize(14).Bold()
			pill(c, p, "PRO", ui.Hex("#ffffff"), p.accent2)
		})
		vdiv(c, p)
		// Session tab: dot + device name.
		ui.Row(c).Gap(6).AlignItems(ui.Center).Padding(4, 10).
			Background(p.surface3).Radius(6).Children(func() {
			dcol := p.faint
			if a.opened {
				dcol = p.green
			}
			dot(c, dcol)
			name := "未连接"
			if a.opened {
				name = a.devName
			}
			ui.Text(c, name).FontSize(11).TextColor(p.text).NoWrap()
		})
		ui.Spacer(c).Grow(1)
		ui.Text(c, "串口调试工作台").FontSize(10).TextColor(p.muted)
		themeLabel := "☀ 浅色"
		if !a.dark {
			themeLabel = "☾ 深色"
		}
		ui.Button(c, themeLabel).FontSize(10.5).OnClick(func() {
			a.dark = !a.dark
		})
	})
}

// viewConfig renders the serial parameter row.
func (a *app) viewConfig(c *ui.Context) {
	p := a.pal()
	ui.Row(c).Gap(10).AlignItems(ui.End).Padding(10, 14).
		Background(p.surface).Children(func() {
		labeledField(c, p, "串口设备", func() {
			ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
				ui.Select(c, &a.portSel, a.portLabels()).
					MinWidth(190).Disabled(a.opened)
				ui.Button(c, "⟳").Tooltip("刷新设备列表").
					OnClick(a.refreshPortsAsync)
			})
		})
		labeledField(c, p, "波特率", func() {
			ui.Combobox(c, &a.baud, baudRates).MinWidth(100).
				OnChange(a.applyMode)
		})
		labeledField(c, p, "数据位", func() {
			ui.Select(c, &a.dataBits, []string{"5", "6", "7", "8"}).
				MinWidth(64).OnChange(a.applyMode)
		})
		labeledField(c, p, "校验", func() {
			ui.Select(c, &a.parity,
				[]string{"None", "Odd", "Even", "Mark", "Space"}).
				MinWidth(80).OnChange(a.applyMode)
		})
		labeledField(c, p, "停止位", func() {
			ui.Select(c, &a.stopBits, []string{"1", "1.5", "2"}).
				MinWidth(64).OnChange(a.applyMode)
		})
		ui.Spacer(c).Grow(1)
		ui.Row(c).Gap(5).AlignItems(ui.Center).Children(func() {
			ui.Switch(c, &a.dtr).OnChange(a.setModemBits)
			ui.Text(c, "DTR").FontSize(10.5).TextColor(p.muted)
		})
		ui.Row(c).Gap(5).AlignItems(ui.Center).Children(func() {
			ui.Switch(c, &a.rts).OnChange(a.setModemBits)
			ui.Text(c, "RTS").FontSize(10.5).TextColor(p.muted)
		})
		label := "连接设备"
		if a.opened {
			label = "断开连接"
		}
		ui.PrimaryButton(c, label).MinWidth(96).OnClick(a.togglePort)
	})
}

// viewConsoleHead renders the console toolbar.
func (a *app) viewConsoleHead(c *ui.Context) {
	p := a.pal()
	ui.Row(c).Gap(8).AlignItems(ui.Center).Padding(6, 12).
		Background(p.surface2).Children(func() {
		ui.Text(c, "串口监视器").FontSize(12).Bold()
		ui.Segmented(c, &a.dispMode, "文本", "HEX", "混合")
		ui.Toggle(c, &a.showRX, "RX")
		ui.Toggle(c, &a.showTX, "TX")
		pill(c, p, "UTF-8", p.muted, p.surface3)
		ui.Spacer(c).Grow(1)
		ui.Checkbox(c, &a.showTS, "时间戳")
		ui.SearchField(c, &a.query).MinWidth(140).
			Placeholder("搜索日志… ⌘K").Bind(&a.searchH)
		if a.wantFocus {
			a.searchH.Focus(c)
			a.wantFocus = false
		}
		ui.Button(c, "⤒ 导出").Tooltip("保存日志到文件").OnClick(a.saveLog)
		ui.Toggle(c, &a.showPanel, "☰")
	})
}

// dirTag returns the label and color for a record direction.
func (a *app) dirTag(dir int) (string, ui.Color) {
	p := a.pal()
	switch dir {
	case dirTX:
		return "TX", p.tx
	case dirSYS:
		return "SYS", p.sys
	}
	return "RX", p.rx
}

// viewConsole renders the log list.
func (a *app) viewConsole(c *ui.Context) {
	p := a.pal()
	a.filterRecords()
	a.rxList.FollowEnd = a.autofollow
	ui.Column(c).Grow(1).Background(p.consoleBg).Children(func() {
		ui.List(c, &a.rxList, len(a.filtered), func(i int) {
		r := a.filtered[i]
		tag, col := a.dirTag(r.dir)
		ui.Row(c).Gap(10).Padding(1, 12).AlignItems(ui.Start).
			Font("monospace").Children(func() {
			if a.showTS {
				ui.Text(c, r.ts.Format("15:04:05.000")).
					FontSize(10.5).TextColor(p.logTime).NoWrap()
			}
			ui.Text(c, tag).FontSize(9.5).Bold().TextColor(col).
				NoWrap().MinWidth(24)
			ui.Text(c, a.recordText(r)).FontSize(11.5).
				TextColor(p.logMsg).Grow(1)
		})
	}).Grow(1)
	})
}

// viewConsoleBottom renders follow/pause and log actions.
func (a *app) viewConsoleBottom(c *ui.Context) {
	p := a.pal()
	ui.Row(c).Gap(10).AlignItems(ui.Center).Padding(5, 12).
		Background(p.surface2).Children(func() {
		ui.Toggle(c, &a.autofollow, "跟随输出")
		ui.Toggle(c, &a.paused, "暂停显示")
		ui.Textf(c, "已显示 %d 行", len(a.filtered)).FontSize(10).
			TextColor(p.muted)
		ui.Spacer(c).Grow(1)
		svc := c.Services()
		ui.Button(c, "复制").Tooltip("复制全部日志").
			OnClick(func() { a.copyLog(svc) })
		ui.Button(c, "清空").OnClick(func() {
			a.rxRecords = a.rxRecords[:0]
			a.rxCount, a.txCount = 0, 0
		})
		ui.Button(c, "保存日志").OnClick(a.saveLog)
	})
}

// copyLog copies the rendered log to the clipboard.
func (a *app) copyLog(svc ui.Services) {
	var s strings.Builder
	for _, r := range a.rxRecords {
		tag, _ := a.dirTag(r.dir)
		fmt.Fprintf(&s, "[%s] %s %s\n",
			r.ts.Format("15:04:05.000"), tag, a.recordText(r))
	}
	svc.WriteClipboard(s.String())
	a.setStatus("日志已复制到剪贴板", 1)
}

// viewComposer renders the send area with tabs and options.
func (a *app) viewComposer(c *ui.Context) {
	p := a.pal()
	ui.Column(c).Background(p.surface).Children(func() {
		ui.Row(c).AlignItems(ui.Center).Padding(2, 12, 0, 12).
			Children(func() {
			ui.Tabs(c, &a.composeTab, "单条发送", "文件发送", "高级封包")
			ui.Spacer(c).Grow(1)
			ui.Text(c, "⌘↵ 发送").FontSize(10).TextColor(p.faint)
		})
		if a.composeTab == 0 {
			a.viewSendRow(c)
		} else {
			ui.Row(c).Padding(18).AlignItems(ui.Center).Children(func() {
				ui.Text(c, "规划中 —— 该功能尚未实现，敬请期待").
					FontSize(11).TextColor(p.muted).Grow(1)
			})
		}
	})
}

// viewSendRow renders the textarea, send button, and option row.
func (a *app) viewSendRow(c *ui.Context) {
	p := a.pal()
	ui.Row(c).Gap(10).Padding(8, 12).AlignItems(ui.Stretch).Children(func() {
		ui.TextArea(c, &a.sendText).Grow(1).MinHeight(64).
			Font("monospace").FontSize(12).
			Placeholder("输入要发送的数据…（HEX 模式下按十六进制解析）").
			OnChange(a.updateSnap)
		ui.PrimaryButton(c, "发送 ↗").FillHeight().MinWidth(88).
			Disabled(!a.opened).OnClick(a.sendNow)
	})
	ui.Row(c).Gap(10).AlignItems(ui.Center).Padding(4, 12, 10, 12).
		Children(func() {
		ui.Checkbox(c, &a.sendHex, "HEX 发送").OnChange(a.updateSnap)
		vdiv(c, p)
		ui.Text(c, "行结束").FontSize(10.5).TextColor(p.muted)
		ui.Select(c, &a.suffix, suffixes).MinWidth(100).
			OnChange(a.updateSnap)
		vdiv(c, p)
		ui.Checkbox(c, &a.periodic, "定时发送").OnChange(a.updateSnap)
		ui.Select(c, &a.interval, intervals).MinWidth(96).
			OnChange(a.updateSnap)
		vdiv(c, p)
		ui.Text(c, "校验").FontSize(10.5).TextColor(p.muted)
		ui.Select(c, &a.checksum, checksums).MinWidth(124).
			OnChange(a.updateSnap)
		ui.Spacer(c).Grow(1)
		ui.Text(c, a.status).FontSize(10.5).TextColor(a.statusColor()).
			NoWrap()
	})
}

// viewPanel renders the right-side command panel.
func (a *app) viewPanel(c *ui.Context) {
	p := a.pal()
	ui.Column(c).Width(280).Background(p.surface).Children(func() {
		ui.Column(c).Padding(4, 10, 0, 10).Children(func() {
			ui.Tabs(c, &a.rightTab, "多条发送", "自动回复", "小工具")
		})
		if a.rightTab == 0 {
			a.viewCommands(c)
		} else {
			ui.Row(c).Padding(20).AlignItems(ui.Center).Grow(1).
				Children(func() {
				ui.Text(c, "规划中 —— 该功能尚未实现").
					FontSize(11).TextColor(p.muted).Grow(1)
			})
		}
	})
}

// viewCommands renders the quick-command group and its cards.
func (a *app) viewCommands(c *ui.Context) {
	p := a.pal()
	ui.Column(c).Gap(8).Padding(10).Grow(1).Children(func() {
		ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
			ui.Text(c, "快捷命令组").FontSize(10).TextColor(p.muted)
			ui.Select(c, &a.cmdGroup, cmdGroupNames).Grow(1)
		})
		ui.Scroll(c).Grow(1).Children(func() {
			ui.Column(c).Gap(8).Children(func() {
				for i, cmd := range a.groupCommands() {
					a.viewCmdCard(c, i, cmd)
				}
			})
		})
		if a.addOpen {
			a.viewAddForm(c)
		} else {
			ui.Button(c, "＋ 添加快捷命令").OnClick(func() {
				a.addOpen = true
				a.addName, a.addValue = "", ""
			})
		}
		ui.Column(c).Gap(4).Padding(8).Background(p.surface2).
			Radius(6).Border(1, p.line).Children(func() {
			ui.Text(c, "提示").FontSize(10).Bold().TextColor(p.gold)
			ui.Text(c, "命令按当前发送选项（HEX / 行尾 / 校验）发送；"+
				"自定义命令保存在内存中。").
				FontSize(10).TextColor(p.muted)
		})
	})
}

// viewCmdCard renders one quick-command card.
func (a *app) viewCmdCard(c *ui.Context, i int, cmd quickCmd) {
	p := a.pal()
	ui.Column(c).Gap(4).Padding(8).Background(p.surface3).Radius(6).
		Border(1, p.line).Children(func() {
		ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
			ui.Textf(c, "#%02d", i+1).FontSize(9.5).TextColor(p.faint)
			ui.Text(c, cmd.Title).FontSize(11).Bold().Grow(1).NoWrap()
			ui.Button(c, "发送").Disabled(!a.opened).
				OnClick(func() { a.sendCommand(cmd.Value) })
		})
		ui.Text(c, cmd.Value).FontSize(10).Font("monospace").
			TextColor(p.muted).NoWrap()
	})
}

// viewAddForm renders the inline form for a custom command.
func (a *app) viewAddForm(c *ui.Context) {
	p := a.pal()
	ui.Column(c).Gap(6).Padding(8).Background(p.surface2).Radius(6).
		Border(1, p.line).Children(func() {
		ui.TextInput(c, &a.addName).Placeholder("命令名称")
		ui.TextInput(c, &a.addValue).Placeholder("命令内容")
		ui.Row(c).Gap(6).Children(func() {
			ui.Button(c, "确定").OnClick(func() {
				if a.addValue == "" {
					return
				}
				name := a.addName
				if name == "" {
					name = a.addValue
					if len(name) > 12 {
						name = name[:12]
					}
				}
				a.customCmds[a.cmdGroup] = append(
					a.customCmds[a.cmdGroup],
					quickCmd{Title: name, Value: a.addValue})
				a.addOpen = false
			})
			ui.Button(c, "取消").OnClick(func() { a.addOpen = false })
		})
	})
}

// viewStatusBar renders the bottom status row.
func (a *app) viewStatusBar(c *ui.Context) {
	p := a.pal()
	ui.Row(c).Height(28).Padding(0, 14).Gap(14).AlignItems(ui.Center).
		Background(p.surface2).FontSize(10.5).Children(func() {
		dcol := p.faint
		state := "已断开"
		if a.opened {
			dcol = p.green
			state = "已连接"
		}
		ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
			dot(c, dcol)
			ui.Text(c, state).TextColor(p.text)
			if a.opened {
				ui.Text(c, a.devName).TextColor(p.faint).NoWrap()
			}
		})
		ui.Textf(c, "RX %d B", a.rxCount).TextColor(p.muted)
		ui.Textf(c, "TX %d B", a.txCount).TextColor(p.muted)
		ui.Textf(c, "速率 %s bps", a.baud).TextColor(p.muted)
		ui.Spacer(c).Grow(1)
		if a.opened {
			ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
				ui.Row(c).Gap(4).AlignItems(ui.Center).Children(func() {
					ui.Text(c, "CTS").TextColor(p.muted)
					dot(c, modemColor(a.cts, p))
				})
				ui.Row(c).Gap(4).AlignItems(ui.Center).Children(func() {
					ui.Text(c, "DSR").TextColor(p.muted)
					dot(c, modemColor(a.dsr, p))
				})
			})
		}
		ui.Text(c, "UTF-8").TextColor(p.faint)
	})
}

func modemColor(on bool, p palette) ui.Color {
	if on {
		return p.green
	}
	return p.faint
}
