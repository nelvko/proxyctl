package log

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
)

var (
	// 定义你的专属 CLI 样式
	infoStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("pink"))
	errorStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("203"))
)

func Ok(msg string, emoji ...string) {
	e := "😼"
	if len(emoji) > 0 {
		e = emoji[0]
	}
	fmt.Printf("%s %s\n", e, infoStyle.Render(strings.TrimSpace(string(msg))))
}

func Fail(msg string, emoji ...string) {
	e := "❌"
	if len(emoji) > 0 {
		e = emoji[0]
	}

	fmt.Printf("%s %s\n", e, errorStyle.Render(strings.TrimSpace(string(msg))))
}
