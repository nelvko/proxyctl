// Package ui owns terminal presentation, forms and interactive subscription management.
package ui

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/signal"
	"reflect"
	"strings"
	"syscall"
	"time"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/list"
	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/huh/v2"
	"github.com/nelvko/proxyctl/internal/app"
	"github.com/nelvko/proxyctl/internal/config"
	"github.com/nelvko/proxyctl/internal/redact"
	"github.com/nelvko/proxyctl/internal/subscription"
)

const (
	doublePressInterval = 300 * time.Millisecond

	statusMessageLifetime = 2 * time.Second
	appBoundaryMargin     = 1
)

type profile = config.Profile

type item profile

func (i item) Title() string       { return i.Name }
func (i item) Description() string { return redact.URL(i.URL) }
func (i item) FilterValue() string {
	u, err := url.Parse(i.URL)
	if err != nil {
		return i.Name + "\x00"
	}
	return i.Name + "\x00" + u.Hostname()
}

// Search names first, then hostnames. Only name matches highlight the title.
func filterProfiles(term string, targets []string) []list.Rank {
	names, hosts := make([]string, len(targets)), make([]string, len(targets))
	for i, target := range targets {
		names[i], hosts[i], _ = strings.Cut(target, "\x00")
	}
	results := list.DefaultFilter(term, names)
	seen := make(map[int]bool, len(results))
	for _, rank := range results {
		seen[rank.Index] = true
	}
	for _, rank := range list.DefaultFilter(term, hosts) {
		if !seen[rank.Index] {
			results = append(results, list.Rank{Index: rank.Index})
		}
	}
	return results
}

func listItemsFromProfiles(profiles *subscription.Service) []list.Item {
	entries := profiles.List()
	listItems := make([]list.Item, 0, len(entries))
	for _, p := range entries {
		listItems = append(listItems, item(p))
	}
	return listItems
}

type keyMap struct {
	More      key.Binding
	EscCancel key.Binding
	Add       key.Binding
	Edit      key.Binding
	Modify    key.Binding
	Show      key.Binding
	Delete    key.Binding
	Use       key.Binding
	Quit      key.Binding
	Refresh   key.Binding
}

var keys = keyMap{
	Modify: key.NewBinding(key.WithKeys("m"), key.WithHelp("m", "modify metadata")),
	Show:   key.NewBinding(key.WithKeys("s"), key.WithHelp("s", "show details")),
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
	Edit: key.NewBinding(
		key.WithKeys("e"),
		key.WithHelp("e", "edit config"),
	),
	Delete: key.NewBinding(
		key.WithKeys("d"),
		key.WithHelp("dd", "remove"),
	),
	Use: key.NewBinding(
		key.WithKeys("enter"),
		key.WithHelp("enter", "use"),
	),
	Quit: key.NewBinding(
		key.WithKeys("q"),
		key.WithHelp("q", "quit"),
	),
	Refresh: key.NewBinding(
		key.WithKeys("u"),
		key.WithHelp("u", "update"),
	),
}

type model struct {
	profiles *subscription.Service
	ctx      context.Context

	height, width int
	list          list.Model

	form         *huh.Form
	formDraft    *config.Profile
	formFocused  bool
	formData     *profileForm
	formOriginal *profile
	details      *viewport.Model

	styles styles

	lastPress struct {
		d time.Time
	}

	busy      bool
	cancel    context.CancelFunc
	quitAfter bool
	err       error
}

type addSubmittedMsg struct{}

func initialModel(ctx context.Context, profiles *subscription.Service) model {
	m := model{
		profiles: profiles,
		ctx:      ctx,
		list:     list.New(listItemsFromProfiles(profiles), list.NewDefaultDelegate(), 0, 0),
	}
	m.list.Styles.TitleBar = m.list.Styles.TitleBar.PaddingLeft(3)
	m.list.SetSpinner(spinner.Dot)
	m.list.Filter = filterProfiles
	m.list.KeyMap.Quit = keys.Quit
	m.list.SetShowHelp(false)
	m.list.AdditionalShortHelpKeys = func() []key.Binding {
		return []key.Binding{keys.Use, keys.Add, keys.Refresh}
	}
	m.list.AdditionalFullHelpKeys = func() []key.Binding {
		return []key.Binding{keys.Use, keys.Add, keys.Refresh, keys.Show, keys.Modify, keys.Edit, keys.Delete}
	}
	m.list.Title = "selected: " + profiles.SelectedName()
	m.list.SetStatusBarItemName("profile", "profiles")
	m.list.StatusMessageLifetime = statusMessageLifetime
	return m
}

func (m model) Init() tea.Cmd {
	return tea.Batch(tea.RequestBackgroundColor, func() tea.Msg { <-m.ctx.Done(); return cancelRequestedMsg{} })
}

