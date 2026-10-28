package ui

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/nelvko/proxyctl/internal/config"
	"github.com/nelvko/proxyctl/internal/i18n"
)

type overviewNodeRequest struct{ group, name string }

func overviewCardInteractionModel(t *testing.T, width, height int) (dashboardModel, <-chan overviewNodeRequest) {
	t.Helper()
	requests := make(chan overviewNodeRequest, 4)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPut && strings.HasPrefix(r.URL.Path, "/proxies/") {
			var payload struct{ Name string }
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Error(err)
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			requests <- overviewNodeRequest{strings.TrimPrefix(r.URL.Path, "/proxies/"), payload.Name}
			w.WriteHeader(http.StatusNoContent)
			return
		}
		if r.URL.Path == "/traffic" {
			fmt.Fprintln(w, `{"up":0,"down":0}`)
			return
		}
		if r.URL.Path == "/connections" {
			fmt.Fprintln(w, `{"connections":[],"uploadTotal":1024,"downloadTotal":2048,"memory":4096}`)
			return
		}
		http.NotFound(w, r)
	}))
	t.Cleanup(server.Close)
	m := overviewTestModel(t, i18n.English, width, height)
	m.snapshot.kernel, m.snapshot.service, m.snapshot.profile = "test-kernel", "运行中", "work-profile"
	m.snapshot.profiles = []config.Profile{{Name: "work-profile", URL: "https://user:password@source.example/private?token=secret", Format: "clash"}}
	m.snapshot.groups = []dashboardProxy{
		{Name: "Alpha group", Type: "Selector", Now: "Node A", All: []string{"Node A", "Node B"}},
		{Name: "Bravo group", Type: "Selector", Now: "Node A", All: []string{"Node A", "Node B"}},
	}
	m.snapshot.proxies = map[string]dashboardProxy{"Node A": {Name: "Node A"}, "Node B": {Name: "Node B"}}
	m.snapshot.controller = &controllerClient{base: server.URL, client: dashboardHTTPClient}
	var err error
	m.snapshot.connections, err = m.snapshot.controller.connections(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	return m, requests
}

func overviewSelectPickerValue(t *testing.T, m dashboardModel, key rune, value string) (dashboardModel, tea.Cmd) {
	t.Helper()
	m, _ = updateInteraction(m, tea.KeyPressMsg{Code: key})
	if m.picker == nil {
		t.Fatalf("%q did not open the overview selector", key)
	}
	m.picker.list.SetFilterText(value)
	return updateInteraction(m, tea.KeyPressMsg{Code: tea.KeyEnter})
}

func TestOverviewCardsEnterScrollAndProfileNavigation(t *testing.T) {
	m, _ := overviewCardInteractionModel(t, 42, 10)
	m, _ = updateInteraction(m, tea.KeyPressMsg{Code: tea.KeyEnter})
	if m.navFocused || m.page != dashboardOverviewPage {
		t.Fatal("Enter did not focus overview content")
	}
	first := ansi.Strip(m.View().Content)
	seen := first
	for range 12 {
		m, _ = updateInteraction(m, tea.KeyPressMsg{Code: tea.KeyPgDown})
		if m.page != dashboardOverviewPage || m.navFocused {
			t.Fatal("overview scrolling changed page/focus")
		}
		seen += ansi.Strip(m.View().Content)
	}
	if first == ansi.Strip(m.View().Content) {
		t.Fatal("small overview did not scroll")
	}
	for _, value := range []string{"work-profile", "source.example", "test-kernel"} {
		if !strings.Contains(seen, value) {
			t.Fatalf("scrolling could not reach %q", value)
		}
	}
	for _, secret := range []string{"password", "token=secret"} {
		if strings.Contains(seen, secret) {
			t.Fatalf("overview cards exposed source credentials %q", secret)
		}
	}
	m, _ = updateInteraction(m, tea.KeyPressMsg{Code: tea.KeyEsc})
	if !m.navFocused {
		t.Fatal("Escape did not return overview focus to navigation")
	}
	m, _ = updateInteraction(m, tea.KeyPressMsg{Code: tea.KeyEnter})
	m, _ = updateInteraction(m, tea.KeyPressMsg{Code: 'p'})
	if m.page != dashboardSubscriptionsPage {
		t.Fatal("overview profile shortcut did not open subscriptions")
	}
	m, _ = updateInteraction(m, tea.KeyPressMsg{Code: '1'})
	m, _ = updateInteraction(m, tea.KeyPressMsg{Code: tea.KeyDown})
	if m.page != dashboardNodesPage || !m.navFocused {
		t.Fatal("overview navigation arrows lost their original page behavior")
	}
	m.stopTraffic()
}

