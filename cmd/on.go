package cmd

import (
	"os"
	"strings"
	"syscall"

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
		var err error
		if err = k.Start(); err != nil {
			return err
		}
		if err = setProxy(); err != nil {
			return err
		}
		log.Ok("代理环境配置成功")
		return ExecShell()
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

func setProxy() error {
	proxyEnv[HTTP_PROXY] = "127.0.0.1:"
	proxyEnv[HTTPS_PROXY] = "127.0.0.1:7890"
	proxyEnv[ALL_PROXY] = "127.0.0.1:7890"

	for k, v := range proxyEnv {
		os.Setenv(k, v)
		os.Setenv(strings.ToLower(k), v)
	}
	return nil
}

func ExecShell() error {
	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "/bin/bash"
	}
	return syscall.Exec(shell, []string{shell, "-i"}, os.Environ())
}

func init() {
	RootCmd.AddCommand(onCmd)
}
