package ui

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/colorprofile"
	"github.com/charmbracelet/x/ansi"
	"github.com/nelvko/proxyctl/internal/app"
	"github.com/nelvko/proxyctl/internal/config"
	"github.com/nelvko/proxyctl/internal/i18n"
	"github.com/nelvko/proxyctl/internal/redact"
	"github.com/nelvko/proxyctl/internal/subscription"
)

const dashboardWideWidth = 110

const (
	dashboardOverviewPage = iota
	dashboardNodesPage
	dashboardSubscriptionsPage
	dashboardConnectionsPage
	dashboardRulesPage
	dashboardLogsPage
	dashboardSettingsPage
)

var dashboardPages = [...]string{"概览", "节点", "订阅", "连接", "规则", "日志", "设置"}

type dashboardSnapshot struct {
	kernel, service, profile, proxy, shell, configFile string
	profiles                                           []config.Profile
	groups                                             []dashboardProxy
	proxies                                            map[string]dashboardProxy
	connections                                        dashboardConnections
	rules                                              []dashboardRule
	controller                                         *controllerClient
	controllerHint                                     string
	err                                                error
	profileErr                                         error
	runtime                                            dashboardRuntimeInfo
}

func readDashboardSnapshot(ctx context.Context, page int) dashboardSnapshot {
	var result dashboardSnapshot
	result.runtime = readDashboardRuntimeInfo(nil, nil)
	result.kernel, result.service, result.profile, result.proxy = "未选择", "-", "未选择", "-"
	result.shell = "未启用"
	for _, name := range []string{"HTTP_PROXY", "http_proxy", "HTTPS_PROXY", "https_proxy", "ALL_PROXY", "all_proxy"} {
		if os.Getenv(name) != "" {
			result.shell = "已启用"
			break
		}
	}
	profiles, err := app.LoadProfileMetadata(ctx)
	if err != nil {
		result.err, result.profileErr = err, err
	} else {
		result.profiles = profiles.List()
		if profiles.SelectedName() != "" {
			result.profile = profiles.SelectedName()
		}
	}
	a, err := app.Load()
	if err != nil {
		result.err = err
		return result
	}
	cfg := a.Config.SelectedKernel()
	if cfg == nil {
		result.controllerHint = "未选择内核；先运行 proxyctl install 或 proxyctl use"
		return result
	}
	result.kernel = cfg.Name
	result.configFile = cfg.ConfigFile
	result.runtime = readDashboardRuntimeInfo(cfg, nil)
	k, err := a.Kernel()
	if err != nil {
		result.service = "未知"
		result.err = err
		return result
	}
	if active, err := k.IsActive(); err != nil {
		result.service = "未知"
		result.err = err
	} else if active {
		result.service = "运行中"
	} else {
		result.service = "已停止"
	}
	result.runtime = readDashboardRuntimeInfo(cfg, k)
	if result.runtime.HTTP != "" {
		result.proxy = result.runtime.HTTP
	} else if result.runtime.SOCKS != "" {
		result.proxy = result.runtime.SOCKS
	}
	controller, err := controllerFromConfig(cfg)
	if err != nil {
		result.controllerHint = redact.Text(err.Error())
		return result
	}
	if controller == nil {
		result.controllerHint = "控制 API 未启用；在生效的内核配置中设置本地 external-controller / clash_api"
		return result
	}
	result.controller = controller
	if result.service != "运行中" {
		result.controllerHint = "内核未运行；启动后可查看节点、连接、规则和实时日志"
		return result
	}
	if page == dashboardOverviewPage || page == dashboardNodesPage {
		result.groups, result.proxies, err = controller.proxies(ctx)
		if err != nil {
			result.controllerHint = "读取节点失败: " + redact.Text(err.Error())
		}
	}
	if page == dashboardOverviewPage || page == dashboardConnectionsPage {
		result.connections, err = controller.connections(ctx)
		if err != nil {
			result.controllerHint = "读取连接失败: " + redact.Text(err.Error())
		}
	}
	if page == dashboardRulesPage {
		result.rules, err = controller.rules(ctx)
		if err != nil {
			result.controllerHint = "读取规则失败: " + redact.Text(err.Error())
		}
	}
	return result
}

type dashboardLoadedMsg struct {
	page     int
	snapshot dashboardSnapshot
}
type dashboardTickMsg struct{}
type dashboardCancelledMsg struct{}
type dashboardActionMsg struct {
	message string
	err     error
}
type dashboardLogMsg struct {
	generation int
	line       string
	err        error
	done       bool
}
type dashboardTrafficMsg struct {
	generation int
	rate       dashboardTraffic
	err        error
	done       bool
}

