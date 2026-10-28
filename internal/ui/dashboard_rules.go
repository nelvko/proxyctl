package ui

import (
	"fmt"

	"github.com/nelvko/proxyctl/internal/redact"
)

func (m dashboardModel) ruleLines(height int) []string {
	if m.snapshot.controllerHint != "" && len(m.snapshot.rules) == 0 {
		return []string{"", "  " + m.snapshot.controllerHint}
	}
	rules := m.snapshot.rules
	lines := []string{fmt.Sprintf("  规则 %d 条  ·  r 刷新", len(rules)), ""}
	if len(rules) == 0 {
		return append(lines, "  当前没有规则")
	}
	available := max(0, height-len(lines))
	start := listStart(m.rule, available, len(rules))
	for i := start; i < len(rules) && i < start+available; i++ {
		rule := rules[i]
		mark := " "
		if i == m.rule {
			mark = "▶"
		}
		match := redact.Text(rule.Type)
		if rule.Payload != "" {
			match += "  " + redact.Text(rule.Payload)
		}
		lines = append(lines, fmt.Sprintf("  %s %d  %s  →  %s", mark, i+1, match, redact.Text(rule.Proxy)))
	}
	return lines
}
