package ui

import (
	"image/color"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/colorprofile"
	"github.com/charmbracelet/x/ansi"
	"github.com/nelvko/proxyctl/internal/config"
	"github.com/nelvko/proxyctl/internal/subscription"
)

func equalTerminalColor(a, b color.Color) bool {
	if a == nil || b == nil {
		return a == b
	}
	ar, ag, ab, aa := a.RGBA()
	br, bg, bb, ba := b.RGBA()
	return ar == br && ag == bg && ab == bb && aa == ba
}

func TestTerminalFrameFillsNestedResetAndBlankCells(t *testing.T) {
	t.Setenv("TERM", "xterm-256color")
	t.Setenv("NO_COLOR", "")
	theme, _ := resolveTheme(defaultThemeConfig(), true)
	title := lipgloss.NewStyle().Foreground(themeColor(theme.Primary)).Bold(true).Render("Title")
	selection := lipgloss.NewStyle().Foreground(themeColor(theme.Foreground)).Background(themeColor(theme.Selected)).Render("Pick")
	input := title + "   " + selection + "\nbody"
	// Reproduce the prior wrapping approach and prove the reset fixture has a
	// default-background cell after the nested title, before the selected item.
	old := lipgloss.NewStyle().Background(themeColor(theme.Background)).Render(input)
	oldCanvas := lipgloss.NewCanvas(20, 4).Compose(lipgloss.NewLayer(old))
	if oldCanvas.CellAt(6, 0).Style.Bg != nil {
		t.Fatal("fixture no longer reproduces nested style background loss")
	}
	frame := terminalFrame(input, 20, 4, theme, colorprofile.TrueColor)
	canvas := lipgloss.NewCanvas(20, 4).Compose(lipgloss.NewLayer(frame))
	for y := 0; y < 4; y++ {
		for x := 0; x < 20; x++ {
			want := themeColor(theme.Background)
			if y == 0 && x >= 8 && x < 12 {
				want = themeColor(theme.Selected)
			}
			if !equalTerminalColor(canvas.CellAt(x, y).Style.Bg, want) {
				t.Fatalf("cell (%d,%d) background not painted", x, y)
			}
		}
	}
	if !equalTerminalColor(canvas.CellAt(0, 0).Style.Fg, themeColor(theme.Primary)) {
		t.Fatal("frame changed an explicit foreground")
	}
	if !equalTerminalColor(canvas.CellAt(0, 1).Style.Fg, themeColor(theme.Foreground)) {
		t.Fatal("frame did not apply default foreground")
	}
	if strings.Contains(frame, "\x1b]") {
		t.Fatal("frame changed terminal defaults with OSC")
	}
}

func TestTerminalFrameKeepsUnicodeGridAndMonochromePolicy(t *testing.T) {
	t.Setenv("TERM", "xterm-256color")
	t.Setenv("NO_COLOR", "")
	theme, _ := resolveTheme(defaultThemeConfig(), false)
	input := lipgloss.NewStyle().Foreground(themeColor(theme.Primary)).Render("👨‍👩‍👦 中文 é")
	for _, size := range [][2]int{{1, 1}, {7, 3}, {40, 8}} {
		frame := terminalFrame(input, size[0], size[1], theme, colorprofile.TrueColor)
		lines := strings.Split(ansi.Strip(frame), "\n")
		if len(lines) != size[1] {
			t.Fatalf("%v incorrect height %d", size, len(lines))
		}
		for _, line := range lines {
			if ansi.StringWidth(line) != size[0] {
				t.Fatalf("%v incorrect width %q", size, line)
			}
		}
		canvas := lipgloss.NewCanvas(size[0], size[1]).Compose(lipgloss.NewLayer(frame))
		for y := 0; y < size[1]; y++ {
			for x := 0; x < size[0]; x++ {
				cell := canvas.CellAt(x, y)
				if cell.Width > 0 && !equalTerminalColor(cell.Style.Bg, themeColor(theme.Background)) {
					t.Fatalf("%v cell (%d,%d) missing light background", size, x, y)
				}
			}
		}
	}
	for _, profile := range []colorprofile.Profile{colorprofile.NoTTY, colorprofile.ASCII} {
		frame := terminalFrame(input, 20, 3, theme, profile)
		if frame != ansi.Strip(frame) {
			t.Fatal("monochrome profile retained styling")
		}
	}
	for _, option := range []struct{ key, value string }{{"NO_COLOR", "1"}, {"TERM", "dumb"}} {
		t.Run(option.key, func(t *testing.T) {
			t.Setenv(option.key, option.value)
			frame := terminalFrame(input, 20, 3, theme, colorprofile.TrueColor)
			if frame != ansi.Strip(frame) {
				t.Fatal("monochrome environment retained styling")
			}
		})
	}
}

