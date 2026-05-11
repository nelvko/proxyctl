package cmd

import (
	"github.com/nelvko/proxyctl/internal/env"
	"github.com/spf13/cobra"
)

// onCmd represents the on command
var onCmd = &cobra.Command{
	Use:   "on",
	Short: "Enable proxy environment",
	Long: `
Start proxy kernel and launch a shell with system proxy`,
	GroupID: manageGroup.ID,
	RunE: func(cmd *cobra.Command, args []string) error {
		IsActive, err := runtimeCtx.Kernel.IsActive()
		if err != nil {
			return err
		}
		if !IsActive {
			if err := runtimeCtx.Kernel.Start(); err != nil {
				return err
			}
		}
		env.SetProxy()
		return env.ExecShell()
	},
}

func init() {
	RootCmd.AddCommand(onCmd)
}
