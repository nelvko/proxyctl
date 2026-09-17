package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/nelvko/proxyctl/internal/config"
)

func prepareSubscriptionCommands(t *testing.T) (string, string) {
	t.Helper()
	isolatedConfig(t)
	binary := filepath.Join(t.TempDir(), "kernel")
	if err := os.WriteFile(binary, []byte("#!/bin/sh\nexit 0\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	runtimeFile := filepath.Join(t.TempDir(), "config.yaml")
	source := filepath.Join(t.TempDir(), "source.yaml")
	for _, p := range []string{runtimeFile, source} {
		if err := os.WriteFile(p, []byte("proxies: []\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := config.Save(&config.Config{Use: "mihomo", Kernels: []config.KernelConfig{{Name: "mihomo", Bin: binary, ConfigFile: runtimeFile}}}); err != nil {
		t.Fatal(err)
	}
	return source, runtimeFile
}

func TestMetadataCommandsAndRedactedInspection(t *testing.T) {
	isolatedConfig(t)
	cfg := &config.SubscriptionConfig{Use: "work", Profiles: []config.Profile{{Name: "work", URL: "https://user:password@example.com/path-secret?token=query-secret", File: "/unused", Download: config.DownloadConfig{DownloadVia: "kernel", UserAgent: "old"}}}}
	if err := config.SaveSubscriptionConfig(cfg); err != nil {
		t.Fatal(err)
	}
	out, _, err := runCommand("sub", "modify", "work", "--name", "office", "--user-agent=", "--download-via=env")
	if err != nil || !strings.Contains(out, "office") {
		t.Fatalf("modify: %s %v", out, err)
	}
	saved, _ := config.LoadSubscriptionConfig()
	if saved.Use != "office" || saved.Profiles[0].Download.DownloadVia != "env" || saved.Profiles[0].Download.UserAgent != "" {
		t.Fatal("modify did not persist clear/false/selected rename")
	}
	for _, args := range [][]string{{"sub", "show"}, {"sub", "list"}} {
		out, _, err := runCommand(args...)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(out, "secret") || strings.Contains(out, "password") || !strings.Contains(out, "example.com") {
			t.Fatalf("inspection output: %s", out)
		}
	}
	out, _, err = runCommand("sub", "show", "--show-url")
	if err != nil || !strings.Contains(out, "query-secret") {
		t.Fatal("explicit full URL unavailable")
	}
	for _, args := range [][]string{{"sub", "modify", "office"}, {"sub", "modify", "office", "--name="}, {"sub", "modify", "office", "--timeout=-1s"}, {"sub", "modify", "office", "--url=http://"}} {
		if _, _, err := runCommand(args...); err == nil {
			t.Fatalf("accepted invalid metadata: %v", args)
		}
	}
}

func TestUpdateScopeIsExplicit(t *testing.T) {
	source, _ := prepareSubscriptionCommands(t)
	for _, name := range []string{"one", "two"} {
		if _, _, err := runCommand("sub", "add", "file://"+source, "--name", name); err != nil {
			t.Fatal(err)
		}
	}
	if _, _, err := runCommand("sub", "update"); err == nil || !strings.Contains(err.Error(), "--all") {
		t.Fatalf("missing selection error: %v", err)
	}
	cfg, _ := config.LoadSubscriptionConfig()
	cfg.Use = "one"
	if err := config.SaveSubscriptionConfig(cfg); err != nil {
		t.Fatal(err)
	}
	out, _, err := runCommand("sub", "update")
	if err != nil || !strings.Contains(out, `"one": unchanged`) || strings.Contains(out, `"two"`) {
		t.Fatalf("default scope: %q %v", out, err)
	}
	out, _, err = runCommand("sub", "update", "two")
	if err != nil || !strings.Contains(out, `"two": unchanged`) || strings.Contains(out, `"one"`) {
		t.Fatalf("named scope: %q %v", out, err)
	}
	out, _, err = runCommand("sub", "update", "--all")
	if err != nil || !strings.Contains(out, `"one"`) || !strings.Contains(out, `"two"`) {
		t.Fatalf("all scope: %q %v", out, err)
	}
	if _, _, err := runCommand("sub", "update", "one", "--all"); err == nil {
		t.Fatal("accepted ambiguous update scope")
	}
}

func TestConcurrentAddsDoNotLoseRegistrations(t *testing.T) {
	source, _ := prepareSubscriptionCommands(t)
	var wg sync.WaitGroup
	errs := make(chan error, 2)
	for _, name := range []string{"one", "two"} {
		wg.Go(func() { _, _, err := runCommand("sub", "add", "file://"+source, "--name", name); errs <- err })
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	cfg, err := config.LoadSubscriptionConfig()
	if err != nil || len(cfg.Profiles) != 2 {
		t.Fatalf("lost registration: %+v %v", cfg, err)
	}
}

func TestCompletionReadsOnlySavedNames(t *testing.T) {
	isolatedConfig(t)
	if err := config.SaveSubscriptionConfig(&config.SubscriptionConfig{Profiles: []config.Profile{{Name: "work profile"}}}); err != nil {
		t.Fatal(err)
	}
	if err := config.Save(&config.Config{Kernels: []config.KernelConfig{{Name: "mihomo", Bin: "/missing"}}}); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"__complete", "sub", "use", "w"}, {"__complete", "sub", "modify", "w"}, {"__complete", "use", "mi"}} {
		out, _, err := runCommand(args...)
		if err != nil || !(strings.Contains(out, "work profile") || strings.Contains(out, "mihomo")) {
			t.Fatalf("completion: %q %v", out, err)
		}
	}
}

func TestDownloadMetadataFlagsAndClearing(t *testing.T) {
	source, _ := prepareSubscriptionCommands(t)
	if _, _, err := runCommand("sub", "add", "file://"+source, "--name", "office", "--download-via=kernel", "--insecure", "--header", "X-Token: original", "--timeout=30s"); err != nil {
		t.Fatal(err)
	}
	if _, _, err := runCommand("sub", "modify", "office", "--download-via=direct", "--insecure=false", "--header", "x-token: replaced", "--timeout=0"); err != nil {
		t.Fatal(err)
	}
	cfg, _ := config.LoadSubscriptionConfig()
	u := cfg.Profiles[0].Download
	if u.DownloadVia != "direct" || u.SkipCert || u.Timeout != 0 || len(u.Headers) != 1 || u.Headers["x-token"] != "replaced" {
		t.Fatalf("metadata options: %+v", u)
	}
	if _, _, err := runCommand("sub", "modify", "office", "--remove-header", "X-TOKEN"); err != nil {
		t.Fatal(err)
	}
	cfg, _ = config.LoadSubscriptionConfig()
	if len(cfg.Profiles[0].Download.Headers) != 0 {
		t.Fatal("case-insensitive removal failed")
	}
	for _, flags := range [][]string{
		{"--download-via=other"}, {"--download-via="}, {"--via-kernel"},
		{"--header", "invalid"}, {"--header", "X: one", "--header", "x: two"},
		{"--header", "User-Agent: one", "--user-agent=two"},
		{"--header", "X: one", "--remove-header", "x"},
	} {
		args := append([]string{"sub", "modify", "office"}, flags...)
		if _, _, err := runCommand(args...); err == nil {
			t.Fatalf("accepted invalid flags: %v", flags)
		}
	}
	out, _, err := runCommand("__complete", "sub", "update", "--download-via", "d")
	if err != nil || !strings.Contains(out, "direct") {
		t.Fatalf("route completion: %s %v", out, err)
	}
}