func assertTerminalFramePainted(t *testing.T, content string, width, height int) {
	t.Helper()
	canvas := lipgloss.NewCanvas(width, height).Compose(lipgloss.NewLayer(content))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			cell := canvas.CellAt(x, y)
			if cell.Width > 0 && cell.Style.Bg == nil {
				t.Fatalf("cell (%d,%d) inherits terminal default background", x, y)
			}
		}
	}
}

func TestDashboardFramesPaintEveryCell(t *testing.T) {
	t.Setenv("NO_COLOR", "")
	t.Setenv("TERM", "xterm-256color")
	for _, size := range [][2]int{{160, 40}, {80, 24}, {18, 8}} {
		m := newDashboardTestModel(t.Context())
		m.width, m.height = size[0], size[1]
		m.colorProfile = colorprofile.TrueColor
		for page := range dashboardPages {
			m.page = page
			assertTerminalFramePainted(t, m.View().Content, size[0], size[1])
		}
		m.page = dashboardNodesPage
		picker := NewSelectModal("Choose", []SelectOption{{Label: "node", Value: "node"}}, "node")
		m.picker = &picker
		m.layoutPicker()
		m.picker.Open()
		assertTerminalFramePainted(t, m.View().Content, size[0], size[1])
	}
}

func TestSubscriptionFramesPaintEveryCell(t *testing.T) {
	t.Setenv("NO_COLOR", "")
	t.Setenv("TERM", "xterm-256color")
	m := initialModel(t.Context(), subscription.NewService(&config.SubscriptionConfig{Profiles: []profile{{Name: "office", URL: "https://example.org"}}}, nil))
	m.width, m.height = 80, 24
	m.colorProfile = colorprofile.TrueColor
	assertTerminalFramePainted(t, m.View().Content, m.width, m.height)
	m.openAddForm()
	assertTerminalFramePainted(t, m.View().Content, m.width, m.height)
	m.errorMessage("Operation failed")
	assertTerminalFramePainted(t, m.View().Content, m.width, m.height)
}

func TestModalNestedStylesKeepTheirSurface(t *testing.T) {
	t.Setenv("TERM", "xterm-256color")
	t.Setenv("NO_COLOR", "")
	theme, _ := resolveTheme(defaultThemeConfig(), true)
	modal := NewModal(ModalOptions{Title: "Diagnostics", Body: "A detailed error\nwith more than one line", Kind: ModalWarning})
	modal.Open()
	modal.SetBounds(Rect{Width: 80, Height: 24})
	canvas := lipgloss.NewCanvas(modal.bounds.Width, modal.bounds.Height).Compose(lipgloss.NewLayer(modal.View(theme, 80, 24)))
	for y := 0; y < modal.bounds.Height; y++ {
		for x := 0; x < modal.bounds.Width; x++ {
			cell := canvas.CellAt(x, y)
			if cell.Width > 0 && !equalTerminalColor(cell.Style.Bg, themeColor(theme.Surface)) && !equalTerminalColor(cell.Style.Bg, themeColor(theme.Selected)) {
				t.Fatalf("modal cell (%d,%d) lost its panel surface", x, y)
			}
		}
	}
}
