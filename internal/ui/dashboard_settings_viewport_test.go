package ui

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/nelvko/proxyctl/internal/config"
)

func settingsViewportModel(t *testing.T) dashboardModel {
	t.Helper()
	m := appearanceTestModel(t)
	m.width, m.height = 80, 12
	m.snapshot.kernel = "kernel-viewport"
	m.snapshot.service = "service-viewport"
	m.snapshot.profile = "subscription-viewport"
	m.snapshot.configFile = "/tmp/viewport/config.yaml"
	m.snapshot.proxy = "http://127.0.0.1:7890"
	m.snapshot.shell = "shell-viewport"
	m.snapshot.err = errors.New("viewport-read-error")
	m.snapshot.controllerHint = "控制 API 未启用；viewport-api-hint"
	return m
}

func settingsPlainView(m dashboardModel) string {
	return ansi.Strip(m.View().Content)
}

func settingsScrollToBottom(m dashboardModel) dashboardModel {
	for range 100 {
		m, _ = dashboardKey(m, tea.KeyPgDown)
	}
	return m
}

// Locate the visible terminal cells, including the Unicode labels, instead of
// duplicating layout offsets from the implementation.
func settingsVisibleCell(t *testing.T, m dashboardModel, label string) (x, y int) {
	t.Helper()
	for row, line := range strings.Split(settingsPlainView(m), "\n") {
		if index := strings.Index(line, label); index >= 0 {
			return lipgloss.Width(line[:index]), row
		}
	}
	t.Fatalf("%q is not visible in settings: %q", label, settingsPlainView(m))
	return 0, 0
}

func TestSettingsViewportShortWindowReachesCompleteDocument(t *testing.T) {
	m := settingsViewportModel(t)
	expected := []string{
		"主题模式", "配色方案", "恢复默认外观", "kernel-viewport", "service-viewport",
		"subscription-viewport", "/tmp/viewport/config.yaml", "http://127.0.0.1:7890",
		"控制 API    未配置", "shell-viewport", "viewport-read-error", "viewport-api-hint",
		"proxyctl use", "proxyctl start / stop", "proxyctl on / off", "proxyctl sub override edit --global",
	}
	seen := make(map[string]bool, len(expected))
	for range 100 {
		view := settingsPlainView(m)
		for _, text := range expected {
			if strings.Contains(view, text) {
				seen[text] = true
			}
		}
		m, _ = dashboardKey(m, tea.KeyPgDown)
	}
	for _, text := range expected {
		if !seen[text] {
			t.Errorf("short settings document never exposed %q", text)
		}
	}
	bottom := settingsPlainView(m)
	if !strings.Contains(bottom, "proxyctl sub override edit --global") {
		t.Fatalf("scrolling beyond the document hid the last CLI hint: %q", bottom)
	}
	m, _ = dashboardKey(m, tea.KeyPgDown)
	if settingsPlainView(m) != bottom {
		t.Fatal("page down moved beyond the real document bottom")
	}
}

func TestSettingsViewportResizeAtDocumentBottom(t *testing.T) {
	m := settingsScrollToBottom(settingsViewportModel(t))
	for _, size := range [][2]int{{80, 20}, {120, 24}, {48, 9}} {
		next, _ := m.Update(tea.WindowSizeMsg{Width: size[0], Height: size[1]})
		m = next.(dashboardModel)
		view := settingsPlainView(m)
		if size[0] >= 80 && !strings.Contains(view, "proxyctl sub override edit --global") {
			t.Fatalf("growing the viewport left its offset beyond the document end: %q", view)
		}
		lines := strings.Split(m.View().Content, "\n")
		if len(lines) != size[1] {
			t.Fatalf("resized settings height=%d, want %d", len(lines), size[1])
		}
		for _, line := range lines {
			if lipgloss.Width(line) != size[0] {
				t.Fatalf("resized settings row width=%d, want %d: %q", lipgloss.Width(line), size[0], line)
			}
		}
		m = settingsScrollToBottom(m)
	}
}

func TestSettingsViewportScrolledResetAndModeMouseHitVisibleRows(t *testing.T) {
	m := settingsViewportModel(t)
	writes := 0
	m.appearance.save = func(_ context.Context, cfg config.ThemeConfig) error {
		writes++
		if !reflect.DeepEqual(cfg, defaultThemeConfig()) {
			t.Fatalf("reset persisted unexpected configuration: %+v", cfg)
		}
		return nil
	}
	b := m.contentBounds()
	next, _ := m.Update(tea.MouseWheelMsg{Button: tea.MouseWheelDown, X: b.X + 2, Y: b.Y + 2})
	m = next.(dashboardModel)
	x, y := settingsVisibleCell(t, m, "恢复默认外观")
	next, cmd := m.Update(tea.MouseClickMsg{Button: tea.MouseLeft, X: x, Y: y})
	m = next.(dashboardModel)
	if cmd == nil || !m.busy || writes != 0 {
		t.Fatal("clicking the visible scrolled reset row did not submit exactly one deferred action")
	}
	next, _ = m.Update(cmd())
	m = next.(dashboardModel)
	if writes != 1 || m.busy {
		t.Fatal("reset action did not complete exactly once")
	}
	m = settingsScrollToBottom(m)
	// Keyboard focus returns a control to view; mouse routing must follow the
	// rendered trigger after the previous scroll and save changed the layout.
	m, _ = dashboardKey(m, tea.KeyUp)
	m, _ = dashboardKey(m, tea.KeyUp)
	x, y = settingsVisibleCell(t, m, "▸ Auto")
	next, cmd = m.Update(tea.MouseClickMsg{Button: tea.MouseLeft, X: x, Y: y})
	m = next.(dashboardModel)
	if cmd != nil || !strings.Contains(settingsPlainView(m), "▾ Auto") || writes != 1 {
		t.Fatal("visible mode trigger did not open a preview without persisting")
	}
}

func TestSettingsViewportArrowRestoresFocusAndDropdownOwnsWheel(t *testing.T) {
	m := settingsScrollToBottom(settingsViewportModel(t))
	m, _ = dashboardKey(m, tea.KeyDown)
	settingsVisibleCell(t, m, "配色方案")
	m, _ = dashboardKey(m, tea.KeyUp)
	settingsVisibleCell(t, m, "主题模式")
	m, _ = dashboardKey(m, tea.KeyEnter)
	before := settingsPlainView(m)
	settingsVisibleCell(t, m, "▾ Auto")
	b := m.contentBounds()
	for range 10 {
		// The label column belongs to the content pane, outside the select.
		next, _ := m.Update(tea.MouseWheelMsg{Button: tea.MouseWheelDown, X: b.X + 1, Y: b.Y + b.Height - 1})
		m = next.(dashboardModel)
	}
	if after := settingsPlainView(m); after != before {
		t.Fatalf("wheel outside an open dropdown scrolled its background:\nbefore=%q\nafter=%q", before, after)
	}
}

func TestSettingsViewportViewLeavesInteractionStateUnchanged(t *testing.T) {
	m := settingsViewportModel(t)
	m, _ = dashboardKey(m, tea.KeyPgDown)
	before := fmt.Sprintf("%#v", m.appearance)
	view := m.View().Content
	if after := fmt.Sprintf("%#v", m.appearance); after != before {
		t.Fatal("View mutated the settings interaction state")
	}
	if m.View().Content != view {
		t.Fatal("rendering the same settings state produced a different view")
	}
}
