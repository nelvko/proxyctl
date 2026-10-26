package cmd

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/nelvko/proxyctl/internal/config"
	"github.com/nelvko/proxyctl/internal/kernel"
)

func isolatedStatusConfig(t *testing.T) string {
	t.Helper()
	dir := isolatedConfig(t)
	for _, key := range []string{"HTTP_PROXY", "http_proxy", "HTTPS_PROXY", "https_proxy", "ALL_PROXY", "all_proxy", "NO_PROXY", "no_proxy"} {
		t.Setenv(key, "")
	}
	return dir
}

func decodeStatus(t *testing.T, output string) statusJSON {
	t.Helper()
	decoder := json.NewDecoder(strings.NewReader(output))
	var result statusJSON
	if err := decoder.Decode(&result); err != nil {
		t.Fatalf("invalid JSON status %q: %v", output, err)
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		t.Fatalf("status contained more than one JSON object: %q (%v)", output, err)
	}
	return result
}

func TestStatusJSONWithoutKernel(t *testing.T) {
	isolatedStatusConfig(t)
	output, stderr, err := runCommand("status", "--json")
	if err != nil || stderr != "" {
		t.Fatalf("status: %v, stderr=%q", err, stderr)
	}
	result := decodeStatus(t, output)
	if result.Kernel.State != "none" || result.Kernel.Name != nil || result.Service.State != "unavailable" || result.Proxy.State != "unavailable" {
		t.Fatalf("unexpected empty runtime: %+v", result)
	}
	if result.Profile.State != "none" || result.Profile.Name != nil || result.Profile.Total == nil || *result.Profile.Total != 0 || result.Profile.Error != nil {
		t.Fatalf("unexpected empty subscriptions: %+v", result.Profile)
	}
	if result.Shell.ProxyVariables == nil || len(result.Shell.ProxyVariables) != 0 || !strings.Contains(output, `"proxy_variables":[]`) {
		t.Fatalf("shell variables must be an empty array: %q", output)
	}
}

func TestStatusJSONListsShellVariableNamesOnly(t *testing.T) {
	isolatedStatusConfig(t)
	for _, key := range []string{"HTTP_PROXY", "https_proxy", "NO_PROXY"} {
		t.Setenv(key, "http://shell-user:shell-password@127.0.0.1:7890/private-token")
	}
	output, _, err := runCommand("status", "--json")
	if err != nil {
		t.Fatal(err)
	}
	result := decodeStatus(t, output)
	if want := []string{"HTTP_PROXY", "https_proxy", "NO_PROXY"}; !reflect.DeepEqual(result.Shell.ProxyVariables, want) {
		t.Fatalf("proxy variables = %v; want %v", result.Shell.ProxyVariables, want)
	}
	for _, secret := range []string{"shell-user", "shell-password", "private-token", "127.0.0.1"} {
		if strings.Contains(output, secret) {
			t.Fatalf("status exposed shell variable value: %q", output)
		}
	}
}

func TestStatusJSONKeepsSubscriptionFailureLocal(t *testing.T) {
	dir := isolatedStatusConfig(t)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	// Unmarshal errors can contain the offending value, including a URL.
	broken := "profiles: https://sub-user:sub-password@example.com/private-token?token=secret-token\n"
	if err := os.WriteFile(filepath.Join(dir, "subscriptions.yaml"), []byte(broken), 0o600); err != nil {
		t.Fatal(err)
	}
	output, stderr, err := runCommand("status", "--json")
	if err != nil || stderr != "" {
		t.Fatalf("local query failure failed status: %v, stderr=%q", err, stderr)
	}
	result := decodeStatus(t, output)
	if result.Kernel.State != "none" || result.Profile.State != "unknown" || result.Profile.Name != nil || result.Profile.Total != nil || result.Profile.Error == nil {
		t.Fatalf("unexpected partial status: %+v", result)
	}
	if !strings.Contains(*result.Profile.Error, "load subscription config") {
		t.Fatalf("missing subscription error: %q", *result.Profile.Error)
	}
	for _, secret := range []string{"sub-user", "sub-password", "private-token", "secret-token"} {
		if strings.Contains(output, secret) {
			t.Fatalf("status exposed a URL credential: %q", output)
		}
	}
}

