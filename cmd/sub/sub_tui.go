package sub

import (
	"errors"
	"image/color"
	"os"
	"os/exec"
	"strings"
	"time"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/list"

	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"charm.land/huh/v2"
	"charm.land/lipgloss/v2"
)

var statusOkStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("10")).Render
var statusFailStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("9"))

const (
	throttleInterval    = 1 * time.Second
	doublePressInterval = 300 * time.Millisecond
	noProfilesMessage   = "No profiles available. Please add a profile first."
)

type item profile

func (i item) Title() string       { return i.Name }
func (i item) Description() string { return i.Url }
func (i item) FilterValue() string { return i.Name }

func listItemsFromProfiles(items []profile) []list.Item {
	listItems := make([]list.Item, 0, len(items))
	for _, p := range items {
		listItems = append(listItems, item(p))
	}
	return listItems
}

type styles struct {
	HeaderText,
	ErrorHeaderText lipgloss.Style

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

	return s
}

type formAction int

const (
	formActionAdd formAction = iota
	formActionModify
)

type keyMap struct {
	ForceQuit key.Binding
	Quit      key.Binding
	EscQuit   key.Binding
	EscCancel key.Binding
	Add       key.Binding
	Modify    key.Binding
	Edit      key.Binding
	Delete    key.Binding
	Use       key.Binding
}

