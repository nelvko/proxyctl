package ui

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strconv"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/nelvko/proxyctl/internal/config"
)

func interactionDashboard(page, width int) dashboardModel {
	m := newDashboardTestModel(context.Background())
	m.page, m.width, m.height = page, width, 24
	m.loading, m.navFocused = false, false
	return m
}

func updateInteraction(m dashboardModel, msg tea.Msg) (dashboardModel, tea.Cmd) {
	next, cmd := m.Update(msg)
	return next.(dashboardModel), cmd
}

func TestDashboardPickerSearchOwnsKeysAndCancelRestoresFocus(t *testing.T) {
	for _, page := range []int{dashboardNodesPage, dashboardSubscriptionsPage} {
		t.Run(dashboardPages[page], func(t *testing.T) {
			m := interactionDashboard(page, 120)
			m.node, m.profile, m.nodeFocus = 1, 1, true
			m.snapshot.groups = []dashboardProxy{{Name: "Group", Now: "Node A", All: []string{"Node A", "Quiet Node"}}}
			m.snapshot.profiles = []config.Profile{{Name: "A"}, {Name: "Quiet subscription"}}
			m, _ = updateInteraction(m, tea.KeyPressMsg{Code: '/', Text: "/"})
			if m.picker == nil || !m.picker.Active {
				t.Fatal("/ did not open picker")
			}
			m, _ = updateInteraction(m, tea.KeyPressMsg{Code: 'q', Text: "q"})
			if m.picker == nil || m.picker.Query() != "q" || !m.picker.Active {
				t.Fatal("search q escaped to dashboard quit")
			}
			m, _ = updateInteraction(m, tea.KeyPressMsg{Code: '2', Text: "2"})
			if m.page != page || m.picker.Query() != "q2" {
				t.Fatal("search digit escaped to page navigation")
			}
			m, _ = updateInteraction(m, tea.PasteMsg{Content: " pasted"})
			if m.picker.Query() != "q2 pasted" {
				t.Fatal("search did not receive paste messages")
			}
			m, _ = updateInteraction(m, tea.KeyPressMsg{Code: tea.KeyEsc})
			if m.picker != nil || m.page != page || m.navFocused || !m.nodeFocus || m.node != 1 || m.profile != 1 {
				t.Fatalf("cancel changed underlying focus/selection: page=%d navigation=%v nodeFocus=%v node=%d profile=%d", m.page, m.navFocused, m.nodeFocus, m.node, m.profile)
			}
		})
	}
}

func TestDashboardNodePickerMouseAppliesSelectedNode(t *testing.T) {
	var method, path, body string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method, path = r.Method, r.URL.EscapedPath()
		payload, _ := io.ReadAll(r.Body)
		body = string(payload)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()
	for _, width := range []int{80, 120} {
		t.Run(fmt.Sprint(width), func(t *testing.T) {
			m := interactionDashboard(dashboardNodesPage, width)
			m.snapshot.groups = []dashboardProxy{{Name: "Group A", Now: "Node 1", All: []string{"Node 1", "Node 2"}}}
			m.snapshot.controller = &controllerClient{base: server.URL, client: server.Client()}
			b := m.contentBounds()
			m, _ = updateInteraction(m, tea.MouseClickMsg{Button: tea.MouseLeft, X: b.X + 3, Y: b.Y})
			if m.picker == nil {
				t.Fatal("search header click did not open node picker")
			}
			var cmd tea.Cmd
			m, cmd = updateInteraction(m, tea.MouseClickMsg{Button: tea.MouseLeft, X: b.X + 3, Y: b.Y + 3})
			if m.picker != nil || !m.busy || !m.nodeFocus || m.node != 1 || cmd == nil {
				t.Fatalf("picker click did not invoke node action: picker=%v busy=%v node=%d cmd=%v", m.picker, m.busy, m.node, cmd != nil)
			}
			defer m.actionCancel()
			result, ok := cmd().(dashboardActionMsg)
			if !ok || result.err != nil || method != http.MethodPut || path != "/proxies/Group%20A" || body != `{"name":"Node 2"}` {
				t.Fatalf("wrong controller selection: result=%+v method=%q path=%q body=%q", result, method, path, body)
			}
		})
	}
}

func TestDashboardSubscriptionPickerMouseStartsExistingApplyAction(t *testing.T) {
	m := interactionDashboard(dashboardSubscriptionsPage, 120)
	m.snapshot.profiles = []config.Profile{{Name: "One"}, {Name: "Two"}}
	b := m.contentBounds()
	m, _ = updateInteraction(m, tea.MouseClickMsg{Button: tea.MouseLeft, X: b.X + 4, Y: b.Y})
	if m.picker == nil {
		t.Fatal("search header click did not open subscription picker")
	}
	m, cmd := updateInteraction(m, tea.MouseClickMsg{Button: tea.MouseLeft, X: b.X + 4, Y: b.Y + 3})
	if m.picker != nil || m.profile != 1 || !m.busy || cmd == nil || m.actionCancel == nil {
		t.Fatalf("subscription selection did not start normal apply: profile=%d busy=%v cmd=%v", m.profile, m.busy, cmd != nil)
	}
	// Do not execute the command against the user's real subscription config.
	m.actionCancel()
}

