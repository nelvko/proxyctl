package ui

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/colorprofile"
	"github.com/charmbracelet/x/ansi"
	"github.com/nelvko/proxyctl/internal/config"
)

func appearanceTestModel(t *testing.T) dashboardModel {
	t.Helper()
	t.Setenv("TERM", "xterm-256color")
	t.Setenv("NO_COLOR", "")
	m := newDashboardTestModel(t.Context())
	m.width, m.height = 120, 24
	m.page, m.navFocused, m.loading = dashboardSettingsPage, false, false
	m.themeConfig = config.ThemeConfig{Name: "custom", Colors: map[string]string{"primary": "#ABCDEF"}}
	m.appearance = newAppearanceState(m.themeConfig)
	m.applyTheme()
	return m
}

func dashboardKey(m dashboardModel, code rune) (dashboardModel, tea.Cmd) {
	next, cmd := m.Update(tea.KeyPressMsg{Code: code})
	return next.(dashboardModel), cmd
}

func TestAppearancePreviewCancelDoesNotSave(t *testing.T) {
	m := appearanceTestModel(t)
	original, cfg := m.theme, m.themeConfig
	writes := 0
	m.appearance.save = func(context.Context, config.ThemeConfig) error { writes++; return nil }
	m, _ = dashboardKey(m, tea.KeyDown) // color scheme
	m, _ = dashboardKey(m, tea.KeyEnter)
	m, cmd := dashboardKey(m, tea.KeyHome)
	if cmd != nil || m.theme == original || !reflect.DeepEqual(m.themeConfig, cfg) {
		t.Fatal("preview did not change palette or changed persisted preferences")
	}
	m, _ = dashboardKey(m, tea.KeyEsc)
	if m.theme != original || m.appearance.dropdownOpen() || writes != 0 || m.navFocused {
		t.Fatal("Esc failed to restore the custom theme and select focus without saving")
	}
	m, _ = dashboardKey(m, tea.KeyEnter)
	m, _ = dashboardKey(m, tea.KeyHome)
	m, _ = dashboardKey(m, tea.KeyTab)
	if m.theme != original || m.page != dashboardOverviewPage || writes != 0 {
		t.Fatal("leaving a page retained preview or saved it")
	}
}

func TestAppearanceConfirmSavesOnceAndFailureRestores(t *testing.T) {
	for _, fail := range []bool{false, true} {
		t.Run(map[bool]string{false: "success", true: "failure"}[fail], func(t *testing.T) {
			m := appearanceTestModel(t)
			original := m.theme
			writes := 0
			m.appearance.save = func(_ context.Context, cfg config.ThemeConfig) error {
				writes++
				if cfg.Name != "custom" || cfg.Mode != "light" || cfg.Colors["primary"] != "#ABCDEF" {
					t.Fatalf("saving mode dropped custom preferences: %+v", cfg)
				}
				if fail {
					return errors.New("disk unavailable")
				}
				return nil
			}
			m, _ = dashboardKey(m, tea.KeySpace)
			m, _ = dashboardKey(m, tea.KeyEnd)
			m, cmd := dashboardKey(m, tea.KeyEnter)
			if !m.busy || cmd == nil || writes != 0 {
				t.Fatal("confirmation did not defer persistence into a command")
			}
			m, _ = dashboardKey(m, tea.KeyEnter) // must not submit twice
			next, _ := m.Update(cmd())
			m = next.(dashboardModel)
			if m.busy || writes != 1 {
				t.Fatal("save did not complete exactly once")
			}
			if fail && (m.theme != original || !strings.Contains(m.notice, "保存失败")) {
				t.Fatal("failed save retained unpersisted preview")
			}
			if !fail && (m.theme == original || m.themeConfig.Mode != "light" || m.appearance.previewing) {
				t.Fatal("successful save failed to commit configuration")
			}
		})
	}
}

func TestAppearanceMousePreviewAndResize(t *testing.T) {
	m := appearanceTestModel(t)
	m.navFocused = true
	m.layoutAppearance()
	bounds := m.appearance.scheme.bounds
	next, _ := m.Update(tea.MouseClickMsg{Button: tea.MouseLeft, X: bounds.X, Y: bounds.Y})
	m = next.(dashboardModel)
	if !m.appearance.scheme.Open || m.navFocused {
		t.Fatal("mouse failed to focus and open scheme dropdown")
	}
	next, _ = m.Update(tea.WindowSizeMsg{Width: 42, Height: 10})
	m = next.(dashboardModel)
	m.layoutAppearance()
	bounds = m.appearance.scheme.bounds
	if bounds.Y < m.contentBounds().Y || bounds.Y+bounds.Height > m.height-1 {
		t.Fatalf("open dropdown escaped resized content pane: %+v", bounds)
	}
	writes := 0
	m.appearance.save = func(context.Context, config.ThemeConfig) error { writes++; return nil }
	next, cmd := m.Update(tea.MouseClickMsg{Button: tea.MouseLeft, X: bounds.X, Y: bounds.Y + 1})
	m = next.(dashboardModel)
	if m.appearance.scheme.Open || !m.busy || cmd == nil {
		t.Fatal("mouse did not commit the visible option")
	}
	next, _ = m.Update(cmd())
	if writes != 1 || next.(dashboardModel).busy {
		t.Fatal("mouse selection failed to save")
	}
}