type dashboardModel struct {
	ctx                                    context.Context
	width, height                          int
	page                                   int
	group, node, profile, connection, rule int
	navFocused                             bool
	nodeFocus                              bool
	loading, busy                          bool
	actionCancel                           context.CancelFunc
	quitAfter                              bool
	snapshot                               dashboardSnapshot
	notice                                 string
	noticeError                            bool
	logs                                   []string
	logPaused                              bool
	logScroll                              int
	logCancel                              context.CancelFunc
	logEvents                              chan dashboardLogMsg
	logGeneration                          int
	traffic                                dashboardTraffic
	trafficAvailable                       bool
	trafficAt                              time.Time
	trafficCancel                          context.CancelFunc
	trafficEvents                          chan dashboardTrafficMsg
	trafficGeneration                      int
	ruleTick                               int
	darkBG                                 bool
	styles                                 styles
	theme                                  Theme
	themeConfig                            config.ThemeConfig
	locale                                 i18n.Locale
	language                               string
	colorProfile                           colorprofile.Profile
	hoverPage                              int
	hoverX, hoverY                         int
	appearance                             appearanceState
	overview                               overviewState
	version                                dashboardVersionState
	dialog                                 Modal
	picker                                 *SelectModal
	pickerGroup                            string
}

func newDashboardModel(ctx context.Context) dashboardModel {
	cfg, loadErr := loadThemeConfig()
	language, languageErr := config.LoadLanguage()
	locale, localeErr := i18n.Resolve(language)
	theme, themeErr := resolveTheme(cfg, true)
	notice := ""
	if loadErr != nil {
		notice = i18n.Text(locale, "appearance.load_failed", loadErr.Error())
	} else if themeErr != nil {
		notice = i18n.Text(locale, "appearance.fallback", themeErr.Error())
	}
	if languageErr != nil {
		notice = i18n.Text(locale, "language.load_failed", languageErr.Error())
	} else if localeErr != nil {
		notice = i18n.Text(locale, "language.load_failed", localeErr.Error())
	}
	return dashboardModel{
		locale: locale, language: language,
		theme: theme, themeConfig: cfg, hoverPage: -1, hoverX: -1, hoverY: -1, notice: notice, noticeError: notice != "",
		appearance: newAppearanceState(cfg),
		overview:   newOverviewState(),
		ctx:        ctx,
		loading:    true,
		navFocused: true,
		darkBG:     true,
		styles:     newThemedStyles(theme),
		snapshot: dashboardSnapshot{
			kernel: "读取中", service: "-", profile: "-", proxy: "-", shell: "-",
		},
	}
}

func (m dashboardModel) Init() tea.Cmd {
	return tea.Batch(tea.RequestBackgroundColor, m.loadSnapshot(), dashboardTick(), func() tea.Msg {
		<-m.ctx.Done()
		return dashboardCancelledMsg{}
	})
}

func (m dashboardModel) loadSnapshot() tea.Cmd {
	page := m.page
	return func() tea.Msg { return dashboardLoadedMsg{page, readDashboardSnapshot(m.ctx, page)} }
}

func dashboardTick() tea.Cmd {
	return tea.Tick(2*time.Second, func(time.Time) tea.Msg { return dashboardTickMsg{} })
}

func (m *dashboardModel) switchPage(page int) tea.Cmd {
	if page < 0 || page >= len(dashboardPages) {
		return nil
	}
	m.cancelAppearance()
	m.picker = nil
	m.overview.pickerKind = ""
	m.hoverPage = -1
	m.hoverX, m.hoverY = -1, -1
	m.navFocused = true
	if page == m.page {
		return nil
	}
	if m.logCancel != nil {
		m.logCancel()
		m.logCancel = nil
		m.logEvents = nil
	}
	m.page = page
	if page == dashboardRulesPage {
		m.ruleTick = 0
	}
	m.notice = ""
	if !m.loading && !m.busy {
		m.loading = true
		return m.loadSnapshot()
	}
	return nil
}

func sameController(a, b *controllerClient) bool {
	return a != nil && b != nil && a.base == b.base && a.secret == b.secret
}

func (m *dashboardModel) startLogs() tea.Cmd {
	if m.page != dashboardLogsPage || m.snapshot.controller == nil || m.snapshot.service != "运行中" || m.logCancel != nil {
		return nil
	}
	ctx, cancel := context.WithCancel(m.ctx)
	m.logCancel = cancel
	m.logGeneration++
	generation := m.logGeneration
	events := make(chan dashboardLogMsg, 128)
	m.logEvents = events
	controller := m.snapshot.controller
	go func() {
		defer close(events)
		lines := make(chan string, 128)
		finished := make(chan error, 1)
		go func() { finished <- controller.streamLogs(ctx, lines); close(lines) }()
		for line := range lines {
			select {
			case events <- dashboardLogMsg{generation: generation, line: line}:
			case <-ctx.Done():
				return
			}
		}
		err := <-finished
		if ctx.Err() == nil {
			select {
			case events <- dashboardLogMsg{generation: generation, err: err, done: true}:
			case <-ctx.Done():
			}
		}
	}()
	return nextDashboardLog(events, generation)
}

func nextDashboardLog(events <-chan dashboardLogMsg, generation int) tea.Cmd {
	return func() tea.Msg {
		event, ok := <-events
		if !ok {
			return dashboardLogMsg{generation: generation, done: true}
		}
		return event
	}
}

