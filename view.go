package main

import (
	"github.com/egoist/mygo"
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

func (s *session) statusColor() ui.Color {
	p := s.a.pal()
	switch s.statusKind {
	case 1:
		return p.green
	case 2:
		return p.red
	}
	return p.muted
}

// themeLabel shows the appearance mode: follow system, light, dark.
func (a *app) themeLabel() string {
	switch a.themeMode {
	case 1:
		return "☀ 浅色"
	case 2:
		return "☾ 深色"
	}
	return "◐ 跟随系统"
}

// cycleTheme rotates 跟随系统 → 浅色 → 深色.
func (a *app) cycleTheme() {
	a.themeMode = (a.themeMode + 1) % 3
	switch a.themeMode {
	case 1:
		mygo.Theme.SetSource(mygo.ThemeLight)
		a.dark = false
	case 2:
		mygo.Theme.SetSource(mygo.ThemeDark)
		a.dark = true
	default:
		mygo.Theme.SetSource(mygo.ThemeSystem)
		a.dark = mygo.Theme.IsDark()
	}
}

func (a *app) view(c *ui.Context) {
	p := a.pal()
	s := a.activeSession()
	c.OnShortcut(ui.Cmd, ui.KeyEnter, func() {
		if s != nil && s.opened && s.composeTab == 0 {
			s.sendNow()
		}
	})
	c.OnShortcut(ui.Cmd, ui.KeyK, func() { a.wantFocus = true })

	ui.Themed(c, proTheme(a.dark)).Fill().Children(func() {
		ui.Column(c).Fill().Background(p.bg).FontSize(12).Children(func() {
			a.viewTopBar(c, s)
			hdiv(c, p)
			ui.Row(c).Grow(1).Children(func() {
				if a.showSide {
					a.viewSidebar(c)
					ui.Column(c).Width(1).FillHeight().Background(p.line)
				}
				ui.Column(c).Grow(1).MinWidth(0).FillHeight().Children(func() {
					if s == nil {
						return
					}
					a.viewConfig(c, s)
					hdiv(c, p)
					a.viewConsoleHead(c, s)
					hdiv(c, p)
					a.viewConsole(c, s)
					hdiv(c, p)
					a.viewConsoleBottom(c, s)
					hdiv(c, p)
					a.viewComposer(c, s)
				})
				if a.showPanel {
					ui.Column(c).Width(1).FillHeight().Background(p.line)
					a.viewPanel(c, s)
				}
			})
			hdiv(c, p)
			a.viewStatusBar(c, s)
		})
	})
}

// viewTopBar renders the sidebar toggle, brand, session tab and theme
// switch.
func (a *app) viewTopBar(c *ui.Context, s *session) {
	p := a.pal()
	ui.Row(c).Height(44).Padding(0, 12).Gap(10).AlignItems(ui.Center).
		Background(p.surface).Children(func() {
		ui.Toggle(c, &a.showSide, "☰").Tooltip("工作区")
		ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
			ui.Text(c, "SerialComm").FontSize(14).Bold()
			pill(c, p, "PRO", ui.Hex("#ffffff"), p.accent2)
		})
		vdiv(c, p)
		// Session tab: dot + name.
		ui.Row(c).Gap(6).AlignItems(ui.Center).Padding(4, 10).
			Background(p.surface3).Radius(6).Children(func() {
			dcol := p.faint
			label := "未连接"
			if s != nil {
				label = s.name
				if s.opened {
					dcol = p.green
				}
			}
			dot(c, dcol)
			ui.Text(c, label).FontSize(11).TextColor(p.text).NoWrap()
		})
		ui.Spacer(c).Grow(1)
		ui.Text(c, "串口调试工作台").FontSize(10).TextColor(p.muted)
		ui.Button(c, a.themeLabel()).FontSize(10.5).OnClick(a.cycleTheme)
	})
}

