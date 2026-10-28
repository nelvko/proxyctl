package ui

import (
	"fmt"
	"io"
	"strings"

	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/nelvko/proxyctl/internal/i18n"
)

type selectItem SelectOption

func (i selectItem) FilterValue() string { return i.Label + " " + i.Value }

type selectDelegate struct {
	theme Theme
	value string
}

func (selectDelegate) Height() int                         { return 1 }
func (selectDelegate) Spacing() int                        { return 0 }
func (selectDelegate) Update(tea.Msg, *list.Model) tea.Cmd { return nil }
func (d selectDelegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
	option := item.(selectItem)
	style := lipgloss.NewStyle().Foreground(themeColor(d.theme.Foreground)).Background(themeColor(d.theme.Surface))
	prefix := "  "
	if option.Value == d.value {
		prefix = "✓ "
	}
	if index == m.Index() {
		prefix = "› "
		style = style.Background(themeColor(d.theme.Selected)).Bold(true)
	}
	if option.Disabled {
		style = style.Foreground(themeColor(d.theme.Disabled))
	}
	fmt.Fprint(w, style.Render(dashboardLine(prefix+option.Label, m.Width(), false)))
}

// SelectModal temporarily owns its parent's content pane. The parent keeps its
// navigation state and restores focus after Confirmed or Cancelled.
type SelectModal struct {
	Title   string
	Options []SelectOption
	Value   string
	Active  bool
	Locale  i18n.Locale
	list    list.Model
	bounds  Rect
}

func NewSelectModal(title string, options []SelectOption, selected string) SelectModal {
	l := list.New(nil, selectDelegate{}, 1, 1)
	l.FilterInput.CharLimit = 256
	l.SetShowTitle(false)
	l.SetShowFilter(false)
	l.SetShowHelp(false)
	l.SetShowPagination(false)
	l.SetShowStatusBar(false)
	l.InfiniteScrolling = false
	return SelectModal{Title: title, Options: options, Value: selected, list: l}
}

func (m *SelectModal) Open() tea.Cmd {
	m.Active = true
	items := make([]list.Item, len(m.Options))
	for i, option := range m.Options {
		items[i] = selectItem(option)
	}
	m.list.ResetFilter()
	m.list.SetItems(items)
	m.list.FilterInput.SetValue("")
	m.filter()
	for i, item := range m.list.VisibleItems() {
		option := item.(selectItem)
		if option.Value == m.Value && !option.Disabled {
			m.list.Select(i)
			break
		}
	}
	return m.list.FilterInput.Focus()
}

func (m *SelectModal) Close() {
	m.Active = false
	m.list.FilterInput.Blur()
}

func (m SelectModal) Query() string { return m.list.FilterInput.Value() }

func (m *SelectModal) SetBounds(bounds Rect) {
	m.bounds = bounds
	m.list.FilterInput.Prompt = i18n.Text(m.Locale, "select.search_prompt")
	m.list.FilterInput.Placeholder = i18n.Text(m.Locale, "select.search_placeholder")
	m.list.SetSize(max(1, bounds.Width), max(1, bounds.Height-3))
	m.list.FilterInput.SetWidth(max(1, bounds.Width-lipgloss.Width(m.list.FilterInput.Prompt)))
}

func (m SelectModal) optionsVisible() bool {
	return m.bounds.Width > 2 && m.bounds.Height > 3
}

func (m *SelectModal) filter() {
	// SetFilterText computes Bubbles' filter matches synchronously. Keep the
	// editable input's cursor/focus because SetFilterText moves it to the end.
	input := m.list.FilterInput
	m.list.SetFilterText(strings.TrimSpace(input.Value()))
	m.list.FilterInput = input
	m.skipDisabled(m.list.CursorDown)
}

func (m *SelectModal) preview(index int) SelectResult {
	items := m.list.VisibleItems()
	if !m.optionsVisible() || index < 0 || index >= len(items) || items[index].(selectItem).Disabled || index == m.list.Index() {
		return SelectResult{}
	}
	m.list.Select(index)
	return SelectResult{Changed: true, Value: items[index].(selectItem).Value}
}

func (m *SelectModal) skipDisabled(step func()) {
	for item, ok := m.list.SelectedItem().(selectItem); ok && item.Disabled; item, ok = m.list.SelectedItem().(selectItem) {
		before := m.list.Index()
		step()
		if before == m.list.Index() {
			break
		}
	}
}

// Bubbles owns cursor bounds and pagination; only disabled-option handling is
// modal-specific. Native filtering mode would instead accept/blur on arrows.
func (m *SelectModal) navigate(move, step func()) SelectResult {
	if !m.optionsVisible() {
		return SelectResult{}
	}
	before := m.list.Index()
	move()
	m.skipDisabled(step)
	item, ok := m.list.SelectedItem().(selectItem)
	if !ok || item.Disabled {
		m.list.Select(before)
		return SelectResult{}
	}
	if m.list.Index() == before {
		return SelectResult{}
	}
	return SelectResult{Changed: true, Value: item.Value}
}

