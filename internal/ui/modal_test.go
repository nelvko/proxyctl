package ui

import (
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/nelvko/proxyctl/internal/i18n"
)

func modalTestModel() Modal {
	m := NewModal(ModalOptions{Title: "Remove subscription", Body: "Remove office?\nThe current selection will remain unchanged until confirmed.", Kind: ModalConfirm, ConfirmLabel: "Remove"})
	m.Open()
	m.SetBounds(Rect{Width: 80, Height: 24})
	return m
}

func TestModalConfirmDefaultsToCancelAndRestoresOnReopen(t *testing.T) {
	m := modalTestModel()
	result, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if !result.Cancelled || result.Confirmed || m.Active || cmd != nil {
		t.Fatal("the default confirmation focus did not cancel")
	}
	m.Open()
	m.Update(tea.KeyPressMsg{Code: tea.KeyRight})
	if !m.confirm.Focused || m.cancel.Focused {
		t.Fatal("right did not focus confirmation")
	}
	result, _ = m.Update(tea.KeyPressMsg{Code: tea.KeySpace})
	if !result.Confirmed || result.Cancelled || m.Active {
		t.Fatal("focused confirmation did not activate")
	}
	m.Open()
	if m.confirm.Focused || !m.cancel.Focused {
		t.Fatal("reopening retained destructive focus")
	}
	m.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	m.Update(tea.KeyPressMsg{Code: tea.KeyTab, Mod: tea.ModShift})
	if !m.cancel.Focused || m.confirm.Focused {
		t.Fatal("tab navigation did not return to cancel")
	}
	result, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEsc})
	if !result.Cancelled || m.Active {
		t.Fatal("escape did not close confirmation")
	}
}

func TestModalMouseRegionsAndBodyScroll(t *testing.T) {
	m := modalTestModel()
	m.options.Body = strings.Repeat("long detail line\n", 80)
	m.SetBounds(Rect{Width: 80, Height: 24})
	b := m.bodyBounds
	m.Update(tea.MouseWheelMsg{Button: tea.MouseWheelDown, X: b.X, Y: b.Y})
	if m.viewport.YOffset() == 0 {
		t.Fatal("body wheel did not scroll")
	}
	offset := m.viewport.YOffset()
	m.Update(tea.MouseWheelMsg{Button: tea.MouseWheelDown, X: m.bounds.X, Y: m.bounds.Y})
	if m.viewport.YOffset() != offset {
		t.Fatal("wheel on border scrolled the modal body")
	}
	button := m.confirm.Bounds
	m.Update(tea.MouseMotionMsg{X: button.X, Y: button.Y})
	if !m.confirm.Hovered || m.confirm.Focused || !m.cancel.Focused {
		t.Fatal("button hover changed keyboard focus")
	}
	result, _ := m.Update(tea.MouseClickMsg{Button: tea.MouseRight, X: button.X, Y: button.Y})
	if result.Confirmed || result.Cancelled || !m.Active {
		t.Fatal("right click activated a modal button")
	}
	result, _ = m.Update(tea.MouseClickMsg{Button: tea.MouseLeft, X: button.X, Y: button.Y})
	if !result.Confirmed || m.Active {
		t.Fatal("visible confirmation button did not activate")
	}
	m.Open()
	result, _ = m.Update(tea.MouseClickMsg{Button: tea.MouseLeft, X: 0, Y: 0})
	if !result.Cancelled || m.Active {
		t.Fatal("outside click did not dismiss the modal")
	}
}

func TestModalTinyWindowCannotConfirmHiddenContent(t *testing.T) {
	for _, size := range [][2]int{{1, 1}, {2, 10}, {12, 6}, {80, 4}} {
		t.Run(fmt.Sprint(size), func(t *testing.T) {
			m := modalTestModel()
			m.Update(tea.KeyPressMsg{Code: tea.KeyRight})
			m.SetBounds(Rect{Width: size[0], Height: size[1]})
			if !m.confirm.Disabled {
				t.Fatal("unreadable confirmation remained enabled")
			}
			m.Update(tea.KeyPressMsg{Code: tea.KeyRight})
			result, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
			if result.Confirmed {
				t.Fatal("shrinking the window submitted hidden confirmation")
			}
			if !m.Active {
				m.Open()
			}
			m.SetBounds(Rect{Width: 80, Height: 24})
			m.Update(tea.KeyPressMsg{Code: tea.KeyRight})
			result, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
			if !result.Confirmed {
				t.Fatal("restoring the window did not restore confirmation")
			}
			m.Open()
			m.SetBounds(Rect{Width: size[0], Height: size[1]})
			result, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEsc})
			if !result.Cancelled || m.Active {
				t.Fatal("tiny window lost keyboard cancellation")
			}
		})
	}
}