// viewSidebar renders the workspace column: session list + device card.
func (a *app) viewSidebar(c *ui.Context) {
	p := a.pal()
	ui.Column(c).Width(184).FillHeight().Background(p.surface).Children(func() {
		ui.Row(c).Padding(10, 12, 6, 12).AlignItems(ui.Center).Children(func() {
			ui.Text(c, "工作区").FontSize(10).Bold().TextColor(p.muted)
			ui.Spacer(c).Grow(1)
			ui.Text(c, "串口调试").FontSize(9.5).TextColor(p.faint)
		})
		ui.Text(c, "活动会话").FontSize(9).TextColor(p.faint).
			Padding(2, 12, 4, 12)
		ui.Scroll(c).Grow(1).Children(func() {
			ui.Column(c).Gap(2).Padding(0, 6).Children(func() {
				for i, s := range a.sessions {
					a.viewSessionRow(c, i, s)
				}
				ui.Row(c).Padding(6, 8).AlignItems(ui.Center).
					Role(ui.RoleButton).Children(func() {
					ui.Text(c, "＋ 新建会话").FontSize(11).TextColor(p.muted)
				}).OnClick(func() { a.newSession() })
			})
		})
		hdiv(c, p)
		// Bottom: current device card.
		ui.Column(c).Gap(4).Padding(10, 12).Children(func() {
			ui.Text(c, "当前设备").FontSize(9).TextColor(p.faint)
			s := a.activeSession()
			var pi *portInfo
			if s != nil {
				pi = s.selectedPort()
			}
			ui.Column(c).Gap(4).Padding(8).Background(p.surface3).
				Radius(6).Border(1, p.line).Children(func() {
				if pi == nil {
					ui.Text(c, "无可用串口").FontSize(11).TextColor(p.muted)
					return
				}
				ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
					dcol := p.faint
					if s.opened {
						dcol = p.green
					}
					dot(c, dcol)
					ui.Text(c, pi.Base).FontSize(11).Bold().NoWrap().Grow(1)
				})
				ui.Text(c, pi.Name).FontSize(9.5).Font("monospace").
					TextColor(p.muted).NoWrap()
				info := pi.Chip
				if pi.VID != "" {
					info += "  " + pi.VID + ":" + pi.PID
				}
				if info != "" {
					ui.Text(c, info).FontSize(9.5).TextColor(p.faint).NoWrap()
				}
			})
			ui.Text(c, "⌘↵ 发送  ·  ⌘K 搜索").FontSize(9).
				TextColor(p.faint)
		})
	})
}

// viewSessionRow renders one session entry in the sidebar: a button to
// switch, with a separate close button beside it.
func (a *app) viewSessionRow(c *ui.Context, i int, s *session) {
	p := a.pal()
	ui.Row(c).Gap(2).AlignItems(ui.Center).Children(func() {
		row := ui.Row(c).Gap(6).Padding(5, 8).AlignItems(ui.Center).
			Radius(6).Grow(1).Role(ui.RoleButton)
		if i == a.active {
			row.Background(p.surface3)
		}
		row.OnClick(func() { a.active = i })
		row.Children(func() {
			dcol := p.faint
			if s.opened {
				dcol = p.green
			}
			dot(c, dcol)
			ui.Column(c).Gap(1).Grow(1).MinWidth(0).Children(func() {
				ui.Text(c, s.name).FontSize(11).TextColor(p.text).NoWrap()
				sub := "未连接"
				if s.opened {
					sub = s.devName
				} else if pi := s.selectedPort(); pi != nil {
					sub = pi.Base
				}
				ui.Text(c, sub).FontSize(9).TextColor(p.faint).NoWrap()
			})
		})
		if len(a.sessions) > 1 {
			ui.Button(c, "×").FontSize(10).Tooltip("关闭会话").
				OnClick(func() { a.closeSession(i) })
		}
	})
}

