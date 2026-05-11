package env

import (
	"os"
	"strings"
	"syscall"
)

const (
	HTTPProxy  = "HTTP_PROXY"
	HTTPSProxy = "HTTPS_PROXY"
	ALLProxy   = "ALL_PROXY"
	NOProxy    = "NO_PROXY"
)

var proxyEnv = map[string]string{
	HTTPProxy:  "127.0.0.1:7890",
	HTTPSProxy: "127.0.0.1:7890",
	ALLProxy:   "127.0.0.1:7890",
	NOProxy:    "localhost,127.0.0.1,::1,.local",
}

func SetProxy() {
	for k, v := range proxyEnv {
		os.Setenv(k, v)
		os.Setenv(strings.ToLower(k), v)
	}
}

func UnsetProxy() {
	for k := range proxyEnv {
		os.Unsetenv(k)
		os.Unsetenv(strings.ToLower(k))
	}
}

func ExecShell() error {
	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "/bin/bash"
	}
	return syscall.Exec(shell, []string{shell, "-i"}, os.Environ())
}
