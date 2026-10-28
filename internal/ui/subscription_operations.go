package ui

import (
	"context"
	"io"

	tea "charm.land/bubbletea/v2"
	"github.com/nelvko/proxyctl/internal/app"
	"github.com/nelvko/proxyctl/internal/download"
	"github.com/nelvko/proxyctl/internal/subscription"
)

type operationFinishedMsg struct {
	profiles *subscription.Service
	message  string
	warnings string
	added    string
	err      error
}
type cancelRequestedMsg struct{}

func (m *model) startRuntimeOperation(run func(*subscription.Service) (string, error), added *profile) tea.Cmd {
	return m.startOperation(app.MutateRuntimeProfiles, run, added)
}

func (m *model) startMetadataOperation(run func(*subscription.Service) (string, error), added *profile) tea.Cmd {
	return m.startOperation(func(ctx context.Context, _ download.Notify, run func(*subscription.Service) error) error {
		return app.MutateProfileMetadata(ctx, run)
	}, run, added)
}

func (m *model) startOperation(mutate func(context.Context, download.Notify, func(*subscription.Service) error) error, run func(*subscription.Service) (string, error), added *profile) tea.Cmd {
	ctx, cancel := context.WithCancel(m.ctx)
	m.cancel = cancel
	m.busy = true
	return tea.Batch(m.pendingMessage(m.tr("sub.working")), m.workSpinner.Tick, func() tea.Msg {
		defer cancel()
		result := operationFinishedMsg{}
		var diagnostics diagnostics
		err := mutate(ctx, diagnostics.Report, func(svc *subscription.Service) error {
			result.profiles = svc
			var err error
			result.message, err = run(svc)
			return err
		})
		result.err = err
		result.warnings = diagnostics.Text()
		if added != nil && err == nil {
			result.added = added.Name
		}
		return result
	})
}

// The editor runs with Bubble Tea's terminal suspended, but the same state
// transaction stays locked through validation and commit.
type tuiEditor struct {
	ctx            context.Context
	diagnostics    diagnostics
	name           string
	profiles       *subscription.Service
	stdin          io.Reader
	stdout, stderr io.Writer
}

func (e *tuiEditor) SetStdin(r io.Reader)  { e.stdin = r }
func (e *tuiEditor) SetStdout(w io.Writer) { e.stdout = w }
func (e *tuiEditor) SetStderr(w io.Writer) { e.stderr = w }
func (e *tuiEditor) Run() error {
	return app.MutateRuntimeProfiles(e.ctx, e.diagnostics.Report, func(svc *subscription.Service) error {
		e.profiles = svc
		return Edit(e.ctx, svc, e.name, "", e.stdin, e.stdout, e.stderr)
	})
}

func (m *model) startEditor(name string) tea.Cmd {
	ctx, cancel := context.WithCancel(m.ctx)
	m.cancel, m.busy = cancel, true
	editor := &tuiEditor{ctx: ctx, name: name}
	return tea.Exec(editor, func(err error) tea.Msg {
		cancel()
		return operationFinishedMsg{profiles: editor.profiles, warnings: editor.diagnostics.Text(), message: m.tr("sub.edited"), err: err}
	})
}