// viewConfig renders the serial parameter row.
func (a *app) viewConfig(c *ui.Context, s *session) {
	p := a.pal()
	ui.Row(c).Gap(8).AlignItems(ui.End).Padding(8, 10).Wrap().
		Background(p.surface).Children(func() {
		labeledField(c, p, "串口设备", func() {
			ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
				ui.Select(c, &s.portSel, a.portLabels()).
					MinWidth(150).Disabled(s.opened)
				ui.Button(c, "刷新").FontSize(10).Tooltip("刷新设备列表").
					OnClick(a.refreshPortsAsync)
			})
		}).Shrink(1)
		labeledField(c, p, "波特率", func() {
			ui.Combobox(c, &s.baud, baudRates).MinWidth(84).
				OnChange(s.applyMode)
		}).Shrink(1)
		labeledField(c, p, "数据位", func() {
			ui.Select(c, &s.dataBits, []string{"5", "6", "7", "8"}).
				MinWidth(58).OnChange(s.applyMode)
		})
		labeledField(c, p, "校验", func() {
			ui.Select(c, &s.parity,
				[]string{"None", "Odd", "Even", "Mark", "Space"}).
				MinWidth(72).OnChange(s.applyMode)
		})
		labeledField(c, p, "停止位", func() {
			ui.Select(c, &s.stopBits, []string{"1", "1.5", "2"}).
				MinWidth(58).OnChange(s.applyMode)
		})
		ui.Spacer(c).Grow(1)
		ui.Row(c).Gap(4).AlignItems(ui.Center).Children(func() {
			ui.Switch(c, &s.dtr).OnChange(s.setModemBits)
			ui.Text(c, "DTR").FontSize(10).TextColor(p.muted)
		})
		ui.Row(c).Gap(4).AlignItems(ui.Center).Children(func() {
			ui.Switch(c, &s.rts).OnChange(s.setModemBits)
			ui.Text(c, "RTS").FontSize(10).TextColor(p.muted)
		})
		label := "连接设备"
		if s.opened {
			label = "断开连接"
		}
		ui.PrimaryButton(c, label).MinWidth(92).OnClick(s.togglePort)
	})
}

// viewConsoleHead renders the console toolbar.
func (a *app) viewConsoleHead(c *ui.Context, s *session) {
	p := a.pal()
	ui.Row(c).Gap(8).AlignItems(ui.Center).Padding(6, 12).Wrap().
		Background(p.surface2).Children(func() {
		ui.Text(c, "串口监视器").FontSize(12).Bold().NoWrap()
		ui.Segmented(c, &s.dispMode, "文本", "HEX", "混合")
		ui.Toggle(c, &s.showRX, "RX")
		ui.Toggle(c, &s.showTX, "TX")
		pill(c, p, "UTF-8", p.muted, p.surface3)
		ui.Spacer(c).Grow(1)
		ui.Checkbox(c, &s.showTS, "时间戳")
		ui.SearchField(c, &s.query).MinWidth(110).
			Placeholder("搜索日志… ⌘K").Bind(&a.searchH)
		if a.wantFocus {
			a.searchH.Focus(c)
			a.wantFocus = false
		}
		ui.Button(c, "导出").Tooltip("保存日志到文件").OnClick(s.saveLog)
		ui.Toggle(c, &a.showPanel, "☷").Tooltip("快捷命令面板")
	})
}

// viewConsole renders the log list.
func (a *app) viewConsole(c *ui.Context, s *session) {
	p := a.pal()
	s.filterRecords()
	s.rxList.FollowEnd = s.autofollow
	ui.Column(c).Basis(0).Grow(1).MinHeight(0).Background(p.consoleBg).
		Selectable().Children(func() {
		ui.List(c, &s.rxList, len(s.filtered), func(i int) {
			r := s.filtered[i]
			tag, col := a.dirTag(r.dir)
			ui.Row(c).Gap(10).Padding(1, 12).AlignItems(ui.Start).
				Font("monospace").Children(func() {
				if s.showTS {
					ui.Text(c, r.ts.Format("15:04:05.000")).
						FontSize(10.5).TextColor(p.logTime).NoWrap()
				}
				ui.Text(c, tag).FontSize(9.5).Bold().TextColor(col).
					NoWrap().MinWidth(24)
				ui.Text(c, s.recordText(r)).FontSize(11.5).
					TextColor(p.logMsg).Grow(1)
			})
		}).Grow(1)
	})
}

// viewConsoleBottom renders follow/pause and log actions.
func (a *app) viewConsoleBottom(c *ui.Context, s *session) {
	p := a.pal()
	ui.Row(c).Gap(10).AlignItems(ui.Center).Padding(5, 12).
		Background(p.surface2).Children(func() {
		ui.Toggle(c, &s.autofollow, "跟随输出")
		ui.Toggle(c, &s.paused, "暂停显示")
		ui.Textf(c, "已显示 %d 行", len(s.filtered)).FontSize(10).
			TextColor(p.muted)
		ui.Spacer(c).Grow(1)
		svc := c.Services()
		ui.Button(c, "复制").Tooltip("复制全部日志").
			OnClick(func() { s.copyLog(svc) })
		ui.Button(c, "清空").OnClick(func() {
			s.rxRecords = s.rxRecords[:0]
			s.rxCount, s.txCount = 0, 0
		})
		ui.Button(c, "保存日志").OnClick(s.saveLog)
	})
}