// message will be persistented if return nil, else will disappear after statusMessageLifetime
func (m *model) errorMessage(message string) tea.Cmd {
	message = redact.Text(message)
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

func (m *model) resetAddForm() {
	m.formFocused = false
	m.form = nil
	m.formDraft = nil
	m.formData = nil
	m.formOriginal = nil
}

func (m *model) cancelForm() tea.Cmd {
	m.resetAddForm()
	return m.successMessage("Canceled")
}

func (m *model) openAddForm() tea.Cmd {
	m.formDraft, m.formOriginal, m.formData = &profile{}, nil, nil
	return m.openProfileForm()
}

func (m *model) openProfileForm() tea.Cmd {
	original := ""
	if m.formOriginal != nil {
		original = m.formOriginal.Name
	}
	previous := m.formData
	f := newProfileForm(m.profiles, m.formDraft, original)
	if previous != nil {
		f.timeout, f.headers, f.advanced = previous.timeout, previous.headers, previous.advanced
	}
	m.formData = f
	m.form = f.form.WithShowHelp(false).WithWidth(max(1, m.width-4)).WithHeight(max(1, m.height-6))
	m.form.SubmitCmd = func() tea.Msg { return addSubmittedMsg{} }
	m.formFocused = true
	return m.form.Init()
}

func saveFormProfile(svc *subscription.Service, original, draft profile) error {
	current, err := svc.Get(original.Name)
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(current, original) {
		return errors.New("profile changed while the form was open; cancel and reopen modify to load the latest settings")
	}
	return svc.Set(original.Name, draft)
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case cancelRequestedMsg:
		if m.busy {
			m.quitAfter = true
			if m.cancel != nil {
				m.cancel()
			}
			return m, m.pendingMessage("Cancelling…")
		}
		return m, tea.Quit
	case tea.KeyMsg:
		if key.Matches(msg, m.list.KeyMap.ForceQuit) {
			if m.busy {
				if m.cancel != nil {
					m.cancel()
				}
				return m, m.pendingMessage("Cancelling…")
			}
			return m, tea.Quit
		}
		if m.busy {
			return m, nil
		}
		if m.err != nil {
			m.err = nil
			return m, m.successMessage("")
		}
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
	case tea.BackgroundColorMsg:
		m.styles = newStyles(msg.IsDark())
		return m, nil
	case addSubmittedMsg:
		if m.busy {
			return m, nil
		}
		if m.formDraft == nil {
			return m, m.errorMessage("add form is not ready")
		}
		if err := m.formData.apply(); err != nil {
			init := m.openProfileForm()
			return m, tea.Batch(init, m.errorMessage(err.Error()))
		}
		draft := *m.formDraft
		// Keep the user's draft separate: Add may infer a name before validation fails.
		m.formFocused = false
		var operation tea.Cmd
		if m.formOriginal != nil {
			original := *m.formOriginal
			operation = m.startMetadataOperation(func(svc *subscription.Service) (string, error) {
				err := saveFormProfile(svc, original, draft)
				return "Metadata saved; press u to update the source.", err
			}, &draft)
		} else {
			operation = m.startRuntimeOperation(func(svc *subscription.Service) (string, error) {
				if err := svc.Add(&draft); err != nil {
					return "", err
				}
				message := "Added " + draft.Name
				if svc.SelectedName() == "" {
					message += "; press enter to select and apply, then run proxyctl on."
				}
				return message, nil
			}, &draft)
		}
		return m, operation
	case operationFinishedMsg:
		if msg.warnings != "" {
			if msg.err != nil {
				msg.err = errors.Join(msg.err, errors.New(msg.warnings))
			} else {
				msg.message += "\n" + msg.warnings
			}
		}
		m.busy = false
		if m.cancel != nil {
			m.cancel()
			m.cancel = nil
		}
		m.list.StopSpinner()
		if msg.profiles != nil {
			// The completed operation context has been cancelled; preserve the UI lifetime.
			m.profiles = msg.profiles.WithContext(m.ctx)
		}
		if m.quitAfter {
			return m, tea.Quit
		}
		if msg.added != "" {
			m.list.ResetFilter()
		}
		refresh := m.list.SetItems(listItemsFromProfiles(m.profiles))
		var restore tea.Cmd
		if m.formDraft != nil {
			if msg.err != nil {
				restore = m.openProfileForm()
			} else {
				m.resetAddForm()
			}
		}
		if msg.added != "" {
			for i, p := range m.profiles.List() {
				if p.Name == msg.added {
					m.list.Select(i)
					break
				}
			}
		}
		m.list.Title = "selected: " + m.profiles.SelectedName()
		if msg.err != nil {
			if errors.Is(msg.err, context.Canceled) {
				message := "Cancelled"
				if m.formDraft != nil {
					message += "; draft retained"
				}
				return m, tea.Batch(refresh, restore, m.successMessage(message))
			}
			return m, tea.Batch(refresh, restore, m.errorMessage(msg.err.Error()))
		}
		return m, tea.Batch(refresh, m.successMessage(msg.message))
	}
	if m.details != nil {
		if k, ok := msg.(tea.KeyMsg); ok && key.Matches(k, keys.EscCancel, keys.Quit) {
			m.details = nil
			return m, nil
		}
		m.details.SetWidth(max(1, m.width-4))
		m.details.SetHeight(max(1, m.height-6))
		v, cmd := m.details.Update(msg)
		m.details = &v
		return m, cmd
	}
	if m.formFocused {
		m.form.WithWidth(max(1, m.width-4)).WithHeight(max(1, m.height-6))
		return m, m.handleForm(msg)
	}
	return m, m.handleList(msg)
}

