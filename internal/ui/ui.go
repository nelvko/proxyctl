package ui

import (
	"fmt"
	"os"
	"strings"

	"charm.land/lipgloss/v2"
)

// Package ui prints styled terminal output (not logs).
var (
	// 定义你的专属 CLI 样式
	okStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("pink"))
	failStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("203"))
)

// Ok prints a success line to stdout.
func Ok(msg string) {
	fmt.Printf("😼 %s\n", okStyle.Render(strings.TrimSpace(msg)))
}

// Err prints a notice line to stderr — for shellEval commands whose stdout
// must stay pure shell code.
func Err(msg string) {
	fmt.Fprintf(os.Stderr, "😼 %s\n", okStyle.Render(strings.TrimSpace(msg)))
}

// Fail prints a failure line to stdout.
func Fail(msg string) {
	fmt.Printf("❌ %s\n", failStyle.Render(strings.TrimSpace(msg)))
}
