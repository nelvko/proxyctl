package ui

import (
	"strings"

	"github.com/nelvko/proxyctl/internal/redact"
)

// settingsLines shows observed runtime settings below the appearance controls.
// Kernel and service changes continue to use the existing CLI commands.
func (m dashboardModel) settingsLines(height int) []string {
	return m.settingsContent(height, false)
}

func (m dashboardModel) settingsContent(height int, includeHelp bool) []string {
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
	lines := []string{m.tr("settings.title")}
	if !compact {
		lines = []string{"", m.tr("settings.title"), ""}
	}
	lines = append(lines,
		m.tr("settings.kernel", redact.Text(m.displayState(s.kernel))),
		m.tr("settings.service", redact.Text(m.displayState(s.service))),
		m.tr("settings.profile", redact.Text(m.displayState(s.profile))),
		m.tr("settings.file", redact.Text(configFile)),
		m.tr("settings.proxy", redact.Text(s.proxy)),
		m.tr("settings.api", m.displayState(apiState)),
		m.tr("settings.shell", redact.Text(m.displayState(s.shell))),
	)
	if s.err != nil {
		if !compact {
			lines = append(lines, "")
		}
		lines = append(lines, m.tr("settings.error", redact.Text(s.err.Error())))
	}
	if s.controllerHint != "" {
		if !compact {
			lines = append(lines, "")
		}
		lines = append(lines, "  "+redact.Text(m.displayHint(s.controllerHint)))
	}
	if includeHelp || !compact && height >= len(lines)+7 {
		lines = append(lines,
			"",
			m.tr("settings.modify"),
			"",
			m.tr("settings.switch"),
			m.tr("settings.service_cmd"),
			m.tr("settings.env_cmd"),
			m.tr("settings.override_cmd"),
		)
	}
	return lines
}
