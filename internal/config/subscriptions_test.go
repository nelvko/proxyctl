package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSubscriptionConfigRejectsUnknownSettings(t *testing.T) {
	for _, fields := range []string{
		"update: {timeout: 5s}",
		"download: {enable: true}",
		"download: {interval: 1h}",
		"download: {cron: '* * * * *'}",
		"download: {timout: 5s}",
	} {
		t.Run(fields, func(t *testing.T) {
			writeTestFile(t, "subscriptions.yaml", "profiles:\n  - name: work\n    url: https://example.com\n    "+fields+"\n")
			if _, err := LoadSubscriptionConfig(); err == nil {
				t.Fatal("unsupported settings were silently ignored")
			}
		})
	}
}

func TestSubscriptionDownloadSchemaRoundTrip(t *testing.T) {
	dir := useTestDir(t)
	want := DownloadConfig{Timeout: 25 * time.Second, UserAgent: "proxyctl-test", DownloadVia: "direct", SkipCert: true, Headers: map[string]string{"X-Test": "value"}}
	if err := SaveSubscriptionConfig(&SubscriptionConfig{Profiles: []Profile{{Name: "work", Download: want}}}); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(dir, "subscriptions.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	for _, old := range []string{"update:", "enable:", "interval:", "cron:"} {
		if strings.Contains(string(raw), old) {
			t.Fatalf("obsolete field %q persisted", old)
		}
	}
	if !strings.Contains(string(raw), "download:") {
		t.Fatal("download settings were not persisted")
	}
	got, err := LoadSubscriptionConfig()
	if err != nil {
		t.Fatal(err)
	}
	d := got.Profiles[0].Download
	if d.Timeout != want.Timeout || d.UserAgent != want.UserAgent || d.DownloadVia != want.DownloadVia || d.SkipCert != want.SkipCert || d.Headers["x-test"] != "value" {
		t.Fatalf("settings lost: %+v", d)
	}
}
