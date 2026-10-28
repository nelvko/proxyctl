package ui

import (
	"context"
	"strings"

	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/nelvko/proxyctl/internal/config"
	"github.com/nelvko/proxyctl/internal/i18n"
	"github.com/nelvko/proxyctl/internal/redact"
)

type appearanceState struct {
	mode, scheme    Dropdown
	language        Dropdown
	languagePreview bool
	saveLanguage    func(context.Context, string) error
	reset           Button
	focus           int
	viewport        viewport.Model
	previewing      bool
	draft           config.ThemeConfig
	save            func(context.Context, config.ThemeConfig) error
}

type languageSavedMsg struct {
	language string
	err      error
}

type appearanceSavedMsg struct {
	config config.ThemeConfig
	err    error
}

func newAppearanceState(cfg config.ThemeConfig) appearanceState {
	mode := cfg.Mode
	if mode == "" || mode != "dark" && mode != "light" {
		mode = "auto"
	}
	options := []SelectOption{{Label: i18n.Text("", "appearance.terminal"), Value: ""}}
	for _, item := range builtInThemes() {
		options = append(options, SelectOption{Label: item.Label, Value: item.Name})
	}
	options = append(options, SelectOption{Label: "Custom", Value: "custom"})
	return appearanceState{
		mode:     NewDropdown([]SelectOption{{Label: "Auto", Value: "auto"}, {Label: "Dark", Value: "dark"}, {Label: "Light", Value: "light"}}, mode),
		scheme:   NewDropdown(options, cfg.Name),
		reset:    Button{Label: i18n.Text("", "appearance.reset")},
		language: NewDropdown([]SelectOption{{Label: i18n.Text("", "common.auto"), Value: ""}, {Label: "简体中文", Value: "zh-CN"}, {Label: "English", Value: "en"}}, ""),
		draft:    cfg,
		viewport: viewport.New(),
	}
}

func (a appearanceState) dropdownOpen() bool { return a.mode.Open || a.scheme.Open || a.language.Open }

func (m *dashboardModel) applyTheme() {
	cfg := m.themeConfig
	if m.appearance.previewing {
		cfg = m.appearance.draft
	}
	m.theme, _ = resolveTheme(cfg, m.darkBG)
	m.styles = newThemedStyles(m.theme)
}

func (m *dashboardModel) cancelAppearance() {
	m.appearance.mode.Open, m.appearance.scheme.Open, m.appearance.language.Open = false, false, false
	if m.appearance.languagePreview {
		m.locale, _ = i18n.Resolve(m.language)
	}
	m.appearance.languagePreview = false
	m.appearance.language.restoreCursor()
	m.appearance.previewing = false
	m.appearance.mode.restoreCursor()
	m.appearance.scheme.restoreCursor()
	m.applyTheme()
}

func (m dashboardModel) contentBounds() Rect {
	x, y := 0, 2
	if dashboardSidebarLayout(m.width, m.height) {
		x, y = 19, 1
	}
	return Rect{X: x, Y: y, Width: max(0, m.width-x), Height: max(0, m.height-y-1)}
}

// appearanceDocument builds the content and its hit regions in the same pass.
// Regions use terminal coordinates before the viewport scroll offset is applied.
func (m dashboardModel) appearanceDocument() ([]string, [4]Rect) {
	b := m.contentBounds()
	indent := min(14, max(0, b.Width/3))
	lines := []string{m.dashboardBodyLine(m.tr("appearance.title"), b.Width)}
	var controls [4]Rect
	appendSelect := func(index int, label string, d Dropdown) {
		r := Rect{X: b.X + indent, Y: b.Y + len(lines), Width: max(0, b.Width-indent-2), Height: d.Height(max(1, b.Height-1))}
		controls[index] = r
		rows := strings.Split(d.View(m.theme, r.Width, r.Height), "\n")
		for i, row := range rows {
			prefix := strings.Repeat(" ", indent)
			if i == 0 {
				prefix = dashboardLine("  "+label, indent, false)
			}
			lines = append(lines, prefix+row)
		}
		lines = append(lines, "")
	}
	appendSelect(0, m.tr("appearance.mode"), m.appearance.mode)
	appendSelect(1, m.tr("appearance.scheme"), m.appearance.scheme)
	controls[2] = Rect{X: b.X + 2, Y: b.Y + len(lines), Width: max(0, b.Width-4), Height: 1}
	lines = append(lines, "  "+m.appearance.reset.View(m.theme, controls[2].Width), "")
	appendSelect(3, m.tr("common.language"), m.appearance.language)
	lines = append(lines, m.dashboardBodyLine(m.tr("appearance.preview"), b.Width),
		"  "+lipgloss.NewStyle().Foreground(themeColor(m.theme.Success)).Render(m.tr("appearance.success"))+"   "+
			lipgloss.NewStyle().Foreground(themeColor(m.theme.Warning)).Render(m.tr("appearance.warning"))+"   "+
			lipgloss.NewStyle().Foreground(themeColor(m.theme.Error)).Render(m.tr("appearance.error")), "")
	for _, line := range m.settingsContent(max(0, b.Height-len(lines)), true) {
		lines = append(lines, m.dashboardBodyLine(line, b.Width))
	}
	return lines, controls
}

