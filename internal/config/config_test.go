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

func TestAppConfigRoundTrip(t *testing.T) {
	dir := useTestDir(t)
	cfg := &AppConfig{
		Use: "mihomo",
		Kernels: []KernelConfig{{
			Name:       "mihomo",
			Bin:        "/b/mihomo",
			ConfigDir:  "/c",
			ConfigFile: "/c/config.yaml",
		}},
	}
	if err := SaveAppConfig(cfg); err != nil {
		t.Fatalf("SaveAppConfig() error = %v", err)
	}

	got, err := LoadAppConfig()
	if err != nil {
		t.Fatalf("LoadAppConfig() error = %v", err)
	}
	if got.Use != "mihomo" || len(got.Kernels) != 1 || got.Kernels[0].Bin != "/b/mihomo" {
		t.Fatalf("round-trip mismatch: %+v", got)
	}
	if got.ActiveKernel() == nil || got.KernelByName("mihomo") == nil {
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

func TestLoadAppConfigMissing(t *testing.T) {
	useTestDir(t)
	got, err := LoadAppConfig()
	if err != nil {
		t.Fatalf("LoadAppConfig() on missing file: %v", err)
	}
	if got.ActiveKernel() != nil {
		t.Fatalf("ActiveKernel() = %+v, want nil", got.ActiveKernel())
	}
}

func TestLoadAppConfigLegacy(t *testing.T) {
	writeTestFile(t, "config.yaml", "use: mihomo\nkernel:\n  name: mihomo\n")
	_, err := LoadAppConfig()
	if err == nil || !strings.Contains(err.Error(), "single-kernel") {
		t.Fatalf("LoadAppConfig(legacy) error = %v, want single-kernel format error", err)
	}
}

func TestSubConfigRoundTrip(t *testing.T) {
	useTestDir(t)
	cfg := &SubConfig{
		Use: "a",
		Profiles: []Profile{{
			Name: "a",
			URL:  "https://example.com/sub?token=x",
			File: "/p/a.yaml",
			Update: UpdateConfig{
				Timeout:  5 * time.Minute,
				Interval: 24 * time.Hour,
				Headers:  map[string]string{"Authorization": "Bearer t"},
			},
		}},
	}
	if err := SaveSubConfig(cfg); err != nil {
		t.Fatalf("SaveSubConfig() error = %v", err)
	}

	got, err := LoadSubConfig()
	if err != nil {
		t.Fatalf("LoadSubConfig() error = %v", err)
	}
	if got.Use != "a" || len(got.Profiles) != 1 {
		t.Fatalf("round-trip mismatch: %+v", got)
	}
	p := got.Profiles[0]
	if p.Update.Timeout != 5*time.Minute || p.Update.Interval != 24*time.Hour {
		t.Fatalf("durations lost in round-trip: %+v", p.Update)
	}
	// viper lowercases map keys; HTTP headers are case-insensitive so the
	// value survives (this lossiness disappears once viper is dropped).
	if p.Update.Headers["authorization"] != "Bearer t" {
		t.Fatalf("headers lost in round-trip: %+v", p.Update)
	}
}

func TestLoadSubConfigEmpty(t *testing.T) {
	writeTestFile(t, "profiles.yaml", "")
	got, err := LoadSubConfig()
	if err != nil {
		t.Fatalf("LoadSubConfig(empty) error = %v", err)
	}
	if got.Use != "" || len(got.Profiles) != 0 {
		t.Fatalf("empty file should load as zero value: %+v", got)
	}
}
