package ui

import (
	"errors"
	"io"
	"reflect"

	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
	"charm.land/huh/v2"
	"charm.land/lipgloss/v2"
	"github.com/nelvko/proxyctl/internal/subscription"
)

type subscriptionAction struct {
	id string
	Button
}

type subscriptionLayout struct {
	actions       []subscriptionAction
	list, message Rect
}

// The parent supplies only cell geometry and pointer mapping. Bubbles continues
// to own filtering, page/cursor state and item rendering.
func (m *model) layoutSubscriptions() subscriptionLayout {
	var actions []string
	switch {
	case m.busy:
		actions = []string{"cancel"}
	case m.formFocused:
		actions = []string{"previous", "next", "cancel"}
		if m.form != nil {
			switch m.form.GetFocusedField().(type) {
			case *huh.Confirm:
				actions = append(actions, "yes", "no")
			case *huh.Select[string]:
				actions = append(actions, "up", "down")
			}
		}
	default:
		actions = []string{"add", "use", "update", "details", "modify", "edit", "remove", "help", "quit"}
	}
	layout := subscriptionLayout{}
	x, row := 0, 0
	maxRows := min(3, max(0, m.height-7))
	for _, id := range actions {
		label := m.tr("sub." + id)
		width := lipgloss.Width(label) + 2
		if width > m.width {
			continue
		}
		if x > 0 && x+width > m.width {
			x, row = 0, row+1
		}
		if row >= maxRows {
			break
		}
		_, hasSelection := m.list.SelectedItem().(item)
		disabled := !m.formFocused && !m.busy && !hasSelection && id != "add" && id != "help" && id != "quit"
		b := Button{Label: label, Disabled: disabled, Focused: !m.formFocused && len(layout.actions) == m.toolbarFocus,
			Bounds: Rect{X: x, Y: 2 + row, Width: width, Height: 1}}
		b.Hovered = !disabled && m.pointerKnown && b.Bounds.Contains(m.pointerX, m.pointerY)
		layout.actions = append(layout.actions, subscriptionAction{id, b})
		x += width + 1
	}
	rows := 0
	if len(layout.actions) > 0 {
		rows = layout.actions[len(layout.actions)-1].Bounds.Y - 1
	}
	layout.message = Rect{Y: 2 + rows, Width: m.width, Height: 1}
	layout.list = Rect{Y: 3 + rows, Width: m.width, Height: max(0, m.height-5-rows)}
	if m.toolbarFocus >= len(layout.actions) {
		m.toolbarFocus = -1
	}
	m.list.SetSize(m.width, layout.list.Height)
	m.list.FilterInput.SetWidth(max(1, m.width-lipgloss.Width(m.list.FilterInput.Prompt)))
	m.hoverIndex = -1
	if m.pointerKnown {
		m.hoverIndex = m.subscriptionRowAt(layout.list, m.pointerX, m.pointerY)
	}
	m.list.SetDelegate(subscriptionDelegate{DefaultDelegate: profileDelegate(m.theme), hover: m.hoverIndex, theme: m.theme})
	if m.form != nil {
		m.form.WithWidth(max(1, m.width-2)).WithHeight(max(1, layout.list.Height))
	}
	m.dialog.SetBounds(Rect{Width: m.width, Height: m.height})
	return layout
}

func (m model) subscriptionRowAt(bounds Rect, x, y int) int {
	if bounds.Width <= 3 || bounds.Height < 2 || !bounds.Contains(x, y) {
		return -1
	}
	const stride = 3 // native DefaultDelegate: title, description, spacing
	row := (y - bounds.Y) / stride
	if (y-bounds.Y)%stride == 2 || row >= m.list.Paginator.PerPage {
		return -1
	}
	start, end := m.list.Paginator.GetSliceBounds(len(m.list.VisibleItems()))
	if start+row >= end || bounds.Y+row*stride+2 > bounds.Y+bounds.Height {
		return -1
	}
	return start + row
}

type subscriptionDelegate struct {
	list.DefaultDelegate
	hover int
	theme Theme
}

func (d subscriptionDelegate) Render(w io.Writer, m list.Model, index int, entry list.Item) {
	if index == d.hover && index != m.Index() {
		d.Styles.NormalTitle = d.Styles.NormalTitle.Background(themeColor(d.theme.Hover))
		d.Styles.NormalDesc = d.Styles.NormalDesc.Background(themeColor(d.theme.Hover))
	}
	d.DefaultDelegate.Render(w, m, index, entry)
}

func (m *model) showDialog(action string, options ModalOptions) {
	m.dialog, m.dialogAction = NewModal(options), action
	m.dialog.Open()
	m.dialog.SetBounds(Rect{Width: m.width, Height: m.height})
}

func (m *model) removeProfile(target profile, requireUnchanged bool) tea.Cmd {
	return m.startMetadataOperation(func(svc *subscription.Service) (string, error) {
		if requireUnchanged {
			current, err := svc.Get(target.Name)
			if err != nil {
				return "", err
			}
			if !reflect.DeepEqual(current, target) {
				return "", errors.New(m.tr("sub.remove_changed"))
			}
		}
		err := svc.Delete(target.Name, false)
		return m.tr("sub.removed", target.Name), err
	}, nil)
}

