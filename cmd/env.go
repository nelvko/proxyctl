package cmd

import (
	"github.com/nelvko/proxyctl/internal/env"
	"github.com/spf13/cobra"
)

// envCmd prints the proxy environment statements without applying them.
func newEnvCmd() *cobra.Command {
	var unset bool
	command := &cobra.Command{
		Use:   "env",
		Short: "Print proxy environment statements",
		Long: `Print the shell statements for the proxy environment, resolved
from the active kernel config, without applying them.

Useful for debugging, or for a custom hook that keeps the
environment in sync:

    eval "$(proxyctl env)"
`,
		GroupID: "shell",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if unset {
				printShell(cmd, env.ProxyEnv{}.Unset(env.Shell()))
				return nil
			}
			a, err := loadApp(cmd)
			if err != nil {
				return err
			}
			k, err := a.Kernel()
			if err != nil {
				return err
			}
			endpoints, err := k.ProxyEndpoints()
			if err != nil {
				return err
			}
			e := env.ForProxy(endpoints.HTTP, endpoints.SOCKS)
			lines := e.Export(env.Shell())
			printShell(cmd, lines)
			return nil
		},
	}
	command.Flags().BoolVarP(&unset, "unset", "u", false, "Print statements that remove the proxy environment")
	return command
}