func (m *SelectModal) confirm() SelectResult {
	item, ok := m.list.SelectedItem().(selectItem)
	if !m.optionsVisible() || !ok || item.Disabled {
		return SelectResult{}
	}
	m.Value = item.Value
	m.Close()
	return SelectResult{Confirmed: true, Value: item.Value}
}

func (m *SelectModal) cancel() SelectResult {
	m.Close()
	return SelectResult{Cancelled: true, Value: m.Value}
}

func (m *SelectModal) Update(msg tea.Msg) (SelectResult, tea.Cmd) {
	if !m.Active {
		return SelectResult{}, nil
	}
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.String() {
		case "esc":
			return m.cancel(), nil
		case "enter":
			return m.confirm(), nil
		case "up":
			return m.navigate(m.list.CursorUp, m.list.CursorUp), nil
		case "down":
			return m.navigate(m.list.CursorDown, m.list.CursorDown), nil
		case "pgup":
			return m.navigate(m.list.PrevPage, m.list.CursorUp), nil
		case "pgdown":
			return m.navigate(m.list.NextPage, m.list.CursorDown), nil
		}
	case tea.MouseClickMsg:
		if msg.Button != tea.MouseLeft {
			return SelectResult{}, nil
		}
		if !m.bounds.Contains(msg.X, msg.Y) {
			return m.cancel(), nil
		}
		row := msg.Y - m.bounds.Y - 2
		if m.optionsVisible() && row >= 0 && row < max(0, m.bounds.Height-3) {
			items := m.list.VisibleItems()
			start, end := m.list.Paginator.GetSliceBounds(len(items))
			index := start + row
			if index < end && !items[index].(selectItem).Disabled {
				m.list.Select(index)
				return m.confirm(), nil
			}
		}
		return SelectResult{}, nil
	case tea.MouseMotionMsg:
		row := msg.Y - m.bounds.Y - 2
		if m.bounds.Contains(msg.X, msg.Y) && row >= 0 && row < max(0, m.bounds.Height-3) {
			start, _ := m.list.Paginator.GetSliceBounds(len(m.list.VisibleItems()))
			return m.preview(start + row), nil
		}
		return SelectResult{}, nil
	case tea.MouseWheelMsg:
		if m.bounds.Contains(msg.X, msg.Y) {
			if msg.Button == tea.MouseWheelUp {
				return m.navigate(m.list.CursorUp, m.list.CursorUp), nil
			}
			if msg.Button == tea.MouseWheelDown {
				return m.navigate(m.list.CursorDown, m.list.CursorDown), nil
			}
		}
		return SelectResult{}, nil
	}
	previous := m.list.FilterInput.Value()
	var cmd tea.Cmd
	m.list.FilterInput, cmd = m.list.FilterInput.Update(msg)
	if previous != m.list.FilterInput.Value() {
		m.filter()
	}
	return SelectResult{}, cmd
}

func (m SelectModal) View(theme Theme, width, height int) string {
	if !m.Active || width <= 0 || height <= 0 {
		return ""
	}
	m.SetBounds(Rect{Width: width, Height: height})
	m.list.FilterInput.SetStyles(themeTextInputStyles(theme))
	m.list.SetDelegate(selectDelegate{theme: theme, value: m.Value})
	base := lipgloss.NewStyle().Foreground(themeColor(theme.Foreground)).Background(themeColor(theme.Surface))
	muted := base.Foreground(themeColor(theme.Muted))
	lines := []string{base.Foreground(themeColor(theme.Primary)).Bold(true).Render(dashboardLine("◇ "+m.Title, width, false))}
	if height >= 2 {
		lines = append(lines, m.list.FilterInput.View())
	}
	if height >= 4 {
		if len(m.list.VisibleItems()) == 0 {
			lines = append(lines, muted.Render(dashboardLine(i18n.Text(m.Locale, "select.empty"), width, false)))
		} else {
			lines = append(lines, strings.Split(m.list.View(), "\n")...)
		}
	}
	if len(lines) > height-1 && height >= 3 {
		lines = lines[:height-1]
	}
	for len(lines) < height-1 {
		lines = append(lines, "")
	}
	if height >= 3 {
		help := i18n.Text(m.Locale, "select.help", len(m.list.VisibleItems()))
		lines = append(lines, muted.Render(dashboardLine(help, width, false)))
	}
	for i, line := range lines {
		line = ansi.Truncate(line, width, "")
		if n := lipgloss.Width(line); n < width {
			line += base.Render(strings.Repeat(" ", width-n))
		}
		lines[i] = line
	}
	return strings.Join(lines, "\n")
}