func (m dashboardModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m.dialog.Active {
		switch input := msg.(type) {
		case tea.KeyPressMsg:
			if input.String() != "ctrl+c" {
				_, cmd := m.dialog.Update(msg)
				return m, cmd
			}
		case tea.MouseMsg:
			_, cmd := m.dialog.Update(msg)
			return m, cmd
		}
	}
	if m.picker != nil {
		switch input := msg.(type) {
		case tea.KeyPressMsg:
			if input.String() != "ctrl+c" {
				return m.updatePicker(msg)
			}
		case tea.MouseMsg:
			return m.updatePicker(msg)
		}
	}
	if m.page == dashboardSettingsPage && !m.busy {
		if handled, cmd := m.updateAppearance(msg); handled {
			return m, cmd
		}
	}
	switch msg := msg.(type) {
	case dashboardCancelledMsg:
		m.stopTraffic()
		if m.busy {
			m.quitAfter = true
			if m.actionCancel != nil {
				m.actionCancel()
			}
			return m, nil
		}
		if m.logCancel != nil {
			m.logCancel()
		}
		return m, tea.Quit
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.hoverPage = -1
		m.hoverX, m.hoverY = -1, -1
		m.layoutAppearance()
		m.layoutPicker()
		m.layoutOverview()
		m.dialog.SetBounds(Rect{Width: m.width, Height: m.height})
		return m, nil
	case tea.BackgroundColorMsg:
		m.darkBG = msg.IsDark()
		m.applyTheme()
		return m, nil
	case tea.ColorProfileMsg:
		m.colorProfile = msg.Profile
		return m, nil
	case appearanceSavedMsg:
		return m, m.appearanceSaved(msg)
	case languageSavedMsg:
		return m, m.languageSaved(msg)
	case dashboardVersionMsg:
		m.version.Apply(msg)
		return m, nil
	case dashboardTickMsg:
		cmds := []tea.Cmd{dashboardTick()}
		if m.trafficCancel != nil && time.Since(m.trafficAt) > 5*time.Second {
			m.stopTraffic()
		}
		if trafficCmd := m.startTraffic(); trafficCmd != nil {
			cmds = append(cmds, trafficCmd)
		}
		// Rules can be large; refresh them less often than live status pages.
		if m.page == dashboardRulesPage {
			m.ruleTick++
		}
		if !m.loading && !m.busy && (m.page != dashboardRulesPage || m.ruleTick%5 == 0) {
			m.loading = true
			cmds = append(cmds, m.loadSnapshot())
		}
		return m, tea.Batch(cmds...)
	case dashboardLoadedMsg:
		if msg.page != m.page {
			return m, m.loadSnapshot()
		}
		m.loading = false
		old := m.snapshot.controller
		m.snapshot = msg.snapshot
		if m.overview.groupName == "" && len(m.snapshot.groups) > 0 {
			m.overview.groupName = m.snapshot.groups[0].Name
		}
		versionCmd := m.version.Refresh(m.ctx, m.snapshot.runtime)
		if m.trafficCancel != nil && (!sameController(old, m.snapshot.controller) || m.snapshot.service != "运行中") {
			m.stopTraffic()
		}
		if m.group >= len(m.snapshot.groups) {
			m.group = max(0, len(m.snapshot.groups)-1)
		}
		if group, ok := m.selectedGroup(); ok {
			m.node = clamp(m.node, len(group.All))
		} else {
			m.node = 0
		}
		if m.profile >= len(m.snapshot.profiles) {
			m.profile = max(0, len(m.snapshot.profiles)-1)
		}
		if m.connection >= len(m.snapshot.connections.Connections) {
			m.connection = max(0, len(m.snapshot.connections.Connections)-1)
		}
		if m.rule >= len(m.snapshot.rules) {
			m.rule = max(0, len(m.snapshot.rules)-1)
		}
		if m.page == dashboardLogsPage {
			if m.logCancel != nil && !sameController(old, m.snapshot.controller) {
				m.logCancel()
				m.logCancel = nil
			}
			return m, tea.Batch(m.startLogs(), m.startTraffic(), versionCmd)
		}
		return m, tea.Batch(m.startTraffic(), versionCmd)
	case dashboardActionMsg:
		m.busy = false
		if m.actionCancel != nil {
			m.actionCancel()
			m.actionCancel = nil
		}
		if m.quitAfter {
			m.stopTraffic()
			if m.logCancel != nil {
				m.logCancel()
			}
			return m, tea.Quit
		}
		if msg.err != nil {
			m.notice = redact.Text(msg.err.Error())
		} else {
			m.notice = msg.message
		}
		m.noticeError = msg.err != nil
		m.loading = true
		return m, m.loadSnapshot()
	case dashboardLogMsg:
		if msg.generation != m.logGeneration || m.page != dashboardLogsPage {
			return m, nil
		}
		if msg.done {
			if msg.err != nil && !errors.Is(msg.err, context.Canceled) {
				m.notice = m.tr("dashboard.logs_lost", redact.Text(msg.err.Error()))
				m.noticeError = true
			}
			m.logCancel = nil
			m.logEvents = nil
			return m, nil
		}
		if !m.logPaused {
			m.logs = append(m.logs, msg.line)
			if len(m.logs) > 1000 {
				m.logs = m.logs[len(m.logs)-1000:]
			}
		}
		return m, nextDashboardLog(m.logEvents, m.logGeneration)
	case dashboardTrafficMsg:
		if msg.generation != m.trafficGeneration {
			return m, nil
		}
		if msg.done {
			m.stopTraffic()
			return m, nil
		}
		m.traffic = msg.rate
		m.trafficAvailable = true
		m.trafficAt = time.Now()
		return m, nextDashboardTraffic(m.trafficEvents, m.trafficGeneration)
	case tea.MouseMotionMsg:
		m.hoverPage = dashboardClickPageWithLabels(m.width, m.height, msg.X, msg.Y, m.pageNames())
		m.hoverX, m.hoverY = msg.X, msg.Y
		return m, nil
	case tea.MouseWheelMsg:
		m.hoverX, m.hoverY = msg.X, msg.Y
		return m, m.dashboardWheel(msg)
	case tea.MouseClickMsg:
		if msg.Button != tea.MouseLeft {
			return m, nil
		}
		if page := dashboardClickPageWithLabels(m.width, m.height, msg.X, msg.Y, m.pageNames()); page >= 0 {
			return m, m.switchPage(page)
		}
		wide := dashboardSidebarLayout(m.width, m.height)
		contentClick := !wide && msg.Y >= 2 || wide && msg.X >= 19
		if contentClick && msg.Y > 0 && msg.Y < m.height-1 {
			m.navFocused = false
		}
		return m, m.dashboardContentClick(msg)
	case tea.KeyPressMsg:
		key := msg.String()
		m.hoverX, m.hoverY = -1, -1
		if !m.busy {
			m.notice = ""
		}
		if key == "ctrl+c" || key == "q" {
			m.cancelAppearance()
			if m.busy {
				m.quitAfter = true
				m.notice = m.tr("state.cancelling")
				m.noticeError = false
				if m.actionCancel != nil {
					m.actionCancel()
				}
				return m, nil
			}
			if m.logCancel != nil {
				m.logCancel()
			}
			m.stopTraffic()
			return m, tea.Quit
		}
		if m.busy {
			return m, nil
		}
		if m.page == dashboardOverviewPage && key == "d" {
			m.openOverviewDiagnostics()
			return m, nil
		}
		if len(key) == 1 && key[0] >= '1' && key[0] < '1'+byte(len(dashboardPages)) {
			return m, m.switchPage(int(key[0] - '1'))
		}
		if key == "tab" {
			return m, m.switchPage((m.page + 1) % len(dashboardPages))
		}
		if key == "shift+tab" {
			return m, m.switchPage((m.page + len(dashboardPages) - 1) % len(dashboardPages))
		}
		if key == "r" && !m.loading {
			m.version.Invalidate()
			m.loading = true
			return m, m.loadSnapshot()
		}
		if m.navFocused {
			switch key {
			case "up", "k", "left":
				return m, m.switchPage((m.page + len(dashboardPages) - 1) % len(dashboardPages))
			case "down", "j", "right":
				return m, m.switchPage((m.page + 1) % len(dashboardPages))
			case "enter":
				m.navFocused = false
				if m.page == dashboardSettingsPage {
					m.focusAppearance()
				}
			}
			return m, nil
		}
		if key == "esc" || (key == "left" && m.page != dashboardNodesPage) {
			m.navFocused = true
			return m, nil
		}
		return m, m.pageKey(key)
	}
	if m.picker != nil {
		return m.updatePicker(msg)
	}
	return m, nil
}

