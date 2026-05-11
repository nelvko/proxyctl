package cmd

import (
	"os"
	"strings"

	"github.com/spf13/cobra"
)

// offCmd represents the off command
var offCmd = &cobra.Command{
	Use:     "off",
	Short:   "Disable proxy environment",
	Long:    `Stop proxy kernel and launch a shell without system proxy`,
	GroupID: manageGroup.ID,
	RunE: func(cmd *cobra.Command, args []string) error {
		IsActive, err := AppCtx.Kernel.IsActive()
		if err != nil {
			return err
		}
		if IsActive {
			if err := AppCtx.Kernel.Stop(); err != nil {
				return err
			}
		}
		unsetProxy()
		return execShell()

	},
}

func unsetProxy() {
	for k := range proxyEnv {
		os.Unsetenv(k)
		os.Unsetenv(strings.ToLower(k))
	}
}

func init() {
	RootCmd.AddCommand(offCmd)
}
