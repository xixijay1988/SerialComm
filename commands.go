package main

import "strings"

// quickCmd is one preset command in the right panel. Each command
// carries its own send options, independent of the send area's.
type quickCmd struct {
	Title    string
	Value    string
	Hex      bool   // send Value as hex bytes, not text
	Suffix   string // one of the suffixes labels
	Checksum string // one of the checksums labels
}

// badges describes the command's send options for the card, e.g.
// "HEX · CRC16-Modbus" or "CRLF".
func (q quickCmd) badges() string {
	var parts []string
	if q.Hex {
		parts = append(parts, "HEX")
	}
	switch q.Suffix {
	case "CR (\\r)":
		parts = append(parts, "CR")
	case "LF (\\n)":
		parts = append(parts, "LF")
	case "CRLF (\\r\\n)":
		parts = append(parts, "CRLF")
	}
	if q.Checksum != "" && q.Checksum != "None" {
		parts = append(parts, q.Checksum)
	}
	return strings.Join(parts, " · ")
}

var cmdGroupNames = []string{
	"SoC / 调试命令",
	"Modem / AT 指令",
	"Modbus RTU",
	"自定义",
}

const (
	sfxNone = "None"
	sfxCR   = "CR (\\r)"
	sfxLF   = "LF (\\n)"
	sfxCRLF = "CRLF (\\r\\n)"
)

var cmdGroups = map[string][]quickCmd{
	"SoC / 调试命令": {
		{Title: "help", Value: "help", Suffix: sfxCRLF},
		{Title: "查看 DDR 状态", Value: "ddr status", Suffix: sfxCRLF},
		{Title: "内存测试", Value: "dram_bist_test", Suffix: sfxCRLF},
		{Title: "系统信息", Value: "sysinfo", Suffix: sfxCRLF},
		{Title: "软件重启", Value: "reboot", Suffix: sfxCRLF},
		{Title: "查看任务", Value: "tasks", Suffix: sfxCRLF},
		{Title: "读取寄存器", Value: "md 0xD0030014 4", Suffix: sfxCRLF},
	},
	"Modem / AT 指令": {
		{Title: "设备识别", Value: "AT", Suffix: sfxCRLF},
		{Title: "设备序列号", Value: "AT+CGSN", Suffix: sfxCRLF},
		{Title: "SIM 状态", Value: "AT+CPIN?", Suffix: sfxCRLF},
		{Title: "信号质量", Value: "AT+CSQ", Suffix: sfxCRLF},
		{Title: "网络注册", Value: "AT+CEREG?", Suffix: sfxCRLF},
		{Title: "运营商信息", Value: "AT+COPS?", Suffix: sfxCRLF},
	},
	"Modbus RTU": {
		{Title: "读取保持寄存器", Value: "01 03 00 00 00 02", Hex: true, Checksum: "CRC16-Modbus"},
		{Title: "写入单寄存器", Value: "01 06 00 01 00 01", Hex: true, Checksum: "CRC16-Modbus"},
		{Title: "读取线圈", Value: "01 01 00 00 00 08", Hex: true, Checksum: "CRC16-Modbus"},
		{Title: "读取输入寄存器", Value: "01 04 00 00 00 02", Hex: true, Checksum: "CRC16-Modbus"},
	},
}

// groupCommands returns the built-in commands plus the user's additions
// for the group.
func (a *app) groupCommands() []quickCmd {
	cmds := append([]quickCmd{}, cmdGroups[a.cmdGroup]...)
	return append(cmds, a.customCmds[a.cmdGroup]...)
}
