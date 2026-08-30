// Package env derives the proxy shell environment from the active kernel
// config and renders it as shell statements.
package env

import (
	"fmt"
	"net/url"
	"os"
	"strings"

	"go.yaml.in/yaml/v3"
)

// ProxyEnv is the shell environment a running kernel exposes.
type ProxyEnv struct {
	HTTP  string
	HTTPS string
	All   string
	NO    string
}

const noProxyValue = "localhost,127.0.0.1,::1,.local"

// kernelConfig is the subset of a clash-family kernel config that
// determines the proxy endpoints.
type kernelConfig struct {
	MixedPort      int      `yaml:"mixed-port"`
	Port           int      `yaml:"port"`
	SocksPort      int      `yaml:"socks-port"`
	Bind           string   `yaml:"bind-address"`
	AllowLan       bool     `yaml:"allow-lan"`
	Authentication []string `yaml:"authentication"`
}

// Resolve derives the proxy endpoints from a clash-family kernel config.
// Port semantics follow mihomo: absent or -1 means disabled.
func Resolve(configFile string) (ProxyEnv, error) {
	raw, err := os.ReadFile(configFile)
	if err != nil {
		return ProxyEnv{}, fmt.Errorf("read kernel config: %w", err)
	}
	var kc kernelConfig
	if err := yaml.Unmarshal(raw, &kc); err != nil {
		return ProxyEnv{}, fmt.Errorf("parse kernel config: %w", err)
	}

	httpPort, socksPort := kc.httpPort(), kc.socksPort()
	if httpPort == 0 && socksPort == 0 {
		return ProxyEnv{}, fmt.Errorf("no usable inbound port in %s (need mixed-port, port or socks-port)", configFile)
	}

	// Old clashctl chain: mixed-port first, then the dedicated ports.
	authPrefix := ""
	if len(kc.Authentication) > 0 {
		user, pass, _ := strings.Cut(kc.Authentication[0], ":")
		authPrefix = url.UserPassword(user, pass).String() + "@"
	}

	e := ProxyEnv{NO: noProxyValue}
	if httpPort != 0 {
		e.HTTP = fmt.Sprintf("http://%s%s:%d", authPrefix, kc.host(), httpPort)
		e.HTTPS = e.HTTP
	}
	if socksPort != 0 {
		// socks5h: resolve hostnames on the proxy side.
		e.All = fmt.Sprintf("socks5h://%s%s:%d", authPrefix, kc.host(), socksPort)
	}
	return e, nil
}

func (kc kernelConfig) httpPort() int {
	if kc.MixedPort > 0 {
		return kc.MixedPort
	}
	if kc.Port > 0 {
		return kc.Port
	}
	return 0
}

func (kc kernelConfig) socksPort() int {
	if kc.MixedPort > 0 {
		return kc.MixedPort
	}
	if kc.SocksPort > 0 {
		return kc.SocksPort
	}
	return 0
}

func (kc kernelConfig) host() string {
	switch {
	case !kc.AllowLan:
		return "127.0.0.1"
	case kc.Bind == "", kc.Bind == "*", kc.Bind == "+":
		// Listening on all interfaces; loopback is the right client target.
		return "127.0.0.1"
	default:
		return kc.Bind
	}
}

// Shell reports the calling shell family. The shell integration exports
// PROXYCTL_SHELL, which wins over marker detection (FISH_VERSION is not
// exported by fish, so detection alone misses it).
func Shell() string {
	switch s := os.Getenv("PROXYCTL_SHELL"); s {
	case "bash", "zsh", "fish":
		return s
	}
	switch {
	case os.Getenv("FISH_VERSION") != "":
		return "fish"
	case os.Getenv("ZSH_VERSION") != "":
		return "zsh"
	default:
		return "bash"
	}
}

// Export renders statements that apply the environment in the given shell
// (bash and zsh share POSIX syntax).
func (e ProxyEnv) Export(shell string) []string {
	var lines []string
	add := func(key, value string) {
		if value == "" {
			return
		}
		// Single quotes: no parameter or command expansion in any of the
		// three shells, so values containing $ or ` survive eval verbatim.
		if shell == "fish" {
			lines = append(lines, fmt.Sprintf("set -gx %s %s", key, shQuote(value)))
			lines = append(lines, fmt.Sprintf("set -gx %s %s", lower(key), shQuote(value)))
			return
		}
		lines = append(lines, fmt.Sprintf("export %s=%s", key, shQuote(value)))
		lines = append(lines, fmt.Sprintf("export %s=%s", lower(key), shQuote(value)))
	}
	add("HTTP_PROXY", e.HTTP)
	add("HTTPS_PROXY", e.HTTPS)
	add("ALL_PROXY", e.All)
	add("NO_PROXY", e.NO)
	return lines
}

// Unset renders statements that remove the environment.
func (e ProxyEnv) Unset(shell string) []string {
	var lines []string
	for _, key := range []string{"HTTP_PROXY", "HTTPS_PROXY", "ALL_PROXY", "NO_PROXY"} {
		if shell == "fish" {
			lines = append(lines, fmt.Sprintf("set -e %s", key), fmt.Sprintf("set -e %s", lower(key)))
			continue
		}
		lines = append(lines, fmt.Sprintf("unset %s", key), fmt.Sprintf("unset %s", lower(key)))
	}
	return lines
}

func lower(s string) string {
	return strings.ToLower(s)
}

// shQuote wraps value in single quotes, escaping embedded quotes with the
// '\' trick that bash, zsh and fish all understand.
func shQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", `'\''`) + "'"
}