func (m *dashboardModel) layoutAppearance() {
	b := m.contentBounds()
	m.appearance.mode.Locale, m.appearance.scheme.Locale, m.appearance.language.Locale = m.locale, m.locale, m.locale
	m.appearance.mode.Focused = !m.navFocused && m.appearance.focus == 0
	m.appearance.scheme.Focused = !m.navFocused && m.appearance.focus == 1
	m.appearance.reset.Focused = !m.navFocused && m.appearance.focus == 2
	m.appearance.reset.Disabled = m.busy
	m.appearance.language.Focused = !m.navFocused && m.appearance.focus == 3
	m.appearance.reset.Label = m.tr("appearance.reset")
	m.appearance.scheme.Options = append([]SelectOption(nil), m.appearance.scheme.Options...)
	m.appearance.scheme.Options[0].Label = m.tr("appearance.terminal")
	m.appearance.language.Options = append([]SelectOption(nil), m.appearance.language.Options...)
	m.appearance.language.Options[0].Label = m.tr("common.auto")
	if !m.appearance.language.Open && !m.appearance.languagePreview {
		value := ""
		if m.language != "" && m.language != "auto" {
			locale, _ := i18n.Resolve(m.language)
			value = string(locale)
		}
		m.appearance.language.Value = value
		m.appearance.language.restoreCursor()
	}
	lines, controls := m.appearanceDocument()
	v := &m.appearance.viewport
	v.SetWidth(b.Width)
	v.SetHeight(b.Height)
	v.SetContentLines(lines)
	// An open select owns focus. Scrolling the document by hand remains possible
	// while selects are closed, so runtime details stay reachable.
	if m.appearance.dropdownOpen() {
		keepAppearanceControlVisible(v, controls[m.appearance.focus], b)
	}
	for i := range controls {
		controls[i].Y -= v.YOffset()
	}
	m.appearance.mode.SetBounds(controls[0])
	m.appearance.scheme.SetBounds(controls[1])
	m.appearance.reset.Bounds = controls[2]
	m.appearance.language.SetBounds(controls[3])
	languageHover := m.appearance.language.Hovered
	modeHover, schemeHover, resetHover := m.appearance.mode.Hovered, m.appearance.scheme.Hovered, m.appearance.reset.Hovered
	if !m.appearance.dropdownOpen() {
		motion := tea.MouseMotionMsg{X: m.hoverX, Y: m.hoverY}
		m.appearance.mode.Update(motion)
		m.appearance.scheme.Update(motion)
		m.appearance.reset.Update(motion)
		m.appearance.language.Update(motion)
	} else {
		m.appearance.reset.Hovered = false
		if !m.appearance.language.Open {
			m.appearance.language.Hovered = false
		}
		if m.appearance.mode.Open {
			m.appearance.scheme.Hovered = false
		} else {
			m.appearance.mode.Hovered = false
		}
	}
	if modeHover != m.appearance.mode.Hovered || schemeHover != m.appearance.scheme.Hovered || resetHover != m.appearance.reset.Hovered || languageHover != m.appearance.language.Hovered {
		// Scrolling or resizing can move a control under a stationary pointer.
		// Hover changes styles only, so the document's geometry stays the same.
		lines, _ = m.appearanceDocument()
		v.SetContentLines(lines)
	}
}

func keepAppearanceControlVisible(v *viewport.Model, control, content Rect) {
	top := control.Y - content.Y
	if top < v.YOffset() {
		v.SetYOffset(top)
	}
	if bottom := top + control.Height; bottom > v.YOffset()+content.Height {
		v.SetYOffset(bottom - content.Height)
	}
}

