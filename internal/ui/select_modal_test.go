package ui

import (
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

func TestSelectModalSearchConfirmAndCancel(t *testing.T) {
	m := NewSelectModal("选择节点", []SelectOption{{Label: "Tokyo", Value: "jp"}, {Label: "Quiet", Value: "q"}, {Label: "Nord", Value: "nord"}}, "jp")
	m.SetBounds(Rect{X: 20, Y: 2, Width: 40, Height: 12})
	m.Open()
	m.Update(tea.KeyPressMsg{Code: 'q', Text: "q"})
	if !m.Active || m.Query() != "q" || len(m.list.VisibleItems()) != 1 {
		t.Fatalf("search key escaped to global quit: active=%v query=%q count=%d", m.Active, m.Query(), len(m.list.VisibleItems()))
	}
	result, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if !result.Confirmed || result.Value != "q" || m.Active {
		t.Fatalf("confirm: %+v active=%v", result, m.Active)
	}
	m.Open()
	m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	result, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEsc})
	if !result.Cancelled || result.Value != "q" || m.Value != "q" || m.Active {
		t.Fatalf("cancel changed committed selection: %+v modal=%+v", result, m)
	}
}

func TestSelectModalEmptyFilterDisabledAndSmallSizes(t *testing.T) {
	m := NewSelectModal("选择", []SelectOption{{Label: "disabled", Value: "d", Disabled: true}}, "")
	m.SetBounds(Rect{Width: 20, Height: 8})
	m.Open()
	if result, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter}); result.Confirmed {
		t.Fatal("disabled option confirmed")
	}
	m.Update(tea.KeyPressMsg{Code: 'z', Text: "z"})
	if view := ansi.Strip(m.View(Theme{}, 40, 8)); !strings.Contains(view, "没有匹配的选项") {
		t.Fatalf("empty result missing: %q", view)
	}
	if result, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter}); result.Confirmed || !m.Active {
		t.Fatal("empty search confirmed or dismissed")
	}
	for width := 1; width <= 20; width++ {
		for height := 1; height <= 8; height++ {
			view := m.View(Theme{}, width, height)
			if lipgloss.Width(view) != width || lipgloss.Height(view) != height {
				t.Fatalf("modal size %dx%d got %dx%d: %q", width, height, lipgloss.Width(view), lipgloss.Height(view), view)
			}
		}
	}
}

func TestSelectModalMouseAndResizeKeepListCoordinates(t *testing.T) {
	var options []SelectOption
	for i := range 100 {
		options = append(options, SelectOption{Label: fmt.Sprintf("节点 %03d", i), Value: fmt.Sprint(i)})
	}
	m := NewSelectModal("节点", options, "0")
	m.SetBounds(Rect{X: 19, Y: 1, Width: 40, Height: 10})
	m.Open()
	for range 20 {
		m.Update(tea.MouseWheelMsg{Button: tea.MouseWheelDown, X: 21, Y: 4})
	}
	if m.list.Index() != 20 {
		t.Fatalf("wheel selected %d, want 20", m.list.Index())
	}
	m.SetBounds(Rect{X: 0, Y: 2, Width: 30, Height: 7})
	first := m.list.Paginator.Page * m.list.Paginator.PerPage
	view := ansi.Strip(m.View(Theme{}, 30, 7))
	if !strings.Contains(view, options[first].Label) {
		t.Fatalf("resized list missed first visible item %d: %q", first, view)
	}
	result, _ := m.Update(tea.MouseClickMsg{Button: tea.MouseLeft, X: 3, Y: 4})
	if !result.Confirmed || result.Value != options[first].Value {
		t.Fatalf("mouse selected different item from visible row: %+v first=%d", result, first)
	}
	m.Open()
	if result, _ := m.Update(tea.MouseClickMsg{Button: tea.MouseLeft, X: 99, Y: 99}); !result.Cancelled {
		t.Fatal("click outside did not cancel modal")
	}
}

