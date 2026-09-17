package kernel

import (
	"os"

	"github.com/nelvko/proxyctl/internal/app"
	"github.com/nelvko/proxyctl/internal/ui"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

func loadApp(cmd *cobra.Command) (*app.App, error) {
	a, err := app.Load()
	if err != nil {
		return nil, err
	}
	a.Notify = ui.Reporter(cmd.ErrOrStderr())
	return a, nil
}

// mutation declares a command's state lock at its definition, independently
// of its name, aliases or location in the command tree.
func mutation(run func(*cobra.Command, []string) error) func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, args []string) error {
		return app.WithStateLock(cmd.Context(), func() error { return run(cmd, args) })
	}
}

func isInteractive() bool {
	return term.IsTerminal(int(os.Stdin.Fd())) && term.IsTerminal(int(os.Stdout.Fd()))
}
