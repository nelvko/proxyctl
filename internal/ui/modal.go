package ui

import (
	"strings"

	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/colorprofile"
	"github.com/nelvko/proxyctl/internal/i18n"
)

type ModalKind uint8

const (
	ModalInfo ModalKind = iota
	ModalConfirm
	ModalWarning
	ModalError
)

type ModalOptions struct {
	Title, Body, ConfirmLabel string
	Kind                      ModalKind
	Locale                    i18n.Locale
}

type ModalResult struct{ Confirmed, Cancelled bool }

// Modal owns input while active; its parent retains and restores page focus.
// SetBounds supplies the full screen frame before routing input. Rendering is
// pure and uses Lip Gloss's cell compositor rather than splicing ANSI strings.
type Modal struct {
	Active                    bool
	options                   ModalOptions
	viewport                  viewport.Model
	frame, bounds, bodyBounds Rect
	confirm, cancel           Button
}

func NewModal(options ModalOptions) Modal {
	label := i18n.Text(options.Locale, "common.close")
	if options.Kind == ModalConfirm || options.ConfirmLabel != "" {
		label = i18n.Text(options.Locale, "common.cancel")
		if options.ConfirmLabel == "" {
			options.ConfirmLabel = i18n.Text(options.Locale, "common.confirm")
		}
	}
	v := viewport.New()
	v.SoftWrap = true
	v.SetHorizontalStep(0)
	return Modal{options: options, viewport: v, cancel: Button{Label: label, Focused: true}, confirm: Button{Label: options.ConfirmLabel}}
}

func (m *Modal) Open() {
	m.Active = true
	m.viewport.GotoTop()
	m.cancel.Focused, m.confirm.Focused = true, false
	m.cancel.Hovered, m.confirm.Hovered = false, false
}

func (m *Modal) Close() {
	m.Active = false
	m.cancel.Hovered, m.confirm.Hovered = false, false
}

func (m Modal) Bounds() Rect { return m.bounds }

func (m *Modal) SetBounds(frame Rect) {
	if frame != m.frame {
		m.cancel.Hovered, m.confirm.Hovered = false, false
	}
	m.frame = frame
	width, availableHeight := max(0, frame.Width), max(0, frame.Height)
	if width >= 12 {
		width -= 4
	}
	if availableHeight >= 10 {
		availableHeight -= 4
	}
	width = min(72, width)
	m.viewport.SetWidth(max(1, width-4))
	// Each body line is plain content. Preserve its line breaks while keeping
	// control characters out of the terminal command stream.
	lines := strings.Split(strings.ReplaceAll(m.options.Body, "\r\n", "\n"), "\n")
	for i := range lines {
		lines[i] = singleLine(lines[i])
	}
	m.viewport.SetContentLines(lines)
	height := min(availableHeight, min(18, max(7, m.viewport.TotalLineCount()+6)))
	m.bounds = Rect{X: frame.X + (max(0, frame.Width)-width)/2, Y: frame.Y + (max(0, frame.Height)-height)/2, Width: width, Height: height}
	m.bodyBounds = Rect{X: m.bounds.X + 2, Y: m.bounds.Y + 3, Width: max(0, width-4), Height: max(0, height-6)}
	m.viewport.SetHeight(m.bodyBounds.Height)
	m.viewport.SetYOffset(m.viewport.YOffset())
	m.cancel.Bounds, m.confirm.Bounds = Rect{}, Rect{}
	m.cancel.Disabled, m.confirm.Disabled = true, true
	innerWidth := m.bodyBounds.Width
	cancelWidth := lipgloss.Width(singleLine(m.cancel.Label)) + 2
	confirmWidth := lipgloss.Width(singleLine(m.confirm.Label)) + 2
	if height >= 4 && innerWidth >= cancelWidth {
		x := m.bounds.X + 2 + (innerWidth-cancelWidth)/2
		m.cancel.Bounds = Rect{X: x, Y: m.bounds.Y + height - 2, Width: cancelWidth, Height: 1}
		m.cancel.Disabled = false
		if m.options.ConfirmLabel != "" && m.bodyBounds.Height > 0 && innerWidth >= cancelWidth+confirmWidth+2 {
			x = m.bounds.X + 2 + (innerWidth-cancelWidth-confirmWidth-2)/2
			m.cancel.Bounds.X = x
			m.confirm.Bounds = Rect{X: x + cancelWidth + 2, Y: m.cancel.Bounds.Y, Width: confirmWidth, Height: 1}
			m.confirm.Disabled = false
		}
	}
	if m.confirm.Disabled {
		m.cancel.Focused, m.confirm.Focused = true, false
	}
}

func (m *Modal) finish(confirmed bool) ModalResult {
	m.Close()
	return ModalResult{Confirmed: confirmed, Cancelled: !confirmed}
}