func TestOverviewGroupAndNodePickersRetainPageAndUseExistingAction(t *testing.T) {
	m, requests := overviewCardInteractionModel(t, 120, 24)
	m.navFocused = false
	m, _ = updateInteraction(m, tea.KeyPressMsg{Code: tea.KeyPgDown})
	before := ansi.Strip(m.View().Content)
	m, _ = updateInteraction(m, tea.KeyPressMsg{Code: 'g'})
	if m.picker == nil {
		t.Fatal("group action did not open picker")
	}
	m, _ = updateInteraction(m, tea.KeyPressMsg{Code: 'q', Text: "q"})
	if m.picker == nil || m.page != dashboardOverviewPage || m.picker.Query() != "q" {
		t.Fatal("picker leaked printable quit key to dashboard")
	}
	m, _ = updateInteraction(m, tea.KeyPressMsg{Code: '2', Text: "2"})
	m, _ = updateInteraction(m, tea.KeyPressMsg{Code: tea.KeyTab})
	if m.picker == nil || m.page != dashboardOverviewPage || m.picker.Query() != "q2" {
		t.Fatal("picker leaked digit or Tab to page navigation")
	}
	m, _ = updateInteraction(m, tea.KeyPressMsg{Code: tea.KeyEsc})
	if m.picker != nil || m.navFocused || ansi.Strip(m.View().Content) != before {
		t.Fatal("cancelled group picker damaged overview state")
	}
	m, cmd := overviewSelectPickerValue(t, m, 'g', "Bravo group")
	if cmd != nil || m.picker != nil || m.busy || m.page != dashboardOverviewPage || m.navFocused {
		t.Fatal("group selection changed more than the displayed group")
	}
	select {
	case <-requests:
		t.Fatal("selecting a group mutated its active node")
	default:
	}
	m, cmd = overviewSelectPickerValue(t, m, '/', "Node B")
	if cmd == nil || !m.busy || m.page != dashboardOverviewPage || m.picker != nil {
		t.Fatal("node confirmation did not dispatch action while retaining overview")
	}
	result, ok := cmd().(dashboardActionMsg)
	if !ok || result.err != nil {
		t.Fatalf("overview node action result: %+v", result)
	}
	select {
	case request := <-requests:
		if request.group != "Bravo group" || request.name != "Node B" {
			t.Fatalf("overview applied the wrong stable target: %+v", request)
		}
	default:
		t.Fatal("successful node action did not contact the controller")
	}
	m, _ = updateInteraction(m, result) // Do not run the returned real configuration reload.
	if m.busy || m.page != dashboardOverviewPage || m.navFocused {
		t.Fatal("action completion did not restore overview state")
	}
	m.stopTraffic()
}

