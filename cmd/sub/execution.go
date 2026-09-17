package sub

import (
	"os"

	"github.com/nelvko/proxyctl/internal/app"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

// mutation keeps command adaptation local; app owns the shared state-lock boundary.
// State must be loaded inside run, after acquiring the state lock.
func mutation(run func(*cobra.Command, []string) error) func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, args []string) error {
		return app.WithStateLock(cmd.Context(), func() error { return run(cmd, args) })
	}
}

func isInteractive() bool {
	return term.IsTerminal(int(os.Stdin.Fd())) && term.IsTerminal(int(os.Stdout.Fd()))
}