func TestAppearanceResetUsesAutomaticDefaults(t *testing.T) {
	m := appearanceTestModel(t)
	m.appearance.save = func(_ context.Context, cfg config.ThemeConfig) error {
		if !reflect.DeepEqual(cfg, defaultThemeConfig()) {
			t.Fatalf("reset left theme preferences: %+v", cfg)
		}
		return nil
	}
	m.appearance.focus = 2
	m, cmd := dashboardKey(m, tea.KeyEnter)
	next, _ := m.Update(cmd())
	if !reflect.DeepEqual(next.(dashboardModel).themeConfig, defaultThemeConfig()) {
		t.Fatal("reset was not committed")
	}
}

func TestDashboardGridFitsThemePreviewAndTinyWindows(t *testing.T) {
	m := appearanceTestModel(t)
	m.snapshot.profile = "👨‍👩‍👦 中文 é a long subscription"
	for _, profile := range []colorprofile.Profile{colorprofile.TrueColor, colorprofile.ANSI256, colorprofile.ANSI, colorprofile.ASCII} {
		m.colorProfile = profile
		for _, size := range [][2]int{{120, 24}, {80, 24}, {42, 10}, {20, 6}, {5, 3}, {1, 1}} {
			m.width, m.height = size[0], size[1]
			for _, focus := range []int{0, 1, 2} {
				m.appearance.focus = focus
				m.appearance.mode.Open, m.appearance.scheme.Open = focus == 0, focus == 1
				view := m.View().Content
				lines := strings.Split(view, "\n")
				if len(lines) != m.height {
					t.Fatalf("grid %v focus %d: height %d", size, focus, len(lines))
				}
				for _, line := range lines {
					if lipgloss.Width(line) != m.width {
						t.Fatalf("grid %v focus %d: line width %d: %q", size, focus, lipgloss.Width(line), line)
					}
				}
				if profile == colorprofile.ASCII && view != ansi.Strip(view) {
					t.Fatal("ASCII profile retained colors")
				}
			}
		}
	}
}

func TestDashboardMonochromeKeepsSelectionAndHoverKeepsFocus(t *testing.T) {
	m := appearanceTestModel(t)
	m.navFocused = true
	t.Setenv("NO_COLOR", "1")
	view := m.View().Content
	if view != ansi.Strip(view) || !strings.Contains(view, "▶ 7  设置") {
		t.Fatal("monochrome sidebar lost selected page marker")
	}
	next, _ := m.Update(tea.MouseMotionMsg{X: 2, Y: 6})
	m = next.(dashboardModel)
	if m.hoverPage != dashboardNodesPage || m.page != dashboardSettingsPage || !m.navFocused {
		t.Fatal("hover changed keyboard focus or navigation")
	}
	if m.View().MouseMode != tea.MouseModeAllMotion {
		t.Fatal("hover event mode is not enabled")
	}
}

func TestShortSettingsScrollAndReturnToFocusedControl(t *testing.T) {
	m := appearanceTestModel(t)
	m.width, m.height = 48, 12
	m.snapshot.kernel = "kernel-visible-after-scroll"
	seen := false
	for range 8 {
		m, _ = dashboardKey(m, tea.KeyPgDown)
		if strings.Contains(ansi.Strip(m.View().Content), m.snapshot.kernel) {
			seen = true
		}
	}
	if !seen || m.appearance.viewport.YOffset() == 0 {
		t.Fatal("short settings could not scroll to runtime information")
	}
	m.notice = "外观设置已保存"
	m, _ = dashboardKey(m, tea.KeyEnter)
	if m.appearance.viewport.YOffset() != 0 || !m.appearance.mode.Open || m.notice != "" {
		t.Fatal("opening a control did not restore focus visibility and preview hints")
	}
	m, _ = dashboardKey(m, tea.KeyEsc)
	b := m.contentBounds()
	next, _ := m.Update(tea.MouseWheelMsg{Button: tea.MouseWheelDown, X: b.X + 2, Y: b.Y + 2})
	m = next.(dashboardModel)
	if m.appearance.viewport.YOffset() == 0 {
		t.Fatal("settings mouse wheel did not scroll")
	}
}
