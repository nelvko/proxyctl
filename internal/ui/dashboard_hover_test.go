package ui

import (
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/nelvko/proxyctl/internal/config"
)

func TestAppearanceHoverKeepsKeyboardFocusAndPreferences(t *testing.T) {
	m := appearanceTestModel(t)
	m.layoutAppearance()
	beforeTheme, beforeView := m.theme, m.View().Content
	mode, reset := m.appearance.mode.bounds, m.appearance.reset.Bounds
	next, cmd := m.Update(tea.MouseMotionMsg{X: mode.X, Y: mode.Y})
	m = next.(dashboardModel)
	if cmd != nil || !m.appearance.mode.Hovered || m.appearance.previewing || m.theme != beforeTheme || m.appearance.focus != 0 {
		t.Fatal("trigger hover changed theme preview or keyboard focus")
	}
	next, cmd = m.Update(tea.MouseMotionMsg{X: reset.X + 2, Y: reset.Y})
	m = next.(dashboardModel)
	if cmd != nil || !m.appearance.reset.Hovered || m.appearance.mode.Hovered || m.appearance.focus != 0 || m.appearance.previewing || m.navFocused {
		t.Fatal("reset hover changed focus or submitted an action")
	}
	if m.View().Content == beforeView {
		t.Fatal("reset hover has no visible theme state")
	}
	next, _ = m.Update(tea.MouseMotionMsg{X: 0, Y: 0})
	m = next.(dashboardModel)
	if m.appearance.reset.Hovered || m.appearance.mode.Hovered || m.appearance.scheme.Hovered {
		t.Fatal("leaving appearance controls retained hover")
	}
}

func TestDashboardContentHoverTracksVisibleRowsWithoutSelection(t *testing.T) {
	t.Setenv("TERM", "xterm-256color")
	t.Setenv("NO_COLOR", "")
	for _, width := range []int{80, 120} {
		for _, page := range []int{dashboardNodesPage, dashboardSubscriptionsPage, dashboardConnectionsPage, dashboardRulesPage} {
			t.Run(fmt.Sprintf("%d/%s", width, dashboardPages[page]), func(t *testing.T) {
				m := interactionDashboard(page, width)
				m.navFocused = true
				m.group, m.node, m.profile, m.connection, m.rule = 1, 40, 40, 40, 40
				for i := range 100 {
					m.snapshot.profiles = append(m.snapshot.profiles, config.Profile{Name: fmt.Sprintf("item-%03d", i)})
					connection := dashboardConnection{}
					connection.Metadata.Host = fmt.Sprintf("item-%03d", i)
					m.snapshot.connections.Connections = append(m.snapshot.connections.Connections, connection)
					m.snapshot.rules = append(m.snapshot.rules, dashboardRule{Type: "DOMAIN", Payload: fmt.Sprintf("item-%03d", i)})
				}
				m.snapshot.groups = []dashboardProxy{{Name: "Group 0"}, {Name: "Group 1"}}
				for i := range 100 {
					m.snapshot.groups[1].All = append(m.snapshot.groups[1].All, fmt.Sprintf("item-%03d", i))
				}
				before := m.View().Content
				row := -1
				for i, line := range strings.Split(ansi.Strip(before), "\n") {
					if strings.Contains(line, "item-") && !strings.Contains(line, "▶") {
						row = i
						break
					}
				}
				if row < 0 {
					t.Fatal("no unselected visible row to hover")
				}
				b := m.contentBounds()
				next, cmd := m.Update(tea.MouseMotionMsg{X: b.X + 8, Y: row})
				m = next.(dashboardModel)
				if cmd != nil || !m.navFocused || m.group != 1 || m.node != 40 || m.profile != 40 || m.connection != 40 || m.rule != 40 {
					t.Fatal("row hover changed navigation focus or selection")
				}
				after := m.View().Content
				if after == before || ansi.Strip(after) != ansi.Strip(before) {
					t.Fatalf("hover did not change only the visible row style: same=%v row=%d hit=%+v hover=(%d,%d) colors=%v token=%q", after == before, row, m.dashboardRowAt(b.X+8, row), m.hoverX, m.hoverY, terminalColorsEnabled(), m.theme.Hover)
				}
				oldRows, newRows := strings.Split(before, "\n"), strings.Split(after, "\n")
				for i := range oldRows {
					if i != row && oldRows[i] != newRows[i] {
						t.Fatalf("hover changed another row: %d (hovered %d)", i, row)
					}
				}
				next, _ = m.Update(tea.MouseMotionMsg{X: m.width + 1, Y: row})
				if next.(dashboardModel).View().Content != before {
					t.Fatal("leaving the list retained row hover")
				}
			})
		}
	}
}

func TestDashboardHeaderButtonsInvokeExistingActions(t *testing.T) {
	m := interactionDashboard(dashboardSubscriptionsPage, 80)
	m.snapshot.profiles = []config.Profile{{Name: "Selected"}}
	x, y := settingsVisibleCell(t, m, "[更新 u]")
	m, cmd := updateInteraction(m, tea.MouseClickMsg{Button: tea.MouseLeft, X: x, Y: y})
	if cmd == nil || !m.busy || m.actionCancel == nil || m.profile != 0 {
		t.Fatal("update button did not invoke the existing subscription action")
	}
	m.actionCancel() // Keep the user's real subscription configuration untouched.
	m = interactionDashboard(dashboardLogsPage, 80)
	m.logs = []string{"one", "two"}
	x, y = settingsVisibleCell(t, m, "[暂停 p]")
	m, cmd = updateInteraction(m, tea.MouseClickMsg{Button: tea.MouseLeft, X: x, Y: y})
	if cmd != nil || !m.logPaused || len(m.logs) != 2 {
		t.Fatal("pause button did not preserve the existing log toggle behavior")
	}
	x, y = settingsVisibleCell(t, m, "[清屏 c]")
	m, cmd = updateInteraction(m, tea.MouseClickMsg{Button: tea.MouseLeft, X: x, Y: y})
	if cmd != nil || len(m.logs) != 0 || !m.logPaused {
		t.Fatal("clear button changed more than the log content")
	}
}
