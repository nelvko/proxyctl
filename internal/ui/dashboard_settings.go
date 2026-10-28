package ui

import (
	"strings"

	"github.com/nelvko/proxyctl/internal/redact"
)

// settingsLines shows the settings that proxyctl can currently observe. Changes
// are made with the existing CLI commands, so the dashboard stays read-only.
func (m dashboardModel) settingsLines(height int) []string {
	s := m.snapshot
	apiState := "不可用"
	if s.controller != nil {
		apiState = "已配置"
	} else if strings.HasPrefix(s.controllerHint, "控制 API 未启用") {
		apiState = "未配置"
	}
	configFile := s.configFile
	if configFile == "" {
		configFile = "-"
	}
	compact := height < 16
	lines := []string{"  ◇ 当前设置"}
	if !compact {
		lines = []string{"", "  ◇ 当前设置", ""}
	}
	lines = append(lines,
		"  内核        "+redact.Text(s.kernel),
		"  服务        "+redact.Text(s.service),
		"  当前订阅    "+redact.Text(s.profile),
		"  生效配置    "+redact.Text(configFile),
		"  代理入口    "+redact.Text(s.proxy),
		"  控制 API    "+apiState,
		"  本 shell    "+redact.Text(s.shell),
	)
	if s.err != nil {
		if !compact {
			lines = append(lines, "")
		}
		lines = append(lines, "  状态读取失败: "+redact.Text(s.err.Error()))
	}
	if s.controllerHint != "" {
		if !compact {
			lines = append(lines, "")
		}
		lines = append(lines, "  "+redact.Text(s.controllerHint))
	}
	if !compact && height >= len(lines)+7 {
		lines = append(lines,
			"",
			"  ◇ 修改方式",
			"",
			"  切换内核    proxyctl use",
			"  启停服务    proxyctl start / stop",
			"  代理环境    proxyctl on / off",
			"  配置覆盖    proxyctl sub override edit --global",
		)
	}
	return lines
}
