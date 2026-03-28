package cmd

import (
	"os"
	"strings"

	"github.com/nelvko/proxyctl/log"
	"github.com/spf13/cobra"
)

// offCmd represents the off command
var offCmd = &cobra.Command{
	Use:     "off",
	Short:   "Disable proxy environment",
	Long:    `Stop proxy kernel and launch a shell without system proxy`,
	GroupID: manageGroup.ID,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := k.Stop(); err != nil {
			return err
		}
		unsetProxy()
		log.Ok("已关闭代理环境")
		return ExecShell()

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