func (m *dashboardModel) pageKey(key string) tea.Cmd {
	if m.page == dashboardOverviewPage {
		return m.overviewKey(key)
	}
	if key == "/" {
		return m.openPicker()
	}
	delta := 0
	if key == "up" || key == "k" {
		delta = -1
	}
	if key == "down" || key == "j" {
		delta = 1
	}
	switch m.page {
	case dashboardNodesPage:
		if key == "left" {
			m.nodeFocus = false
			return nil
		}
		if key == "right" {
			m.nodeFocus = true
			return nil
		}
		if !m.nodeFocus {
			m.group = clamp(m.group+delta, len(m.snapshot.groups))
			if delta != 0 {
				m.node = 0
			}
			if key == "enter" {
				m.nodeFocus = true
			}
			return nil
		}
		group, ok := m.selectedGroup()
		if !ok {
			return nil
		}
		m.node = clamp(m.node+delta, len(group.All))
		if key == "enter" && len(group.All) > 0 {
			return m.applyDashboardNode(group.Name, group.All[m.node])
		}
	case dashboardSubscriptionsPage:
		m.profile = clamp(m.profile+delta, len(m.snapshot.profiles))
		if (key == "enter" || key == "u") && len(m.snapshot.profiles) > 0 {
			m.busy = true
			name := m.snapshot.profiles[m.profile].Name
			ctx, cancel := context.WithCancel(m.ctx)
			m.actionCancel = cancel
			return func() tea.Msg {
				var result string
				err := app.MutateRuntimeProfiles(ctx, nil, func(svc *subscription.Service) error {
					if key == "u" {
						_, err := svc.Update(subscription.DownloadOptions{}, name)
						result = m.tr("dashboard.updated", name)
						return err
					}
					_, err := svc.Use(name)
					result = m.tr("dashboard.applied", name)
					return err
				})
				return dashboardActionMsg{message: result, err: err}
			}
		}
	case dashboardConnectionsPage:
		m.connection = clamp(m.connection+delta, len(m.snapshot.connections.Connections))
	case dashboardRulesPage:
		m.rule = clamp(m.rule+delta, len(m.snapshot.rules))
	case dashboardLogsPage:
		if key == "p" {
			m.logPaused = !m.logPaused
		}
		if key == "c" {
			m.logs = nil
			m.logScroll = 0
		}
		m.logScroll = max(0, min(m.logScroll-delta, max(0, len(m.logs)-1)))
	}
	return nil
}

