package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func useTestDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	prev := testDir
	testDir = dir
	t.Cleanup(func() { testDir = prev })
	return dir
}

func writeTestFile(t *testing.T, name, content string) {
	t.Helper()
	path := filepath.Join(useTestDir(t), name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
}

func TestConfigRoundTrip(t *testing.T) {
	dir := useTestDir(t)
	cfg := &Config{
		Use: "mihomo",
		Kernels: []KernelConfig{{
			Name:       "mihomo",
			Bin:        "/b/mihomo",
			ConfigDir:  "/c",
			ConfigFile: "/c/config.yaml",
		}},
	}
	if err := Save(cfg); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	got, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if got.Use != "mihomo" || len(got.Kernels) != 1 || got.Kernels[0].Bin != "/b/mihomo" {
		t.Fatalf("round-trip mismatch: %+v", got)
	}
	if got.SelectedKernel() == nil || got.KernelByName("mihomo") == nil {
		t.Fatalf("lookups failed after round-trip: %+v", got)
	}

	// Saves are atomic: no temp litter left behind.
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("dir has %d entries, want 1 (no temp litter)", len(entries))
	}
}

func TestLoadMissing(t *testing.T) {
	useTestDir(t)
	got, err := Load()
	if err != nil {
		t.Fatalf("Load() on missing file: %v", err)
	}
	if got.SelectedKernel() != nil {
		t.Fatalf("SelectedKernel() = %+v, want nil", got.SelectedKernel())
	}
}

func TestLoadLegacy(t *testing.T) {
	writeTestFile(t, "config.yaml", "use: mihomo\nkernel:\n  name: mihomo\n")
	_, err := Load()
	if err == nil || !strings.Contains(err.Error(), "single-kernel") {
		t.Fatalf("Load(legacy) error = %v, want single-kernel format error", err)
	}
}

func TestSubConfigRoundTrip(t *testing.T) {
	useTestDir(t)
	cfg := &SubscriptionConfig{
		Use: "a",
		Profiles: []Profile{{
			Name: "a",
			URL:  "https://example.com/sub?token=x",
			File: "/p/a.yaml",
			Download: DownloadConfig{
				Timeout: 5 * time.Minute,
				Headers: map[string]string{"Authorization": "Bearer t"},
			},
		}},
	}
	if err := SaveSubscriptionConfig(cfg); err != nil {
		t.Fatalf("SaveSubscriptionConfig() error = %v", err)
	}

	got, err := LoadSubscriptionConfig()
	if err != nil {
		t.Fatalf("LoadSubscriptionConfig() error = %v", err)
	}
	if got.Use != "a" || len(got.Profiles) != 1 {
		t.Fatalf("round-trip mismatch: %+v", got)
	}
	p := got.Profiles[0]
	if p.Download.Timeout != 5*time.Minute {
		t.Fatalf("durations lost in round-trip: %+v", p.Download)
	}
	// viper lowercases map keys; HTTP headers are case-insensitive so the
	// value survives (this lossiness disappears once viper is dropped).
	if p.Download.Headers["authorization"] != "Bearer t" {
		t.Fatalf("headers lost in round-trip: %+v", p.Download)
	}
}

func TestLoadSubConfigEmpty(t *testing.T) {
	writeTestFile(t, "subscriptions.yaml", "")
	got, err := LoadSubscriptionConfig()
	if err != nil {
		t.Fatalf("LoadSubscriptionConfig(empty) error = %v", err)
	}
	if got.Use != "" || len(got.Profiles) != 0 {
		t.Fatalf("empty file should load as zero value: %+v", got)
	}
}