func (m *dashboardModel) focusAppearance() {
	m.appearance.viewport.GotoTop()
	m.layoutAppearance()
	control := m.appearance.mode.bounds
	if m.appearance.focus == 1 {
		control = m.appearance.scheme.bounds
	} else if m.appearance.focus == 2 {
		control = m.appearance.reset.Bounds
	} else if m.appearance.focus == 3 {
		control = m.appearance.language.bounds
	}
	control.Y += m.appearance.viewport.YOffset()
	keepAppearanceControlVisible(&m.appearance.viewport, control, m.contentBounds())
	m.layoutAppearance()
}

func (m dashboardModel) settingsViewLines(width, height int) []string {
	if width <= 0 || height <= 0 {
		return nil
	}
	m.layoutAppearance()
	return strings.Split(m.appearance.viewport.View(), "\n")
}

func (m *dashboardModel) previewAppearance(index int, value string) {
	m.appearance.draft = m.themeConfig
	if index == 0 {
		m.appearance.draft.Mode = value
	} else {
		m.appearance.draft.Name = value
	}
	m.appearance.previewing = true
	m.applyTheme()
}

func (m *dashboardModel) persistAppearance(cfg config.ThemeConfig) tea.Cmd {
	m.busy = true
	ctx, cancel := context.WithCancel(m.ctx)
	m.actionCancel = cancel
	save := m.appearance.save
	if save == nil {
		save = saveThemeConfig
	}
	return func() tea.Msg { return appearanceSavedMsg{config: cfg, err: save(ctx, cfg)} }
}

func (m *dashboardModel) appearanceSaved(msg appearanceSavedMsg) tea.Cmd {
	m.busy = false
	m.noticeError = msg.err != nil
	if m.actionCancel != nil {
		m.actionCancel()
		m.actionCancel = nil
	}
	if msg.err != nil {
		m.notice = m.tr("appearance.save_failed", redact.Text(msg.err.Error()))
	} else {
		m.themeConfig = msg.config
		m.notice = m.tr("appearance.saved")
	}
	focus, save, saveLanguage := m.appearance.focus, m.appearance.save, m.appearance.saveLanguage
	m.appearance = newAppearanceState(m.themeConfig)
	m.appearance.focus, m.appearance.save, m.appearance.saveLanguage = focus, save, saveLanguage
	m.applyTheme()
	m.focusAppearance()
	if m.quitAfter {
		m.stopTraffic()
		if m.logCancel != nil {
			m.logCancel()
		}
		return tea.Quit
	}
	return nil
}

