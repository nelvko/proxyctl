package ui

import (
	"image/color"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/list"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/huh/v2"
	"charm.land/lipgloss/v2"
	"github.com/nelvko/proxyctl/internal/config"
)

// themeColor keeps every adapter on the same terminal color policy.
func themeColor(token string) color.Color {
	if !terminalColorsEnabled() || token == "" {
		return nil
	}
	return lipgloss.Color(token)
}

func themeHelpStyles(t Theme) help.Styles {
	key := lipgloss.NewStyle().Foreground(themeColor(t.Foreground))
	description := lipgloss.NewStyle().Foreground(themeColor(t.Muted))
	separator := lipgloss.NewStyle().Foreground(themeColor(t.Border))
	return help.Styles{
		ShortKey: key, FullKey: key,
		ShortDesc: description, FullDesc: description,
		ShortSeparator: separator, FullSeparator: separator, Ellipsis: separator,
	}
}

func themeTextInputStyles(t Theme) textinput.Styles {
	state := textinput.StyleState{
		Text:        lipgloss.NewStyle().Foreground(themeColor(t.Foreground)),
		Placeholder: lipgloss.NewStyle().Foreground(themeColor(t.Disabled)),
		Suggestion:  lipgloss.NewStyle().Foreground(themeColor(t.Muted)),
		Prompt:      lipgloss.NewStyle().Foreground(themeColor(t.Primary)),
	}
	return textinput.Styles{
		Focused: state, Blurred: state,
		Cursor: textinput.CursorStyle{Color: themeColor(t.Primary), Shape: tea.CursorBlock, Blink: true},
	}
}

func (m *model) applyTheme(darkBG bool) {
	m.theme, _ = resolveTheme(m.themeConfig, darkBG)
	m.styles = newThemedStyles(m.theme)
	m.workSpinner.Style = lipgloss.NewStyle().Foreground(themeColor(m.theme.Primary))
	applyListTheme(&m.list, m.theme)
	if m.form != nil {
		m.form.WithTheme(staticHuhTheme(m.theme))
	}
}

// applyListTheme preserves the Bubbles layout and replaces its default palette.
func applyListTheme(m *list.Model, t Theme) {
	foreground, muted := themeColor(t.Foreground), themeColor(t.Muted)
	primary, border := themeColor(t.Primary), themeColor(t.Border)
	s := &m.Styles
	s.TitleBar = s.TitleBar.Foreground(foreground)
	s.Title = s.Title.Foreground(foreground).Background(themeColor(t.Selected)).Bold(true)
	s.Spinner = s.Spinner.Foreground(primary)
	s.Filter = themeTextInputStyles(t)
	s.DefaultFilterCharacterMatch = lipgloss.NewStyle().Foreground(primary).Underline(true)
	s.StatusBar = s.StatusBar.Foreground(muted)
	s.StatusEmpty = s.StatusEmpty.Foreground(muted)
	s.StatusBarActiveFilter = s.StatusBarActiveFilter.Foreground(foreground)
	s.StatusBarFilterCount = s.StatusBarFilterCount.Foreground(muted)
	s.NoItems = s.NoItems.Foreground(muted)
	s.ArabicPagination = s.ArabicPagination.Foreground(muted)
	s.ActivePaginationDot = s.ActivePaginationDot.Foreground(primary)
	s.InactivePaginationDot = s.InactivePaginationDot.Foreground(border)
	s.DividerDot = s.DividerDot.Foreground(border)
	m.Help.Styles = themeHelpStyles(t)
	m.FilterInput.SetStyles(s.Filter)

	m.SetDelegate(profileDelegate(t))
}

