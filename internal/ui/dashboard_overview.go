package ui

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"runtime"
	"strings"

	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/nelvko/proxyctl/internal/redact"
)

type overviewState struct {
	viewport              viewport.Model
	groupName, pickerKind string
}

func newOverviewState() overviewState { return overviewState{viewport: viewport.New()} }

func (m dashboardModel) overviewError() error {
	err := m.snapshot.err
	if m.snapshot.profileErr != nil && !errors.Is(err, m.snapshot.profileErr) {
		err = errors.Join(err, m.snapshot.profileErr)
	}
	return errors.Join(err, m.snapshot.runtime.EndpointErr, m.version.Err)
}

func (m *dashboardModel) openOverviewDiagnostics() {
	if err := m.overviewError(); err != nil && !m.busy {
		m.dialog = NewModal(ModalOptions{Title: m.tr("overview.diagnostics"),
			Body: redact.Text(err.Error()) + "\n\n" + m.tr("overview.retry_hint"), Kind: ModalWarning, Locale: m.locale})
		m.dialog.Open()
		m.dialog.SetBounds(Rect{Width: m.width, Height: m.height})
	}
}

func (m dashboardModel) overviewGroup() (dashboardProxy, bool) {
	for _, group := range m.snapshot.groups {
		if group.Name == m.overview.groupName || m.overview.groupName == "" {
			return group, true
		}
	}
	return dashboardProxy{}, false
}

func (m *dashboardModel) layoutOverview() {
	b := m.contentBounds()
	v := &m.overview.viewport
	v.SetWidth(b.Width)
	v.SetHeight(b.Height)
	lines, _ := m.buildOverview(b.Width)
	offset := v.YOffset()
	v.SetContentLines(lines)
	if offset != v.YOffset() {
		lines, _ = m.buildOverview(b.Width)
		v.SetContentLines(lines)
	}
}

func (m dashboardModel) overviewViewLines(width, height int) []string {
	if width <= 0 || height <= 0 {
		return nil
	}
	m.layoutOverview()
	return strings.Split(m.overview.viewport.View(), "\n")
}

// The document and its controls share one layout. Hit regions are clipped to
// fully visible buttons, so scrolling cannot activate an off-screen action.
func (m dashboardModel) overviewActions() []dashboardHeaderAction {
	_, actions := m.buildOverview(m.contentBounds().Width)
	visible := actions[:0]
	for _, action := range actions {
		if action.button.Bounds.Width > 0 && action.button.Bounds.Height > 0 {
			visible = append(visible, action)
		}
	}
	return visible
}

func (m dashboardModel) overviewDocument(width, height int) ([]string, Rect) {
	if width <= 0 || height <= 0 {
		return nil, Rect{}
	}
	lines, actions := m.buildOverview(width)
	for _, action := range actions {
		if action.key == "d" {
			return lines, action.button.Bounds
		}
	}
	return lines, Rect{}
}

func (m *dashboardModel) overviewKey(key string) tea.Cmd {
	switch key {
	case "p":
		return m.switchPage(dashboardSubscriptionsPage)
	case "n":
		return m.switchPage(dashboardNodesPage)
	case "g":
		m.overview.pickerKind = "group"
		return m.openOverviewPicker()
	case "/":
		m.overview.pickerKind = "node"
		return m.openOverviewPicker()
	case "d":
		m.openOverviewDiagnostics()
	default:
		m.layoutOverview()
		v := &m.overview.viewport
		switch key {
		case "up", "k":
			v.ScrollUp(1)
		case "down", "j":
			v.ScrollDown(1)
		case "pgup":
			v.PageUp()
		case "pgdown", "space":
			v.PageDown()
		case "home":
			v.GotoTop()
		case "end":
			v.GotoBottom()
		}
	}
	return nil
}

