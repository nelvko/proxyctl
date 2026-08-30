package env

import (
	"os"
	"path/filepath"
	"strings"
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
	e, err := Resolve(writeConfig(t, "mixed-port: 7890\n"))
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if got, want := e.HTTP, "http://127.0.0.1:7890"; got != want {
		t.Fatalf("HTTP = %q, want %q", got, want)
	}
	if got, want := e.HTTPS, "http://127.0.0.1:7890"; got != want {
		t.Fatalf("HTTPS = %q, want %q", got, want)
	}
	if got, want := e.All, "socks5h://127.0.0.1:7890"; got != want {
		t.Fatalf("All = %q, want %q", got, want)
	}
}

func TestResolvePortFallbackChain(t *testing.T) {
	// No mixed-port: http falls to port, socks falls to socks-port.
	e, err := Resolve(writeConfig(t, "port: 1080\nsocks-port: 1081\n"))
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if got, want := e.HTTP, "http://127.0.0.1:1080"; got != want {
		t.Fatalf("HTTP = %q, want %q", got, want)
	}
	if got, want := e.All, "socks5h://127.0.0.1:1081"; got != want {
		t.Fatalf("All = %q, want %q", got, want)
	}
}

func TestResolveDisabledPorts(t *testing.T) {
	// -1 means explicitly disabled; the chain must skip it.
	e, err := Resolve(writeConfig(t, "mixed-port: -1\nport: 7890\n"))
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if got, want := e.HTTP, "http://127.0.0.1:7890"; got != want {
		t.Fatalf("HTTP = %q, want %q", got, want)
	}
	if e.All != "" {
		t.Fatalf("All = %q, want empty", e.All)
	}
}

func TestResolveNoPorts(t *testing.T) {
	if _, err := Resolve(writeConfig(t, "mode: rule\n")); err == nil {
		t.Fatal("Resolve() with no ports should error")
	}
}

func TestResolveAuthenticationAndLan(t *testing.T) {
	e, err := Resolve(writeConfig(t, `mixed-port: 7890
allow-lan: true
bind-address: "*"
authentication:
  - "user:pass"
`))
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if got, want := e.HTTP, "http://user:pass@127.0.0.1:7890"; got != want {
		t.Fatalf("HTTP = %q, want %q", got, want)
	}

	// Illegal userinfo characters are percent-encoded; '$' is a legal
	// sub-delim and stays literal (single-quoting keeps it safe in eval).
	e, err = Resolve(writeConfig(t, `mixed-port: 7890
authentication:
  - "u$er:p:$$w"
`))
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if got, want := e.HTTP, "http://u$er:p%3A$$w@127.0.0.1:7890"; got != want {
		t.Fatalf("HTTP = %q, want %q", got, want)
	}

	e, err = Resolve(writeConfig(t, "mixed-port: 7890\nallow-lan: true\nbind-address: 192.168.1.10\n"))
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if got, want := e.HTTP, "http://192.168.1.10:7890"; got != want {
		t.Fatalf("HTTP = %q, want %q", got, want)
	}
}

func TestExportUnset(t *testing.T) {
	e := ProxyEnv{HTTP: "http://127.0.0.1:7890", NO: noProxyValue}

	export := e.Export("bash")
	joined := strings.Join(export, "\n")
	for _, want := range []string{
		`export HTTP_PROXY='http://127.0.0.1:7890'`,
		`export http_proxy='http://127.0.0.1:7890'`,
		`export NO_PROXY='` + noProxyValue + `'`,
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("Export(bash) missing %q:\n%s", want, joined)
		}
	}
	if strings.Contains(joined, "ALL_PROXY") {
		t.Fatalf("Export(bash) must skip empty values:\n%s", joined)
	}

	fishExport := e.Export("fish")
	if !strings.Contains(strings.Join(fishExport, "\n"), `set -gx HTTP_PROXY 'http://127.0.0.1:7890'`) {
		t.Fatalf("Export(fish) wrong:\n%s", strings.Join(fishExport, "\n"))
	}

	unsetLines := strings.Join(e.Unset("bash"), "\n")
	for _, want := range []string{"unset HTTP_PROXY", "unset http_proxy", "unset all_proxy"} {
		if !strings.Contains(unsetLines, want) {
			t.Fatalf("Unset(bash) missing %q:\n%s", want, unsetLines)
		}
	}
}

func TestShQuoteSurvivesEval(t *testing.T) {
	// Values with shell metacharacters must be quoted so eval keeps them
	// verbatim.
	e := ProxyEnv{HTTP: "http://u:p$ASS`id`@127.0.0.1:7890"}
	for _, line := range e.Export("bash") {
		if strings.ContainsAny(line, "\"") {
			t.Fatalf("Export must use single quotes, got %q", line)
		}
	}
	want := `export HTTP_PROXY='http://u:p$ASS` + "`id`" + `@127.0.0.1:7890'`
	if got := e.Export("bash")[0]; got != want {
		t.Fatalf("Export(bash)[0] = %q, want %q", got, want)
	}
}