func clamp(value, size int) int {
	if size == 0 {
		return 0
	}
	return min(max(value, 0), size-1)
}

func (m dashboardModel) selectedGroup() (dashboardProxy, bool) {
	if m.group >= len(m.snapshot.groups) {
		return dashboardProxy{}, false
	}
	return m.snapshot.groups[m.group], true
}

func RunDashboard(ctx context.Context) error {
	ctx, cancel := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer cancel()
	p := tea.NewProgram(newDashboardModel(ctx), tea.WithoutSignalHandler())
	if _, err := p.Run(); err != nil {
		return fmt.Errorf("run dashboard: %w", err)
	}
	return ctx.Err()
}

func (m dashboardModel) View() tea.View {
	w, h := max(1, m.width), max(1, m.height)
	wide := dashboardSidebarLayout(w, h)
	contentWidth := w
	if wide {
		contentWidth = max(1, w-19)
	}
	pages := m.pageNames()
	header := pages[m.page]
	if !wide {
		header = "proxyctl · " + header
	}
	lines := []string{m.dashboardBoundary(header, w, lipgloss.Center,
		lipgloss.NewStyle().Foreground(themeColor(m.theme.Primary)).Bold(true))}
	if !wide {
		lines = append(lines, m.dashboardTabs(w))
	}
	bodyHeight := max(0, h-len(lines)-1)
	body := m.pageLines(bodyHeight)
	styledBody := false
	if m.page == dashboardOverviewPage {
		body = m.overviewViewLines(contentWidth, bodyHeight)
		styledBody = true
	}
	if m.page == dashboardSettingsPage {
		body = m.settingsViewLines(contentWidth, bodyHeight)
		styledBody = true
	}
	if m.picker != nil {
		body = strings.Split(m.picker.View(m.theme, contentWidth, bodyHeight), "\n")
		styledBody = true
	}
	renderBody := func(value string, width, row int) string {
		if styledBody {
			return gridLine(value, width)
		}
		if row == 0 {
			if header, ok := m.dashboardHeaderLine(width); ok {
				return header
			}
		}
		line := m.dashboardBodyLine(value, width)
		b := m.contentBounds()
		hover := m.dashboardRowAt(m.hoverX, m.hoverY)
		if hover.kind != dashboardNoRow && hover == m.dashboardRowAt(b.X, b.Y+row) {
			line = lipgloss.NewStyle().Background(themeColor(m.theme.Hover)).Render(line)
		}
		return line
	}
	for len(body) < bodyHeight {
		body = append(body, "")
	}
	const sidebarBottomGap = 1
	var sidebarRates []string
	if wide {
		upload, download := "—", "—"
		if m.trafficAvailable {
			upload = dashboardRate(m.traffic.Up)
			download = dashboardRate(m.traffic.Down)
		}
		rateRows := []string{
			"  ──────────────",
			"  ↑ " + upload,
			"  ↓ " + download,
		}
		// Keep one empty row between navigation/rates and below rates.
		rateRoom := bodyHeight - (2*len(dashboardPages) + 3 + sidebarBottomGap)
		if rateRoom >= len(rateRows) {
			sidebarRates = rateRows
		}
	}
	sidebarRatesStart := bodyHeight - len(sidebarRates) - sidebarBottomGap
	for i := 0; i < bodyHeight; i++ {
		if wide {
			nav := ""
			selected := false
			if i == 1 {
				nav = "  ◆ proxyctl"
			}
			if i >= 3 && (i-3)%2 == 0 && (i-3)/2 < len(dashboardPages) {
				index := (i - 3) / 2
				nav = fmt.Sprintf("  %d  %s", index+1, pages[index])
				selected = index == m.page
				if selected {
					nav = fmt.Sprintf(" ▶ %d  %s", index+1, pages[index])
				}
			}
			navStyle := lipgloss.NewStyle().Foreground(themeColor(m.theme.Muted))
			if i == 1 {
				navStyle = navStyle.Foreground(m.styles.Indigo).Bold(true)
			}
			if selected {
				if m.navFocused {
					navStyle = navStyle.Foreground(themeColor(m.theme.Foreground)).Background(themeColor(m.theme.Selected)).Bold(true)
				} else {
					navStyle = navStyle.Foreground(m.styles.Indigo).Background(themeColor(m.theme.Selected)).Bold(true)
				}
			}
			if i >= 3 && (i-3)%2 == 0 && (i-3)/2 == m.hoverPage && !selected {
				navStyle = navStyle.Foreground(themeColor(m.theme.Foreground)).Background(themeColor(m.theme.Hover))
			}
			if len(sidebarRates) > 0 && i >= sidebarRatesStart && i < sidebarRatesStart+len(sidebarRates) {
				rateRow := i - sidebarRatesStart
				nav = sidebarRates[rateRow]
				switch rateRow {
				case 0:
					navStyle = navStyle.Foreground(themeColor(m.theme.Border))
				case 1:
					if m.trafficAvailable {
						navStyle = navStyle.Foreground(themeColor(m.theme.Warning))
					}
				case 2:
					if m.trafficAvailable {
						navStyle = navStyle.Foreground(themeColor(m.theme.Primary))
					}
				}
			}
			left := navStyle.Render(dashboardLine(nav, 18, false))
			divider := lipgloss.NewStyle().Foreground(themeColor(m.theme.Border)).Render("│")
			lines = append(lines, left+divider+renderBody(body[i], contentWidth, i))
		} else {
			lines = append(lines, renderBody(body[i], w, i))
		}
	}
	footer := m.tr("help.content")
	if m.navFocused {
		footer = m.tr("help.nav")
		if !wide {
			footer = m.tr("help.tabs")
		}
		if m.page == dashboardOverviewPage {
			footer = m.tr("help.overview")
			if !wide {
				footer = m.tr("help.overview_tabs")
			}
		}
	} else {
		switch m.page {
		case dashboardOverviewPage:
			footer = m.tr("help.overview_content")
			if w < 80 {
				footer = m.tr("help.overview_compact")
			}
		case dashboardNodesPage:
			footer = m.tr("help.nodes")
		case dashboardSubscriptionsPage:
			footer = m.tr("help.subscriptions")
		case dashboardConnectionsPage:
			footer = m.tr("help.connections")
		case dashboardRulesPage:
			footer = m.tr("help.rules")
		case dashboardLogsPage:
			footer = m.tr("help.logs")
		case dashboardSettingsPage:
			footer = m.tr("help.appearance")
		}
	}
	if m.appearance.dropdownOpen() {
		footer = m.tr("help.preview")
	}
	if m.picker != nil {
		footer = m.tr("help.picker")
	}
	if m.busy {
		footer = m.tr("state.working")
	}
	if m.notice != "" {
		footer = m.notice
	}
	footerStyle := lipgloss.NewStyle().Foreground(themeColor(m.theme.Muted))
	if m.busy {
		footerStyle = footerStyle.Foreground(m.styles.Indigo).Bold(true)
	} else if m.notice != "" {
		footerStyle = footerStyle.Foreground(m.styles.Green).Bold(true)
		if m.noticeError {
			footerStyle = footerStyle.Foreground(m.styles.Red)
		}
	}
	if m.page == dashboardOverviewPage && m.overviewError() != nil && m.notice == "" {
		footer += "  ·  " + m.tr("overview.details_short")
	}
	lines = append(lines, m.dashboardBoundary(footer, w, lipgloss.Left, footerStyle))
	// Keep even a 1-row terminal within its reported character grid.
	if len(lines) > h {
		lines = lines[:h]
	}
	content := strings.Join(lines, "\n")
	if m.dialog.Active {
		content = m.dialog.Overlay(content, m.theme, w, h)
	}
	v := tea.NewView(terminalFrame(content, w, h, m.theme, m.colorProfile))
	v.AltScreen = true
	v.MouseMode = tea.MouseModeAllMotion
	return v
}

