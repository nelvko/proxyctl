/*
Copyright © 2025 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"os"
	"strings"

	"github.com/nelvko/proxyctl/kernel"
	"github.com/nelvko/proxyctl/log"
	"github.com/spf13/cobra"
)

// offCmd represents the off command
var offCmd = &cobra.Command{
	Use:   "off",
	Short: "Disable proxy environment",
	Long:  `Stop proxy kernel and launch a shell without system proxy`,
	GroupID: manageGroup.ID,
	RunE: func(cmd *cobra.Command, args []string) error {
		svc := kernel.New()
		isActive, err := svc.IsActive()
		// fmt.Printf("%s",err)
		if err != nil {
			return err
		}
		if isActive {
			if err := svc.Stop(); err != nil {
				return err
			}
		}
		log.Ok("已关闭代理环境")
		LaunchShell(withoutProxy())
		return nil
	},
}

func withoutProxy() []string {
	for _, pe := range proxyEnv {
		os.Unsetenv(pe)
		os.Unsetenv(strings.ToLower(pe))
	}
	return os.Environ()
}

func init() {
	rootCmd.AddCommand(offCmd)
}
