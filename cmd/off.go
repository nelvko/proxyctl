package cmd

import (
	"github.com/nelvko/proxyctl/internal/env"
	"github.com/spf13/cobra"
)

// offCmd represents the off command
func newOffCmd() *cobra.Command {
	command := &cobra.Command{
		Use:   "off",
		Short: "Disable the proxy environment in the current shell",
		Long: `Disable the proxy environment in the current shell.

Print shell statements that remove the proxy environment from the current
shell. The shared kernel service keeps running; stop it explicitly with
proxyctl stop when it is no longer needed.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := requireShellIntegration(cmd.Name()); err != nil {
				return err
			}
			printShell(cmd, env.ProxyEnv{}.Unset(env.Shell()))
			cmd.PrintErrln("proxy off")
			return nil
		},
	}
	return command
}
