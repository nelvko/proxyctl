package ui

import (
	"strings"
	"testing"

	"github.com/nelvko/proxyctl/internal/config"
	"github.com/nelvko/proxyctl/internal/subscription"
)

func TestDownloadDetailsMatchDefaultsAndSourceType(t *testing.T) {
	out := ProfileDetails(config.Profile{URL: "https://example.com/secret"}, "", false)
	if !strings.Contains(out, subscription.DefaultDownloadTimeout.String()) || !strings.Contains(out, subscription.DefaultDownloadUserAgent+" (default") || strings.Contains(out, "secret") {
		t.Fatal(out)
	}
	p := config.Profile{URL: "file:///private/source.yaml", Download: config.DownloadConfig{DownloadVia: "kernel", SkipCert: true}}
	out = ProfileDetails(p, "", false)
	if !strings.Contains(out, "linked local file") || strings.Contains(out, "verify TLS") || strings.Contains(out, "running kernel") {
		t.Fatal(out)
	}
	p.URL = "https://example.com"
	p.Download.Headers = map[string]string{"user-agent": "private-agent"}
	out = ProfileDetails(p, "", false)
	if strings.Contains(out, "private-agent") || !strings.Contains(out, "value hidden") {
		t.Fatal(out)
	}
}
