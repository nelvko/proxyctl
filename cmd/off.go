package cmd

import (
	"github.com/nelvko/proxyctl/internal/env"
	"github.com/nelvko/proxyctl/internal/ui"
	"github.com/spf13/cobra"
)

// offCmd represents the off command
var offCmd = &cobra.Command{
	Use:   "off",
	Short: "Disable the proxy environment in the current shell",
	Long: `Disable the proxy environment in the current shell.

Stops the kernel service and prints shell statements that remove
the proxy environment to stdout.`,
	Annotations: map[string]string{"shellEval": "true"},
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := requireShellIntegration(cmd.Name()); err != nil {
			return err
		}
		k, err := App().Kernel()
		if err != nil {
			return err
		}

		running, err := k.IsActive()
		if err != nil {
			return err
		}
		if running {
			if err := k.Stop(); err != nil {
				return err
			}
		}

		printShell(env.ProxyEnv{}.Unset(env.Shell()))
		ui.Err("proxy off")
		return nil
	},
}

func init() {
	RootCmd.AddCommand(offCmd)
}
