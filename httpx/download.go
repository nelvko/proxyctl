package httpx

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	URL "net/url"
	"os"
	"path"

	"strings"
	"time"

	"charm.land/bubbles/v2/progress"
	"charm.land/bubbles/v2/spinner"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)
// GhProxy returns the URL of the GitHub proxy if the GITHUB_PROXY environment variable is set, 
// otherwise it returns the original URL.
func GhProxy(raw string) string {
	ghProxy := os.Getenv("GITHUB_PROXY")
	if ghProxy == "" {
		return raw
	}
	ghProxyURL, err := URL.Parse(ghProxy)
	if err != nil {
		return raw
	}
	rawURL, err := URL.Parse(raw)
	if err != nil {
		return raw
	}
	ghProxyURL.Path = path.Join(ghProxyURL.Path, rawURL.String())
	return ghProxyURL.String()
}

var p *tea.Program

type progressWriter struct {
	total      int
	downloaded int
	writer     io.Writer
	reader     io.Reader
	onProgress func(float64)
}

func (pw *progressWriter) Start() {
	// TeeReader calls pw.Write() each time a new response is received
	_, err := io.Copy(pw.writer, io.TeeReader(pw.reader, pw))
	if err != nil {
		p.Send(progressErrMsg{err})
	}
}

func (pw *progressWriter) Write(p []byte) (int, error) {
	pw.downloaded += len(p)
	if pw.total > 0 && pw.onProgress != nil {
		pw.onProgress(float64(pw.downloaded) / float64(pw.total))
	}
	return len(p), nil
}

func Download(ctx context.Context, url string, dst io.Writer) error {
	m := model{
		ctx:      ctx,
		url:      url,
		dst:      dst,
		spinner:  spinner.New(spinner.WithSpinner(spinner.Dot), spinner.WithStyle(lipgloss.NewStyle().Foreground(lipgloss.Color("205")))),
		progress: progress.New(progress.WithDefaultBlend()),
	}

	p = tea.NewProgram(m)

	// Start the download
	// go pw.Start()
	returnModel, err := p.Run()
	m = returnModel.(model)
	if m.err != nil {
		return m.err
	}
	if err != nil {
		return fmt.Errorf("error running program: %w", err)
	}
	return nil
}

var helpStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#626262")).Render

const (
	padding  = 2
	maxWidth = 80
)

type progressMsg float64

type progressErrMsg struct{ err error }

func finalPause() tea.Cmd {
	return tea.Tick(time.Millisecond*750, func(_ time.Time) tea.Msg {
		return nil
	})
}

type model struct {
	spinner  spinner.Model
	progress progress.Model
	pw       *progressWriter
	ctx      context.Context
	url      string
	dst      io.Writer
	err      error
}

func (m model) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, func() tea.Msg {
		req, err := http.NewRequestWithContext(m.ctx, http.MethodGet, m.url, nil)
		if err != nil {
			return progressErrMsg{err}
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return progressErrMsg{err}
		}

		if resp.ContentLength <= 0 {
			return progressErrMsg{errors.New("can't parse content length, aborting download")}
		}

		pw := &progressWriter{
			total:  int(resp.ContentLength),
			writer: m.dst,
			reader: resp.Body,
			onProgress: func(ratio float64) {
				p.Send(progressMsg(ratio))
			},
		}
		m.pw = pw
		// 使用 goroutine 进行下载
		go func() {
			_, err := io.Copy(pw.writer, io.TeeReader(pw.reader, pw))
			if err != nil {
				p.Send(progressErrMsg{err})
			}
			// 这里确保下载完成后再关闭响应体
			resp.Body.Close()
		}()
		return nil
	})
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q", "esc":
			m.err = tea.ErrInterrupted
			return m, tea.Quit
		}
		return m, nil

	case tea.WindowSizeMsg:
		m.progress.SetWidth(msg.Width - padding*2 - 4)
		if m.progress.Width() > maxWidth {
			m.progress.SetWidth(maxWidth)
		}
		return m, nil

	case progressErrMsg:
		m.err = msg.err
		return m, tea.Quit

	case progressMsg:
		var cmds []tea.Cmd

		if msg >= 1.0 {
			cmds = append(cmds, tea.Sequence(finalPause(), tea.Quit))
		}

		cmds = append(cmds, m.progress.SetPercent(float64(msg)))
		return m, tea.Batch(cmds...)

	// FrameMsg is sent when the progress bar wants to animate itself
	case progress.FrameMsg:
		var cmd tea.Cmd
		m.progress, cmd = m.progress.Update(msg)
		return m, cmd

	default:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	}
}

func (m model) View() tea.View {
	pad := strings.Repeat(" ", padding)
	content := lipgloss.JoinVertical(
		lipgloss.Top,
		m.spinner.View()+" Downloading...",
		pad+m.progress.View(),
		pad+helpStyle("Press q to quit.\n"),
	)
	return tea.NewView(content)
}
