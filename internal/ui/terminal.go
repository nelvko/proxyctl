package ui

import (
	"os"
	"strings"
	"unicode"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/colorprofile"
	"github.com/charmbracelet/x/ansi"
)

func terminalColorsEnabled() bool {
	return os.Getenv("NO_COLOR") == "" && os.Getenv("TERM") != "dumb"
}

// Bubble Tea converts RGB styles to the detected profile. Monochrome output
// retains explicit text markers for selection and status.
func terminalContent(content string, profile colorprofile.Profile) string {
	if !terminalColorsEnabled() || profile == colorprofile.NoTTY || profile == colorprofile.ASCII {
		return ansi.Strip(content)
	}
	return content
}

func singleLine(value string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return ' '
		}
		return r
	}, value)
}

func gridLine(content string, width int) string {
	content = ansi.Truncate(content, max(0, width), "")
	return content + strings.Repeat(" ", max(0, width-lipgloss.Width(content)))
}