func dashboardSidebarLayout(width, height int) bool {
	return width >= dashboardWideWidth && height >= 2*len(dashboardPages)+5
}

func dashboardClickPage(width, height, x, y int) int {
	return dashboardClickPageWithLabels(width, height, x, y, dashboardPages[:])
}

func dashboardClickPageWithLabels(width, height, x, y int, pages []string) int {
	if dashboardSidebarLayout(width, height) {
		if x < 0 || x >= 18 || y < 4 || y >= height-1 {
			return -1
		}
		index := (y - 4) / 2
		if (y-4)%2 == 0 && index < len(dashboardPages) {
			return index
		}
		return -1
	}
	if x < 0 || x >= width || y != 1 || y >= height-1 {
		return -1
	}
	start := 0
	mode := dashboardTabLayoutWithLabels(width, pages)
	for i, label := range pages {
		end := start + lipgloss.Width(dashboardTab(i, label, false, mode))
		if x >= start && x < end {
			return i
		}
		start = end
	}
	return -1
}

func (m dashboardModel) dashboardTabs(width int) string {
	pages := m.pageNames()
	var parts []string
	used := 0
	mode := dashboardTabLayoutWithLabels(width, pages)
	for i, label := range pages {
		part := dashboardTab(i, label, i == m.page, mode)
		partWidth := lipgloss.Width(part)
		if used+partWidth > width {
			break
		}
		style := lipgloss.NewStyle().Foreground(themeColor(m.theme.Muted))
		if i == m.page {
			if m.navFocused {
				style = style.Foreground(themeColor(m.theme.Foreground)).Background(themeColor(m.theme.Selected)).Bold(true)
			} else {
				style = style.Foreground(m.styles.Indigo).Background(themeColor(m.theme.Selected)).Bold(true)
			}
		} else if i == m.hoverPage {
			style = style.Foreground(themeColor(m.theme.Foreground)).Background(themeColor(m.theme.Hover))
		}
		parts = append(parts, style.Render(part))
		used += partWidth
	}
	if used < width {
		parts = append(parts, strings.Repeat(" ", width-used))
	}
	return strings.Join(parts, "")
}