func TestSelectModalNativeFilterKeepsSearchEditable(t *testing.T) {
	m := NewSelectModal("节点", []SelectOption{{Label: "Singapore", Value: "sg"}, {Label: "Tokyo", Value: "jp"}}, "sg")
	m.SetBounds(Rect{Width: 30, Height: 7})
	m.Open()
	m.Update(tea.PasteMsg{Content: "sgp"})
	if len(m.list.VisibleItems()) != 1 || m.list.SelectedItem().(selectItem).Value != "sg" {
		t.Fatalf("native fuzzy filtering did not match Singapore: %+v", m.list.VisibleItems())
	}
	if !m.list.FilterInput.Focused() {
		t.Fatal("filtering blurred the editable search")
	}
	m.Open()
	m.Update(tea.PasteMsg{Content: "ab"})
	m.Update(tea.KeyPressMsg{Code: tea.KeyLeft})
	m.Update(tea.KeyPressMsg{Code: 'x', Text: "x"})
	m.Update(tea.KeyPressMsg{Code: 'y', Text: "y"})
	if m.Query() != "axyb" {
		t.Fatalf("filter refresh moved the text cursor to the end: %q", m.Query())
	}
}

func TestSelectModalNativePagesAndDisabledOptions(t *testing.T) {
	var options []SelectOption
	for i := range 10 {
		options = append(options, SelectOption{Label: fmt.Sprint(i), Value: fmt.Sprint(i), Disabled: i == 3 || i == 7})
	}
	m := NewSelectModal("选择", options, "2")
	m.SetBounds(Rect{Width: 30, Height: 7}) // Four option rows per native List page.
	m.Open()
	result, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyPgDown})
	if !result.Changed || result.Value != "6" {
		t.Fatalf("page down did not preserve the native cursor row: %+v", result)
	}
	result, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyPgUp})
	if !result.Changed || result.Value != "2" {
		t.Fatalf("page up did not restore cursor row: %+v", result)
	}
	result, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	if !result.Changed || result.Value != "4" {
		t.Fatalf("down did not skip disabled option across a page boundary: %+v", result)
	}
	result, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyUp})
	if !result.Changed || result.Value != "2" {
		t.Fatalf("up did not skip disabled option across a page boundary: %+v", result)
	}
	for range 20 {
		m.Update(tea.KeyPressMsg{Code: tea.KeyUp})
	}
	if m.list.Index() != 0 {
		t.Fatalf("native list cursor wrapped at the upper boundary: %d", m.list.Index())
	}
}

func TestSelectModalCannotChooseHiddenOptionsAfterResize(t *testing.T) {
	for _, cramped := range []Rect{{Width: 20, Height: 3}, {Width: 2, Height: 6}, {Width: 0, Height: 6}, {Width: 20, Height: 0}} {
		t.Run(fmt.Sprintf("%dx%d", cramped.Width, cramped.Height), func(t *testing.T) {
			m := NewSelectModal("选择", []SelectOption{{Label: "One", Value: "one"}, {Label: "Two", Value: "two"}}, "one")
			m.SetBounds(Rect{Width: 20, Height: 6})
			m.Open()
			m.SetBounds(cramped)
			if result, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyDown}); result.Changed || m.list.Index() != 0 {
				t.Fatal("navigation selected a hidden option")
			}
			if result, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter}); result.Confirmed || !m.Active || m.Value != "one" {
				t.Fatal("Enter committed a hidden option")
			}
			if cramped.Width > 0 && cramped.Width <= 2 && cramped.Height >= 5 {
				if result, _ := m.Update(tea.MouseClickMsg{Button: tea.MouseLeft, Y: 3}); result.Confirmed || !m.Active || m.list.Index() != 0 {
					t.Fatal("click selected an option with no visible label area")
				}
			}
			m.Update(tea.KeyPressMsg{Code: 't', Text: "t"})
			if m.Query() != "t" || !m.list.FilterInput.Focused() {
				t.Fatal("tiny modal stopped accepting search input")
			}
			m.Update(tea.KeyPressMsg{Code: tea.KeyBackspace})
			m.SetBounds(Rect{Width: 20, Height: 6})
			m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
			if result, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter}); !result.Confirmed || result.Value != "two" {
				t.Fatal("resizing larger did not restore selection")
			}
			m.Open()
			m.SetBounds(cramped)
			if result, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyEsc}); !result.Cancelled || m.Active || m.Value != "two" {
				t.Fatal("tiny modal could not be cancelled with Escape")
			}
			m.Open()
			if result, _ := m.Update(tea.MouseClickMsg{Button: tea.MouseLeft, X: 100, Y: 100}); !result.Cancelled || m.Active {
				t.Fatal("tiny modal could not be cancelled by an outside click")
			}
		})
	}
}
