package kernel

import (
	"os"
	"path/filepath"
	"testing"
)

func writeConfig(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	return path
}

func TestResolveMixedPort(t *testing.T) {
	e, err := resolveMihomoEndpoints(writeConfig(t, "mixed-port: 7890\n"))
	if err != nil {
		t.Fatalf("resolveMihomoEndpoints() error = %v", err)
	}
	if got, want := e.HTTP, "http://127.0.0.1:7890"; got != want {
		t.Fatalf("HTTP = %q, want %q", got, want)
	}

	if got, want := e.SOCKS, "socks5h://127.0.0.1:7890"; got != want {
		t.Fatalf("All = %q, want %q", got, want)
	}
}

func TestResolvePortFallbackChain(t *testing.T) {
	// No mixed-port: http falls to port, socks falls to socks-port.
	e, err := resolveMihomoEndpoints(writeConfig(t, "port: 1080\nsocks-port: 1081\n"))
	if err != nil {
		t.Fatalf("resolveMihomoEndpoints() error = %v", err)
	}
	if got, want := e.HTTP, "http://127.0.0.1:1080"; got != want {
		t.Fatalf("HTTP = %q, want %q", got, want)
	}
	if got, want := e.SOCKS, "socks5h://127.0.0.1:1081"; got != want {
		t.Fatalf("All = %q, want %q", got, want)
	}
}

func TestResolveDisabledPorts(t *testing.T) {
	// -1 means explicitly disabled; the chain must skip it.
	e, err := resolveMihomoEndpoints(writeConfig(t, "mixed-port: -1\nport: 7890\n"))
	if err != nil {
		t.Fatalf("resolveMihomoEndpoints() error = %v", err)
	}
	if got, want := e.HTTP, "http://127.0.0.1:7890"; got != want {
		t.Fatalf("HTTP = %q, want %q", got, want)
	}
	if e.SOCKS != "" {
		t.Fatalf("All = %q, want empty", e.SOCKS)
	}
}

func TestResolveNoPorts(t *testing.T) {
	if _, err := resolveMihomoEndpoints(writeConfig(t, "mode: rule\n")); err == nil {
		t.Fatal("resolveMihomoEndpoints() with no ports should error")
	}
}

func TestResolveAuthenticationAndLan(t *testing.T) {
	e, err := resolveMihomoEndpoints(writeConfig(t, `mixed-port: 7890
allow-lan: true
bind-address: "*"
authentication:
  - "user:pass"
`))
	if err != nil {
		t.Fatalf("resolveMihomoEndpoints() error = %v", err)
	}
	if got, want := e.HTTP, "http://user:pass@127.0.0.1:7890"; got != want {
		t.Fatalf("HTTP = %q, want %q", got, want)
	}

	// Illegal userinfo characters are percent-encoded; '$' is a legal
	// sub-delim and stays literal (single-quoting keeps it safe in eval).
	e, err = resolveMihomoEndpoints(writeConfig(t, `mixed-port: 7890
authentication:
  - "u$er:p:$$w"
`))
	if err != nil {
		t.Fatalf("resolveMihomoEndpoints() error = %v", err)
	}
	if got, want := e.HTTP, "http://u$er:p%3A$$w@127.0.0.1:7890"; got != want {
		t.Fatalf("HTTP = %q, want %q", got, want)
	}

	e, err = resolveMihomoEndpoints(writeConfig(t, "mixed-port: 7890\nallow-lan: true\nbind-address: 192.168.1.10\n"))
	if err != nil {
		t.Fatalf("resolveMihomoEndpoints() error = %v", err)
	}
	if got, want := e.HTTP, "http://192.168.1.10:7890"; got != want {
		t.Fatalf("HTTP = %q, want %q", got, want)
	}
}

func TestResolveIPv6AndPortValidation(t *testing.T) {
	file := filepath.Join(t.TempDir(), "config.yaml")
	for _, input := range []string{"allow-lan: true\nbind-address: '::'\nmixed-port: 7890\nauthentication: ['u:p']\n", "allow-lan: true\nbind-address: '[::1]'\nmixed-port: 7890\nauthentication: ['u:p']\n"} {
		if err := os.WriteFile(file, []byte(input), 0o600); err != nil {
			t.Fatal(err)
		}
		e, err := resolveMihomoEndpoints(file)
		if err != nil || e.HTTP != "http://u:p@[::1]:7890" || e.SOCKS != "socks5h://u:p@[::1]:7890" {
			t.Fatalf("IPv6: %+v %v", e, err)
		}
	}
	if err := os.WriteFile(file, []byte("port: 65536\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := resolveMihomoEndpoints(file); err == nil {
		t.Fatal("invalid port accepted")
	}
}
