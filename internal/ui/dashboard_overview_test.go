package ui

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/nelvko/proxyctl/internal/config"
	"github.com/nelvko/proxyctl/internal/i18n"
)

func overviewTestModel(t *testing.T, locale i18n.Locale, width, height int) dashboardModel {
	t.Helper()
	m := newDashboardTestModel(t.Context())
	m.locale, m.language = locale, string(locale)
	m.width, m.height, m.page = width, height, dashboardOverviewPage
	m.loading = false
	m.snapshot = dashboardSnapshot{kernel: "未选择", profile: "未选择", service: "-", proxy: "-", shell: "未启用"}
	return m
}

func overviewVisibleField(t *testing.T, m dashboardModel, label string) string {
	t.Helper()
	for _, line := range strings.Split(ansi.Strip(m.View().Content), "\n") {
		if _, tail, ok := strings.Cut(line, label); ok && strings.HasPrefix(tail, " ") {
			value := strings.TrimLeft(tail, " ")
			if !strings.HasPrefix(value, "│") {
				return value
			}
		}
	}
	t.Fatalf("visible overview omitted %q", label)
	return ""
}

func assertOverviewGrid(t *testing.T, m dashboardModel) string {
	t.Helper()
	view := ansi.Strip(m.View().Content)
	lines := strings.Split(view, "\n")
	if len(lines) != max(1, m.height) {
		t.Fatalf("overview %dx%d has %d rows", m.width, m.height, len(lines))
	}
	for row, line := range lines {
		if width := ansi.StringWidth(line); width != max(1, m.width) {
			t.Fatalf("overview row %d width=%d, want %d: %q", row, width, m.width, line)
		}
	}
	return view
}

func TestOverviewLocalizedResponsiveSummaryAndSetup(t *testing.T) {
	for _, locale := range []i18n.Locale{i18n.Chinese, i18n.English} {
		for _, width := range []int{80, 120, 171} {
			for _, failed := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/%d/error=%v", locale, width, failed), func(t *testing.T) {
					m := overviewTestModel(t, locale, width, 24)
					if failed {
						m.snapshot.profileErr = errors.New("invalid subscription metadata")
					}
					view := assertOverviewGrid(t, m)
					m, _ = updateInteraction(m, tea.KeyPressMsg{Code: tea.KeyEnter})
					for range 12 {
						m, _ = updateInteraction(m, tea.KeyPressMsg{Code: tea.KeyPgDown})
						view += "\n" + assertOverviewGrid(t, m)
					}
					for _, key := range []string{"overview.kernel_label", "overview.service_label", "overview.profile_label", "overview.count_label", "overview.start_title"} {
						if !strings.Contains(view, m.tr(key)) {
							t.Fatalf("scrolling could not reach localized %s", key)
						}
					}
					for _, command := range []string{"proxyctl install", "proxyctl sub add <url>"} {
						if !strings.Contains(view, command) {
							t.Fatalf("scrolling could not reach setup guidance %q", command)
						}
					}
				})
			}
		}
	}
}

func TestOverviewMetadataFailureKeepsUnknownSeparateFromZero(t *testing.T) {
	for _, locale := range []i18n.Locale{i18n.Chinese, i18n.English} {
		m := overviewTestModel(t, locale, 120, 24)
		count := func() string { return overviewVisibleField(t, m, m.tr("overview.count_label")) }
		if !strings.HasPrefix(count(), "0") {
			t.Fatal("valid empty metadata did not display zero")
		}
		m.snapshot.profiles = []config.Profile{{Name: "one"}, {Name: "two"}}
		m.snapshot.profile = "one"
		m.snapshot.err = errors.New("kernel configuration unavailable")
		if !strings.HasPrefix(count(), "2") {
			t.Fatal("unrelated error hid a known subscription count")
		}
		m.snapshot.profileErr = errors.New("subscription metadata unavailable")
		if !strings.HasPrefix(count(), "—") {
			t.Fatal("failed metadata read was presented as a known count")
		}
		if selected := overviewVisibleField(t, m, m.tr("overview.profile_label")); !strings.HasPrefix(selected, "—") {
			t.Fatalf("failed metadata read presented a known selected profile for %s: %q", locale, selected)
		}
	}
}