func (m *model) handleList(msg tea.Msg) tea.Cmd {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		if m.list.FilterState() == list.Filtering {
			break
		}

		selectedItem, ok := m.list.SelectedItem().(item)
		if (!ok || len(m.list.Items()) == 0) && key.Matches(msg, keys.Delete, keys.Edit, keys.Modify, keys.Show, keys.Refresh, keys.Use) {
			return m.errorMessage("No profiles available. Please add a profile first.")
		}

		switch {
		case key.Matches(msg, keys.Add):
			return m.openAddForm()
		case key.Matches(msg, keys.Refresh):
			return m.startRuntimeOperation(func(svc *subscription.Service) (string, error) {
				results, err := svc.Update(subscription.DownloadOptions{}, selectedItem.Name)
				if err != nil {
					return "", err
				}
				return ProfileUpdate(results[0]), nil
			}, nil)
		case key.Matches(msg, keys.Show):
			svc, err := app.LoadProfileMetadata(m.ctx)
			if err != nil {
				return m.errorMessage(err.Error())
			}
			p, err := svc.Get(selectedItem.Name)
			if err != nil {
				return m.errorMessage(err.Error())
			}
			v := viewport.New()
			v.SetWidth(max(1, m.width-4))
			v.SetHeight(max(1, m.height-6))
			v.SetContent(ProfileDetails(p, svc.SelectedName(), false))
			m.details = &v
			return nil
		case key.Matches(msg, keys.Modify):
			svc, err := app.LoadProfileMetadata(m.ctx)
			if err != nil {
				return m.errorMessage(err.Error())
			}
			p, err := svc.Get(selectedItem.Name)
			if err != nil {
				return m.errorMessage(err.Error())
			}
			original := p
			m.formOriginal, m.formDraft, m.formData = &original, &p, nil
			return m.openProfileForm()
		case key.Matches(msg, keys.Edit):
			return m.startEditor(selectedItem.Name)
		case key.Matches(msg, keys.Delete):
			if time.Since(m.lastPress.d) > doublePressInterval {
				m.lastPress.d = time.Now()
				return nil
			}
			m.lastPress.d = time.Time{}
			return m.startMetadataOperation(func(svc *subscription.Service) (string, error) {
				err := svc.Delete(selectedItem.Name, false)
				return "Removed " + selectedItem.Name, err
			}, nil)
		case key.Matches(msg, keys.Use):
			return m.startRuntimeOperation(func(svc *subscription.Service) (string, error) {
				result, err := svc.Use(selectedItem.Name)
				return Application(result), err
			}, nil)
		case key.Matches(msg, keys.Quit):
			return tea.Quit
		case key.Matches(msg, keys.More):
			m.list.SetShowHelp(!m.list.ShowHelp())
			m.list.Help.ShowAll = m.list.ShowHelp()
			return nil
		}
	}

	newListModel, cmd := m.list.Update(msg)
	m.list = newListModel
	cmds = append(cmds, cmd)
	return tea.Batch(cmds...)
}

func (m *model) handleForm(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if key.Matches(msg, keys.EscCancel) {
			return m.cancelForm()
		}
	}

	newForm, cmd := m.form.Update(msg)
	m.form = newForm.(*huh.Form)
	return cmd
}

func RunSubscriptions(ctx context.Context, profiles *subscription.Service) error {
	// The full-screen TUI renders its own spinner; a stderr progress line
	// from geodata/kernel downloads would corrupt the altscreen.
	ctx, cancel := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer cancel()
	p := tea.NewProgram(initialModel(ctx, profiles.WithContext(ctx)), tea.WithoutSignalHandler())
	if _, err := p.Run(); err != nil {
		return fmt.Errorf("error running program: %w", err)
	}
	return ctx.Err()
}
