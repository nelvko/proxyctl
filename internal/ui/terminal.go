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

// terminalFrame fills only unspecified cell colors after composing nested
// styles. Wrapping ANSI text in a background style is insufficient: resets
// inside child styles also reset that outer background in Lip Gloss v2.
// Explicit component surfaces and selections keep their own colors.
func terminalFrame(content string, width, height int, theme Theme, profile colorprofile.Profile) string {
	if width <= 0 || height <= 0 {
		return ""
	}
	if !terminalColorsEnabled() || profile == colorprofile.NoTTY || profile == colorprofile.ASCII {
		return ansi.Strip(modalGrid(content, width, height))
	}
	canvas := lipgloss.NewCanvas(width, height).Compose(lipgloss.NewLayer(content))
	foreground, background := themeColor(theme.Foreground), themeColor(theme.Background)
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			cell := canvas.CellAt(x, y)
			// Wide glyph continuation cells must remain zero width.
			if cell == nil || cell.Width == 0 {
				continue
			}
			if cell.Style.Fg == nil {
				cell.Style.Fg = foreground
			}
			if cell.Style.Bg == nil {
				cell.Style.Bg = background
			}
		}
	}
	return modalGrid(canvas.Render(), width, height)
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
