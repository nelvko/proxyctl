package ui

import (
	"image/color"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/colorprofile"
	"github.com/charmbracelet/x/ansi"
	"github.com/nelvko/proxyctl/internal/config"
	"github.com/nelvko/proxyctl/internal/subscription"
)

func TestSubscriptionThemeFollowsBackgroundWithoutChangingSelection(t *testing.T) {
	m := initialModel(t.Context(), subscription.NewService(&config.SubscriptionConfig{
		Profiles: []profile{{Name: "one"}, {Name: "two"}},
	}, nil))
	m.themeConfig = defaultThemeConfig()
	m.applyTheme(true)
	m.list.Select(1)
	dark := m.theme
	next, _ := m.Update(tea.BackgroundColorMsg{Color: color.White})
	m = next.(model)
	if m.theme.Background == dark.Background || m.list.Index() != 1 {
		t.Fatal("background detection failed to update palette or changed list selection")
	}
	m.themeConfig = config.ThemeConfig{Name: "nord"}
	m.applyTheme(true)
	explicit := m.theme
	next, _ = m.Update(tea.BackgroundColorMsg{Color: color.White})
	if next.(model).theme != explicit {
		t.Fatal("background detection overrode an explicit theme")
	}
}

func TestSubscriptionMonochromePreservesKeyboardSelection(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	m := initialModel(t.Context(), subscription.NewService(&config.SubscriptionConfig{
		Profiles: []profile{{Name: "one"}, {Name: "two"}},
	}, nil))
	m.width, m.height = 80, 24
	if view := m.View().Content; view != ansi.Strip(view) || !strings.Contains(view, "one") {
		t.Fatalf("monochrome list is unreadable: %q", view)
	}
	next, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	if next.(model).list.Index() != 1 {
		t.Fatal("monochrome list lost keyboard navigation")
	}
	s := themedHuhStyles(m.theme)
	for _, c := range []color.Color{s.Focused.Title.GetForeground(), s.Focused.FocusedButton.GetBackground(), s.Focused.TextInput.Placeholder.GetForeground()} {
		if _, ok := c.(lipgloss.NoColor); c != nil && !ok {
			t.Fatal("Huh adapter retained default colors with NO_COLOR")
		}
	}
}

func TestSubscriptionASCIIProfileStripsColors(t *testing.T) {
	m := initialModel(t.Context(), subscription.NewService(&config.SubscriptionConfig{}, nil))
	m.width, m.height = 80, 24
	next, _ := m.Update(tea.ColorProfileMsg{Profile: colorprofile.ASCII})
	view := next.(model).View().Content
	if view != ansi.Strip(view) {
		t.Fatal("ASCII profile retained terminal color sequences")
	}
}

func TestSubscriptionRethemeKeepsFormDraft(t *testing.T) {
	m := initialModel(t.Context(), subscription.NewService(&config.SubscriptionConfig{}, nil))
	m.width, m.height = 80, 24
	m.openAddForm()
	m.formDraft.URL = "https://example.com/source"
	m.formData.headers = "X-Token: value"
	form := m.form
	m.themeConfig = config.ThemeConfig{Name: "dracula"}
	m.applyTheme(false)
	if m.form != form || !m.formFocused || m.formDraft.URL != "https://example.com/source" || m.formData.headers != "X-Token: value" {
		t.Fatal("theme update replaced an active form or lost its draft")
	}
}

func TestHuhThemePaintsExplicitBackground(t *testing.T) {
	t.Setenv("TERM", "xterm-256color")
	t.Setenv("NO_COLOR", "")
	light, _ := resolveTheme(config.ThemeConfig{Mode: "light"}, true)
	s := themedHuhStyles(light)
	if s.Form.Base.GetBackground() != themeColor(light.Background) || s.Form.Base.GetForeground() != themeColor(light.Foreground) {
		t.Fatal("light form relies on a potentially dark terminal background")
	}
}
