package config

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestLanguageRoundTripPreservesCurrentSettings(t *testing.T) {
	dir := useTestDir(t)
	theme := ThemeConfig{Name: "custom", Base: "nord", Colors: map[string]string{"primary": "#ABCDEF"}}
	cfg := &Config{Use: "mihomo", Kernels: []KernelConfig{{Name: "mihomo", Type: "mihomo"}}, Mirror: "https://mirror.example", Theme: theme}
	if err := Save(cfg); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(filepath.Join(dir, "config.yaml"))
	if strings.Contains(string(data), "language:") {
		t.Fatal("empty preference changed legacy YAML")
	}
	if err := SaveLanguage(t.Context(), "en"); err != nil {
		t.Fatal(err)
	}
	got, err := Load()
	if err != nil || got.Language != "en" || got.Use != cfg.Use || got.Mirror != cfg.Mirror || !reflect.DeepEqual(got.Theme, theme) || !reflect.DeepEqual(got.Kernels, cfg.Kernels) {
		t.Fatalf("language save lost current config: %+v,%v", got, err)
	}
	if err := SaveTheme(t.Context(), ThemeConfig{Name: "dracula"}); err != nil {
		t.Fatal(err)
	}
	language, err := LoadLanguage()
	if err != nil || language != "en" {
		t.Fatalf("theme save lost language=%q,%v", language, err)
	}
	if err := SaveLanguage(t.Context(), ""); err != nil {
		t.Fatal(err)
	}
	language, err = LoadLanguage()
	if err != nil || language != "" {
		t.Fatalf("reset=%q,%v", language, err)
	}
}

func TestMalformedLanguageDoesNotBlockCoreConfig(t *testing.T) {
	writeTestFile(t, "config.yaml", "use: mihomo\nkernels:\n  - name: mihomo\n    type: mihomo\nlanguage: [en, zh-CN]\n")
	cfg, err := Load()
	if err != nil || cfg.SelectedKernel() == nil || cfg.Language != "" {
		t.Fatalf("optional language blocked core: %+v,%v", cfg, err)
	}
	if language, warning := LoadLanguage(); language != "" || warning == nil {
		t.Fatal("malformed preference did not fall back and warn")
	}
}

func TestLanguageLoadIndependentOfCoreShape(t *testing.T) {
	writeTestFile(t, "config.yaml", "kernels: bad-shape\nlanguage: en\n")
	if _, err := Load(); err == nil {
		t.Fatal("core config shape error disappeared")
	}
	if language, err := LoadLanguage(); err != nil || language != "en" {
		t.Fatalf("language-only load blocked: %q,%v", language, err)
	}
}

func TestCancelledLanguageSaveDoesNotWrite(t *testing.T) {
	dir := useTestDir(t)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if err := SaveLanguage(ctx, "en"); err != context.Canceled {
		t.Fatalf("cancelled=%v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "config.yaml")); !os.IsNotExist(err) {
		t.Fatal("cancelled save wrote configuration")
	}
}
