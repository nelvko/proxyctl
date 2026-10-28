package ui

import (
	"context"
	"errors"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/nelvko/proxyctl/internal/config"
	"github.com/nelvko/proxyctl/internal/i18n"
)

func localeDashboard(t *testing.T) dashboardModel {
	t.Helper()
	m := appearanceTestModel(t)
	m.language = "zh-CN"
	m.locale = i18n.Chinese
	m.appearance.focus = 3
	return m
}

func TestDashboardLanguagePreviewCancelAndSave(t *testing.T) {
	m := localeDashboard(t)
	theme := m.theme
	saves := 0
	m.appearance.saveLanguage = func(_ context.Context, language string) error {
		saves++
		if language != "en" {
			t.Fatalf("save=%q", language)
		}
		return nil
	}
	m, _ = dashboardKey(m, tea.KeyEnter)
	m, _ = dashboardKey(m, tea.KeyEnd)
	if m.locale != i18n.English || !strings.Contains(ansi.Strip(m.View().Content), "Language") || m.language != "zh-CN" || saves != 0 {
		t.Fatal("preview failed or wrote preferences")
	}
	m, _ = dashboardKey(m, tea.KeyEsc)
	if m.locale != i18n.Chinese || m.theme != theme || saves != 0 {
		t.Fatal("cancel failed to restore language/theme")
	}
	m, _ = dashboardKey(m, tea.KeyEnter)
	m, _ = dashboardKey(m, tea.KeyEnd)
	m, cmd := dashboardKey(m, tea.KeyEnter)
	if cmd == nil || !m.busy || saves != 0 {
		t.Fatal("save should be deferred")
	}
	next, _ := m.Update(cmd())
	m = next.(dashboardModel)
	if m.language != "en" || m.locale != i18n.English || m.busy || saves != 1 || m.theme != theme {
		t.Fatal("save failed or affected theme")
	}
}

func TestDashboardLanguageSaveFailureRestoresLocale(t *testing.T) {
	m := localeDashboard(t)
	m.appearance.saveLanguage = func(context.Context, string) error { return errors.New("disk") }
	m, _ = dashboardKey(m, tea.KeyEnter)
	m, _ = dashboardKey(m, tea.KeyEnd)
	m, cmd := dashboardKey(m, tea.KeyEnter)
	next, _ := m.Update(cmd())
	m = next.(dashboardModel)
	if m.locale != i18n.Chinese || m.language != "zh-CN" || !strings.Contains(m.notice, "语言保存失败") {
		t.Fatal("failed save retained preview")
	}
}

func TestDashboardEnglishNavigationSharesRenderedHitRegions(t *testing.T) {
	m := localeDashboard(t)
	m.locale = i18n.English
	m.language = "en"
	m.page = dashboardOverviewPage
	for _, width := range []int{120, 90, 80, 50, 30, 20} {
		m.width, m.height = width, 24
		view := m.View().Content
		for _, line := range strings.Split(view, "\n") {
			if lipgloss.Width(line) != width {
				t.Fatalf("English row exceeded %d: %q", width, line)
			}
		}
		if dashboardSidebarLayout(width, 24) {
			continue
		}
		mode := dashboardTabLayoutWithLabels(width, m.pageNames())
		x := 0
		for i, label := range m.pageNames() {
			cells := lipgloss.Width(dashboardTab(i, label, i == m.page, mode))
			if x+cells > width {
				break
			}
			next, _ := m.Update(tea.MouseClickMsg{Button: tea.MouseLeft, X: x, Y: 1})
			if next.(dashboardModel).page != i {
				t.Fatalf("width%d hit %d selected %d", width, i, next.(dashboardModel).page)
			}
			x += cells
		}
	}
}

func TestLanguagePreviewRestoresOnPageChangeAndAppearanceResetKeepsLanguage(t *testing.T) {
	m := localeDashboard(t)
	m, _ = dashboardKey(m, tea.KeyEnter)
	m, _ = dashboardKey(m, tea.KeyEnd)
	m, _ = dashboardKey(m, tea.KeyTab)
	if m.locale != i18n.Chinese || m.language != "zh-CN" {
		t.Fatal("page change retained language preview")
	}
	m = localeDashboard(t)
	m.language = "en"
	m.locale = i18n.English
	m.appearance.focus = 2
	m.appearance.save = func(_ context.Context, configTheme config.ThemeConfig) error { return nil }
	m, cmd := dashboardKey(m, tea.KeyEnter)
	next, _ := m.Update(cmd())
	m = next.(dashboardModel)
	if m.language != "en" || m.locale != i18n.English {
		t.Fatal("reset appearance cleared independent language")
	}
}

func TestEnglishFeedbackKeepsStatusSemantics(t *testing.T) {
	m := localeDashboard(t)
	m.locale = i18n.English
	m.language = "en"
	m.busy = true
	next, _ := m.Update(dashboardActionMsg{err: errors.New("unavailable")})
	m = next.(dashboardModel)
	if !m.noticeError {
		t.Fatal("error severity depended on translated message text")
	}
	next, _ = m.Update(dashboardActionMsg{message: "Applied"})
	m = next.(dashboardModel)
	if m.noticeError {
		t.Fatal("success retained prior error severity")
	}
	line := m.tr("settings.error", "unavailable")
	want := lipgloss.NewStyle().Foreground(m.styles.Red).Render(dashboardLine(line, 80, false))
	if got := m.dashboardBodyLine(line, 80); got != want {
		t.Fatal("English status error lost semantic color")
	}
}

func TestEnglishSelectComponents(t *testing.T) {
	d := NewDropdown(nil, "")
	d.Locale = i18n.English
	if view := ansi.Strip(d.View(Theme{}, 40, 1)); !strings.Contains(view, "Choose an option") {
		t.Fatalf("dropdown fallback: %q", view)
	}
	m := NewSelectModal("Select", nil, "")
	m.Locale = i18n.English
	m.SetBounds(Rect{Width: 60, Height: 8})
	m.Open()
	view := ansi.Strip(m.View(Theme{}, 60, 8))
	for _, text := range []string{"Search:", "No matching options", "0 items"} {
		if !strings.Contains(view, text) {
			t.Fatalf("selector missing %q: %q", text, view)
		}
	}
}

func TestDashboardLanguageSelectorDisplaysSavedAliases(t *testing.T) {
	m := newDashboardTestModel(t.Context())
	for _, tc := range []struct{ preference, value string }{
		{"auto", ""}, {"", ""}, {"en_US.UTF-8", "en"}, {"zh_CN.UTF-8", "zh-CN"}, {"missing", "zh-CN"},
	} {
		m.language = tc.preference
		m.layoutAppearance()
		if got := m.appearance.language.Value; got != tc.value {
			t.Fatalf("preference %q displayed as %q, want %q", tc.preference, got, tc.value)
		}
		if m.language != tc.preference {
			t.Fatal("display normalization changed saved preference")
		}
	}
}
