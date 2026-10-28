package ui

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/colorprofile"
	"github.com/charmbracelet/x/ansi"
)

func TestDashboardSlashBoundariesAdaptToWidth(t *testing.T) {
	t.Setenv("TERM", "xterm-256color")
	t.Setenv("NO_COLOR", "")
	m := newDashboardTestModel(t.Context())
	style := lipgloss.NewStyle().Foreground(themeColor(m.theme.Primary))
	for _, width := range []int{1, 4, 7, 8, 18, 42, 80, 120, 171} {
		for _, label := range []string{"概览", "Nodes / 香港 👨‍👩‍👦", strings.Repeat("long label ", 30)} {
			for _, position := range []lipgloss.Position{lipgloss.Left, lipgloss.Center} {
				line := m.dashboardBoundary(label, width, position, style)
				if strings.Contains(line, "\n") || ansi.StringWidth(line) != width {
					t.Fatalf("%d-column boundary overflow: %q", width, line)
				}
				canvas := lipgloss.NewCanvas(width, 1).Compose(lipgloss.NewLayer(line))
				for x := 0; x < width; x++ {
					cell := canvas.CellAt(x, 0)
					if cell.Width > 0 && !equalTerminalColor(cell.Style.Bg, themeColor(m.theme.Background)) {
						t.Fatalf("boundary regained a solid surface bar at column %d", x)
					}
				}
			}
		}
	}
	slashCount := func(width int) int {
		return strings.Count(ansi.Strip(m.dashboardBoundary("概览", width, lipgloss.Center, style)), "/")
	}
	if slashCount(120)-slashCount(80) != 40 {
		t.Fatal("slash fill did not expand with the terminal width")
	}
	t.Setenv("NO_COLOR", "1")
	line := m.dashboardBoundary("概览", 80, lipgloss.Center, style)
	frame := terminalFrame(line, 80, 1, m.theme, colorprofile.TrueColor)
	if frame != ansi.Strip(frame) || !strings.Contains(frame, "概览") || !strings.Contains(frame, "/") {
		t.Fatal("monochrome boundary lost its title or slash fill")
	}
}