func TestOverviewGroupIdentitySurvivesReorderAndStalePickerCannotApply(t *testing.T) {
	m, requests := overviewCardInteractionModel(t, 120, 24)
	m.navFocused = false
	m, _ = overviewSelectPickerValue(t, m, 'g', "Bravo group")
	snapshot := m.snapshot
	snapshot.groups = []dashboardProxy{snapshot.groups[1], snapshot.groups[0]}
	m, _ = updateInteraction(m, dashboardLoadedMsg{page: dashboardOverviewPage, snapshot: snapshot})
	m, _ = updateInteraction(m, tea.KeyPressMsg{Code: '/'})
	if m.picker == nil || !strings.Contains(m.picker.Title, "Bravo group") {
		t.Fatal("snapshot reorder changed the selected group identity")
	}
	m.picker.list.SetFilterText("Node B")
	snapshot = m.snapshot
	snapshot.groups = []dashboardProxy{snapshot.groups[1]}
	m, _ = updateInteraction(m, dashboardLoadedMsg{page: dashboardOverviewPage, snapshot: snapshot})
	m, cmd := updateInteraction(m, tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd != nil || m.busy || m.picker != nil || m.notice != m.tr("dashboard.stale") {
		t.Fatal("stale overview picker did not reject its removed group")
	}
	select {
	case request := <-requests:
		t.Fatalf("stale picker applied to a different group: %+v", request)
	default:
	}
	m.stopTraffic()
}

func overviewContentCells(m dashboardModel) string {
	bounds := m.contentBounds()
	rows := strings.Split(ansi.Strip(m.View().Content), "\n")
	var body []string
	for y := bounds.Y; y < bounds.Y+bounds.Height && y < len(rows); y++ {
		body = append(body, ansi.Cut(rows[y], bounds.X, bounds.X+bounds.Width))
	}
	return strings.Join(body, "\n")
}

func TestOverviewDoesNotRepeatSidebarRates(t *testing.T) {
	m, _ := overviewCardInteractionModel(t, 120, 40)
	m.trafficAvailable = true
	m.traffic = dashboardTraffic{Up: 12345, Down: 67890}
	m.navFocused = false
	view := ansi.Strip(m.View().Content)
	for _, rate := range []string{dashboardRate(m.traffic.Up), dashboardRate(m.traffic.Down)} {
		if !strings.Contains(view, rate) {
			t.Fatalf("sidebar no longer displays %q", rate)
		}
	}
	var body string
	for range 12 {
		body += overviewContentCells(m) + "\n"
		m, _ = updateInteraction(m, tea.KeyPressMsg{Code: tea.KeyPgDown})
	}
	for _, rate := range []string{dashboardRate(m.traffic.Up), dashboardRate(m.traffic.Down)} {
		if strings.Contains(body, rate) {
			t.Fatalf("overview duplicates sidebar rate %q", rate)
		}
	}
	for _, metric := range []string{"1.0 KiB", "2.0 KiB", "4.0 KiB"} {
		if !strings.Contains(body, metric) {
			t.Fatalf("removing rates also hid cumulative/core metric %q", metric)
		}
	}
}

func TestOverviewAutomaticGroupsCannotMutateNodeSelection(t *testing.T) {
	for _, kind := range []string{"URLTest", "Fallback", "LoadBalance", "Relay", "unknown"} {
		t.Run(kind, func(t *testing.T) {
			m, requests := overviewCardInteractionModel(t, 120, 24)
			m.navFocused = false
			m.snapshot.groups[0].Type = kind
			m, _ = overviewSelectPickerValue(t, m, 'g', "Alpha group")
			m, cmd := updateInteraction(m, tea.KeyPressMsg{Code: '/'})
			if cmd != nil || m.busy || m.picker != nil || m.page != dashboardOverviewPage {
				t.Fatalf("read-only %s group accepted node editing", kind)
			}
			select {
			case request := <-requests:
				t.Fatalf("read-only group mutated controller: %+v", request)
			default:
			}
		})
	}
}

func TestOverviewActionBoundsTrackScrollingAndResize(t *testing.T) {
	for _, size := range [][2]int{{42, 10}, {80, 24}, {120, 24}} {
		t.Run(fmt.Sprintf("%dx%d", size[0], size[1]), func(t *testing.T) {
			m, requests := overviewCardInteractionModel(t, size[0], size[1])
			m, _ = updateInteraction(m, tea.KeyPressMsg{Code: tea.KeyEnter})
			found := false
			var lastBounds Rect
			for range 20 {
				view := strings.Split(ansi.Strip(m.View().Content), "\n")
				for _, action := range m.overviewActions() {
					bounds := action.button.Bounds
					content := m.contentBounds()
					if bounds.Width <= 0 || bounds.Height <= 0 || !content.Contains(bounds.X, bounds.Y) || !content.Contains(bounds.X+bounds.Width-1, bounds.Y+bounds.Height-1) {
						t.Fatalf("action %q escaped the visible content: %+v in %+v", action.key, bounds, content)
					}
					cells := ansi.Cut(view[bounds.Y], bounds.X, bounds.X+bounds.Width)
					if !strings.Contains(cells, action.button.Label) {
						t.Fatalf("action %q mouse bounds do not match visible label %q: %q", action.key, action.button.Label, cells)
					}
					if action.key != "g" {
						continue
					}
					lastBounds = bounds
					m, _ = updateInteraction(m, tea.MouseClickMsg{Button: tea.MouseRight, X: bounds.X + 1, Y: bounds.Y})
					if m.picker != nil {
						t.Fatal("right click opened a group picker")
					}
					m, _ = updateInteraction(m, tea.MouseClickMsg{Button: tea.MouseLeft, X: bounds.X + 1, Y: bounds.Y})
					if m.picker == nil || m.navFocused {
						t.Fatal("visible group button did not open its picker with content focus")
					}
					m, _ = updateInteraction(m, tea.KeyPressMsg{Code: tea.KeyEsc})
					found = true
				}
				m, _ = updateInteraction(m, tea.KeyPressMsg{Code: tea.KeyPgDown})
			}
			if !found {
				t.Fatal("group button was not reachable by scrolling")
			}
			m, _ = updateInteraction(m, tea.WindowSizeMsg{Width: 1, Height: 1})
			m, cmd := updateInteraction(m, tea.MouseClickMsg{Button: tea.MouseLeft, X: lastBounds.X + 1, Y: lastBounds.Y})
			if cmd != nil || m.picker != nil || m.busy {
				t.Fatal("resize retained an invisible action hit area")
			}
			select {
			case request := <-requests:
				t.Fatalf("viewing group choices changed controller: %+v", request)
			default:
			}
		})
	}
}

func TestOverviewNodePickerRevalidatesMembershipAndEditability(t *testing.T) {
	for _, change := range []string{"removed-node", "automatic-group"} {
		t.Run(change, func(t *testing.T) {
			m, requests := overviewCardInteractionModel(t, 120, 24)
			m.navFocused = false
			m, _ = overviewSelectPickerValue(t, m, 'g', "Bravo group")
			m, _ = updateInteraction(m, tea.KeyPressMsg{Code: '/'})
			if m.picker == nil {
				t.Fatal("selector group did not allow node selection")
			}
			m.picker.list.SetFilterText("Node B")
			snapshot := m.snapshot
			snapshot.groups = append([]dashboardProxy(nil), snapshot.groups...)
			if change == "removed-node" {
				snapshot.groups[1].All = []string{"Node A"}
			} else {
				snapshot.groups[1].Type = "URLTest"
			}
			m, _ = updateInteraction(m, dashboardLoadedMsg{page: dashboardOverviewPage, snapshot: snapshot})
			m, cmd := updateInteraction(m, tea.KeyPressMsg{Code: tea.KeyEnter})
			if cmd != nil || m.busy || m.picker != nil || m.page != dashboardOverviewPage || m.navFocused {
				t.Fatal("changed group accepted a stale node action or damaged focus")
			}
			select {
			case request := <-requests:
				t.Fatalf("stale node choice reached controller: %+v", request)
			default:
			}
			m.stopTraffic()
		})
	}
}

func TestOverviewWheelScrollsOnlyContentWithoutChangingNavigationFocus(t *testing.T) {
	for _, width := range []int{42, 120} {
		t.Run(fmt.Sprint(width), func(t *testing.T) {
			m, _ := overviewCardInteractionModel(t, width, 10)
			first := overviewContentCells(m)
			m, _ = updateInteraction(m, tea.MouseWheelMsg{Button: tea.MouseWheelDown, X: 0, Y: 0})
			if overviewContentCells(m) != first || !m.navFocused {
				t.Fatal("wheel outside content changed overview state")
			}
			bounds := m.contentBounds()
			m, _ = updateInteraction(m, tea.MouseWheelMsg{Button: tea.MouseWheelDown, X: bounds.X + 1, Y: bounds.Y + 1})
			if overviewContentCells(m) == first || !m.navFocused || m.page != dashboardOverviewPage {
				t.Fatal("content wheel did not scroll independently of keyboard navigation")
			}
			m, _ = updateInteraction(m, tea.MouseWheelMsg{Button: tea.MouseWheelUp, X: bounds.X + 1, Y: bounds.Y + 1})
			if overviewContentCells(m) != first {
				t.Fatal("wheel up did not restore the original content position")
			}
		})
	}
}
