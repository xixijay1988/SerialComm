package main

// quickCmd is one preset command in the right panel.
type quickCmd struct {
	Title string
	Value string
}

var cmdGroupNames = []string{
	"SoC / 调试命令",
	"Modem / AT 指令",
	"Modbus RTU",
	"自定义",
}

var cmdGroups = map[string][]quickCmd{
	"SoC / 调试命令": {
		{"help", "help"},
		{"查看 DDR 状态", "ddr status"},
		{"内存测试", "dram_bist_test"},
		{"系统信息", "sysinfo"},
		{"软件重启", "reboot"},
		{"查看任务", "tasks"},
		{"读取寄存器", "md 0xD0030014 4"},
	},
	"Modem / AT 指令": {
		{"设备识别", "AT"},
		{"设备序列号", "AT+CGSN"},
		{"SIM 状态", "AT+CPIN?"},
		{"信号质量", "AT+CSQ"},
		{"网络注册", "AT+CEREG?"},
		{"运营商信息", "AT+COPS?"},
	},
	"Modbus RTU": {
		{"读取保持寄存器", "01 03 00 00 00 02"},
		{"写入单寄存器", "01 06 00 01 00 01"},
		{"读取线圈", "01 01 00 00 00 08"},
		{"读取输入寄存器", "01 04 00 00 00 02"},
	},
}

// groupCommands returns the built-in commands plus the user's additions
// for the group.
func (a *app) groupCommands() []quickCmd {
	cmds := append([]quickCmd{}, cmdGroups[a.cmdGroup]...)
	return append(cmds, a.customCmds[a.cmdGroup]...)
}
