package env

import (
	"strings"
	"testing"
)

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
	if !strings.Contains(joined, "unset ALL_PROXY") {
		t.Fatalf("Export(bash) must clear obsolete proxy variables:\n%s", joined)
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
