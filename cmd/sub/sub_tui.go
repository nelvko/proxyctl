package sub

import (
	"errors"
	"fmt"
	"image/color"
	"strings"
	"time"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/list"
	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"charm.land/huh/v2"
	"charm.land/lipgloss/v2"
)

const (
	throttleInterval    = 1 * time.Second
	doublePressInterval = 300 * time.Millisecond

	statusMessageLifetime = 2 * time.Second
	appBoundaryMargin     = 1
)

type item profile

func (i item) Title() string       { return i.Name }
func (i item) Description() string { return i.URL }
func (i item) FilterValue() string { return i.Name }

func listItemsFromProfiles() []list.Item {
	profiles := subCfg.Profiles
	listItems := make([]list.Item, 0, len(profiles))
	for _, p := range profiles {
		listItems = append(listItems, item(p))
	}
	return listItems
}

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

type formAction int

const (
	formActionAdd formAction = iota
	formActionSet
)

type keyMap struct {
	More      key.Binding
	EscCancel key.Binding
	Add       key.Binding
	Set       key.Binding
	Edit      key.Binding
	Delete    key.Binding
	Use       key.Binding
}

var keys = keyMap{
	More: key.NewBinding(
		key.WithKeys("?"),
		key.WithHelp("?", "more"),
	),
	EscCancel: key.NewBinding(
		key.WithKeys("esc"),
		key.WithHelp("esc", "cancel"),
	),
	Add: key.NewBinding(
		key.WithKeys("a"),
		key.WithHelp("a", "add"),
	),
	Set: key.NewBinding(
		key.WithKeys("s"),
		key.WithHelp("s", "set"),
	),
	Edit: key.NewBinding(
		key.WithKeys("e"),
		key.WithHelp("e", "edit"),
	),
	Delete: key.NewBinding(
		key.WithKeys("d"),
		key.WithHelp("dd", "delete"),
	),
	Use: key.NewBinding(
		key.WithKeys("enter"),
		key.WithHelp("enter", "use"),
	),
}

type model struct {
	height, width int

	list list.Model

	form        *huh.Form
	formAction  formAction
	formFocused bool

	styles styles

	lastPress struct {
		d, enter time.Time
	}

	err error
}

func initialModel() model {
	m := model{
		list: list.New(listItemsFromProfiles(), list.NewDefaultDelegate(), 0, 0),
	}
	m.list.Styles.TitleBar = m.list.Styles.TitleBar.PaddingLeft(3)
	m.list.SetSpinner(spinner.Dot)

	m.list.SetShowHelp(false)

	m.list.AdditionalShortHelpKeys = func() []key.Binding {
		return []key.Binding{
			keys.Edit,
			keys.Add,
			keys.Set,
			keys.Delete,
			keys.Use,
		}
	}
	m.list.AdditionalFullHelpKeys = m.list.AdditionalShortHelpKeys

	m.list.Title = "use: " + subCfg.Use

	m.list.SetStatusBarItemName("profile", "profiles")
	m.list.StatusMessageLifetime = statusMessageLifetime
	return m
}
func (m model) Init() tea.Cmd {
	return tea.RequestBackgroundColor
}

// message will be persistented if return nil, else will disappear after statusMessageLifetime
func (m *model) errorMessage(message string) tea.Cmd {
	m.err = errors.New(message)
	m.list.NewStatusMessage(m.styles.statusFail.Render((message)))
	return nil
}

func (m *model) pendingMessage(message string) tea.Cmd {
	m.list.NewStatusMessage(message)
	return nil
}

func (m *model) successMessage(message string) tea.Cmd {
	return m.list.NewStatusMessage(m.styles.statusOk.Render(message))
}

func (m *model) cancelForm() tea.Cmd {
	m.formFocused = false
	m.form = nil
	option.Name = ""
	option.URL = ""
	return m.successMessage("Canceled")
}

type editorFinishedMsg struct {
	item item
	err  error
}
type useFinishedMsg struct{ err error }
type addFinishedMsg struct {
	err error
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if m.err != nil {
			m.err = nil
			return m, m.successMessage("")
		}
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	case tea.BackgroundColorMsg:
		m.styles = newStyles(msg.IsDark())
		return m, nil
	}
	if m.formFocused {
		cmds = append(cmds, m.handleForm(msg))
	} else {
		cmds = append(cmds, m.handleList(msg))
	}
	return m, tea.Batch(cmds...)
}

