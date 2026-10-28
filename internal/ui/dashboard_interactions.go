package ui

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/nelvko/proxyctl/internal/redact"
)

func (m *dashboardModel) layoutPicker() {
	if m.picker != nil {
		m.picker.SetBounds(m.contentBounds())
	}
}

func (m *dashboardModel) openPicker() tea.Cmd {
	var options []SelectOption
	var title, value string
	switch m.page {
	case dashboardNodesPage:
		group, ok := m.selectedGroup()
		if !ok {
			return nil
		}
		title, value, m.pickerGroup = m.tr("dashboard.search_nodes", redact.Text(group.Name)), group.Now, group.Name
		for _, name := range group.All {
			options = append(options, SelectOption{Label: redact.Text(name), Value: name})
		}
	case dashboardSubscriptionsPage:
		title = m.tr("dashboard.search_subscriptions")
		for i, profile := range m.snapshot.profiles {
			options = append(options, SelectOption{Label: redact.Text(profile.Name), Value: profile.Name})
			if i == m.profile {
				value = profile.Name
			}
		}
	default:
		return nil
	}
	picker := NewSelectModal(title, options, value)
	picker.Locale = m.locale
	m.picker = &picker
	m.layoutPicker()
	return m.picker.Open()
}

func (m dashboardModel) updatePicker(msg tea.Msg) (tea.Model, tea.Cmd) {
	m.layoutPicker()
	result, cmd := m.picker.Update(msg)
	if result.Cancelled {
		m.picker = nil
		return m, nil
	}
	if !result.Confirmed {
		return m, cmd
	}
	m.picker = nil
	// Periodic refreshes can reorder or remove the underlying options while
	// searching. Resolve stable names against the current snapshot on commit.
	if m.page == dashboardNodesPage {
		for i, group := range m.snapshot.groups {
			if group.Name == m.pickerGroup {
				for j, name := range group.All {
					if name == result.Value {
						m.group, m.node, m.nodeFocus = i, j, true
						return m, m.pageKey("enter")
					}
				}
			}
		}
	} else if m.page == dashboardSubscriptionsPage {
		for i, profile := range m.snapshot.profiles {
			if profile.Name == result.Value {
				m.profile = i
				return m, m.pageKey("enter")
			}
		}
	}
	m.notice = m.tr("dashboard.stale")
	m.noticeError = false
	return m, nil
}

func (m *dashboardModel) dashboardWheel(msg tea.MouseWheelMsg) tea.Cmd {
	if m.busy || !m.contentBounds().Contains(msg.X, msg.Y) {
		return nil
	}
	key := ""
	if msg.Button == tea.MouseWheelUp {
		key = "up"
	} else if msg.Button == tea.MouseWheelDown {
		key = "down"
	}
	if key == "" {
		return nil
	}
	if m.page == dashboardNodesPage {
		m.nodeFocus = msg.Y-m.contentBounds().Y >= m.nodeListRow(m.contentBounds().Height)
	}
	return m.pageKey(key)
}

func (m dashboardModel) nodeListRow(height int) int {
	return min(len(m.snapshot.groups), max(2, height/3)) + 3
}

type dashboardRowKind uint8

const (
	dashboardNoRow dashboardRowKind = iota
	dashboardGroupRow
	dashboardNodeRow
	dashboardProfileRow
	dashboardConnectionRow
	dashboardRuleRow
)

type dashboardRow struct {
	kind  dashboardRowKind
	index int
}

// Hover and click share this mapping of visible terminal rows to list items.
func (m dashboardModel) dashboardRowAt(x, y int) dashboardRow {
	b := m.contentBounds()
	if m.busy || !b.Contains(x, y) {
		return dashboardRow{}
	}
	row := y - b.Y
	switch m.page {
	case dashboardNodesPage:
		limit := min(len(m.snapshot.groups), max(2, b.Height/3))
		if row >= 1 && row <= limit {
			return dashboardRow{dashboardGroupRow, listStart(m.group, limit, len(m.snapshot.groups)) + row - 1}
		} else if group, ok := m.selectedGroup(); ok {
			startRow := m.nodeListRow(b.Height)
			if row >= startRow {
				index := listStart(m.node, b.Height-startRow, len(group.All)) + row - startRow
				if index < len(group.All) {
					return dashboardRow{dashboardNodeRow, index}
				}
			}
		}
	case dashboardSubscriptionsPage:
		index := listStart(m.profile, max(0, b.Height-2), len(m.snapshot.profiles)) + row - 2
		if row >= 2 && index < len(m.snapshot.profiles) {
			return dashboardRow{dashboardProfileRow, index}
		}
	case dashboardConnectionsPage:
		available := max(0, b.Height-5)
		index := listStart(m.connection, available, len(m.snapshot.connections.Connections)) + row - 2
		if row >= 2 && row < 2+available && index < len(m.snapshot.connections.Connections) {
			return dashboardRow{dashboardConnectionRow, index}
		}
	case dashboardRulesPage:
		index := listStart(m.rule, max(0, b.Height-2), len(m.snapshot.rules)) + row - 2
		if row >= 2 && index < len(m.snapshot.rules) {
			return dashboardRow{dashboardRuleRow, index}
		}
	}
	return dashboardRow{}
}

