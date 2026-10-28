package ui

import (
	"context"
	"errors"
	"fmt"
	"image/color"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
	"unicode"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/nelvko/proxyctl/internal/app"
	"github.com/nelvko/proxyctl/internal/config"
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
}

func readDashboardSnapshot(ctx context.Context, page int) dashboardSnapshot {
	var result dashboardSnapshot
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
		result.err = err
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
	if endpoints, err := k.ProxyEndpoints(); err == nil {
		result.proxy = redact.URL(endpoints.Preferred())
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
	logs                                   []string
	logPaused                              bool
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
}

func newDashboardModel(ctx context.Context) dashboardModel {
	return dashboardModel{
		ctx:        ctx,
		loading:    true,
		navFocused: true,
		darkBG:     true,
		styles:     newStyles(true),
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
		return m, nil
	case tea.BackgroundColorMsg:
		m.darkBG = msg.IsDark()
		m.styles = newStyles(m.darkBG)
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
			return m, tea.Batch(m.startLogs(), m.startTraffic())
		}
		return m, m.startTraffic()
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
		m.loading = true
		return m, m.loadSnapshot()
	case dashboardLogMsg:
		if msg.generation != m.logGeneration || m.page != dashboardLogsPage {
			return m, nil
		}
		if msg.done {
			if msg.err != nil && !errors.Is(msg.err, context.Canceled) {
				m.notice = "日志流已断开: " + redact.Text(msg.err.Error())
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
	case tea.MouseClickMsg:
		if page := dashboardClickPage(m.width, m.height, msg.X, msg.Y); page >= 0 {
			return m, m.switchPage(page)
		}
		wide := dashboardSidebarLayout(m.width, m.height)
		contentClick := !wide && msg.Y >= 2 || wide && msg.X >= 19
		if contentClick && m.page != dashboardOverviewPage && m.page != dashboardSettingsPage && msg.Y > 0 && msg.Y < m.height-1 {
			m.navFocused = false
		}
		return m, nil
	case tea.KeyPressMsg:
		key := msg.String()
		if !m.busy {
			m.notice = ""
		}
		if key == "ctrl+c" || key == "q" {
			if m.busy {
				m.quitAfter = true
				m.notice = "正在取消操作…"
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
				if m.page != dashboardOverviewPage && m.page != dashboardSettingsPage {
					m.navFocused = false
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
	return m, nil
}

func (m *dashboardModel) pageKey(key string) tea.Cmd {
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
		if key == "enter" && len(group.All) > 0 && m.snapshot.controller != nil {
			m.busy = true
			name := group.All[m.node]
			controller := m.snapshot.controller
			ctx, cancel := context.WithCancel(m.ctx)
			m.actionCancel = cancel
			return func() tea.Msg {
				err := controller.selectProxy(ctx, group.Name, name)
				return dashboardActionMsg{message: "已选择 " + name, err: err}
			}
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
						result = "已更新 " + name
						return err
					}
					_, err := svc.Use(name)
					result = "已应用 " + name
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
		}
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
	boundaryColor := m.dashboardColor("#B7B3E4", "#4C4A79")
	header := dashboardPages[m.page]
	if !wide {
		header = "proxyctl · " + header
	}
	lines := []string{m.dashboardBoundary(header, w, lipgloss.Center, lipgloss.NewStyle().Foreground(m.styles.Indigo).Bold(true), boundaryColor)}
	if !wide {
		lines = append(lines, m.dashboardTabs(w))
	}
	bodyHeight := max(0, h-len(lines)-1)
	body := m.pageLines(bodyHeight)
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
				nav = fmt.Sprintf("  %d  %s", index+1, dashboardPages[index])
				selected = index == m.page
			}
			navStyle := lipgloss.NewStyle().Foreground(m.dashboardColor("#68758A", "#8B95A7"))
			if i == 1 {
				navStyle = navStyle.Foreground(m.styles.Indigo).Bold(true)
			}
			if selected {
				if m.navFocused {
					navStyle = navStyle.Foreground(lipgloss.Color("#FFFFFF")).Background(m.styles.Indigo).Bold(true)
				} else {
					navStyle = navStyle.Foreground(m.styles.Indigo).Bold(true)
				}
			}
			if len(sidebarRates) > 0 && i >= sidebarRatesStart && i < sidebarRatesStart+len(sidebarRates) {
				rateRow := i - sidebarRatesStart
				nav = sidebarRates[rateRow]
				switch rateRow {
				case 0:
					navStyle = navStyle.Foreground(m.dashboardColor("#C4C7D2", "#373A55"))
				case 1:
					if m.trafficAvailable {
						navStyle = navStyle.Foreground(m.dashboardColor("#C95E42", "#FF9470"))
					}
				case 2:
					if m.trafficAvailable {
						navStyle = navStyle.Foreground(m.dashboardColor("#2869C9", "#6EA8FF"))
					}
				}
			}
			left := navStyle.Render(dashboardLine(nav, 18, false))
			divider := lipgloss.NewStyle().Foreground(m.styles.Indigo).Render("│")
			lines = append(lines, left+divider+m.dashboardBodyLine(body[i], contentWidth))
		} else {
			lines = append(lines, m.dashboardBodyLine(body[i], w))
		}
	}
	footer := "内容: ↑↓ 选择  Esc 返回导航  Tab 切页  q 退出"
	if m.navFocused {
		footer = "导航: ↑↓ 切页  Enter 进入内容  1-7 直达  q 退出"
		if !wide {
			footer = "导航: ←→/↑↓ 切页  Enter 进入内容  q 退出"
		}
		if m.page == dashboardOverviewPage || m.page == dashboardSettingsPage {
			footer = "导航: ↑↓ 切页  1-7 直达  r 刷新  q 退出"
			if !wide {
				footer = "导航: ←→/↑↓ 切页  r 刷新  q 退出"
			}
		}
	} else {
		switch m.page {
		case dashboardNodesPage:
			footer = "节点: ←→ 切换焦点  ↑↓ 选择  Enter 操作  Esc 返回导航"
		case dashboardSubscriptionsPage:
			footer = "订阅: ↑↓ 选择  Enter 应用  u 更新  Esc 返回导航"
		case dashboardConnectionsPage:
			footer = "连接: ↑↓ 查看详情  Esc 返回导航"
		case dashboardRulesPage:
			footer = "规则: ↑↓ 浏览  r 刷新  Esc 返回导航"
		case dashboardLogsPage:
			footer = "日志: p 暂停  c 清屏  Esc 返回导航"
		}
	}
	if m.busy {
		footer = "处理中…  请稍候"
	}
	if m.notice != "" {
		footer = m.notice
	}
	footerStyle := lipgloss.NewStyle().Foreground(m.dashboardColor("#909090", "#626262"))
	if m.busy {
		footerStyle = footerStyle.Foreground(m.styles.Indigo).Bold(true)
	} else if m.notice != "" {
		footerStyle = footerStyle.Foreground(m.styles.Green).Bold(true)
		if strings.Contains(m.notice, "失败") || strings.Contains(m.notice, "error") || strings.Contains(m.notice, "HTTP ") {
			footerStyle = footerStyle.Foreground(m.styles.Red)
		}
	}
	if m.notice != "" && (strings.Contains(m.notice, "失败") || strings.Contains(m.notice, "error") || strings.Contains(m.notice, "HTTP ")) {
		boundaryColor = m.styles.Red
	}
	lines = append(lines, m.dashboardBoundary(footer, w, lipgloss.Left, footerStyle, boundaryColor))
	v := tea.NewView(strings.Join(lines, "\n"))
	v.AltScreen = true
	v.MouseMode = tea.MouseModeCellMotion
	return v
}

func dashboardSidebarLayout(width, height int) bool {
	return width >= dashboardWideWidth && height >= 2*len(dashboardPages)+5
}

func dashboardClickPage(width, height, x, y int) int {
	if dashboardSidebarLayout(width, height) {
		if x >= 18 || y < 4 {
			return -1
		}
		index := (y - 4) / 2
		if (y-4)%2 == 0 && index < len(dashboardPages) {
			return index
		}
		return -1
	}
	if y != 1 {
		return -1
	}
	start := 0
	mode := dashboardTabLayout(width)
	for i, label := range dashboardPages {
		end := start + lipgloss.Width(dashboardTab(i, label, false, mode))
		if x >= start && x < end {
			return i
		}
		start = end
	}
	return -1
}

func (m dashboardModel) dashboardTabs(width int) string {
	var parts []string
	used := 0
	mode := dashboardTabLayout(width)
	for i, label := range dashboardPages {
		part := dashboardTab(i, label, i == m.page, mode)
		partWidth := lipgloss.Width(part)
		if used+partWidth > width {
			break
		}
		style := lipgloss.NewStyle().Foreground(m.dashboardColor("#68758A", "#8B95A7"))
		if i == m.page {
			if m.navFocused {
				style = style.Foreground(lipgloss.Color("#FFFFFF")).Background(m.styles.Indigo).Bold(true)
			} else {
				style = style.Foreground(m.styles.Indigo).Bold(true)
			}
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
	fullWidth := 0
	for i, label := range dashboardPages {
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

func (m dashboardModel) dashboardColor(light, dark string) color.Color {
	return lipgloss.LightDark(m.darkBG)(lipgloss.Color(light), lipgloss.Color(dark))
}

func (m dashboardModel) dashboardBoundary(text string, width int, position lipgloss.Position, style lipgloss.Style, accent color.Color) string {
	if width < 8 {
		return style.Render(dashboardLine(text, width, false))
	}
	labelWidth := min(lipgloss.Width(text), width-8)
	label := style.Padding(0, 2).Render(dashboardLine(text, labelWidth, false))
	return lipgloss.PlaceHorizontal(
		width,
		position,
		label,
		lipgloss.WithWhitespaceChars("/"),
		lipgloss.WithWhitespaceStyle(lipgloss.NewStyle().Foreground(accent)),
	)
}

func (m dashboardModel) dashboardBodyLine(value string, width int) string {
	style := lipgloss.NewStyle().Foreground(m.dashboardColor("#202B40", "#D8DEE9"))
	if strings.Contains(value, "▶") {
		if m.navFocused {
			style = style.Foreground(m.dashboardColor("#68758A", "#8B95A7"))
		} else {
			style = style.Foreground(m.styles.Indigo).Bold(true)
		}
	} else if strings.Contains(value, "✓ 当前") {
		style = style.Foreground(m.styles.Green)
	} else if strings.HasPrefix(value, "  ◇") || strings.HasPrefix(value, "  代理组") || strings.HasPrefix(value, "  已保存订阅") || strings.HasPrefix(value, "  活跃连接") || strings.HasPrefix(value, "  规则") || strings.HasPrefix(value, "  实时日志") {
		style = style.Foreground(m.styles.Indigo).Bold(true)
	} else if strings.Contains(value, "失败") || strings.Contains(value, "错误") {
		style = style.Foreground(m.styles.Red)
	} else if strings.Contains(value, "运行中") || strings.Contains(value, "已启用") {
		style = style.Foreground(m.styles.Green)
	} else if strings.Contains(value, "未启用") || strings.Contains(value, "未选择") {
		style = style.Foreground(m.dashboardColor("#68758A", "#8B95A7"))
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
	value = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return ' '
		}
		return r
	}, value)
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
	case dashboardOverviewPage:
		return m.overviewLines()
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

func (m dashboardModel) overviewLines() []string {
	s := m.snapshot
	lines := []string{"", "  ◇ 运行状态", "", "  内核        " + s.kernel, "  服务        " + s.service,
		"  代理入口    " + s.proxy, "  本 shell    " + s.shell,
		"", "  ◇ 订阅与流量", "", "  当前订阅    " + s.profile, fmt.Sprintf("  订阅数量    %d", len(s.profiles))}
	if s.controller != nil && s.service == "运行中" {
		lines = append(lines, "", fmt.Sprintf("  活跃连接    %d", len(s.connections.Connections)))
		if len(s.groups) > 0 {
			lines = append(lines, "  代理组      "+redact.Text(s.groups[0].Name+" → "+s.groups[0].Now))
		}
	}
	if s.controllerHint != "" {
		lines = append(lines, "", "  "+s.controllerHint)
	}
	if s.err != nil {
		lines = append(lines, "", "  状态读取失败: "+redact.Text(s.err.Error()))
	}
	return lines
}

func (m dashboardModel) nodeLines(height int) []string {
	if m.snapshot.controllerHint != "" && len(m.snapshot.groups) == 0 {
		return []string{"", "  " + m.snapshot.controllerHint}
	}
	if len(m.snapshot.groups) == 0 {
		return []string{"", "  没有可选择的代理组"}
	}
	focus := "代理组"
	if m.nodeFocus {
		focus = "节点"
	}
	lines := []string{"  代理组  ←/→ 切换焦点，Enter 选择  当前焦点: " + focus}
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
	lines = append(lines, "", "  "+redact.Text(group.Name)+" 的节点")
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
			selected = "  ✓ 当前"
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
	lines := []string{"  已保存订阅  Enter 应用 · u 更新  （添加/编辑：proxyctl sub）", ""}
	if len(m.snapshot.profiles) == 0 {
		return append(lines, "  尚无订阅；运行 proxyctl sub add <url>")
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
			selected = "  ✓ 当前"
		}
		lines = append(lines, "  "+mark+" "+redact.Text(p.Name)+selected+"  "+redact.URL(p.URL))
	}
	return lines
}

func (m dashboardModel) connectionLines(height int) []string {
	if m.snapshot.controllerHint != "" && len(m.snapshot.connections.Connections) == 0 {
		return []string{"", "  " + m.snapshot.controllerHint}
	}
	items := m.snapshot.connections.Connections
	lines := []string{fmt.Sprintf("  活跃连接 %d  ↑%s  ↓%s", len(items), byteCount(m.snapshot.connections.UploadTotal), byteCount(m.snapshot.connections.DownloadTotal)), ""}
	if len(items) == 0 {
		return append(lines, "  当前没有连接")
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
	lines = append(lines, "", "  进程: "+redact.Text(selected.Metadata.Process)+"  代理链: "+redact.Text(strings.Join(selected.Chains, " → ")))
	return lines
}

func (m dashboardModel) logLines(height int) []string {
	lines := []string{"  实时日志  p 暂停/继续 · c 清屏"}
	if m.snapshot.controllerHint != "" && m.logCancel == nil {
		return append(lines, "", "  "+m.snapshot.controllerHint)
	}
	if m.logPaused {
		lines[0] += "  [已暂停]"
	}
	available := max(0, height-len(lines))
	start := max(0, len(m.logs)-available)
	for _, line := range m.logs[start:] {
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