func (m *dashboardModel) openOverviewPicker() tea.Cmd {
	var title, value string
	var options []SelectOption
	if m.overview.pickerKind == "group" {
		title = m.tr("overview.select_group")
		if group, ok := m.overviewGroup(); ok {
			value = group.Name
		}
		for _, group := range m.snapshot.groups {
			options = append(options, SelectOption{Label: redact.Text(group.Name), Value: group.Name})
		}
	} else {
		group, ok := m.overviewGroup()
		if !ok || !strings.EqualFold(group.Type, "selector") || m.snapshot.controller == nil {
			m.overview.pickerKind = ""
			return nil
		}
		m.overview.pickerKind = "node"
		title, value, m.pickerGroup = m.tr("dashboard.search_nodes", redact.Text(group.Name)), group.Now, group.Name
		for _, name := range group.All {
			options = append(options, SelectOption{Label: redact.Text(name), Value: name})
		}
	}
	if len(options) == 0 {
		m.overview.pickerKind = ""
		return nil
	}
	picker := NewSelectModal(title, options, value)
	picker.Locale = m.locale
	m.picker = &picker
	m.layoutPicker()
	return m.picker.Open()
}

func (m *dashboardModel) confirmOverviewPicker(value string) tea.Cmd {
	kind := m.overview.pickerKind
	m.overview.pickerKind = ""
	for _, group := range m.snapshot.groups {
		if kind == "group" && group.Name == value {
			m.overview.groupName = value
			return nil
		}
		if kind == "node" && group.Name == m.pickerGroup && strings.EqualFold(group.Type, "selector") {
			for _, name := range group.All {
				if name == value {
					return m.applyDashboardNode(group.Name, name)
				}
			}
		}
	}
	m.notice, m.noticeError = m.tr("dashboard.stale"), false
	return nil
}

// Both the node page and Overview use the existing controller mutation path.
func (m *dashboardModel) applyDashboardNode(group, name string) tea.Cmd {
	if m.busy || m.snapshot.controller == nil {
		return nil
	}
	m.busy = true
	ctx, cancel := context.WithCancel(m.ctx)
	m.actionCancel = cancel
	controller, message := m.snapshot.controller, m.tr("dashboard.selected", redact.Text(name))
	return func() tea.Msg {
		return dashboardActionMsg{message: message, err: controller.selectProxy(ctx, group, name)}
	}
}

type overviewField struct{ label, value, color string }
type overviewCardContent struct {
	title   string
	fields  []overviewField
	actions []dashboardHeaderAction
}

func (m dashboardModel) overviewCards() []overviewCardContent {
	fg, muted := m.theme.Foreground, m.theme.Muted
	field := func(key, value string) overviewField {
		if value == "" || value == "-" {
			value = "—"
		}
		return overviewField{m.tr(key), singleLine(redact.Text(value)), fg}
	}
	profileName, source, format, count := m.displayState(m.snapshot.profile), "—", "—", fmt.Sprint(len(m.snapshot.profiles))
	for _, profile := range m.snapshot.profiles {
		if profile.Name != m.snapshot.profile {
			continue
		}
		format = profile.Format
		if format == "" {
			format = "clash"
		}
		if profile.File != "" {
			source = m.tr("overview.local_file")
		} else if parsed, err := url.Parse(profile.URL); err == nil && parsed.Hostname() != "" {
			source = parsed.Hostname()
		}
		break
	}
	if m.snapshot.profileErr != nil {
		profileName, source, format, count = "—", "—", "—", "—"
	}
	profiles := overviewCardContent{title: m.tr("overview.profile_card"), fields: []overviewField{
		field("overview.profile_label", profileName), field("overview.source_label", source),
		field("overview.format_label", format), field("overview.count_label", count),
	}, actions: []dashboardHeaderAction{{button: Button{Label: m.tr("overview.open_profiles")}, key: "p"}}}

	group, groupOK := m.overviewGroup()
	groupName, groupType, current, latency := "—", "—", "—", "—"
	if groupOK {
		groupName, groupType, current = group.Name, group.Type, group.Now
		if proxy, ok := m.snapshot.proxies[current]; ok && len(proxy.History) > 0 {
			if delay := proxy.History[len(proxy.History)-1].Delay; delay > 0 {
				latency = fmt.Sprintf("%d ms", delay)
			}
		}
	}
	nodes := overviewCardContent{title: m.tr("overview.node_card"), fields: []overviewField{
		field("overview.group_label", groupName), field("overview.type_label", groupType),
		field("overview.current_label", current), field("overview.latency_label", latency),
	}, actions: []dashboardHeaderAction{
		{button: Button{Label: m.tr("overview.choose_group"), Disabled: len(m.snapshot.groups) == 0}, key: "g"},
		{button: Button{Label: m.tr("overview.choose_node"), Disabled: !groupOK || !strings.EqualFold(group.Type, "selector") || m.snapshot.controller == nil}, key: "/"},
	}}
	connections, uploaded, downloaded, memory := "—", "—", "—", "—"
	metrics := m.snapshot.connections
	if metrics.Available {
		connections = fmt.Sprint(len(metrics.Connections))
	}
	if metrics.UploadTotalKnown {
		uploaded = overviewBytes(metrics.UploadTotal)
	}
	if metrics.DownloadTotalKnown {
		downloaded = overviewBytes(metrics.DownloadTotal)
	}
	if metrics.Memory != nil {
		memory = overviewBytes(*metrics.Memory)
	}
	traffic := overviewCardContent{title: m.tr("overview.traffic_card"), fields: []overviewField{
		field("overview.connection_label", connections), field("overview.upload_total", uploaded),
		field("overview.download_total", downloaded), field("overview.memory_label", memory),
	}}
	info := m.snapshot.runtime
	platform := info.Platform
	if platform == "" {
		platform = runtime.GOOS + "/" + runtime.GOARCH
	}
	version := "—"
	if m.version.Key != "" && m.version.Key == info.VersionKey {
		if m.version.Loading {
			version = m.tr("state.loading")
		} else if m.version.Value != "" {
			version = m.version.Value
		}
	}
	service := m.displayState(m.snapshot.service)
	if info.Service != "" {
		if info.ServiceAvailable {
			service += " · " + info.Service
		} else {
			service = m.tr("overview.service_unsupported")
		}
	}
	environment := overviewCardContent{title: m.tr("overview.environment_card"), fields: []overviewField{
		field("overview.kernel_label", m.displayState(m.snapshot.kernel)), field("overview.type_label", info.KernelType),
		field("overview.service_label", service), field("overview.version_label", version),
		field("overview.http_label", info.HTTP), field("overview.socks_label", info.SOCKS),
		field("overview.platform_label", platform), field("overview.shell_label", m.displayState(m.snapshot.shell)),
	}}
	environment.fields[2].color = muted
	if info.ServiceAvailable && m.snapshot.service == "运行中" {
		environment.fields[2].color = m.theme.Success
	}
	return []overviewCardContent{profiles, nodes, traffic, environment}
}

