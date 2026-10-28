package ui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

func TestButtonActivationHoverAndDisabledState(t *testing.T) {
	b := Button{Label: "Apply", Bounds: Rect{X: 4, Y: 6, Width: 14, Height: 1}}
	if b.Update(tea.KeyPressMsg{Code: tea.KeyEnter}) {
		t.Fatal("unfocused button handled Enter")
	}
	if b.Update(tea.MouseMotionMsg{X: 5, Y: 6}) || !b.Hovered || b.Focused {
		t.Fatal("hover activated the button or changed keyboard focus")
	}
	if b.Update(tea.MouseClickMsg{Button: tea.MouseRight, X: 5, Y: 6}) || !b.Update(tea.MouseClickMsg{Button: tea.MouseLeft, X: 5, Y: 6}) || b.Focused {
		t.Fatal("button mouse activation changed focus or used the wrong button")
	}
	b.Update(tea.MouseMotionMsg{X: 18, Y: 6})
	if b.Hovered {
		t.Fatal("hover crossed the excluded right edge")
	}
	b.Focused = true
	if !b.Update(tea.KeyPressMsg{Code: tea.KeyEnter}) || !b.Update(tea.KeyPressMsg{Code: tea.KeySpace}) {
		t.Fatal("focused button cannot activate using Enter/Space")
	}
	b.Disabled = true
	for _, msg := range []tea.Msg{tea.MouseMotionMsg{X: 5, Y: 6}, tea.MouseClickMsg{Button: tea.MouseLeft, X: 5, Y: 6}, tea.KeyPressMsg{Code: tea.KeyEnter}} {
		if b.Update(msg) || b.Hovered {
			t.Fatal("disabled button activated or retained hover")
		}
	}
	b.Disabled = false
	b.Bounds.Width = 2
	if b.Update(tea.KeyPressMsg{Code: tea.KeyEnter}) {
		t.Fatal("button with no visible label area activated")
	}
}

func TestButtonThemeStatesFitUnicodeLabels(t *testing.T) {
	theme := appearanceTestModel(t).theme
	b := Button{Label: "👨‍👩‍👦 应用\n订阅", Bounds: Rect{Width: 24, Height: 1}}
	plain := b.View(theme, 24)
	b.Hovered = true
	if hovered := b.View(theme, 24); hovered == plain || ansi.Strip(hovered) != ansi.Strip(plain) {
		t.Fatal("hover did not change only the themed presentation")
	}
	for _, state := range []struct{ focused, disabled bool }{{false, false}, {true, false}, {true, true}} {
		b.Focused, b.Disabled = state.focused, state.disabled
		for width := 1; width <= 24; width++ {
			view := b.View(theme, width)
			if lipgloss.Width(view) != width || strings.Contains(view, "\n") {
				t.Fatalf("button did not fit width %d: %q", width, view)
			}
		}
	}
	t.Setenv("NO_COLOR", "1")
	b.Focused, b.Disabled = true, false
	if !strings.HasPrefix(ansi.Strip(b.View(theme, 24)), "▶ ") {
		t.Fatal("monochrome button lost keyboard focus marker")
	}
}
