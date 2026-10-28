package ui

import (
	"fmt"
	"image/color"
	"strings"

	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

type styles struct {
	HeaderText, ErrorHeaderText lipgloss.Style
	statusOk, statusFail        lipgloss.Style

	Red, Indigo, Green color.Color
}

func newStyles(darkBG bool) styles {
	theme, _ := resolveTheme(defaultThemeConfig(), darkBG)
	return newThemedStyles(theme)
}

func newThemedStyles(theme Theme) styles {
	s := styles{Red: themeColor(theme.Error), Indigo: themeColor(theme.Primary), Green: themeColor(theme.Success)}

	s.HeaderText = lipgloss.NewStyle().
		Foreground(s.Indigo).
		Bold(true).
		Padding(0, 2)
	s.ErrorHeaderText = s.HeaderText.
		Foreground(s.Red)

	s.statusOk = lipgloss.NewStyle().Foreground(s.Green)
	s.statusFail = lipgloss.NewStyle().Foreground(s.Red)

	return s
}

func (m model) View() tea.View {
	layout := m.layoutSubscriptions()
	lines := make([]string, max(0, m.height))
	put := func(y int, content string) {
		if y >= 0 && y < len(lines) {
			lines[y] = content
		}
	}
	put(0, m.appBoundaryView(lipgloss.Center, m.tr("sub.title")))
	put(1, " "+m.tr("sub.selected", singleLine(m.profiles.SelectedName())))
	for _, action := range layout.actions {
		gap := action.Bounds.X - lipgloss.Width(lines[action.Bounds.Y])
		put(action.Bounds.Y, lines[action.Bounds.Y]+strings.Repeat(" ", max(0, gap))+action.View(m.theme, action.Bounds.Width))
	}
	message := m.feedback
	if m.busy {
		message = m.workSpinner.View() + " " + message
	} else if !m.formFocused {
		if m.list.FilterState() == list.Filtering {
			message = m.list.FilterInput.View()
		} else if message == "" {
			message = m.tr("sub.filter_prompt") + m.list.FilterValue() + "  /"
		}
	}
	put(layout.message.Y, message)
	body := ""
	if m.formFocused && m.form != nil {
		body = m.form.View()
	} else if len(m.list.VisibleItems()) == 0 {
		key := "sub.no_matches"
		if len(m.list.Items()) == 0 {
			key = "sub.empty"
		}
		body = " " + m.tr(key)
	} else {
		body = m.list.View()
	}
	for i, line := range strings.Split(body, "\n") {
		if i >= layout.list.Height {
			break
		}
		put(layout.list.Y+i, line)
	}
	count := m.tr("sub.count", len(m.list.Items()))
	if m.list.IsFiltered() {
		count = m.tr("sub.filtered_count", len(m.list.VisibleItems()), len(m.list.Items()))
	}
	if m.list.Paginator.TotalPages > 1 {
		count += fmt.Sprintf("  %d/%d", m.list.Paginator.Page+1, m.list.Paginator.TotalPages)
	}
	if !m.formFocused {
		put(m.height-2, count)
	}
	help := m.tr("sub.key_help")
	switch {
	case m.err != nil:
		help = m.tr("sub.ack_error")
	case m.busy:
		help = "ctrl+c " + m.tr("sub.cancel")
	case m.formFocused:
		help = m.tr("sub.form_help")
	case m.list.FilterState() == list.Filtering:
		help = m.tr("sub.filter_help")
	case m.toolbarFocus >= 0:
		help = m.tr("sub.toolbar_help")
	}
	put(m.height-1, lipgloss.NewStyle().Foreground(themeColor(m.theme.Muted)).Render(help))
	for i := range lines {
		lines[i] = gridLine(lines[i], m.width)
	}
	content := lipgloss.NewStyle().Foreground(themeColor(m.theme.Foreground)).Background(themeColor(m.theme.Background)).Render(strings.Join(lines, "\n"))
	if m.dialog.Active {
		content = m.dialog.Overlay(content, m.theme, m.width, m.height)
	}
	v := tea.NewView(terminalFrame(content, m.width, m.height, m.theme, m.colorProfile))
	v.AltScreen = true
	v.MouseMode = tea.MouseModeAllMotion
	return v
}

func (m model) appBoundaryView(position lipgloss.Position, text string) string {
	return lipgloss.PlaceHorizontal(
		m.width,
		position,
		m.styles.HeaderText.Render(text),
		lipgloss.WithWhitespaceChars("/"),
		lipgloss.WithWhitespaceStyle(lipgloss.NewStyle().Foreground(themeColor(m.theme.Border))),
	)
}
