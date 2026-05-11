package cmd

import (
	"github.com/nelvko/proxyctl/internal/env"
	"github.com/spf13/cobra"
)

// offCmd represents the off command
var offCmd = &cobra.Command{
	Use:     "off",
	Short:   "Disable proxy environment",
	Long:    `Stop proxy kernel and launch a shell without system proxy`,
	GroupID: manageGroup.ID,
	RunE: func(cmd *cobra.Command, args []string) error {
		IsActive, err := runtimeCtx.Kernel.IsActive()
		if err != nil {
			return err
		}
		if IsActive {
			if err := runtimeCtx.Kernel.Stop(); err != nil {
				return err
			}
		}
		env.UnsetProxy()
		return env.ExecShell()

	},
}

func init() {
	RootCmd.AddCommand(offCmd)
}