func (m *model) handleList(msg tea.Msg) tea.Cmd {
	var cmds []tea.Cmd
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if filterState := m.list.FilterState(); filterState == list.Filtering {
			break
		}
		selectedItem, ok := m.list.SelectedItem().(item)
		if (!ok || len(m.list.Items()) == 0) && key.Matches(msg, keys.Delete, keys.Edit, keys.Set, keys.Use) {
			return m.errorMessage("No profiles available. Please add a profile first.")
		}
		switch {
		case key.Matches(msg, keys.Add, keys.Set):
			if key.Matches(msg, keys.Set) {
				m.formAction = formActionSet
				option.Name = selectedItem.Name
				option.URL = selectedItem.URL
			} else {
				m.formAction = formActionAdd
			}
			m.formFocused = true
			form := initialForm().WithShowHelp(false)
			m.form = form
			return form.Init()
		case key.Matches(msg, keys.Edit):
			cmd, err := getEditorCommand(selectedItem.File)
			if err != nil {
				return m.errorMessage(err.Error())
			}
			return tea.ExecProcess(cmd, func(err error) tea.Msg {
				return editorFinishedMsg{item: selectedItem, err: err}
			})
		case key.Matches(msg, keys.Delete):
			if time.Since(m.lastPress.d) > doublePressInterval {
				m.lastPress.d = time.Now()
				return nil
			}
			if err := deleteProfile(selectedItem.Name); err != nil {
				return m.errorMessage(err.Error())
			}
			m.list.RemoveItem(m.list.GlobalIndex())
			m.lastPress.d = time.Now()
			return m.successMessage("Deleted " + selectedItem.Name)
		case key.Matches(msg, keys.Use):
			if time.Since(m.lastPress.enter) < throttleInterval {
				return nil
			}
			m.lastPress.enter = time.Now()
			return tea.Batch(
				m.pendingMessage("wait a moment..."),
				m.list.StartSpinner(),
				func() tea.Msg {
					if err := useFunc(selectedItem.Name); err != nil {
						return useFinishedMsg{err}
					}
					return useFinishedMsg{}
				})

		case key.Matches(msg, keys.More):
			m.list.SetShowHelp(!m.list.ShowHelp())

		}
	case editorFinishedMsg:
		if msg.err != nil {
			return m.errorMessage(msg.err.Error())
		}
		if err := k.TestConfig(msg.item.File); err != nil {
			return m.errorMessage(fmt.Errorf("%w\nplease check profile %s and correct any errors", err, msg.item.Name).Error())
		}
		return m.successMessage("Edited")
	case useFinishedMsg:
		m.list.StopSpinner()
		if msg.err != nil {
			return m.errorMessage(msg.err.Error())
		}
		m.list.Title = "use: " + subCfg.Use
		return m.successMessage("used")
	case addFinishedMsg:
		m.list.StopSpinner()
		if msg.err != nil {
			return m.errorMessage(strings.TrimRight(msg.err.Error(), "\n"))
		}
		cmds = append(cmds, m.list.SetItems(listItemsFromProfiles()))
		m.list.Select(len(subCfg.Profiles) - 1)
		m.list.Title = "use: " + subCfg.Use
		cmds = append(cmds, m.successMessage("Added "))
		return tea.Batch(cmds...)
	}
	newListModel, cmd := m.list.Update(msg)
	m.list = newListModel
	cmds = append(cmds, cmd)
	return tea.Batch(cmds...)
}

func (m *model) handleForm(msg tea.Msg) tea.Cmd {
	var cmds []tea.Cmd
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, keys.EscCancel):
			return m.cancelForm()
		}

	}
	newForm, cmd := m.form.Update(msg)
	m.form = newForm.(*huh.Form)
	cmds = append(cmds, cmd)
	return tea.Batch(cmds...)
}

func (m model) View() tea.View {
	var header, body, footer string
	list := m.list
	if m.err != nil {
		header = m.appErrorBoundaryView(lipgloss.Center, "Subscription")
		footer = m.appErrorBoundaryView(lipgloss.Left, "Press any key to continue")
		bodyHeight := m.height - 2*appBoundaryMargin - lipgloss.Height(header) - lipgloss.Height(footer)
		if bodyHeight < 0 {
			bodyHeight = 0
		}
		// messageWidth = total width - titlebar horizontal padding
		// - title width
		// - title horizontal padding  https://github.com/charmbracelet/bubbles/blob/f1daacfa0cfee07e31a12498078426d275aa5286/list/style.go#L55
		// - message left margin       https://github.com/charmbracelet/bubbles/blob/f1daacfa0cfee07e31a12498078426d275aa5286/list/list.go#L1113
		messageWidth := m.width - 2*m.list.Styles.TitleBar.GetPaddingLeft() - lipgloss.Width(m.list.Title) - 2 - 2
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
		case m.formFocused:
			footer = m.appBoundaryView(lipgloss.Left, m.form.Help().ShortHelpView(append(m.form.KeyBinds(), keys.EscCancel)))
		case m.list.ShowHelp():
			footer = m.appBoundaryView(lipgloss.Left, "")
		default:
			footer = m.appBoundaryView(lipgloss.Left, m.list.Help.ShortHelpView(m.list.ShortHelp()))
		}
		bodyHeight := m.height - 2*appBoundaryMargin - lipgloss.Height(header) - lipgloss.Height(footer)
		if bodyHeight < 0 {
			bodyHeight = 0
		}
		if m.formFocused {
			listWidth := m.width / 2
			list.SetSize(listWidth, bodyHeight)
			body = lipgloss.JoinHorizontal(lipgloss.Top, list.View(), m.form.View())
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