var keys = keyMap{
	ForceQuit: key.NewBinding(
		key.WithKeys("ctrl+c"),
	),
	Quit: key.NewBinding(
		key.WithKeys("q"),
		key.WithHelp("q", "quit"),
	),
	EscQuit: key.NewBinding(
		key.WithKeys("esc"),
		key.WithHelp("esc", "quit"),
	),
	EscCancel: key.NewBinding(
		key.WithKeys("esc"),
		key.WithHelp("esc", "cancel"),
	),
	Add: key.NewBinding(
		key.WithKeys("a"),
		key.WithHelp("a", "add"),
	),
	Modify: key.NewBinding(
		key.WithKeys("m"),
		key.WithHelp("m", "modify"),
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
	styles styles
	list   list.Model

	form        *huh.Form
	formAction  formAction
	formFocused bool

	lastPress struct {
		d, enter time.Time
	}

	err error
}

func initialModel() model {
	m := model{
		list: list.New(nil, list.NewDefaultDelegate(), 0, 0),
	}
	m.list.SetSpinner(spinner.MiniDot)

	m.list.SetShowHelp(false)
	m.list.SetItems(listItemsFromProfiles(cfg.Items))
	m.list.DisableQuitKeybindings()
	keyBindingFn := func() []key.Binding {
		return []key.Binding{
			keys.Edit,
			keys.Add,
			keys.Modify,
			keys.Delete,
			keys.Use,
			keys.Quit,
		}
	}
	m.list.AdditionalShortHelpKeys = keyBindingFn
	m.list.AdditionalFullHelpKeys = keyBindingFn

	m.list.Title = "use: " + cfg.Use

	m.list.SetStatusBarItemName("profile", "profiles")
	m.list.StatusMessageLifetime = 2 * time.Second
	return m
}
func (m model) Init() tea.Cmd {
	return tea.RequestBackgroundColor
}
func (m *model) errorMessage(message string) tea.Cmd {
	m.err = errors.New(message)
	m.list.NewStatusMessage(statusFailStyle.Render((message)))
	return nil
}

func (m *model) noProfilesError() tea.Cmd {
	return m.errorMessage(noProfilesMessage)
}

func (m *model) pendingMessage(message string) tea.Cmd {
	m.list.NewStatusMessage(message)
	return nil
}

func (m *model) successMessage(message string) tea.Cmd {
	return m.list.NewStatusMessage(statusOkStyle(message))
}

func (m *model) cancelForm() tea.Cmd {
	m.formFocused = false
	m.form = nil
	m.formAction = formActionAdd
	name = ""
	url = ""
	return m.successMessage("Canceled")
}

type editorFinishedMsg struct{ err error }
type useFinishedMsg struct{ err error }
type addFinishedMsg struct {
	profile profile
	err     error
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	var selectedItem *item
	if sel := m.list.SelectedItem(); sel != nil {
		a := sel.(item)
		selectedItem = &a
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		if key.Matches(msg, keys.ForceQuit) {
			return m, tea.Quit

		}
		filterState := m.list.FilterState()
		if filterState == list.Filtering {
			break
		}
		if filterState == list.FilterApplied && key.Matches(msg, keys.EscQuit) {
			break
		}
		if m.err != nil {
			m.err = nil
			return m, m.successMessage("")
		}

		if m.formFocused {
			switch {
			case key.Matches(msg, keys.ForceQuit):
				return m, tea.Quit
			case key.Matches(msg, keys.EscCancel):
				return m, m.cancelForm()
			}
			break
		}

		switch {
		case key.Matches(msg, keys.Quit):
			return m, tea.Quit
		case key.Matches(msg, keys.EscQuit):
			if m.form != nil {
				return m, m.cancelForm()
			}
			return m, tea.Quit
		case key.Matches(msg, keys.Add):
			m.formFocused = true
			m.formAction = formActionAdd
			form := initialForm().WithShowHelp(false)
			m.form = form
			return m, form.Init()
		case key.Matches(msg, keys.Modify), key.Matches(msg, keys.Edit), key.Matches(msg, keys.Delete), key.Matches(msg, keys.Use):
			if selectedItem == nil {
				return m, m.noProfilesError()
			}

			switch {
			case key.Matches(msg, keys.Modify):
				m.formFocused = true
				m.formAction = formActionModify
				name = selectedItem.Name
				url = selectedItem.Url
				form := initialForm().WithShowHelp(false)
				m.form = form
				return m, form.Init()
			case key.Matches(msg, keys.Edit):
				var editorPath string
				if envEd := os.Getenv("EDITOR"); envEd != "" {
					if p, err := exec.LookPath(envEd); err == nil {
						editorPath = p
					}
				}
				if editorPath == "" {
					priorityList := []string{"nvim", "vim", "vi", "nano", "code", "notepad"}
					// priorityList := []string{"nvim"}
					for _, name := range priorityList {
						if p, err := exec.LookPath(name); err == nil {
							editorPath = p
							break
						}
					}
				}
				if editorPath == "" {
					return m, m.errorMessage("no supported editor")
				}
				file := selectedItem.File
				c := exec.Command(editorPath, file)
				return m, tea.ExecProcess(c, func(err error) tea.Msg {
					return editorFinishedMsg{err: err}
				})
			case key.Matches(msg, keys.Delete):
				if time.Since(m.lastPress.d) > doublePressInterval {
					m.lastPress.d = time.Now()
					return m, nil
				}
				if err := delProfile(selectedItem.Name); err != nil {
					return m, m.errorMessage(err.Error())
				}
				m.list.RemoveItem(m.list.GlobalIndex())
				m.lastPress.d = time.Now()
				return m, m.successMessage("Deleted " + selectedItem.Name)
			case key.Matches(msg, keys.Use):
				if time.Since(m.lastPress.enter) < throttleInterval {
					return m, nil
				}
				m.lastPress.enter = time.Now()
				return m, tea.Batch(
					m.pendingMessage("wait a moment..."),
					m.list.StartSpinner(),
					func() tea.Msg {
						if err := useFunc(selectedItem.Name); err != nil {
							return useFinishedMsg{err}
						}
						return useFinishedMsg{}
					})
			}
		}
	case editorFinishedMsg:
		//  todo valid config
		err := msg.err
		if err != nil {

			return m, m.errorMessage(err.Error())
		}
	case useFinishedMsg:
		m.list.StopSpinner()
		err := msg.err
		if err != nil {
			return m, m.errorMessage(strings.TrimRight(err.Error(), "\n"))
		}
		m.list.Title = "use: " + cfg.Use
		return m, m.successMessage("used")
	case addFinishedMsg:
		m.list.StopSpinner()
		if msg.err != nil {
			return m, m.errorMessage(strings.TrimRight(msg.err.Error(), "\n"))
		}
		cmds = append(cmds, m.list.SetItems(listItemsFromProfiles(cfg.Items)))
		m.list.Select(len(cfg.Items) - 1)
		m.list.Title = "use: " + cfg.Use
		cmds = append(cmds, m.successMessage("Added "+msg.profile.Name))
		return m, tea.Batch(cmds...)
	case tea.WindowSizeMsg:

		h := lipgloss.Height(header) + lipgloss.Height(footer)
		m.list.SetSize(msg.Width, msg.Height-h)
	case tea.BackgroundColorMsg:
		m.styles = newStyles(msg.IsDark())
		return m, nil
	}
	if m.formFocused {
		newForm, cmd := m.form.Update(msg)
		m.form = newForm.(*huh.Form)
		cmds = append(cmds, cmd)
		if m.form.State == huh.StateCompleted {
			submittedName := name
			submittedURL := url
			action := m.formAction
			m.formFocused = false
			m.form = nil
			m.formAction = formActionAdd
			name = ""
			url = ""

			switch action {
			case formActionAdd:
				cmds = append(cmds,
					m.pendingMessage("adding profile..."),
					m.list.StartSpinner(),
					func() tea.Msg {
						p, err := prepareProfile(submittedName, submittedURL, plainDownloadProfile)
						if err == nil {
							err = addProfile(p)
						}
						return addFinishedMsg{profile: p, err: err}
					},
				)
			case formActionModify:
				return m, m.errorMessage("modify is not implemented yet")
			}
		}
	} else {
		newListModel, cmd := m.list.Update(msg)
		m.list = newListModel
		cmds = append(cmds, cmd)
	}
	return m, tea.Batch(cmds...)
}

var (
	appBoundaryTopStyle    = lipgloss.NewStyle().MarginBottom(1)
	appBoundaryBottomStyle = lipgloss.NewStyle().MarginTop(1)
)

var header, footer string

func (m model) View() tea.View {
	body := m.list.View()
	m.list.KeyMap.ShowFullHelp.SetEnabled(false)
	helpView := m.list.Help.ShortHelpView(m.list.ShortHelp())
	if m.formFocused {
		body = lipgloss.JoinHorizontal(lipgloss.Top, m.list.View(), "   ", m.form.View())
		formKeys := m.form.KeyBinds()
		formKeys = append(formKeys, keys.EscCancel)
		helpView = m.form.Help().ShortHelpView(formKeys)

	}
	header = m.appBoundaryView(lipgloss.Center, "Subscription")
	footer = m.appBoundaryView(lipgloss.Left, helpView)

	if m.err != nil {
		header = m.appErrorBoundaryView(lipgloss.Center, "Subscription")
		// footer = m.appErrorBoundaryView(lipgloss.Left, "warning")
		footer = m.appErrorBoundaryView(lipgloss.Left, "Press any key to continue")
	}
	header = appBoundaryTopStyle.Render(header)
	footer = appBoundaryBottomStyle.Render(footer)
	v := tea.NewView(lipgloss.JoinVertical(lipgloss.Left, header, body, footer))
	v.AltScreen = true
	return v
}

func (m model) appBoundaryView(position lipgloss.Position, text string) string {
	return lipgloss.PlaceHorizontal(
		m.list.Width(),
		position,
		m.styles.HeaderText.Render(text),
		lipgloss.WithWhitespaceChars("/"),
		lipgloss.WithWhitespaceStyle(lipgloss.NewStyle().Foreground(m.styles.Indigo)),
	)
}

func (m model) appErrorBoundaryView(position lipgloss.Position, text string) string {
	return lipgloss.PlaceHorizontal(
		m.list.Width(),
		position,
		m.styles.ErrorHeaderText.Render(text),
		lipgloss.WithWhitespaceChars("/"),
		lipgloss.WithWhitespaceStyle(lipgloss.NewStyle().Foreground(m.styles.Red)),
	)
}
