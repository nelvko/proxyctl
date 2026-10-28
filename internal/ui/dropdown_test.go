package ui

import (
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

func TestDropdownPreviewCommitAndCancel(t *testing.T) {
	d := NewDropdown([]SelectOption{{Label: "Default", Value: "default"}, {Label: "Unavailable", Value: "disabled", Disabled: true}, {Label: "Nord", Value: "nord"}}, "default")
	d.Focused = true
	d.SetBounds(Rect{X: 2, Y: 3, Width: 20, Height: 5})
	d.Update(tea.KeyPressMsg{Code: tea.KeySpace})
	if !d.Open {
		t.Fatal("space did not open dropdown")
	}
	preview := d.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	if !preview.Changed || preview.Value != "nord" || d.Value != "default" {
		t.Fatalf("preview committed or selected disabled option: result=%+v dropdown=%+v", preview, d)
	}
	cancel := d.Update(tea.KeyPressMsg{Code: tea.KeyEsc})
	if !cancel.Cancelled || d.Open || d.Value != "default" || d.Cursor != 0 {
		t.Fatalf("cancel did not restore original selection: %+v %+v", cancel, d)
	}
	d.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	d.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	commit := d.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if !commit.Confirmed || commit.Value != "nord" || d.Value != "nord" || d.Open {
		t.Fatalf("enter did not commit: %+v %+v", commit, d)
	}

	d.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	d.Disabled = true
	if result := d.Update(tea.KeyPressMsg{Code: tea.KeyDown}); !result.Cancelled || d.Open || d.Value != "nord" {
		t.Fatal("disabling an open select retained preview or changed its value")
	}
	d.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	d.Update(tea.MouseClickMsg{Button: tea.MouseLeft, X: 3, Y: 3})
	if d.Open {
		t.Fatal("disabled dropdown opened")
	}
}

func TestDropdownMouseScrollAndResizeShareVisibleRows(t *testing.T) {
	var options []SelectOption
	for i := range 20 {
		options = append(options, SelectOption{Label: fmt.Sprintf("节点 👨‍👩‍👦 %02d", i), Value: fmt.Sprint(i)})
	}
	d := NewDropdown(options, "0")
	d.SetBounds(Rect{X: 5, Y: 4, Width: 14, Height: 4})
	d.Update(tea.MouseClickMsg{Button: tea.MouseLeft, X: 6, Y: 4})
	if !d.Open || !d.Focused {
		t.Fatal("click did not focus/open dropdown")
	}
	for range 7 {
		d.Update(tea.MouseWheelMsg{Button: tea.MouseWheelDown, X: 6, Y: 5})
	}
	if d.Cursor != 7 || d.Height(4) != 4 {
		t.Fatalf("wheel/height: cursor=%d height=%d", d.Cursor, d.Height(4))
	}
	view := d.View(Theme{}, 14, 4)
	lines := strings.Split(view, "\n")
	if len(lines) != 4 || !strings.Contains(ansi.Strip(lines[3]), "07") {
		t.Fatalf("focused row not visible after scroll: %q", view)
	}
	for _, line := range lines {
		if lipgloss.Width(line) != 14 {
			t.Fatalf("wide Unicode label broke width: %q", line)
		}
	}
	startBeforeHover := d.start(4)
	d.Update(tea.MouseMotionMsg{X: 6, Y: 5})
	if d.start(4) != startBeforeHover {
		t.Fatal("hovering the first visible row scrolled the viewport under the pointer")
	}
	d.SetBounds(Rect{X: 5, Y: 4, Width: 14, Height: 3})
	first := d.start(3)
	clicked := d.Update(tea.MouseClickMsg{Button: tea.MouseLeft, X: 6, Y: 5})
	if !clicked.Confirmed || clicked.Value != fmt.Sprint(first) {
		t.Fatalf("resize changed mouse mapping: result=%+v first=%d", clicked, first)
	}
	d.Update(tea.MouseClickMsg{Button: tea.MouseLeft, X: 6, Y: 4})
	if cancelled := d.Update(tea.MouseClickMsg{Button: tea.MouseLeft, X: 0, Y: 0}); !cancelled.Cancelled || d.Open {
		t.Fatal("click outside did not cancel")
	}
}

func TestDropdownDisabledAndEmptyOptionsCannotCommit(t *testing.T) {
	for _, options := range [][]SelectOption{nil, {{Label: "disabled", Value: "x", Disabled: true}}} {
		d := NewDropdown(options, "")
		d.SetBounds(Rect{Width: 20, Height: 5})
		d.Focused = true
		d.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
		if result := d.Update(tea.KeyPressMsg{Code: tea.KeyEnter}); result.Confirmed {
			t.Fatalf("committed unavailable option: %+v", result)
		}
		for width := 1; width <= 5; width++ {
			for height := 1; height <= 3; height++ {
				view := d.View(Theme{}, width, height)
				if lipgloss.Width(view) > width || lipgloss.Height(view) > height {
					t.Fatalf("view exceeded %dx%d: %q", width, height, view)
				}
			}
		}
	}
}

func TestDropdownCannotChooseHiddenOptionsAfterResize(t *testing.T) {
	for _, cramped := range []Rect{{Width: 20, Height: 1}, {Width: 2, Height: 4}, {Width: 0, Height: 4}, {Width: 20, Height: 0}} {
		t.Run(fmt.Sprintf("%dx%d", cramped.Width, cramped.Height), func(t *testing.T) {
			d := NewDropdown([]SelectOption{{Label: "One", Value: "one"}, {Label: "Two", Value: "two"}}, "one")
			d.SetBounds(Rect{Width: 20, Height: 4})
			d.Focused = true
			d.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
			d.SetBounds(cramped)
			if result := d.Update(tea.KeyPressMsg{Code: tea.KeyDown}); result.Changed || d.Cursor != 0 {
				t.Fatal("navigation selected a hidden option")
			}
			if result := d.Update(tea.KeyPressMsg{Code: tea.KeyEnter}); result.Confirmed || !d.Open || d.Value != "one" {
				t.Fatal("Enter committed a hidden option")
			}
			if cramped.Width > 0 && cramped.Width <= 2 && cramped.Height >= 3 {
				if result := d.Update(tea.MouseClickMsg{Button: tea.MouseLeft, Y: 2}); result.Confirmed || !d.Open || d.Cursor != 0 {
					t.Fatal("click selected an option with no visible label area")
				}
			}
			d.SetBounds(Rect{Width: 20, Height: 4})
			d.Update(tea.KeyPressMsg{Code: tea.KeyDown})
			if result := d.Update(tea.KeyPressMsg{Code: tea.KeyEnter}); !result.Confirmed || result.Value != "two" {
				t.Fatal("resizing larger did not restore selection")
			}
			d.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
			d.SetBounds(cramped)
			if result := d.Update(tea.KeyPressMsg{Code: tea.KeyEsc}); !result.Cancelled || d.Open || d.Value != "two" {
				t.Fatal("tiny dropdown could not be cancelled with Escape")
			}
			d.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
			if result := d.Update(tea.MouseClickMsg{Button: tea.MouseLeft, X: 100, Y: 100}); !result.Cancelled || d.Open {
				t.Fatal("tiny dropdown could not be cancelled by an outside click")
			}
			if cramped.Width > 0 && cramped.Height > 0 {
				d.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
				if result := d.Update(tea.MouseClickMsg{Button: tea.MouseLeft}); !result.Cancelled || d.Open {
					t.Fatal("tiny dropdown trigger could not close it")
				}
			}
		})
	}
}

func TestRectExcludesOutsideAndEmptyBounds(t *testing.T) {
	r := Rect{X: 2, Y: 3, Width: 5, Height: 4}
	if !r.Contains(2, 3) || !r.Contains(6, 6) || r.Contains(7, 6) || r.Contains(6, 7) || r.Contains(-1, 3) || (Rect{}).Contains(0, 0) {
		t.Fatal("incorrect half-open terminal bounds")
	}
}

func TestDropdownClosedTriggerHoverDoesNotPreviewOrFocus(t *testing.T) {
	theme := appearanceTestModel(t).theme
	d := NewDropdown([]SelectOption{{Label: "Auto", Value: "auto"}, {Label: "Dark", Value: "dark"}}, "auto")
	d.SetBounds(Rect{X: 14, Y: 4, Width: 24, Height: 4})
	before := d.View(theme, 24, 4)
	result := d.Update(tea.MouseMotionMsg{X: 15, Y: 4})
	if result != (SelectResult{}) || !d.Hovered || d.Open || d.Focused || d.Value != "auto" || d.Cursor != 0 {
		t.Fatal("closed trigger hover changed selection, preview, or keyboard focus")
	}
	if hovered := d.View(theme, 24, 4); hovered == before || ansi.Strip(hovered) != ansi.Strip(before) {
		t.Fatal("closed trigger hover did not change only its theme style")
	}
	d.Update(tea.MouseMotionMsg{X: 15, Y: 5})
	if d.Hovered || d.View(theme, 24, 4) != before {
		t.Fatal("leaving the trigger retained hover")
	}
	d.Disabled = true
	d.Update(tea.MouseMotionMsg{X: 15, Y: 4})
	if d.Hovered {
		t.Fatal("disabled trigger acquired hover")
	}
}