func TestStatusJSONWithSelectedProfile(t *testing.T) {
	isolatedStatusConfig(t)
	// An unsupported adapter fails assembly before any system service query.
	if err := config.Save(&config.Config{Use: "clash", Kernels: []config.KernelConfig{{Name: "clash"}}}); err != nil {
		t.Fatal(err)
	}
	if err := config.SaveSubscriptionConfig(&config.SubscriptionConfig{
		Use: "work",
		Profiles: []config.Profile{
			{Name: "work", URL: "https://user:password@example.com/private-token"},
			{Name: "backup"},
		},
	}); err != nil {
		t.Fatal(err)
	}
	output, _, err := runCommand("status", "--json")
	if err != nil {
		t.Fatal(err)
	}
	result := decodeStatus(t, output)
	if result.Kernel.State != "selected" || result.Kernel.Name == nil || *result.Kernel.Name != "clash" || result.Service.State != "unknown" || result.Proxy.State != "unknown" {
		t.Fatalf("unexpected selected kernel: %+v", result)
	}
	if result.Profile.State != "selected" || result.Profile.Name == nil || *result.Profile.Name != "work" || result.Profile.Total == nil || *result.Profile.Total != 2 {
		t.Fatalf("unexpected profile: %+v", result.Profile)
	}
	if strings.Contains(output, "example.com") || strings.Contains(output, "password") {
		t.Fatalf("status exposed the profile source: %q", output)
	}
}

func TestStatusJSONMainConfigFailureHasNoOutput(t *testing.T) {
	dir := isolatedStatusConfig(t)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "config.yaml"), []byte("broken: ["), 0o600); err != nil {
		t.Fatal(err)
	}
	output, _, err := runCommand("status", "--json")
	if err == nil || output != "" {
		t.Fatalf("invalid main config returned output=%q, error=%v", output, err)
	}
}

type statusTestRuntime struct {
	running      bool
	serviceErr   error
	endpoints    kernel.Endpoints
	endpointsErr error
}

func (k statusTestRuntime) IsActive() (bool, error) { return k.running, k.serviceErr }
func (k statusTestRuntime) ProxyEndpoints() (kernel.Endpoints, error) {
	return k.endpoints, k.endpointsErr
}

func TestStatusJSONRuntimeRedactsEndpoints(t *testing.T) {
	k := statusTestRuntime{
		running: true,
		endpoints: kernel.Endpoints{
			HTTP:  "http://http-user:http-password@127.0.0.1:7890/private-token?token=query-token#fragment-token",
			SOCKS: "socks5h://socks-user:socks-password@[::1]:7891",
		},
	}
	service, proxy := readStatusRuntime(k, nil)
	if service.State != "running" || service.Scope != "user" || service.Error != nil || proxy.State != "available" || proxy.Error != nil {
		t.Fatalf("unexpected runtime: service=%+v, proxy=%+v", service, proxy)
	}
	if proxy.HTTP == nil || *proxy.HTTP != "http://127.0.0.1:7890/…?[redacted]#[redacted]" || proxy.SOCKS == nil || *proxy.SOCKS != "socks5h://[::1]:7891" {
		t.Fatalf("unexpected redacted endpoints: %+v", proxy)
	}
	var output bytes.Buffer
	if err := json.NewEncoder(&output).Encode(proxy); err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{"http-user", "http-password", "socks-user", "socks-password", "private-token", "query-token", "fragment-token"} {
		if strings.Contains(output.String(), secret) {
			t.Fatalf("status exposed an endpoint credential: %s", output.String())
		}
	}
}