func profileDelegate(t Theme) list.DefaultDelegate {
	foreground, muted := themeColor(t.Foreground), themeColor(t.Muted)
	primary := themeColor(t.Primary)
	d := list.NewDefaultDelegate()
	normal := lipgloss.NewStyle().Foreground(foreground).PaddingLeft(2)
	selected := lipgloss.NewStyle().Foreground(foreground).Background(themeColor(t.Selected)).
		Border(lipgloss.NormalBorder(), false, false, false, true).BorderForeground(primary).PaddingLeft(1)
	d.Styles = list.DefaultItemStyles{
		NormalTitle: normal, NormalDesc: normal.Foreground(muted),
		SelectedTitle: selected.Bold(true), SelectedDesc: selected.Foreground(muted),
		DimmedTitle: normal.Foreground(muted), DimmedDesc: normal.Foreground(themeColor(t.Disabled)),
		FilterMatch: lipgloss.NewStyle().Foreground(primary).Underline(true),
	}
	return d
}

func profileHuhTheme(cfg config.ThemeConfig) huh.Theme {
	return huh.ThemeFunc(func(darkBG bool) *huh.Styles {
		t, _ := resolveTheme(cfg, darkBG)
		return themedHuhStyles(t)
	})
}

// HuhTheme applies the saved appearance to CLI forms as well as TUI forms.
// Invalid or missing appearance settings use the same safe palette fallback.
func HuhTheme() huh.Theme {
	cfg, _ := loadThemeConfig()
	return profileHuhTheme(cfg)
}

func staticHuhTheme(t Theme) huh.Theme {
	return huh.ThemeFunc(func(bool) *huh.Styles { return themedHuhStyles(t) })
}

func themedHuhStyles(t Theme) *huh.Styles {
	s := huh.ThemeBase(true)
	foreground, muted := themeColor(t.Foreground), themeColor(t.Muted)
	primary, selected := themeColor(t.Primary), themeColor(t.Selected)
	s.Form.Base = s.Form.Base.Foreground(foreground).Background(themeColor(t.Background))
	f := &s.Focused
	f.Base = f.Base.BorderForeground(primary)
	f.Card = f.Base
	f.Title = f.Title.Foreground(primary).Bold(true)
	f.NoteTitle = f.Title
	f.Description = f.Description.Foreground(muted)
	f.ErrorIndicator = f.ErrorIndicator.Foreground(themeColor(t.Error))
	f.ErrorMessage = f.ErrorMessage.Foreground(themeColor(t.Error))
	f.SelectSelector = f.SelectSelector.Foreground(primary)
	f.MultiSelectSelector = f.MultiSelectSelector.Foreground(primary)
	f.NextIndicator = f.NextIndicator.Foreground(primary)
	f.PrevIndicator = f.PrevIndicator.Foreground(primary)
	f.Option = f.Option.Foreground(foreground)
	f.Directory = f.Directory.Foreground(primary)
	f.File = f.File.Foreground(foreground)
	f.SelectedOption = f.SelectedOption.Foreground(themeColor(t.Success))
	f.SelectedPrefix = f.SelectedPrefix.Foreground(themeColor(t.Success))
	f.UnselectedOption = f.UnselectedOption.Foreground(foreground)
	f.UnselectedPrefix = f.UnselectedPrefix.Foreground(muted)
	f.FocusedButton = f.FocusedButton.Foreground(foreground).Background(selected).Bold(true)
	f.BlurredButton = f.BlurredButton.Foreground(muted).Background(themeColor(t.Surface))
	f.Next = f.FocusedButton
	f.TextInput = huh.TextInputStyles{
		Cursor:      lipgloss.NewStyle().Foreground(primary),
		CursorText:  lipgloss.NewStyle().Foreground(themeColor(t.Background)),
		Placeholder: lipgloss.NewStyle().Foreground(themeColor(t.Disabled)),
		Prompt:      lipgloss.NewStyle().Foreground(primary),
		Text:        lipgloss.NewStyle().Foreground(foreground),
	}
	s.Blurred = *f
	s.Blurred.Base = f.Base.BorderStyle(lipgloss.HiddenBorder())
	s.Blurred.Card = s.Blurred.Base
	s.Blurred.Title = s.Blurred.Title.Foreground(muted)
	s.Blurred.NextIndicator = lipgloss.NewStyle()
	s.Blurred.PrevIndicator = lipgloss.NewStyle()
	s.Group.Title = f.Title
	s.Group.Description = f.Description
	s.Help = themeHelpStyles(t)
	return s
}
