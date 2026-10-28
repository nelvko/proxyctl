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
	tea "charm.land/bubbletea/v2"
	"charm.land/huh/v2"
	"github.com/charmbracelet/colorprofile"
	"github.com/nelvko/proxyctl/internal/app"
	"github.com/nelvko/proxyctl/internal/config"
	"github.com/nelvko/proxyctl/internal/i18n"
	"github.com/nelvko/proxyctl/internal/redact"
	"github.com/nelvko/proxyctl/internal/subscription"
)

const (
	doublePressInterval = 300 * time.Millisecond

	statusMessageLifetime = 2 * time.Second
)

type profile = config.Profile

type item profile

func (i item) Title() string       { return singleLine(i.Name) }
func (i item) Description() string { return singleLine(redact.URL(i.URL)) }
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
	Modify: key.NewBinding(key.WithKeys("m"), key.WithHelp("m", "")),
	Show:   key.NewBinding(key.WithKeys("s"), key.WithHelp("s", "")),
	More: key.NewBinding(
		key.WithKeys("?"),
		key.WithHelp("?", ""),
	),
	EscCancel: key.NewBinding(
		key.WithKeys("esc"),
		key.WithHelp("esc", ""),
	),
	Add: key.NewBinding(
		key.WithKeys("a"),
		key.WithHelp("a", ""),
	),
	Edit: key.NewBinding(
		key.WithKeys("e"),
		key.WithHelp("e", ""),
	),
	Delete: key.NewBinding(
		key.WithKeys("d"),
		key.WithHelp("dd", ""),
	),
	Use: key.NewBinding(
		key.WithKeys("enter"),
		key.WithHelp("enter", ""),
	),
	Quit: key.NewBinding(
		key.WithKeys("q"),
		key.WithHelp("q", ""),
	),
	Refresh: key.NewBinding(
		key.WithKeys("u"),
		key.WithHelp("u", ""),
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

	locale             i18n.Locale
	dialog             Modal
	dialogAction       string
	removeTarget       *profile
	toolbarFocus       int // -1 leaves keyboard focus on the list/form
	hoverIndex         int
	pointerX, pointerY int
	pointerKnown       bool
	feedback           string
	feedbackID         uint64
	workSpinner        spinner.Model

	styles       styles
	theme        Theme
	themeConfig  config.ThemeConfig
	colorProfile colorprofile.Profile

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
		profiles:     profiles,
		ctx:          ctx,
		list:         list.New(listItemsFromProfiles(profiles), list.NewDefaultDelegate(), 0, 0),
		toolbarFocus: -1, hoverIndex: -1,
		workSpinner: spinner.New(spinner.WithSpinner(spinner.Dot)),
	}
	preference, languageErr := config.LoadLanguage()
	var resolveErr error
	m.locale, resolveErr = i18n.Resolve(preference)
	languageErr = errors.Join(languageErr, resolveErr)
	m.list.Filter = filterProfiles
	m.list.KeyMap.Quit = keys.Quit
	// Keep native filtering, pagination and navigation. Render their labels in
	// our locale because Bubbles' built-in status/empty strings are English.
	m.list.SetShowTitle(false)
	m.list.SetShowFilter(false)
	m.list.SetShowStatusBar(false)
	m.list.SetShowHelp(false)
	m.list.SetShowPagination(false)
	m.list.FilterInput.Prompt = m.tr("sub.filter_prompt")
	m.list.FilterInput.Placeholder = m.tr("sub.filter_placeholder")
	if languageErr != nil {
		m.feedback = m.tr("language.load_failed", languageErr)
	}
	m.themeConfig, _ = loadThemeConfig()
	m.applyTheme(true)
	m.layoutSubscriptions()
	return m
}

func (m model) Init() tea.Cmd {
	return tea.Batch(tea.RequestBackgroundColor, func() tea.Msg { <-m.ctx.Done(); return cancelRequestedMsg{} })
}

type subscriptionFeedbackExpiredMsg uint64

func (m *model) errorMessage(message string) tea.Cmd {
	message = redact.Text(message)
	m.err = errors.New(message)
	m.showDialog("error", ModalOptions{Title: m.tr("sub.error"), Body: message, Kind: ModalError, Locale: m.locale})
	return nil
}

func (m *model) pendingMessage(message string) tea.Cmd {
	m.feedbackID++
	m.feedback = redact.Text(message)
	return nil
}