func TestOverviewDiagnosticSummaryAndMouseBounds(t *testing.T) {
	for _, locale := range []i18n.Locale{i18n.Chinese, i18n.English} {
		for _, width := range []int{42, 80, 120, 171} {
			t.Run(fmt.Sprintf("%s/%d", locale, width), func(t *testing.T) {
				m := overviewTestModel(t, locale, width, 24)
				path := "/Users/private/" + strings.Repeat("long-directory/", 15) + "subscriptions.yaml"
				m.snapshot.err = errors.New(path + ": obsolete update key; fetch https://user:password@example.com/private?token=secret")
				m.snapshot.profileErr = m.snapshot.err
				view := assertOverviewGrid(t, m)
				for _, detail := range []string{"/Users/private", "long-directory", "obsolete update", "password", "secret"} {
					if strings.Contains(view, detail) {
						t.Fatalf("summary exposed diagnostic detail %q", detail)
					}
				}
				if !strings.Contains(view, m.tr("overview.profile_warning")) {
					t.Fatal("localized summary was not visible")
				}
				_, bounds := m.overviewDocument(m.contentBounds().Width, m.contentBounds().Height)
				if bounds.Width <= 0 || !m.contentBounds().Contains(bounds.X, bounds.Y) {
					t.Fatal("diagnostic action has no visible content bounds")
				}
				row := strings.Split(view, "\n")[bounds.Y]
				if label := ansi.Cut(row, bounds.X, bounds.X+bounds.Width); !strings.Contains(label, m.tr("overview.details")) {
					t.Fatalf("click bounds differ from displayed diagnostic action: %q", label)
				}
				next, cmd := m.Update(tea.MouseClickMsg{Button: tea.MouseRight, X: bounds.X + 2, Y: bounds.Y})
				m = next.(dashboardModel)
				if cmd != nil || m.dialog.Active {
					t.Fatal("right click activated diagnostic action")
				}
				next, cmd = m.Update(tea.MouseClickMsg{Button: tea.MouseLeft, X: bounds.X + 2, Y: bounds.Y})
				m = next.(dashboardModel)
				if cmd != nil || !m.dialog.Active || m.navFocused {
					t.Fatal("visible action did not open details with content focus")
				}
				if !strings.Contains(m.dialog.options.Body, path) {
					t.Fatal("diagnostic modal lost the full file path")
				}
				for _, secret := range []string{"password", "/private?token", "secret"} {
					if strings.Contains(m.dialog.options.Body, secret) {
						t.Fatalf("diagnostic modal exposed URL credentials: %q", secret)
					}
				}
			})
		}
	}
}

func TestOverviewDiagnosticsOwnInputAndRestoreFocusAfterResize(t *testing.T) {
	for _, focused := range []bool{false, true} {
		for _, closeKey := range []rune{'q', tea.KeyEsc} {
			t.Run(fmt.Sprintf("navigation=%v/close=%d", focused, closeKey), func(t *testing.T) {
				m := overviewTestModel(t, i18n.English, 120, 30)
				m.navFocused = focused
				m.snapshot.err = errors.New(strings.Repeat("diagnostic line\n", 60))
				m, cmd := updateInteraction(m, tea.KeyPressMsg{Code: 'd'})
				if cmd != nil || !m.dialog.Active || m.navFocused != focused {
					t.Fatal("d did not open diagnostics without changing focus")
				}
				for _, key := range []tea.KeyPressMsg{{Code: '7'}, {Code: tea.KeyTab}, {Code: 'r'}, {Code: tea.KeyDown}, {Code: tea.KeyPgDown}} {
					m, cmd = updateInteraction(m, key)
					if cmd != nil || !m.dialog.Active || m.page != dashboardOverviewPage || m.navFocused != focused || m.loading || m.busy {
						t.Fatalf("modal leaked key %q to dashboard", key.String())
					}
				}
				if m.dialog.viewport.YOffset() == 0 {
					t.Fatal("long diagnostics did not scroll from the keyboard")
				}
				m, _ = updateInteraction(m, tea.MouseClickMsg{Button: tea.MouseLeft, X: 2, Y: 6})
				if m.page != dashboardOverviewPage || m.navFocused != focused {
					t.Fatal("modal click changed underlying navigation")
				}
				// The click may close the modal from outside; reopen for resize.
				if !m.dialog.Active {
					m, _ = updateInteraction(m, tea.KeyPressMsg{Code: 'd'})
				}
				for _, size := range [][2]int{{42, 10}, {1, 1}, {171, 40}, {80, 24}} {
					m, _ = updateInteraction(m, tea.WindowSizeMsg{Width: size[0], Height: size[1]})
					assertOverviewGrid(t, m)
					if !m.dialog.Active || m.dialog.frame.Width != size[0] || m.dialog.frame.Height != size[1] {
						t.Fatal("resize closed diagnostics or retained stale input bounds")
					}
				}
				m, cmd = updateInteraction(m, tea.KeyPressMsg{Code: closeKey})
				if cmd != nil || m.dialog.Active || m.page != dashboardOverviewPage || m.navFocused != focused {
					t.Fatal("closing diagnostics did not restore underlying focus")
				}
			})
		}
	}
}

func TestOverviewDiagnosticsRetainOpenedErrorDuringRefresh(t *testing.T) {
	m := overviewTestModel(t, i18n.English, 120, 24)
	m.snapshot.err = errors.New("original read failure")
	m, _ = updateInteraction(m, tea.KeyPressMsg{Code: 'd'})
	m, _ = updateInteraction(m, dashboardLoadedMsg{page: dashboardOverviewPage, snapshot: dashboardSnapshot{kernel: "mihomo", service: "已停止"}})
	if !m.dialog.Active || !strings.Contains(m.dialog.options.Body, "original read failure") {
		t.Fatal("background snapshot replaced the diagnostic being read")
	}
	m, _ = updateInteraction(m, tea.KeyPressMsg{Code: tea.KeyEsc})
	m, cmd := updateInteraction(m, tea.KeyPressMsg{Code: 'd'})
	if cmd != nil || m.dialog.Active {
		t.Fatal("d reopened a diagnostic after the underlying error disappeared")
	}
}
