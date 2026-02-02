/*
Copyright © 2025 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"os"
	"strings"
	"syscall"

	"github.com/nelvko/proxyctl/kernel"
	"github.com/nelvko/proxyctl/log"
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
		svc := kernel.New()
		IsActive, err := svc.IsActive()
		if err != nil {
			return err
		}
		if !IsActive {
			if err := svc.Start(); err != nil {
				return err
			}
		}
		log.Ok("已开启代理环境")
		LaunchShell(withProxy())
		return nil
	},
}

const (
	HTTP_PROXY  = "HTTP_PROXY"
	HTTPS_PROXY = "HTTPS_PROXY"
	ALL_PROXY   = "ALL_PROXY"
	NO_PROXY    = "NO_PROXY"
)

var proxyEnv = map[string]string{
	HTTP_PROXY:  "",
	HTTPS_PROXY: "",
	ALL_PROXY:   "",
	NO_PROXY:    "localhost,127.0.0.1,::1,.local",
}

func getProxyEnv(env map[string]string) {
	env[HTTP_PROXY] = "127.0.0.1:7890"
	env[HTTPS_PROXY] = "127.0.0.1:7890"
	env[ALL_PROXY] = "127.0.0.1:7890"
}

func withProxy() []string {
	getProxyEnv(proxyEnv)
	for k, v := range proxyEnv {
		os.Setenv(k, v)
		os.Setenv(strings.ToLower(k), v)
	}
	return os.Environ()
}

func LaunchShell(env []string) {
	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "/bin/bash"
	}
	syscall.Exec(shell, []string{shell, "-i"}, env)
}

func init() {
	rootCmd.AddCommand(onCmd)
}