// viewComposer renders the send area with tabs and options.
func (a *app) viewComposer(c *ui.Context, s *session) {
	p := a.pal()
	ui.Column(c).Background(p.surface).Children(func() {
		ui.Row(c).AlignItems(ui.Center).Padding(2, 12, 0, 12).
			Children(func() {
				ui.Tabs(c, &s.composeTab, "单条发送", "文件发送", "高级封包")
				ui.Spacer(c).Grow(1)
				ui.Text(c, "⌘↵ 发送").FontSize(10).TextColor(p.faint)
			})
		if s.composeTab == 0 {
			a.viewSendRow(c, s)
		} else {
			ui.Row(c).Padding(18).AlignItems(ui.Center).Children(func() {
				ui.Text(c, "规划中 —— 该功能尚未实现，敬请期待").
					FontSize(11).TextColor(p.muted).Grow(1)
			})
		}
	})
}

// viewSendRow renders the textarea, send button, and option row.
func (a *app) viewSendRow(c *ui.Context, s *session) {
	p := a.pal()
	ui.Row(c).Gap(10).Padding(8, 12).AlignItems(ui.Stretch).Children(func() {
		ui.TextArea(c, &s.sendText).Grow(1).MinHeight(64).
			Font("monospace").FontSize(12).
			Placeholder("输入要发送的数据…（HEX 模式下按十六进制解析）").
			OnChange(s.updateSnap)
		ui.PrimaryButton(c, "发送 ↗").FillHeight().MinWidth(88).
			Disabled(!s.opened).OnClick(s.sendNow)
	})
	ui.Row(c).Gap(10).AlignItems(ui.Center).Padding(4, 12, 10, 12).
		Wrap().Children(func() {
		ui.Checkbox(c, &s.sendHex, "HEX 发送").OnChange(s.updateSnap)
		vdiv(c, p)
		ui.Text(c, "行结束").FontSize(10.5).TextColor(p.muted).NoWrap()
		ui.Select(c, &s.suffix, suffixes).MinWidth(100).
			OnChange(s.updateSnap)
		vdiv(c, p)
		ui.Checkbox(c, &s.periodic, "定时发送").OnChange(s.updateSnap)
		ui.Select(c, &s.interval, intervals).MinWidth(96).
			OnChange(s.updateSnap)
		vdiv(c, p)
		ui.Text(c, "校验").FontSize(10.5).TextColor(p.muted).NoWrap()
		ui.Select(c, &s.checksum, checksums).MinWidth(124).
			OnChange(s.updateSnap)
		ui.Spacer(c).Grow(1)
		ui.Text(c, s.status).FontSize(10.5).TextColor(s.statusColor()).
			NoWrap()
	})
}

