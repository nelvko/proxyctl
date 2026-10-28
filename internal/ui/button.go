package ui

import (
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// Button keeps pointer hover separate from keyboard focus. Its Bounds must
// match the visible cells supplied by the parent layout.
type Button struct {
	Label                      string
	Disabled, Focused, Hovered bool
	Bounds                     Rect
}

func (b *Button) Update(msg tea.Msg) bool {
	if motion, ok := msg.(tea.MouseMotionMsg); ok {
		b.Hovered = !b.Disabled && b.Bounds.Contains(motion.X, motion.Y)
	}
	if b.Disabled || b.Bounds.Width <= 2 || b.Bounds.Height <= 0 {
		b.Hovered = false
		return false
	}
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		return b.Focused && (msg.String() == "enter" || msg.String() == "space")
	case tea.MouseClickMsg:
		return msg.Button == tea.MouseLeft && b.Bounds.Contains(msg.X, msg.Y)
	}
	return false
}

func (b Button) View(theme Theme, width int) string {
	if width <= 0 {
		return ""
	}
	style := lipgloss.NewStyle().Foreground(themeColor(theme.Foreground))
	marker := "  "
	if b.Hovered {
		style = style.Background(themeColor(theme.Hover))
	}
	if b.Focused {
		marker = "▶ "
		style = style.Foreground(themeColor(theme.Primary)).Background(themeColor(theme.Selected)).Bold(true)
	}
	if b.Disabled {
		marker = "  "
		style = lipgloss.NewStyle().Foreground(themeColor(theme.Disabled))
	}
	return style.Render(dashboardLine(marker+b.Label, width, false))
}
