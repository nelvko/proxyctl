package sub

import (
	"os"
	"os/exec"
	"strings"
	"time"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
	"charm.land/huh/v2"
	"charm.land/lipgloss/v2"
)

var docStyle = lipgloss.NewStyle().Margin(1, 2)
var statusOkStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("10")).Render
var statusFailStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Render

var statusUseStyle = lipgloss.NewStyle().
	Border(lipgloss.NormalBorder(), false, false, false, true).
	BorderForeground(lipgloss.Color("#AD58B4")).
	Foreground(lipgloss.Color("#EE6FF8")).
	Padding(0, 0, 0, 1).Render

type item profile

func (i item) Title() string       { return i.Name }
func (i item) Description() string { return i.Url }
func (i item) FilterValue() string { return i.Name }

const statusMessageLifetime = 3 * time.Second

type restoreStatusMessageMsg struct{}

func (m model) setStatusMessage(msg string) tea.Cmd {
	tickCmd := tea.Tick(statusMessageLifetime, func(time.Time) tea.Msg {
		return restoreStatusMessageMsg{}
	})
	return tea.Sequence(m.list.NewStatusMessage(msg), tickCmd)
}

type model struct {
	list list.Model
	form *huh.Form

	isAdd       bool
	focusOnForm bool

	lastDPress time.Time
}

func initialModel() model {
	m := model{list: list.New(nil, list.NewDefaultDelegate(), 0, 0)}

	var items []list.Item
	for _, v := range cfg.Items {
		items = append(items, item(v))
	}
	m.list.SetItems(items)

	keyBindingFn := func() []key.Binding {
		return []key.Binding{
			key.NewBinding(
				key.WithKeys("e"),
				key.WithHelp("e", "edit"),
			),
			key.NewBinding(
				key.WithKeys("a"),
				key.WithHelp("a", "add"),
			),
			key.NewBinding(
				key.WithKeys("d"),
				key.WithHelp("dd", "delete"),
			),
			key.NewBinding(
				key.WithKeys("enter"),
				key.WithHelp("enter", "use"),
			),
		}
	}
	m.list.AdditionalShortHelpKeys = keyBindingFn
	m.list.AdditionalFullHelpKeys = keyBindingFn

	m.list.Title = "Subscriptions"

	m.list.SetStatusBarItemName("profile", "profiles")
	m.list.StatusMessageLifetime = statusMessageLifetime
	return m
}
func (m model) Init() tea.Cmd {
	return func() tea.Msg { return restoreStatusMessageMsg{} }
}

const doublePressInterval = 300 * time.Millisecond

type editorFinishedMsg struct{ err error }
type useFinishedMsg struct{ err error }

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd
	selected := m.list.SelectedItem()
	if selected == nil {
		return m, nil
	}
	selectedItem := selected.(item)

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit
		case "esc", "q":
			if m.isAdd {
				m.isAdd = false
				return m, nil
			}
			return m, tea.Quit
		// case "a":
		// 	m.isAdd = true
		// 	m.focusOnForm = true

		// 	form := initialForm()
		// 	m.form = form
		// 	return m, form.Init()
		case "tab":
			m.focusOnForm = !m.focusOnForm
			return m, nil
		case "e":
			var editorPath string
			if envEd := os.Getenv("EDITOR"); envEd != "" {
				if p, err := exec.LookPath(envEd); err == nil {
					editorPath = p
				}
			}
			if editorPath == "" {
				priorityList := []string{"nvim", "vim", "vi", "nano", "code", "notepad"}
				for _, name := range priorityList {
					if p, err := exec.LookPath(name); err == nil {
						editorPath = p
						break
					}
				}
			}
			if editorPath == "" {
				return m, m.list.NewStatusMessage(statusFailStyle("no supported editor"))
			}
			file := selectedItem.File
			c := exec.Command(editorPath, file)
			return m, tea.ExecProcess(c, func(err error) tea.Msg {
				return editorFinishedMsg{err: err}
			})
		case "d":
			now := time.Now()
			if now.Sub(m.lastDPress) <= doublePressInterval {
				m.lastDPress = time.Time{}
				if err := delProfile(selectedItem.Name); err != nil {
					return m, m.list.NewStatusMessage(statusFailStyle(err.Error()))
				}
				m.list.RemoveItem(m.list.GlobalIndex())
				return m, m.list.NewStatusMessage(statusOkStyle("Deleted " + selectedItem.Name))

			}
			m.lastDPress = now
			return m, nil
		case "enter":
			selected := m.list.SelectedItem()
			if selected == nil {
				return m, nil
			}

			usedName := selected.(item).Name
			err := useFunc(usedName)

			if err != nil {
				return m, m.list.NewStatusMessage(statusFailStyle(strings.TrimRight(err.Error(), "\n")))
			}
			return m, m.list.NewStatusMessage(statusOkStyle("used"))

		}
	case useFinishedMsg:
		m.list.StopSpinner()
		err := msg.err
		if err != nil {
			return m, m.list.NewStatusMessage(statusFailStyle(err.Error()))
		}
		return m, m.list.NewStatusMessage(statusOkStyle("used"))

	case editorFinishedMsg:
		//  todo valid config
		err := msg.err
		if err != nil {
			return m, m.list.NewStatusMessage(statusFailStyle(err.Error()))
		}

	case tea.WindowSizeMsg:
		h, v := docStyle.GetFrameSize()
		m.list.SetSize(msg.Width-h, msg.Height-v)
	}

	// if m.isAdd && m.focusOnForm {
	// 	newForm, cmd := m.form.Update(msg)
	// 	m.form = newForm.(*huh.Form)
	// 	if m.form.State == huh.StateCompleted {
	// 		m.isAdd = false

	// 	}
	// 	cmds = append(cmds, cmd)
	// } else {
	newListModel, cmd := m.list.Update(msg)
	m.list = newListModel
	cmds = append(cmds, cmd)
	use := cfg.Use
	if use != "" {
		m.list.NewStatusMessage(statusUseStyle("use: " + use))
	}

	return m, tea.Batch(cmds...)
}

func (m model) View() tea.View {
	if !m.isAdd {
		v := tea.NewView(docStyle.Render(m.list.View()))
		v.AltScreen = true
		return v

	}

	// 使用 JoinHorizontal 拼接，并确保 Top 对齐
	view := lipgloss.JoinHorizontal(
		lipgloss.Top,
		m.list.View(),
		m.form.View(),
	)
	v := tea.NewView(docStyle.Render(view))
	v.AltScreen = true
	return v

}