func (m *Modal) Update(msg tea.Msg) (ModalResult, tea.Cmd) {
	if !m.Active {
		return ModalResult{}, nil
	}
	switch input := msg.(type) {
	case tea.KeyPressMsg:
		switch input.String() {
		case "esc", "q":
			return m.finish(false), nil
		case "left":
			m.cancel.Focused, m.confirm.Focused = true, false
			return ModalResult{}, nil
		case "right":
			if !m.confirm.Disabled {
				m.cancel.Focused, m.confirm.Focused = false, true
			}
			return ModalResult{}, nil
		case "tab", "shift+tab":
			if !m.confirm.Disabled {
				m.cancel.Focused, m.confirm.Focused = m.confirm.Focused, m.cancel.Focused
			}
			return ModalResult{}, nil
		case "enter", "space":
			if m.confirm.Update(msg) {
				return m.finish(true), nil
			}
			if m.cancel.Update(msg) {
				return m.finish(false), nil
			}
			return ModalResult{}, nil
		case "home":
			m.viewport.GotoTop()
			return ModalResult{}, nil
		case "end":
			m.viewport.GotoBottom()
			return ModalResult{}, nil
		}
	case tea.MouseMotionMsg:
		m.cancel.Update(msg)
		m.confirm.Update(msg)
		return ModalResult{}, nil
	case tea.MouseClickMsg:
		if input.Button != tea.MouseLeft {
			return ModalResult{}, nil
		}
		if !m.bounds.Contains(input.X, input.Y) {
			return m.finish(false), nil
		}
		if m.confirm.Update(msg) {
			return m.finish(true), nil
		}
		if m.cancel.Update(msg) {
			return m.finish(false), nil
		}
		return ModalResult{}, nil
	case tea.MouseWheelMsg:
		if !m.bodyBounds.Contains(input.X, input.Y) {
			return ModalResult{}, nil
		}
	}
	var cmd tea.Cmd
	m.viewport, cmd = m.viewport.Update(msg)
	return ModalResult{}, cmd
}

func (m Modal) View(theme Theme, width, height int) string {
	if !m.Active || width <= 0 || height <= 0 {
		return ""
	}
	m.SetBounds(Rect{Width: width, Height: height})
	w, h := m.bounds.Width, m.bounds.Height
	innerWidth, innerHeight := max(0, w-2), max(0, h-2)
	lines := make([]string, innerHeight)
	accent := theme.Primary
	if m.options.Kind == ModalWarning {
		accent = theme.Warning
	} else if m.options.Kind == ModalError {
		accent = theme.Error
	}
	if innerHeight > 0 {
		lines[0] = " " + lipgloss.NewStyle().Foreground(themeColor(accent)).Bold(true).Render(singleLine(m.options.Title))
	}
	if m.bodyBounds.Width > 0 && m.bodyBounds.Height > 0 {
		m.viewport.Style = lipgloss.NewStyle().Foreground(themeColor(theme.Foreground)).Background(themeColor(theme.Surface))
		for i, line := range strings.Split(m.viewport.View(), "\n") {
			lines[i+2] = " " + line
		}
		lines[h-4] = " " + lipgloss.NewStyle().Foreground(themeColor(theme.Muted)).Render(i18n.Text(m.options.Locale, "modal.scroll_hint"))
	}
	if !m.cancel.Disabled {
		buttons := m.cancel.View(theme, m.cancel.Bounds.Width)
		if !m.confirm.Disabled {
			buttons += "  " + m.confirm.View(theme, m.confirm.Bounds.Width)
		}
		lines[h-3] = strings.Repeat(" ", m.cancel.Bounds.X-m.bounds.X-1) + buttons
	}
	for i := range lines {
		lines[i] = gridLine(lines[i], innerWidth)
	}
	panel := lipgloss.NewStyle().Foreground(themeColor(theme.Foreground)).Background(themeColor(theme.Surface)).
		Border(lipgloss.RoundedBorder()).BorderForeground(themeColor(accent)).BorderBackground(themeColor(theme.Surface)).
		Render(strings.Join(lines, "\n"))
	surface := theme
	surface.Background = theme.Surface
	return terminalFrame(panel, w, h, surface, colorprofile.TrueColor)
}

func (m Modal) Overlay(base string, theme Theme, width, height int) string {
	if width <= 0 || height <= 0 {
		return ""
	}
	base = modalGrid(base, width, height)
	if !m.Active {
		return base
	}
	m.SetBounds(Rect{Width: width, Height: height})
	layers := lipgloss.NewCompositor(lipgloss.NewLayer(base), lipgloss.NewLayer(m.View(theme, width, height)).X(m.bounds.X).Y(m.bounds.Y).Z(1))
	canvas := lipgloss.NewCanvas(width, height)
	return modalGrid(canvas.Compose(layers).Render(), width, height)
}

// Canvas trims trailing blank cells. Restore the declared terminal extent
// after native cell composition without altering its ANSI/grapheme handling.
func modalGrid(content string, width, height int) string {
	if width <= 0 || height <= 0 {
		return ""
	}
	input := strings.Split(content, "\n")
	lines := make([]string, height)
	for i := range lines {
		if i < len(input) {
			lines[i] = input[i]
		}
		lines[i] = gridLine(lines[i], width)
	}
	return strings.Join(lines, "\n")
}
