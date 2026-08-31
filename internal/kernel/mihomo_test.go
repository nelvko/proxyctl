package kernel

import "testing"

func TestParseVersion(t *testing.T) {
	const output = "Mihomo Meta v1.19.30 linux amd64 with go1.26.6 Sun Aug 16 10:02:25 UTC 2026\nUse tags: with_gvisor\n"

	if got := parseVersion(output); got != "v1.19.30" {
		t.Fatalf("parseVersion() = %q, want %q", got, "v1.19.30")
	}
	if got := parseVersion("mihomo unknown version"); got != "" {
		t.Fatalf("parseVersion() = %q, want empty", got)
	}
}