func TestDashboardPickerRejectsOptionsRemovedDuringRefresh(t *testing.T) {
	for _, page := range []int{dashboardNodesPage, dashboardSubscriptionsPage} {
		t.Run(dashboardPages[page], func(t *testing.T) {
			m := interactionDashboard(page, 120)
			m.snapshot.groups = []dashboardProxy{{Name: "Group", Now: "A", All: []string{"A", "B"}}}
			m.snapshot.profiles = []config.Profile{{Name: "A"}, {Name: "B"}}
			m, _ = updateInteraction(m, tea.KeyPressMsg{Code: '/', Text: "/"})
			m, _ = updateInteraction(m, tea.KeyPressMsg{Code: tea.KeyDown})
			refreshed := dashboardSnapshot{groups: []dashboardProxy{{Name: "Group", Now: "A", All: []string{"A"}}}, profiles: []config.Profile{{Name: "A"}}}
			m, _ = updateInteraction(m, dashboardLoadedMsg{page: page, snapshot: refreshed})
			m, cmd := updateInteraction(m, tea.KeyPressMsg{Code: tea.KeyEnter})
			if m.picker != nil || m.busy || cmd != nil || !strings.Contains(m.notice, "已变化") {
				t.Fatalf("stale picker applied an unrelated item: busy=%v cmd=%v notice=%q", m.busy, cmd != nil, m.notice)
			}
		})
	}
}

func rowItemIndex(t *testing.T, m dashboardModel, row int, prefix string) int {
	t.Helper()
	line := strings.Split(ansi.Strip(m.View().Content), "\n")[row]
	match := regexp.MustCompile(regexp.QuoteMeta(prefix) + `(\d+)`).FindStringSubmatch(line)
	if len(match) != 2 {
		t.Fatalf("row %d has no %s item: %q", row, prefix, line)
	}
	index, err := strconv.Atoi(match[1])
	if err != nil {
		t.Fatal(err)
	}
	return index
}

func TestDashboardListMouseMappingMatchesRenderedRows(t *testing.T) {
	for _, width := range []int{80, 120} {
		for _, page := range []int{dashboardSubscriptionsPage, dashboardConnectionsPage, dashboardRulesPage} {
			t.Run(fmt.Sprintf("%d/%s", width, dashboardPages[page]), func(t *testing.T) {
				m := interactionDashboard(page, width)
				m.profile, m.connection, m.rule = 40, 40, 40
				for i := range 100 {
					m.snapshot.profiles = append(m.snapshot.profiles, config.Profile{Name: fmt.Sprintf("item-%03d", i)})
					connection := dashboardConnection{}
					connection.Metadata.Host = fmt.Sprintf("item-%03d", i)
					m.snapshot.connections.Connections = append(m.snapshot.connections.Connections, connection)
					m.snapshot.rules = append(m.snapshot.rules, dashboardRule{Type: "DOMAIN", Payload: fmt.Sprintf("item-%03d", i)})
				}
				b := m.contentBounds()
				clickY := b.Y + 3
				expected := rowItemIndex(t, m, clickY, "item-")
				m, _ = updateInteraction(m, tea.MouseClickMsg{Button: tea.MouseLeft, X: b.X + 6, Y: clickY})
				selected := func() int {
					switch page {
					case dashboardSubscriptionsPage:
						return m.profile
					case dashboardConnectionsPage:
						return m.connection
					default:
						return m.rule
					}
				}
				if selected() != expected {
					t.Fatalf("clicked item %d, selected %d", expected, selected())
				}
				m, _ = updateInteraction(m, tea.MouseWheelMsg{Button: tea.MouseWheelDown, X: b.X + 6, Y: clickY})
				if selected() != expected+1 {
					t.Fatalf("wheel selected %d, want %d", selected(), expected+1)
				}
				m, _ = updateInteraction(m, tea.MouseWheelMsg{Button: tea.MouseWheelUp, X: m.width + 1, Y: clickY})
				if selected() != expected+1 {
					t.Fatal("wheel outside content changed selection")
				}
			})
		}
	}
}

func TestDashboardNodeMouseMappingTracksGroupAndNodeViewport(t *testing.T) {
	for _, width := range []int{80, 120} {
		t.Run(fmt.Sprint(width), func(t *testing.T) {
			m := interactionDashboard(dashboardNodesPage, width)
			for i := range 20 {
				group := dashboardProxy{Name: fmt.Sprintf("group-%03d", i)}
				for j := range 100 {
					group.All = append(group.All, fmt.Sprintf("node-%03d", j))
				}
				m.snapshot.groups = append(m.snapshot.groups, group)
			}
			m.group, m.node = 11, 40
			b := m.contentBounds()
			expectedGroup := rowItemIndex(t, m, b.Y+1, "group-")
			m, _ = updateInteraction(m, tea.MouseClickMsg{Button: tea.MouseLeft, X: b.X + 5, Y: b.Y + 1})
			if m.group != expectedGroup || m.node != 0 || m.nodeFocus {
				t.Fatalf("group click: group=%d want=%d node=%d focus=%v", m.group, expectedGroup, m.node, m.nodeFocus)
			}
			m.node = 40
			clickY := -1
			for row, line := range strings.Split(ansi.Strip(m.View().Content), "\n") {
				if strings.Contains(line, "node-") {
					clickY = row
					break
				}
			}
			if clickY < 0 {
				t.Fatal("node list not rendered")
			}
			expectedNode := rowItemIndex(t, m, clickY, "node-")
			m, _ = updateInteraction(m, tea.MouseClickMsg{Button: tea.MouseLeft, X: b.X + 5, Y: clickY})
			if m.node != expectedNode || !m.nodeFocus {
				t.Fatalf("node click selected %d want=%d focus=%v", m.node, expectedNode, m.nodeFocus)
			}
			m, _ = updateInteraction(m, tea.MouseWheelMsg{Button: tea.MouseWheelDown, X: b.X + 5, Y: clickY})
			if m.node != expectedNode+1 || !m.nodeFocus {
				t.Fatalf("node wheel selected %d want=%d focus=%v", m.node, expectedNode+1, m.nodeFocus)
			}
		})
	}
}