func overviewBytes(value int64) string {
	if value < 1024 {
		return fmt.Sprintf("%d B", max(0, value))
	}
	n := float64(value)
	for i, unit := range [...]string{"KiB", "MiB", "GiB", "TiB"} {
		n /= 1024
		if n < 1024 || i == 3 {
			return fmt.Sprintf("%.1f %s", n, unit)
		}
	}
	return "—"
}

func (m dashboardModel) buildOverview(width int) ([]string, []dashboardHeaderAction) {
	if width <= 0 {
		return nil, nil
	}
	inset := min(2, max(0, (width-1)/2))
	panelWidth := min(112, max(1, width-2*inset))
	prefix := strings.Repeat(" ", inset)
	lines := []string{""}
	var actions []dashboardHeaderAction
	b := m.contentBounds()
	offset := m.overview.viewport.YOffset()
	place := func(action dashboardHeaderAction, x, y int) dashboardHeaderAction {
		action.button.Bounds.X += b.X + x
		action.button.Bounds.Y += b.Y + y - offset
		action.button.Disabled = action.button.Disabled || m.busy
		r := action.button.Bounds
		if r.X < b.X || r.X+r.Width > b.X+b.Width || r.Y < b.Y || r.Y+r.Height > b.Y+b.Height {
			action.button.Bounds = Rect{}
		}
		action.button.Update(tea.MouseMotionMsg{X: m.hoverX, Y: m.hoverY})
		return action
	}
	if m.overviewError() != nil {
		message := m.tr("overview.status_warning")
		if m.snapshot.profileErr != nil {
			message = m.tr("overview.profile_warning")
		}
		warning := lipgloss.NewStyle().Foreground(themeColor(m.theme.Warning))
		lines = append(lines, prefix+warning.Render(dashboardLine("! "+message, panelWidth, false)))
		button := dashboardHeaderAction{key: "d", button: Button{Label: m.tr("overview.details"), Bounds: Rect{Width: min(panelWidth, lipgloss.Width(m.tr("overview.details"))+2), Height: 1}}}
		button = place(button, inset, len(lines))
		actions = append(actions, button)
		lines = append(lines, prefix+button.button.View(m.theme, min(panelWidth, lipgloss.Width(button.button.Label)+2)), "")
	}
	cards := m.overviewCards()
	columns := 1
	if panelWidth >= 76 {
		columns = 2
	}
	cardWidth := (panelWidth - 2*(columns-1)) / columns
	for start := 0; start < len(cards); start += columns {
		var rowCards [][]string
		rowHeight := 0
		for col := 0; col < columns && start+col < len(cards); col++ {
			x, y := inset+col*(cardWidth+2), len(lines)
			card, controls := m.overviewCard(cards[start+col], cardWidth, func(a dashboardHeaderAction) dashboardHeaderAction { return place(a, x, y) })
			rowCards = append(rowCards, card)
			actions = append(actions, controls...)
			rowHeight = max(rowHeight, len(card))
		}
		for row := 0; row < rowHeight; row++ {
			parts := []string{prefix}
			for col, card := range rowCards {
				if col > 0 {
					parts = append(parts, "  ")
				}
				line := ""
				if row < len(card) {
					line = card[row]
				}
				parts = append(parts, gridLine(line, cardWidth))
			}
			lines = append(lines, strings.Join(parts, ""))
		}
		lines = append(lines, "")
	}
	if m.snapshot.kernel == "未选择" || m.snapshot.kernel == "" {
		lines = append(lines, prefix+lipgloss.NewStyle().Foreground(themeColor(m.theme.Primary)).Bold(true).Render(m.tr("overview.start_title")),
			prefix+m.tr("overview.install_step"), prefix+"proxyctl install / proxyctl use",
			prefix+m.tr("overview.subscription_step"), prefix+"proxyctl sub add <url>")
	} else if m.snapshot.controllerHint != "" {
		// Long hints wrap in the same viewport instead of disappearing off-grid.
		lines = append(lines, strings.Split(lipgloss.NewStyle().Foreground(themeColor(m.theme.Muted)).Width(panelWidth).Render(m.displayHint(m.snapshot.controllerHint)), "\n")...)
	}
	return lines, actions
}

