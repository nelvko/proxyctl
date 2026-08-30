package cmd

import (
	"fmt"
	"os"

	"github.com/nelvko/proxyctl/internal/env"
	"github.com/spf13/cobra"
)

// offCmd represents the off command
var offCmd = &cobra.Command{
	Use:   "off",
	Short: "Disable the proxy environment in the current shell",
	Long: `Disable the proxy environment in the current shell.

Stops the kernel service and prints shell statements that remove
the proxy environment to stdout.`,
	GroupID:     manageGroup.ID,
	Annotations: map[string]string{"shellEval": "true"},
	RunE: func(cmd *cobra.Command, args []string) error {
		k := Runtime().Kernel

		active, err := k.IsActive()
		if err != nil {
			return err
		}
		if active {
			if err := k.Stop(); err != nil {
				return err
			}
		}

		printShell(env.ProxyEnv{}.Unset(env.Shell()))
		fmt.Fprintln(os.Stderr, "😼 proxy off")
		return nil
	},
}

func init() {
	RootCmd.AddCommand(offCmd)
}