type dashboardTabMode uint8

const (
	dashboardTabsFull dashboardTabMode = iota
	dashboardTabsCompact
	dashboardTabsDigits
	dashboardTabsBareDigits
)

func dashboardTabLayout(width int) dashboardTabMode {
	return dashboardTabLayoutWithLabels(width, dashboardPages[:])
}

func dashboardTabLayoutWithLabels(width int, pages []string) dashboardTabMode {
	fullWidth := 0
	for i, label := range pages {
		fullWidth += lipgloss.Width(dashboardTab(i, label, false, dashboardTabsFull))
	}
	switch {
	case width >= fullWidth:
		return dashboardTabsFull
	case width >= 5*len(dashboardPages):
		return dashboardTabsCompact
	case width >= 3*len(dashboardPages):
		return dashboardTabsDigits
	default:
		return dashboardTabsBareDigits
	}
}

func (m dashboardModel) dashboardBoundary(text string, width int, position lipgloss.Position, style lipgloss.Style) string {
	if width <= 0 {
		return ""
	}
	style = style.Background(themeColor(m.theme.Background))
	if width < 8 {
		return style.Render(dashboardLine(text, width, false))
	}
	labelWidth := min(lipgloss.Width(singleLine(text)), width-6)
	label := style.Padding(0, 2).Render(dashboardLine(text, labelWidth, false))
	return lipgloss.PlaceHorizontal(width, position, label,
		lipgloss.WithWhitespaceChars("/"),
		lipgloss.WithWhitespaceStyle(lipgloss.NewStyle().Foreground(themeColor(m.theme.Border)).Background(themeColor(m.theme.Background))))
}

func (m dashboardModel) dashboardBodyLine(value string, width int) string {
	style := lipgloss.NewStyle().Foreground(themeColor(m.theme.Foreground))
	if strings.Contains(value, "▶") {
		if m.navFocused {
			style = style.Foreground(themeColor(m.theme.Muted))
		} else {
			style = style.Foreground(themeColor(m.theme.Foreground)).Background(themeColor(m.theme.Selected)).Bold(true)
		}
	} else if strings.Contains(value, strings.TrimSpace(m.tr("dashboard.current"))) {
		style = style.Foreground(m.styles.Green)
	} else if strings.HasPrefix(value, "  ◇") || m.hasMessagePrefix(value, "dashboard.nodes_header", "dashboard.sub_header", "dashboard.connections", "dashboard.rules", "dashboard.logs_header") {
		style = style.Foreground(m.styles.Indigo).Bold(true)
	} else if m.hasMessagePrefix(value, "settings.error", "hint.nodes_failed", "hint.connections_failed", "hint.rules_failed") {
		style = style.Foreground(m.styles.Red)
	} else if strings.Contains(value, m.tr("state.running")) || strings.Contains(value, m.tr("state.enabled")) {
		style = style.Foreground(m.styles.Green)
	} else if strings.Contains(value, m.tr("state.disabled")) || strings.Contains(value, m.tr("state.unselected")) {
		style = style.Foreground(themeColor(m.theme.Muted))
	}
	return style.Render(dashboardLine(value, width, false))
}

func dashboardTab(index int, label string, selected bool, mode dashboardTabMode) string {
	switch mode {
	case dashboardTabsBareDigits:
		return fmt.Sprint(index + 1)
	case dashboardTabsDigits:
		if selected {
			return fmt.Sprintf("[%d]", index+1)
		}
		return fmt.Sprintf(" %d ", index+1)
	case dashboardTabsCompact:
		label = string([]rune(label)[0])
		if selected {
			return fmt.Sprintf("[%d%s]", index+1, label)
		}
		return fmt.Sprintf(" %d%s ", index+1, label)
	}
	if selected {
		return fmt.Sprintf("[%d %s]", index+1, label)
	}
	return fmt.Sprintf(" %d %s ", index+1, label)
}

func dashboardLine(value string, width int, highlight bool) string {
	// Sanitize controls, then preserve complete grapheme clusters when clipping.
	value = singleLine(value)
	line := ansi.Truncate(value, width, "")
	if used := lipgloss.Width(line); used < width {
		line += strings.Repeat(" ", width-used)
	}
	if highlight {
		return lipgloss.NewStyle().Reverse(true).Render(line)
	}
	return line
}

func (m dashboardModel) pageLines(height int) []string {
	switch m.page {
	case dashboardNodesPage:
		return m.nodeLines(height)
	case dashboardSubscriptionsPage:
		return m.profileLines(height)
	case dashboardConnectionsPage:
		return m.connectionLines(height)
	case dashboardRulesPage:
		return m.ruleLines(height)
	case dashboardLogsPage:
		return m.logLines(height)
	case dashboardSettingsPage:
		return m.settingsLines(height)
	}
	return nil
}