func (m *model) activateSubscriptionAction(id string) tea.Cmd {
	if id == "cancel" {
		if m.busy {
			if m.cancel != nil {
				m.cancel()
			}
			return m.pendingMessage(m.tr("sub.cancelling"))
		}
		return m.cancelForm()
	}
	if m.busy {
		return nil
	}
	if m.formFocused {
		input := map[string]tea.KeyPressMsg{
			"previous": {Code: tea.KeyTab, Mod: tea.ModShift}, "next": {Code: tea.KeyEnter},
			"yes": {Code: 'y'}, "no": {Code: 'n'}, "up": {Code: tea.KeyUp}, "down": {Code: tea.KeyDown},
		}
		if key, ok := input[id]; ok {
			// Send normal keys through Huh so mouse Next respects validation.
			return m.handleForm(key)
		}
		return nil
	}
	if id == "remove" {
		if selected, ok := m.list.SelectedItem().(item); ok {
			target := profile(selected)
			m.removeTarget = &target
			m.showDialog("remove", ModalOptions{Title: m.tr("sub.remove_title"), Body: m.tr("sub.remove_body", target.Name),
				ConfirmLabel: m.tr("sub.remove"), Kind: ModalConfirm, Locale: m.locale})
		}
		return nil
	}
	code := map[string]rune{"add": 'a', "use": tea.KeyEnter, "update": 'u', "details": 's', "modify": 'm', "edit": 'e', "help": '?', "quit": 'q'}[id]
	if code == 0 {
		return nil
	}
	var accept tea.Cmd
	if m.list.FilterState() == list.Filtering {
		m.list, accept = m.list.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	}
	return tea.Batch(accept, m.handleList(tea.KeyPressMsg{Code: code}))
}

func (m *model) handleSubscriptionPointer(msg tea.Msg) tea.Cmd {
	cmd, _ := m.handleSubscriptionActions(msg)
	return cmd
}

func (m *model) handleSubscriptionActions(msg tea.Msg) (tea.Cmd, bool) {
	layout := m.layoutSubscriptions()
	switch input := msg.(type) {
	case tea.MouseMotionMsg:
		m.pointerX, m.pointerY, m.pointerKnown = input.X, input.Y, true
		return nil, true
	case tea.MouseClickMsg:
		if input.Button != tea.MouseLeft {
			return nil, true
		}
		for _, action := range layout.actions {
			if action.Button.Update(msg) {
				return m.activateSubscriptionAction(action.id), true
			}
		}
		if m.busy || m.formFocused {
			return nil, true
		}
		m.toolbarFocus = -1
		if layout.message.Contains(input.X, input.Y) {
			if m.list.FilterState() != list.Filtering {
				var cmd tea.Cmd
				m.list, cmd = m.list.Update(tea.KeyPressMsg{Code: '/'})
				return cmd, true
			}
		} else if index := m.subscriptionRowAt(layout.list, input.X, input.Y); index >= 0 {
			var cmd tea.Cmd
			if m.list.FilterState() == list.Filtering {
				m.list, cmd = m.list.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
			}
			m.list.Select(index)
			return cmd, true
		}
		return nil, true
	case tea.MouseWheelMsg:
		if m.busy || m.formFocused || !layout.list.Contains(input.X, input.Y) {
			return nil, true
		}
		if input.Button == tea.MouseWheelUp {
			m.list.CursorUp()
		} else if input.Button == tea.MouseWheelDown {
			m.list.CursorDown()
		}
		return nil, true
	case tea.KeyPressMsg:
		if m.busy || m.formFocused || m.list.FilterState() == list.Filtering {
			return nil, false
		}
		if input.String() == "tab" || input.String() == "shift+tab" {
			direction := 1
			if input.String() == "shift+tab" {
				direction = -1
				if m.toolbarFocus < 0 {
					m.toolbarFocus = len(layout.actions)
				}
			}
			for step := 0; step <= len(layout.actions); step++ {
				m.toolbarFocus += direction
				if m.toolbarFocus < 0 || m.toolbarFocus >= len(layout.actions) {
					m.toolbarFocus = -1
					return nil, true
				}
				if !layout.actions[m.toolbarFocus].Disabled {
					return nil, true
				}
			}
		}
		if m.toolbarFocus >= 0 && m.toolbarFocus < len(layout.actions) {
			switch input.String() {
			case "left", "right":
				direction := -1
				if input.String() == "right" {
					direction = 1
				}
				for i := m.toolbarFocus + direction; i >= 0 && i < len(layout.actions); i += direction {
					if !layout.actions[i].Disabled {
						m.toolbarFocus = i
						break
					}
				}
				return nil, true
			case "enter", "space":
				action := layout.actions[m.toolbarFocus]
				if !action.Disabled {
					return m.activateSubscriptionAction(action.id), true
				}
				return nil, true
			case "esc", "up", "down":
				m.toolbarFocus = -1
				return nil, true
			}
		}
	}
	return nil, false
}