func (m *model) successMessage(message string) tea.Cmd {
	m.pendingMessage(message)
	id := m.feedbackID
	return tea.Tick(statusMessageLifetime, func(time.Time) tea.Msg { return subscriptionFeedbackExpiredMsg(id) })
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
	return m.successMessage(m.tr("sub.cancelled"))
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
	f := newProfileFormLocale(m.profiles, m.formDraft, original, m.locale)
	if previous != nil {
		f.timeout, f.headers, f.advanced = previous.timeout, previous.headers, previous.advanced
	}
	m.formData = f
	m.form = f.form.WithTheme(staticHuhTheme(m.theme)).WithShowHelp(false)
	m.form.SubmitCmd = func() tea.Msg { return addSubmittedMsg{} }
	m.formFocused = true
	m.toolbarFocus = -1
	m.layoutSubscriptions()
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
	m.layoutSubscriptions()
	switch msg := msg.(type) {
	case cancelRequestedMsg:
		if m.busy {
			m.quitAfter = true
			if m.cancel != nil {
				m.cancel()
			}
			return m, m.pendingMessage(m.tr("sub.cancelling"))
		}
		return m, tea.Quit
	case tea.KeyMsg:
		if key.Matches(msg, m.list.KeyMap.ForceQuit) {
			if m.busy {
				if m.cancel != nil {
					m.cancel()
				}
				return m, m.pendingMessage(m.tr("sub.cancelling"))
			}
			return m, tea.Quit
		}
		if m.busy {
			return m, nil
		}
		if m.err != nil {
			switch msg.String() {
			case "up", "down", "pgup", "pgdown", "home", "end", "ctrl+u", "ctrl+d":
				// Long diagnostics remain readable from the keyboard; other
				// keys retain the existing acknowledge-and-return behavior.
			default:
				m.err = nil
				m.dialog.Close()
				return m, nil
			}
		}
	case tea.WindowSizeMsg:
		m.width, m.height = max(0, msg.Width), max(0, msg.Height)
		m.layoutSubscriptions()
	case tea.BackgroundColorMsg:
		m.applyTheme(msg.IsDark())
		return m, nil
	case tea.ColorProfileMsg:
		m.colorProfile = msg.Profile
		return m, nil
	case spinner.TickMsg:
		var cmd tea.Cmd
		if m.busy {
			m.workSpinner, cmd = m.workSpinner.Update(msg)
		}
		return m, cmd
	case subscriptionFeedbackExpiredMsg:
		if uint64(msg) == m.feedbackID && !m.busy {
			m.feedback = ""
		}
		return m, nil
	case addSubmittedMsg:
		if m.busy {
			return m, nil
		}
		if m.formDraft == nil {
			return m, m.errorMessage(m.tr("sub.form_not_ready"))
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
				return m.tr("sub.metadata_saved"), err
			}, &draft)
		} else {
			operation = m.startRuntimeOperation(func(svc *subscription.Service) (string, error) {
				if err := svc.Add(&draft); err != nil {
					return "", err
				}
				message := m.tr("sub.added", draft.Name)
				if svc.SelectedName() == "" {
					message += m.tr("sub.next_steps")
				}
				return message, nil
			}, &draft)
		}
		return m, operation
	case operationFinishedMsg:
		if msg.warnings != "" && msg.err != nil {
			msg.err = errors.Join(msg.err, errors.New(msg.warnings))
		}
		m.busy = false
		if m.cancel != nil {
			m.cancel()
			m.cancel = nil
		}
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
		if msg.err != nil {
			if errors.Is(msg.err, context.Canceled) {
				message := m.tr("sub.cancelled")
				if m.formDraft != nil {
					message = m.tr("sub.cancelled_draft")
				}
				return m, tea.Batch(refresh, restore, m.successMessage(message))
			}
			return m, tea.Batch(refresh, restore, m.errorMessage(msg.err.Error()))
		}
		if msg.warnings != "" {
			m.showDialog("notice", ModalOptions{Title: m.tr("sub.warning"), Body: redact.Text(msg.warnings), Kind: ModalWarning, Locale: m.locale})
		}
		return m, tea.Batch(refresh, m.successMessage(msg.message))
	}
	if m.dialog.Active {
		result, cmd := m.dialog.Update(msg)
		if result.Cancelled || result.Confirmed {
			m.err = nil
			if result.Confirmed && m.dialogAction == "remove" && m.removeTarget != nil {
				target := *m.removeTarget
				m.removeTarget = nil
				return m, m.removeProfile(target, true)
			}
			m.removeTarget = nil
		}
		return m, cmd
	}
	if m.busy {
		return m, m.handleSubscriptionPointer(msg)
	}
	if cmd, handled := m.handleSubscriptionActions(msg); handled {
		return m, cmd
	}
	if m.formFocused {
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
			return m.errorMessage(m.tr("sub.no_available"))
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
				return m.profileUpdate(results[0]), nil
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
			m.showDialog("details", ModalOptions{Title: m.tr("sub.details"), Body: m.profileDetails(p, svc.SelectedName()), Locale: m.locale})
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
			return m.removeProfile(profile(selectedItem), false)
		case key.Matches(msg, keys.Use):
			return m.startRuntimeOperation(func(svc *subscription.Service) (string, error) {
				result, err := svc.Use(selectedItem.Name)
				return m.application(result), err
			}, nil)
		case key.Matches(msg, keys.Quit):
			return tea.Quit
		case key.Matches(msg, keys.More):
			m.showDialog("help", ModalOptions{Title: m.tr("sub.help"), Body: m.tr("sub.full_help"), Locale: m.locale})
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