// viewPanel renders the right-side command panel.
func (a *app) viewPanel(c *ui.Context, s *session) {
	p := a.pal()
	ui.Column(c).Width(264).FillHeight().Background(p.surface).Children(func() {
		ui.Column(c).Padding(4, 10, 0, 10).Children(func() {
			ui.Tabs(c, &a.rightTab, "多条发送", "自动回复", "小工具")
		})
		if a.rightTab == 0 {
			a.viewCommands(c, s)
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
func (a *app) viewCommands(c *ui.Context, s *session) {
	p := a.pal()
	ui.Column(c).Gap(8).Padding(10).Grow(1).Children(func() {
		ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
			ui.Text(c, "快捷命令组").FontSize(10).TextColor(p.muted)
			ui.Select(c, &a.cmdGroup, cmdGroupNames).Grow(1)
		})
		ui.Scroll(c).Grow(1).Children(func() {
			ui.Column(c).Gap(8).Children(func() {
				for i, cmd := range a.groupCommands() {
					a.viewCmdCard(c, i, cmd, s)
				}
			})
		})
		if a.addOpen {
			a.viewAddForm(c)
		} else {
			ui.Button(c, "＋ 添加快捷命令").OnClick(func() {
				a.addOpen = true
				a.addName, a.addValue = "", ""
				a.addHex = false
				a.addSuffix = sfxCRLF
				a.addChecksum = "None"
			})
		}
		ui.Column(c).Gap(4).Padding(8).Background(p.surface2).
			Radius(6).Border(1, p.line).Children(func() {
			ui.Text(c, "提示").FontSize(10).Bold().TextColor(p.gold)
			ui.Text(c, "每条命令自带发送选项（HEX / 行尾 / 校验）；"+
				"自定义命令保存在内存中。").
				FontSize(10).TextColor(p.muted)
		})
	})
}

// viewCmdCard renders one quick-command card.
func (a *app) viewCmdCard(c *ui.Context, i int, cmd quickCmd, s *session) {
	p := a.pal()
	ui.Column(c).Gap(4).Padding(8).Background(p.surface3).Radius(6).
		Border(1, p.line).Children(func() {
		ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
			ui.Textf(c, "#%02d", i+1).FontSize(9.5).TextColor(p.faint)
			ui.Text(c, cmd.Title).FontSize(11).Bold().Grow(1).NoWrap()
			ui.Button(c, "发送").Disabled(!s.opened).
				OnClick(func() { s.sendCommand(cmd) })
		})
		ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
			ui.Text(c, cmd.Value).FontSize(10).Font("monospace").
				TextColor(p.muted).Grow(1).NoWrap()
			if b := cmd.badges(); b != "" {
				ui.Text(c, b).FontSize(9).TextColor(p.faint).NoWrap()
			}
		})
	})
}

// viewAddForm renders the inline form for a custom command.
func (a *app) viewAddForm(c *ui.Context) {
	p := a.pal()
	ui.Column(c).Gap(6).Padding(8).Background(p.surface2).Radius(6).
		Border(1, p.line).Children(func() {
		ui.TextInput(c, &a.addName).Placeholder("命令名称")
		ui.TextInput(c, &a.addValue).Placeholder("命令内容")
		ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
			ui.Checkbox(c, &a.addHex, "HEX")
			ui.Text(c, "行尾").FontSize(11).TextColor(p.muted).NoWrap()
			ui.Select(c, &a.addSuffix, suffixes).MinWidth(90).Grow(1)
			ui.Text(c, "校验").FontSize(11).TextColor(p.muted).NoWrap()
			ui.Select(c, &a.addChecksum, checksums).MinWidth(90).Grow(1)
		})
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
					quickCmd{Title: name, Value: a.addValue,
						Hex: a.addHex, Suffix: a.addSuffix,
						Checksum: a.addChecksum})
				a.addOpen = false
			})
			ui.Button(c, "取消").OnClick(func() { a.addOpen = false })
		})
	})
}

// viewStatusBar renders the bottom status row.
func (a *app) viewStatusBar(c *ui.Context, s *session) {
	p := a.pal()
	ui.Row(c).Height(28).Padding(0, 14).Gap(14).AlignItems(ui.Center).
		Background(p.surface2).FontSize(10.5).Children(func() {
		dcol := p.faint
		state := "已断开"
		if s != nil && s.opened {
			dcol = p.green
			state = "已连接"
		}
		ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
			dot(c, dcol)
			ui.Text(c, state).TextColor(p.text)
			if s != nil && s.opened {
				ui.Text(c, s.devName).TextColor(p.faint).NoWrap()
			}
		})
		if s != nil {
			ui.Textf(c, "RX %d B", s.rxCount).TextColor(p.muted)
			ui.Textf(c, "TX %d B", s.txCount).TextColor(p.muted)
			ui.Textf(c, "速率 %s bps", s.baud).TextColor(p.muted)
		}
		ui.Spacer(c).Grow(1)
		if s != nil && s.opened {
			ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
				ui.Row(c).Gap(4).AlignItems(ui.Center).Children(func() {
					ui.Text(c, "CTS").TextColor(p.muted)
					dot(c, modemColor(s.cts, p))
				})
				ui.Row(c).Gap(4).AlignItems(ui.Center).Children(func() {
					ui.Text(c, "DSR").TextColor(p.muted)
					dot(c, modemColor(s.dsr, p))
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