func TestStatusJSONRuntimeDistinguishesStoppedEmptyAndUnknown(t *testing.T) {
	service, proxy := readStatusRuntime(statusTestRuntime{}, nil)
	if service.State != "stopped" || service.Error != nil || proxy.State != "none" || proxy.HTTP != nil || proxy.SOCKS != nil || proxy.Error != nil {
		t.Fatalf("unexpected stopped runtime: service=%+v, proxy=%+v", service, proxy)
	}
	privateErr := errors.New("query https://query-user:query-password@example.com/private-token?token=secret-token failed")
	service, proxy = readStatusRuntime(statusTestRuntime{serviceErr: privateErr, endpointsErr: privateErr}, nil)
	if service.State != "unknown" || service.Error == nil || proxy.State != "unknown" || proxy.Error == nil {
		t.Fatalf("errors were treated as empty states: service=%+v, proxy=%+v", service, proxy)
	}
	for _, secret := range []string{"query-user", "query-password", "private-token", "secret-token"} {
		if strings.Contains(*service.Error, secret) || strings.Contains(*proxy.Error, secret) {
			t.Fatalf("status exposed an error credential: service=%q, proxy=%q", *service.Error, *proxy.Error)
		}
	}
	service, proxy = readStatusRuntime(nil, privateErr)
	if service.State != "unknown" || service.Error == nil || proxy.State != "unknown" || proxy.Error == nil {
		t.Fatalf("kernel assembly error was lost: service=%+v, proxy=%+v", service, proxy)
	}
}

func TestStatusDefaultOutputRemainsHumanReadable(t *testing.T) {
	isolatedStatusConfig(t)
	output, stderr, err := runCommand("status")
	want := "kernel:    none (run `proxyctl install` or `proxyctl use <name>`)\nservice:   -\nproxy:     -\nshell:     no proxy variables set\nprofile:   none\n"
	if err != nil || stderr != "" || output != want {
		t.Fatalf("default status changed: output=%q, stderr=%q, error=%v", output, stderr, err)
	}
}

func TestStatusReportsDefaultConfigWithoutInitializingIt(t *testing.T) {
	for _, kind := range []string{"mihomo", "sing-box"} {
		t.Run(kind, func(t *testing.T) {
			isolatedStatusConfig(t)
			cfg, err := config.DefaultKernelConfig("status-default")
			if err != nil {
				t.Fatal(err)
			}
			cfg.Type = kind
			if err := config.Save(&config.Config{Use: cfg.Name, Kernels: []config.KernelConfig{cfg}}); err != nil {
				t.Fatal(err)
			}
			output, _, err := runCommand("status", "--json")
			if err != nil || decodeStatus(t, output).Proxy.DefaultConfig != nil {
				t.Fatalf("missing config reported a known state: %s, %v", output, err)
			}
			if _, err := os.Stat(cfg.ConfigFile); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("status initialized the config: %v", err)
			}
			k, err := kernel.New(&cfg)
			if err != nil {
				t.Fatal(err)
			}
			if err := kernel.EnsureDefaultConfig(k); err != nil {
				t.Fatal(err)
			}
			output, _, err = runCommand("status", "--json")
			if err != nil {
				t.Fatal(err)
			}
			result := decodeStatus(t, output)
			if result.Proxy.DefaultConfig == nil || !*result.Proxy.DefaultConfig || result.Proxy.State != "available" || result.Profile.State != "none" {
				t.Fatalf("default config status: %s", output)
			}
			output, _, err = runCommand("status")
			if err != nil || !strings.Contains(output, "config:    default (direct)\n") {
				t.Fatalf("default mode missing: %q, %v", output, err)
			}
			if err := os.WriteFile(cfg.ConfigFile, []byte("{}\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			output, _, err = runCommand("status", "--json")
			if err != nil {
				t.Fatal(err)
			}
			result = decodeStatus(t, output)
			if result.Proxy.DefaultConfig == nil || *result.Proxy.DefaultConfig {
				t.Fatalf("manual config mistaken for default: %s", output)
			}
			output, _, err = runCommand("on")
			if err == nil || output != "" || !strings.Contains(err.Error(), "no usable") {
				t.Fatalf("on without inbound: output=%q error=%v", output, err)
			}
			data, err := os.ReadFile(cfg.ConfigFile)
			if err != nil || string(data) != "{}\n" {
				t.Fatalf("on replaced manual configuration: %q, %v", data, err)
			}
		})
	}
}