func (m dashboardModel) nodeLines(height int) []string {
	if m.snapshot.controllerHint != "" && len(m.snapshot.groups) == 0 {
		return []string{"", "  " + m.displayHint(m.snapshot.controllerHint)}
	}
	if len(m.snapshot.groups) == 0 {
		return []string{"", m.tr("dashboard.no_groups")}
	}
	focus := m.tr("dashboard.groups")
	if m.nodeFocus {
		focus = m.tr("dashboard.nodes")
	}
	lines := []string{m.tr("dashboard.nodes_header", focus)}
	groupLimit := min(len(m.snapshot.groups), max(2, height/3))
	groupStart := listStart(m.group, groupLimit, len(m.snapshot.groups))
	for i := groupStart; i < len(m.snapshot.groups) && i < groupStart+groupLimit; i++ {
		g := m.snapshot.groups[i]
		mark := " "
		if i == m.group {
			mark = "▶"
		}
		lines = append(lines, "  "+mark+" "+redact.Text(g.Name+" → "+g.Now))
	}
	group, _ := m.selectedGroup()
	lines = append(lines, "", m.tr("dashboard.group_nodes", redact.Text(group.Name)))
	available := max(0, height-len(lines))
	start := listStart(m.node, available, len(group.All))
	for i := start; i < len(group.All) && i < start+available; i++ {
		name := group.All[i]
		mark := " "
		if i == m.node {
			mark = "▶"
		}
		selected := ""
		if name == group.Now {
			selected = m.tr("dashboard.current")
		}
		latency := ""
		if p, ok := m.snapshot.proxies[name]; ok && len(p.History) > 0 {
			latency = fmt.Sprintf("  %dms", p.History[len(p.History)-1].Delay)
		}
		lines = append(lines, "  "+mark+" "+redact.Text(name)+latency+selected)
	}
	return lines
}

func (m dashboardModel) profileLines(height int) []string {
	lines := []string{m.tr("dashboard.sub_header"), ""}
	if len(m.snapshot.profiles) == 0 {
		return append(lines, m.tr("dashboard.no_subscriptions"))
	}
	available := max(0, height-len(lines))
	start := listStart(m.profile, available, len(m.snapshot.profiles))
	for i := start; i < len(m.snapshot.profiles) && i < start+available; i++ {
		p := m.snapshot.profiles[i]
		mark, selected := " ", ""
		if i == m.profile {
			mark = "▶"
		}
		if p.Name == m.snapshot.profile {
			selected = m.tr("dashboard.current")
		}
		lines = append(lines, "  "+mark+" "+redact.Text(p.Name)+selected+"  "+redact.URL(p.URL))
	}
	return lines
}

func (m dashboardModel) connectionLines(height int) []string {
	if m.snapshot.controllerHint != "" && len(m.snapshot.connections.Connections) == 0 {
		return []string{"", "  " + m.displayHint(m.snapshot.controllerHint)}
	}
	items := m.snapshot.connections.Connections
	lines := []string{m.tr("dashboard.connections", len(items), byteCount(m.snapshot.connections.UploadTotal), byteCount(m.snapshot.connections.DownloadTotal)), ""}
	if len(items) == 0 {
		return append(lines, m.tr("dashboard.no_connections"))
	}
	available := max(0, height-len(lines)-3)
	start := listStart(m.connection, available, len(items))
	for i := start; i < len(items) && i < start+available; i++ {
		item := items[i]
		mark := " "
		if i == m.connection {
			mark = "▶"
		}
		host := item.Metadata.Host
		if host == "" {
			host = item.Metadata.DestinationIP
		}
		line := fmt.Sprintf("  %s %-30s %-14s ↑%s ↓%s", mark, redact.Text(host), redact.Text(item.Rule), byteCount(item.Upload), byteCount(item.Download))
		lines = append(lines, line)
	}
	selected := items[m.connection]
	lines = append(lines, "", m.tr("dashboard.process", redact.Text(selected.Metadata.Process), redact.Text(strings.Join(selected.Chains, " → "))))
	return lines
}

func (m dashboardModel) logLines(height int) []string {
	lines := []string{m.tr("dashboard.logs_header")}
	if m.snapshot.controllerHint != "" && m.logCancel == nil {
		return append(lines, "", "  "+m.displayHint(m.snapshot.controllerHint))
	}
	if m.logPaused {
		lines[0] += m.tr("dashboard.paused")
	}
	available := max(0, height-len(lines))
	end := max(0, len(m.logs)-m.logScroll)
	start := max(0, end-available)
	for _, line := range m.logs[start:end] {
		lines = append(lines, "  "+line)
	}
	return lines
}

func listStart(cursor, available, size int) int {
	if available <= 0 {
		return 0
	}
	return min(max(0, cursor-available+1), max(0, size-available))
}

func byteCount(value int64) string {
	if value < 1024 {
		return fmt.Sprintf("%dB", value)
	}
	if value < 1024*1024 {
		return fmt.Sprintf("%.1fK", float64(value)/1024)
	}
	if value < 1024*1024*1024 {
		return fmt.Sprintf("%.1fM", float64(value)/(1024*1024))
	}
	return fmt.Sprintf("%.1fG", float64(value)/(1024*1024*1024))
}