func TestModalOverlayFitsGridAndPreservesBackground(t *testing.T) {
	t.Setenv("TERM", "xterm-256color")
	t.Setenv("NO_COLOR", "")
	theme, _ := resolveTheme(defaultThemeConfig(), true)
	m := NewModal(ModalOptions{Title: "Details 👨‍👩‍👦", Body: strings.Repeat("👨‍👩‍👦 中文 é ✈️\n", 40), Kind: ModalInfo})
	m.Open()
	for _, size := range [][2]int{{120, 40}, {80, 24}, {40, 12}, {20, 8}, {12, 6}, {4, 4}, {2, 2}, {1, 1}} {
		t.Run(fmt.Sprint(size), func(t *testing.T) {
			width, height := size[0], size[1]
			baseLine := lipgloss.NewStyle().Foreground(themeColor(theme.Primary)).Render(dashboardLine("underlying 👨‍👩‍👦 内容", width, false))
			base := strings.Repeat(baseLine+"\n", height-1) + baseLine
			m.SetBounds(Rect{Width: width, Height: height})
			before := fmt.Sprintf("%#v", m)
			overlay := m.Overlay(base, theme, width, height)
			if after := fmt.Sprintf("%#v", m); before != after {
				t.Fatal("rendering changed modal state")
			}
			rows := strings.Split(overlay, "\n")
			if len(rows) != height {
				t.Fatalf("overlay height=%d, want %d", len(rows), height)
			}
			for _, row := range rows {
				if lipgloss.Width(row) != width {
					t.Fatalf("overlay row width=%d, want %d: %q", lipgloss.Width(row), width, row)
				}
			}
			if m.bounds.Y > 0 && ansi.Strip(rows[0]) != ansi.Strip(baseLine) {
				t.Fatal("overlay changed the uncovered background row")
			}
			if width >= 40 && !strings.Contains(ansi.Strip(overlay), "👨‍👩‍👦") {
				t.Fatal("overlay lost a complete grapheme cluster")
			}
			m.Close()
			if inactive := m.Overlay(base, theme, width, height); inactive != base {
				t.Fatal("closing overlay changed underlying content")
			}
			m.Open()
		})
	}
}

func TestModalSanitizesControlsAndClosesDetails(t *testing.T) {
	m := NewModal(ModalOptions{Title: "title\x1b]0;unsafe\a", Body: "one\r\ntwo\x00\t\x1b[31m", Kind: ModalWarning})
	m.Open()
	m.SetBounds(Rect{Width: 80, Height: 24})
	theme, _ := resolveTheme(defaultThemeConfig(), true)
	view := m.View(theme, 80, 24)
	if strings.Contains(view, "\x1b]") || strings.Contains(view, "\x00") || strings.Contains(view, "\x1b[31m") {
		t.Fatal("raw terminal controls entered modal content")
	}
	if !strings.Contains(ansi.Strip(view), "one") || !strings.Contains(ansi.Strip(view), "two") {
		t.Fatal("sanitization lost body lines")
	}
	result, _ := m.Update(tea.KeyPressMsg{Code: 'q'})
	if !result.Cancelled || result.Confirmed || m.Active {
		t.Fatal("q did not close details")
	}
}

func TestModalLocalizesDefaultActionsAndKeepsExplicitLabels(t *testing.T) {
	theme, _ := resolveTheme(defaultThemeConfig(), true)
	for _, test := range []struct {
		locale                i18n.Locale
		confirm, cancel, hint string
	}{
		{"", "确认", "取消", "滚动"},
		{i18n.English, "Confirm", "Cancel", "Scroll"},
	} {
		m := NewModal(ModalOptions{Title: "title", Body: "body", Kind: ModalConfirm, Locale: test.locale})
		m.Open()
		m.SetBounds(Rect{Width: 80, Height: 24})
		view := ansi.Strip(m.View(theme, 80, 24))
		for _, text := range []string{test.confirm, test.cancel, test.hint} {
			if !strings.Contains(view, text) {
				t.Fatalf("locale %q modal missing %q: %q", test.locale, text, view)
			}
		}
		m = NewModal(ModalOptions{Title: "title", Body: "body", Kind: ModalInfo, Locale: test.locale})
		m.Open()
		close := i18n.Text(test.locale, "common.close")
		if view = ansi.Strip(m.View(theme, 80, 24)); !strings.Contains(view, close) {
			t.Fatalf("locale %q details missing localized close action: %q", test.locale, view)
		}
	}
	m := NewModal(ModalOptions{Title: "title", Body: "body", ConfirmLabel: "Delete office", Kind: ModalWarning, Locale: i18n.English})
	m.Open()
	if view := ansi.Strip(m.View(theme, 80, 24)); !strings.Contains(view, "Delete office") || !strings.Contains(view, "Cancel") {
		t.Fatal("localizing defaults changed an explicit action label")
	}
}
