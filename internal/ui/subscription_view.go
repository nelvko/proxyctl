package ui

import (
	"image/color"

	"charm.land/bubbles/v2/key"
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
	lightDark := lipgloss.LightDark(darkBG)
	s := styles{}

	s.Red = lightDark(lipgloss.Color("#FE5F86"), lipgloss.Color("#FE5F86"))
	s.Indigo = lightDark(lipgloss.Color("#5A56E0"), lipgloss.Color("#7571F9"))
	s.Green = lightDark(lipgloss.Color("#02BA84"), lipgloss.Color("#02BF87"))

	s.HeaderText = lipgloss.NewStyle().
		Foreground(s.Indigo).
		Bold(true).
		Padding(0, 2)
	s.ErrorHeaderText = s.HeaderText.
		Foreground(s.Red)

	s.statusOk = lipgloss.NewStyle().Foreground(lipgloss.Color("10"))
	s.statusFail = lipgloss.NewStyle().Foreground(lipgloss.Color("9"))

	return s
}

func (m model) View() tea.View {
	var header, body, footer string
	list := m.list

	if m.err != nil {
		header = m.appErrorBoundaryView(lipgloss.Center, "Subscription")
		footer = m.appErrorBoundaryView(lipgloss.Left, "Press any key to continue")
		bodyHeight := m.bodyHeight(header, footer)

		messageWidth := m.width - 2*m.list.Styles.TitleBar.GetPaddingLeft() - lipgloss.Width(m.list.Title) - 4
		if lipgloss.Width(m.err.Error()) > messageWidth {
			header = m.appErrorBoundaryView(lipgloss.Center, "WARNING")
			body = lipgloss.NewStyle().Width(m.width).Height(bodyHeight).AlignVertical(lipgloss.Center).Render(m.err.Error())
		} else {
			list.SetSize(m.width, bodyHeight)
			body = list.View()
		}
	} else {
		header = m.appBoundaryView(lipgloss.Center, "Subscription")
		switch {
		case m.details != nil:
			footer = m.appBoundaryView(lipgloss.Left, "↑/↓ scroll · esc/q back")
		case m.formFocused:
			footer = m.appBoundaryView(lipgloss.Left, m.form.Help().ShortHelpView(append(m.form.KeyBinds(), keys.EscCancel)))
		case m.busy:
			footer = m.appBoundaryView(lipgloss.Left, "Working…  ctrl+c cancel")
		case m.list.ShowHelp():
			footer = m.appBoundaryView(lipgloss.Left, "")
		default:
			footer = m.appBoundaryView(lipgloss.Left, m.shortHelp())
		}

		bodyHeight := m.bodyHeight(header, footer)
		if m.details != nil {
			body = m.details.View()
		} else if m.formFocused {
			body = m.form.View()
		} else {
			list.SetSize(m.width, bodyHeight)
			body = list.View()
		}
	}

	body = lipgloss.NewStyle().Margin(appBoundaryMargin, 0).Render(body)
	v := tea.NewView(lipgloss.JoinVertical(lipgloss.Left, header, body, footer))
	v.AltScreen = true
	return v
}

func (m model) bodyHeight(header, footer string) int {
	bodyHeight := m.height - 2*appBoundaryMargin - lipgloss.Height(header) - lipgloss.Height(footer)
	if bodyHeight < 0 {
		return 0
	}
	return bodyHeight
}

func (m model) appBoundaryView(position lipgloss.Position, text string) string {
	return lipgloss.PlaceHorizontal(
		m.width,
		position,
		m.styles.HeaderText.Render(text),
		lipgloss.WithWhitespaceChars("/"),
		lipgloss.WithWhitespaceStyle(lipgloss.NewStyle().Foreground(m.styles.Indigo)),
	)
}

func (m model) appErrorBoundaryView(position lipgloss.Position, text string) string {
	return lipgloss.PlaceHorizontal(
		m.width,
		position,
		m.styles.ErrorHeaderText.Render(text),
		lipgloss.WithWhitespaceChars("/"),
		lipgloss.WithWhitespaceStyle(lipgloss.NewStyle().Foreground(m.styles.Red)),
	)
}

func (m model) shortHelp() string {
	if m.list.FilterState() == list.Filtering {
		return m.list.Help.ShortHelpView(m.list.ShortHelp())
	}
	if m.width < 45 {
		return "enter use · / filter · ? help · q quit"
	}
	bindings := []key.Binding{m.list.KeyMap.CursorUp, m.list.KeyMap.CursorDown, m.list.KeyMap.Filter, keys.Use, keys.Add, keys.Refresh, keys.More, keys.Quit}
	return m.list.Help.ShortHelpView(bindings)
}
