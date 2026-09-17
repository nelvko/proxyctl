package kernel

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"

	"go.yaml.in/yaml/v3"
)

func (m Mihomo) ProxyEndpoints() (Endpoints, error) { return resolveMihomoEndpoints(m.cfg.ConfigFile) }

// inboundConfig is the subset of a clash-family kernel config that
// determines the proxy endpoints.
type inboundConfig struct {
	MixedPort      int      `yaml:"mixed-port"`
	Port           int      `yaml:"port"`
	SocksPort      int      `yaml:"socks-port"`
	Bind           string   `yaml:"bind-address"`
	AllowLan       bool     `yaml:"allow-lan"`
	Authentication []string `yaml:"authentication"`
}

// resolveMihomoEndpoints derives the proxy endpoints from a clash-family kernel config.
// Port semantics follow mihomo: absent or -1 means disabled.
func resolveMihomoEndpoints(configFile string) (Endpoints, error) {
	raw, err := os.ReadFile(configFile)
	if err != nil {
		return Endpoints{}, fmt.Errorf("read kernel config: %w", err)
	}
	var kc inboundConfig
	if err := yaml.Unmarshal(raw, &kc); err != nil {
		return Endpoints{}, fmt.Errorf("parse kernel config: %w", err)
	}

	for _, port := range []int{kc.MixedPort, kc.Port, kc.SocksPort} {
		if port < -1 || port > 65535 {
			return Endpoints{}, fmt.Errorf("invalid inbound port %d", port)
		}
	}
	httpPort, socksPort := kc.httpPort(), kc.socksPort()
	if httpPort == 0 && socksPort == 0 {
		return Endpoints{}, fmt.Errorf("no usable inbound port in %s (need mixed-port, port or socks-port)", configFile)
	}

	// Optional user:pass from the first authentication entry.
	authPrefix := ""
	if len(kc.Authentication) > 0 {
		user, pass, _ := strings.Cut(kc.Authentication[0], ":")
		authPrefix = url.UserPassword(user, pass).String() + "@"
	}

	e := Endpoints{}
	if httpPort != 0 {
		e.HTTP = "http://" + authPrefix + net.JoinHostPort(kc.host(), strconv.Itoa(httpPort))
	}
	if socksPort != 0 {
		// socks5h: resolve hostnames on the proxy side.
		e.SOCKS = "socks5h://" + authPrefix + net.JoinHostPort(kc.host(), strconv.Itoa(socksPort))
	}
	return e, nil
}

// Port chain from the old clashctl: mixed-port first, then the dedicated
// ports. -1 (explicitly disabled) falls through like absent.
func (kc inboundConfig) httpPort() int {
	if kc.MixedPort > 0 {
		return kc.MixedPort
	}
	if kc.Port > 0 {
		return kc.Port
	}
	return 0
}

func (kc inboundConfig) socksPort() int {
	if kc.MixedPort > 0 {
		return kc.MixedPort
	}
	if kc.SocksPort > 0 {
		return kc.SocksPort
	}
	return 0
}

func (kc inboundConfig) host() string {
	switch {
	case !kc.AllowLan:
		return "127.0.0.1"
	case kc.Bind == "::", kc.Bind == "[::]":
		return "::1"
	case kc.Bind == "", kc.Bind == "*", kc.Bind == "+", kc.Bind == "0.0.0.0":
		// Listening on all interfaces; loopback is the right client target.
		return "127.0.0.1"
	default:
		return strings.Trim(kc.Bind, "[]")
	}
}
