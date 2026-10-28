package ui

import (
	"strings"

	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/nelvko/proxyctl/internal/i18n"
)

// Dropdown is an inline select. Opening it pushes the following rows down,
// avoiding overlay/cell-width assumptions in the parent view.
type Dropdown struct {
	Options                 []SelectOption
	Value                   string
	Cursor                  int
	Open, Disabled, Focused bool
	Hovered                 bool
	MaxVisible              int
	Locale                  i18n.Locale
	bounds                  Rect
	viewport                viewport.Model
}

func NewDropdown(options []SelectOption, selected string) Dropdown {
	d := Dropdown{Options: options, Value: selected, MaxVisible: 6, viewport: viewport.New()}
	d.restoreCursor()
	return d
}

func (d *Dropdown) SetBounds(bounds Rect) {
	d.bounds = bounds
	d.layoutOptions(bounds.Width, bounds.Height)
}

func (d *Dropdown) restoreCursor() {
	d.Cursor = optionIndex(d.Options, d.Value)
	if d.Cursor < 0 || d.Options[d.Cursor].Disabled {
		d.Cursor = enabledOption(d.Options, 0, 1)
	}
	d.layoutOptions(d.bounds.Width, d.bounds.Height)
}

func (d Dropdown) visible(height int) int {
	return min(max(0, height-1), len(d.Options), max(1, d.MaxVisible))
}

func (d Dropdown) optionsVisible() bool {
	return d.bounds.Width > 2 && d.visible(d.bounds.Height) > 0
}

func (d Dropdown) start(height int) int {
	d.layoutOptions(d.bounds.Width, height)
	return d.viewport.YOffset()
}

func (d *Dropdown) layoutOptions(width, height int) {
	d.viewport.SetWidth(max(0, width))
	d.viewport.SetHeight(d.visible(height))
	lines := make([]string, len(d.Options))
	for i, option := range d.Options {
		prefix := "  "
		if option.Value == d.Value {
			prefix = "✓ "
		}
		if i == d.Cursor {
			prefix = "› "
		}
		line := singleLine(prefix + option.Label)
		lines[i] = line + strings.Repeat(" ", max(0, width-lipgloss.Width(line)))
	}
	d.viewport.SetContentLines(lines)
	// Keep the focused option visible with minimal scrolling. Viewport owns
	// offset bounds, clipping and resizing; hover within the view stays put.
	if d.Cursor < d.viewport.YOffset() {
		d.viewport.SetYOffset(d.Cursor)
	} else if d.Cursor >= d.viewport.YOffset()+d.viewport.Height() {
		d.viewport.SetYOffset(d.Cursor - d.viewport.Height() + 1)
	}
}

func (d Dropdown) Height(availableHeight int) int {
	if availableHeight <= 0 {
		return 0
	}
	if !d.Open || d.Disabled {
		return 1
	}
	return 1 + d.visible(availableHeight)
}

func (d *Dropdown) cancel() SelectResult {
	d.Open = false
	d.restoreCursor()
	return SelectResult{Cancelled: true, Value: d.Value}
}

func (d *Dropdown) preview(index int) SelectResult {
	if !d.optionsVisible() || index < 0 || index >= len(d.Options) || d.Options[index].Disabled || index == d.Cursor {
		return SelectResult{}
	}
	d.Cursor = index
	d.layoutOptions(d.bounds.Width, d.bounds.Height)
	return SelectResult{Changed: true, Value: d.Options[index].Value}
}

func (d *Dropdown) move(delta int) SelectResult {
	if delta == 0 || !d.optionsVisible() {
		return SelectResult{}
	}
	direction := 1
	if delta < 0 {
		direction = -1
	}
	index := d.Cursor
	for range max(delta, -delta) {
		next := enabledOption(d.Options, index+direction, direction)
		if next < 0 {
			break
		}
		index = next
	}
	return d.preview(index)
}

func (d *Dropdown) confirm() SelectResult {
	if !d.optionsVisible() || d.Cursor < 0 || d.Cursor >= len(d.Options) || d.Options[d.Cursor].Disabled {
		return SelectResult{}
	}
	d.Value, d.Open = d.Options[d.Cursor].Value, false
	return SelectResult{Confirmed: true, Value: d.Value}
}