type dashboardHeaderAction struct {
	button Button
	key    string
}

func (m dashboardModel) dashboardHeaderActions() ([]dashboardHeaderAction, string) {
	var actions []dashboardHeaderAction
	tail := ""
	switch m.page {
	case dashboardNodesPage:
		if len(m.snapshot.groups) == 0 {
			return nil, ""
		}
		actions = []dashboardHeaderAction{{button: Button{Label: m.tr("toolbar.search")}, key: "/"}}
		focus := m.tr("dashboard.groups")
		if m.nodeFocus {
			focus = m.tr("dashboard.nodes")
		}
		tail = m.tr("toolbar.nodes_tail", focus)
	case dashboardSubscriptionsPage:
		actions = []dashboardHeaderAction{
			{button: Button{Label: m.tr("toolbar.search")}, key: "/"},
			{button: Button{Label: m.tr("toolbar.apply"), Disabled: len(m.snapshot.profiles) == 0}, key: "enter"},
			{button: Button{Label: m.tr("toolbar.update"), Disabled: len(m.snapshot.profiles) == 0}, key: "u"},
		}
		tail = m.tr("toolbar.subs_tail")
	case dashboardLogsPage:
		pause := m.tr("toolbar.pause")
		if m.logPaused {
			pause = m.tr("toolbar.resume")
		}
		actions = []dashboardHeaderAction{{button: Button{Label: pause}, key: "p"}, {button: Button{Label: m.tr("toolbar.clear")}, key: "c"}}
		tail = m.tr("toolbar.logs_tail")
		if m.logPaused {
			tail += m.tr("toolbar.paused_tail")
		}
	default:
		return nil, ""
	}
	b := m.contentBounds()
	x := b.X
	for i := range actions {
		button := &actions[i].button
		width := lipgloss.Width(button.Label) + 2
		if b.Height == 0 || x+width > b.X+b.Width {
			return actions[:i], tail
		}
		button.Bounds = Rect{X: x, Y: b.Y, Width: width, Height: 1}
		button.Disabled = button.Disabled || m.busy
		button.Update(tea.MouseMotionMsg{X: m.hoverX, Y: m.hoverY})
		x += width
	}
	return actions, tail
}

func (m dashboardModel) dashboardHeaderLine(width int) (string, bool) {
	actions, tail := m.dashboardHeaderActions()
	if len(actions) == 0 {
		return "", false
	}
	var parts []string
	used := 0
	for _, action := range actions {
		parts = append(parts, action.button.View(m.theme, action.button.Bounds.Width))
		used += action.button.Bounds.Width
	}
	parts = append(parts, lipgloss.NewStyle().Foreground(themeColor(m.theme.Primary)).Render(dashboardLine(tail, max(0, width-used), false)))
	return strings.Join(parts, ""), true
}

func (m *dashboardModel) dashboardContentClick(msg tea.MouseClickMsg) tea.Cmd {
	if m.busy || !m.contentBounds().Contains(msg.X, msg.Y) {
		return nil
	}
	actions, _ := m.dashboardHeaderActions()
	for _, action := range actions {
		if action.button.Update(msg) {
			return m.pageKey(action.key)
		}
	}
	hit := m.dashboardRowAt(msg.X, msg.Y)
	switch hit.kind {
	case dashboardGroupRow:
		m.group, m.node, m.nodeFocus = hit.index, 0, false
	case dashboardNodeRow:
		m.node, m.nodeFocus = hit.index, true
	case dashboardProfileRow:
		m.profile = hit.index
	case dashboardConnectionRow:
		m.connection = hit.index
	case dashboardRuleRow:
		m.rule = hit.index
	}
	return nil
}