func (m *dashboardModel) updateAppearance(msg tea.Msg) (bool, tea.Cmd) {
	m.layoutAppearance()
	index := m.appearance.focus
	open := m.appearance.dropdownOpen()
	switch input := msg.(type) {
	case tea.KeyPressMsg:
		m.hoverX, m.hoverY = -1, -1
		m.appearance.mode.Hovered, m.appearance.scheme.Hovered, m.appearance.reset.Hovered, m.appearance.language.Hovered = false, false, false, false
		key := input.String()
		if key == "ctrl+c" || key == "q" {
			return false, nil
		}
		if m.navFocused {
			return false, nil
		}
		if open {
			// Keep page shortcuts from escaping the select's focus scope.
			if key == "tab" || key == "shift+tab" {
				m.cancelAppearance()
				return false, nil
			}
		} else {
			switch key {
			case "up", "k":
				m.notice = ""
				m.appearance.focus = max(0, index-1)
				m.focusAppearance()
				return true, nil
			case "down", "j":
				m.notice = ""
				m.appearance.focus = min(3, index+1)
				m.focusAppearance()
				return true, nil
			case "pgup", "pgdown":
				m.scrollAppearance(key == "pgdown")
				return true, nil
			case "enter", "space":
				m.focusAppearance()
				if index == 2 {
					if m.appearance.reset.Update(msg) {
						m.appearance.draft = defaultThemeConfig()
						m.appearance.previewing = true
						m.applyTheme()
						return true, m.persistAppearance(defaultThemeConfig())
					}
					return true, nil
				}
			default:
				return false, nil
			}
		}
	case tea.MouseMsg:
		mouse := input.Mouse()
		m.hoverX, m.hoverY = mouse.X, mouse.Y
		if _, motion := msg.(tea.MouseMotionMsg); motion && !open {
			m.appearance.mode.Update(msg)
			m.appearance.scheme.Update(msg)
			m.appearance.reset.Update(msg)
			m.appearance.language.Update(msg)
			return false, nil
		}
		if !open {
			if _, wheel := msg.(tea.MouseWheelMsg); wheel && m.contentBounds().Contains(mouse.X, mouse.Y) {
				if mouse.Button == tea.MouseWheelUp || mouse.Button == tea.MouseWheelDown {
					m.scrollAppearance(mouse.Button == tea.MouseWheelDown)
					return true, nil
				}
			}
		}
		if !open {
			if _, click := msg.(tea.MouseClickMsg); !click || mouse.Button != tea.MouseLeft {
				return false, nil
			}
			b := m.contentBounds()
			if !b.Contains(mouse.X, mouse.Y) {
				return false, nil
			}
			if m.appearance.reset.Update(msg) {
				m.navFocused, m.appearance.focus = false, 2
				m.appearance.draft, m.appearance.previewing = defaultThemeConfig(), true
				m.applyTheme()
				return true, m.persistAppearance(defaultThemeConfig())
			}
			if m.appearance.language.bounds.Contains(mouse.X, mouse.Y) {
				index = 3
			} else if m.appearance.mode.bounds.Contains(mouse.X, mouse.Y) {
				index = 0
			} else if m.appearance.scheme.bounds.Contains(mouse.X, mouse.Y) {
				index = 1
			} else {
				return false, nil
			}
			m.navFocused, m.appearance.focus = false, index
			m.layoutAppearance()
		}
	default:
		return false, nil
	}
	m.notice = ""
	d := &m.appearance.mode
	if index == 1 {
		d = &m.appearance.scheme
	} else if index == 3 {
		d = &m.appearance.language
	}
	result := d.Update(msg)
	if d.Open {
		value := d.Value
		if d.Cursor >= 0 && d.Cursor < len(d.Options) {
			value = d.Options[d.Cursor].Value
		}
		if index == 3 {
			m.previewLanguage(value)
		} else {
			m.previewAppearance(index, value)
		}
	}
	if result.Cancelled {
		m.cancelAppearance()
	}
	if result.Confirmed {
		if index == 3 {
			m.previewLanguage(result.Value)
			return true, m.persistLanguage(result.Value)
		}
		m.previewAppearance(index, result.Value)
		return true, m.persistAppearance(m.appearance.draft)
	}
	return true, nil
}

func (m *dashboardModel) scrollAppearance(down bool) {
	m.notice = ""
	m.layoutAppearance()
	v := &m.appearance.viewport
	// Reuse Viewport's bounds against the actual document, including status
	// errors and CLI hints, rather than estimating its total line count.
	step := max(1, v.Height()/2)
	if down {
		v.ScrollDown(step)
	} else {
		v.ScrollUp(step)
	}
	m.layoutAppearance()
}

func (m *dashboardModel) previewLanguage(language string) {
	m.appearance.languagePreview = true
	m.locale, _ = i18n.Resolve(language)
}

func (m *dashboardModel) persistLanguage(language string) tea.Cmd {
	m.busy = true
	ctx, cancel := context.WithCancel(m.ctx)
	m.actionCancel = cancel
	save := m.appearance.saveLanguage
	if save == nil {
		save = config.SaveLanguage
	}
	return func() tea.Msg { return languageSavedMsg{language: language, err: save(ctx, language)} }
}

func (m *dashboardModel) languageSaved(msg languageSavedMsg) tea.Cmd {
	m.busy = false
	m.noticeError = msg.err != nil
	if m.actionCancel != nil {
		m.actionCancel()
		m.actionCancel = nil
	}
	if msg.err == nil {
		m.language = msg.language
	}
	m.locale, _ = i18n.Resolve(m.language)
	m.appearance.language.Open = false
	m.appearance.languagePreview = false
	if msg.err != nil {
		m.notice = m.tr("language.save_failed", redact.Text(msg.err.Error()))
	} else {
		m.notice = m.tr("language.saved")
	}
	m.focusAppearance()
	if m.quitAfter {
		m.stopTraffic()
		if m.logCancel != nil {
			m.logCancel()
		}
		return tea.Quit
	}
	return nil
}
