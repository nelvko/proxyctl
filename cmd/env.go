package cmd

import (
	"github.com/nelvko/proxyctl/internal/env"
	"github.com/nelvko/proxyctl/internal/ui"
	"github.com/spf13/cobra"
)

// envCmd prints the proxy environment statements without applying them.
var envCmd = &cobra.Command{
	Use:   "env",
	Short: "Print proxy environment statements",
	Long: `Print the shell statements for the proxy environment, resolved
from the active kernel config, without applying them.

Useful for debugging, or for a custom hook that keeps the
environment in sync:

    eval "$(proxyctl env)"
`,
	RunE: func(cmd *cobra.Command, args []string) error {
		k, err := App().Kernel()
		if err != nil {
			return err
		}
		e, err := env.Resolve(k.ConfigFile())
		if err != nil {
			return err
		}
		lines := e.Export(env.Shell())
		if unset {
			lines = e.Unset(env.Shell())
		}
		printShell(lines)
		ui.Err("proxy env")
		return nil
	},
}

var unset bool

func init() {
	RootCmd.AddCommand(envCmd)
	envCmd.Flags().BoolVarP(&unset, "unset", "u", false, "Print statements that remove the environment instead")
}