func (d *Dropdown) Update(msg tea.Msg) SelectResult {
	if motion, ok := msg.(tea.MouseMotionMsg); ok {
		d.Hovered = !d.Disabled && d.bounds.Contains(motion.X, motion.Y) && motion.Y == d.bounds.Y
	}
	if d.Disabled {
		d.Hovered = false
		if d.Open {
			return d.cancel()
		}
		return SelectResult{}
	}
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		if !d.Focused {
			return SelectResult{}
		}
		key := msg.String()
		if !d.Open {
			if key == "enter" || key == "space" {
				d.Open = true
				d.restoreCursor()
			}
			return SelectResult{}
		}
		switch key {
		case "esc":
			return d.cancel()
		case "enter", "space":
			return d.confirm()
		case "up", "k":
			return d.move(-1)
		case "down", "j":
			return d.move(1)
		case "pgup":
			return d.move(-max(1, d.visible(d.bounds.Height)))
		case "pgdown":
			return d.move(max(1, d.visible(d.bounds.Height)))
		case "home":
			return d.preview(enabledOption(d.Options, 0, 1))
		case "end":
			return d.preview(enabledOption(d.Options, len(d.Options)-1, -1))
		}
	case tea.MouseClickMsg:
		if msg.Button != tea.MouseLeft {
			return SelectResult{}
		}
		if !d.bounds.Contains(msg.X, msg.Y) || msg.Y-d.bounds.Y >= d.Height(d.bounds.Height) {
			if d.Open {
				return d.cancel()
			}
			return SelectResult{}
		}
		d.Focused = true
		if msg.Y == d.bounds.Y {
			if d.Open {
				return d.cancel()
			}
			d.Open = true
			d.restoreCursor()
			return SelectResult{}
		}
		if d.Open && d.optionsVisible() {
			index := d.start(d.bounds.Height) + msg.Y - d.bounds.Y - 1
			if index >= 0 && index < len(d.Options) && !d.Options[index].Disabled {
				d.Cursor = index
				return d.confirm()
			}
		}
	case tea.MouseMotionMsg:
		if d.Open && d.bounds.Contains(msg.X, msg.Y) && msg.Y > d.bounds.Y && msg.Y-d.bounds.Y < d.Height(d.bounds.Height) {
			return d.preview(d.start(d.bounds.Height) + msg.Y - d.bounds.Y - 1)
		}
	case tea.MouseWheelMsg:
		if d.Open && d.bounds.Contains(msg.X, msg.Y) {
			if msg.Button == tea.MouseWheelUp {
				return d.move(-1)
			}
			if msg.Button == tea.MouseWheelDown {
				return d.move(1)
			}
		}
	}
	return SelectResult{}
}

func (d Dropdown) View(theme Theme, width, height int) string {
	if width <= 0 || height <= 0 {
		return ""
	}
	base := lipgloss.NewStyle().Foreground(themeColor(theme.Foreground)).Background(themeColor(theme.Surface))
	label := i18n.Text(d.Locale, "common.choose")
	if index := optionIndex(d.Options, d.Value); index >= 0 {
		label = d.Options[index].Label
	}
	marker := "▸ "
	if d.Open {
		marker = "▾ "
	}
	trigger := base
	if d.Hovered {
		trigger = trigger.Background(themeColor(theme.Hover))
	}
	if d.Focused {
		trigger = trigger.Foreground(themeColor(theme.Primary)).Background(themeColor(theme.Selected)).Bold(true)
	}
	if d.Disabled {
		trigger = base.Foreground(themeColor(theme.Disabled))
	}
	lines := []string{trigger.Render(dashboardLine(marker+label, width, false))}
	if d.Open && !d.Disabled && d.visible(height) > 0 {
		d.layoutOptions(width, height)
		d.viewport.StyleLineFunc = func(i int) lipgloss.Style {
			style := base
			if i == d.Cursor {
				style = style.Background(themeColor(theme.Selected)).Bold(true)
			}
			if d.Options[i].Disabled {
				style = base.Foreground(themeColor(theme.Disabled))
			}
			return style
		}
		lines = append(lines, d.viewport.View())
	}
	return strings.Join(lines, "\n")
}