func (m dashboardModel) overviewCard(card overviewCardContent, width int, place func(dashboardHeaderAction) dashboardHeaderAction) ([]string, []dashboardHeaderAction) {
	inner := max(0, width-2)
	border := lipgloss.NewStyle().Foreground(themeColor(m.theme.Border)).Background(themeColor(m.theme.Surface))
	base := lipgloss.NewStyle().Foreground(themeColor(m.theme.Foreground)).Background(themeColor(m.theme.Surface))
	wrap := func(content string) string { return border.Render("│") + content + border.Render("│") }
	if width < 4 {
		return []string{gridLine(card.title, width)}, nil
	}
	lines := []string{border.Render("╭" + strings.Repeat("─", inner) + "╮"),
		wrap(base.Foreground(themeColor(m.theme.Primary)).Bold(true).Render(dashboardLine(" "+card.title, inner, false)))}
	labelWidth := 0
	for _, f := range card.fields {
		labelWidth = max(labelWidth, lipgloss.Width(f.label)+2)
	}
	fieldWidth := max(0, inner-2)
	labelWidth = min(labelWidth, fieldWidth/2)
	for _, f := range card.fields {
		label := base.Foreground(themeColor(m.theme.Muted)).Render(dashboardLine(f.label, labelWidth, false))
		value := base.Foreground(themeColor(f.color)).Render(dashboardLine(f.value, max(0, fieldWidth-labelWidth), false))
		lines = append(lines, wrap(base.Render(" ")+label+value+base.Render(" ")))
	}
	var actions []dashboardHeaderAction
	x := 1
	buttonLine := ""
	flush := func() {
		if buttonLine != "" {
			lines = append(lines, wrap(base.Render(" ")+buttonLine+base.Render(strings.Repeat(" ", max(0, inner-x)))))
			buttonLine = ""
			x = 1
		}
	}
	for _, action := range card.actions {
		w := min(max(0, inner-2), lipgloss.Width(action.button.Label)+2)
		if x+w > inner-1 {
			flush()
		}
		action.button.Bounds = Rect{X: 1 + x, Y: len(lines), Width: w, Height: 1}
		action = place(action)
		actions = append(actions, action)
		buttonLine += base.Render(action.button.View(m.theme, w))
		x += w
	}
	flush()
	return append(lines, border.Render("╰"+strings.Repeat("─", inner)+"╯")), actions
}
