package config

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestThemeConfigBackwardCompatible(t *testing.T) {
	dir := useTestDir(t)
	cfg := &Config{Use: "mihomo", Mirror: "https://mirror.example", Kernels: []KernelConfig{{Name: "mihomo", Type: "mihomo", Bin: "/bin/mihomo"}}}
	if err := Save(cfg); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(dir, "config.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "theme:") {
		t.Fatalf("zero theme unexpectedly changed old config: %s", data)
	}
	loaded, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(*cfg, *loaded) {
		t.Fatalf("old config changed: got %+v, want %+v", loaded, cfg)
	}
}

func TestSaveThemePreservesLatestApplicationConfig(t *testing.T) {
	useTestDir(t)
	// SaveTheme must load this latest state, not a stale dashboard snapshot.
	latest := &Config{Use: "sing-box", Mirror: "https://new-mirror.example", Kernels: []KernelConfig{{Name: "sing-box", Type: "sing-box", Bin: "/new/bin"}}}
	if err := Save(latest); err != nil {
		t.Fatal(err)
	}
	theme := ThemeConfig{Name: "custom", Mode: "dark", Base: "nord", Colors: map[string]string{"primary": "#ABCDEF", "surface": "#123456"}}
	if err := SaveTheme(t.Context(), theme); err != nil {
		t.Fatal(err)
	}
	got, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if got.Use != latest.Use || got.Mirror != latest.Mirror || !reflect.DeepEqual(got.Kernels, latest.Kernels) || !reflect.DeepEqual(got.Theme, theme) {
		t.Fatalf("appearance save lost settings: %+v", got)
	}
	if err := SaveTheme(t.Context(), ThemeConfig{}); err != nil {
		t.Fatal(err)
	}
	reset, err := Load()
	if err != nil || !reflect.DeepEqual(reset.Theme, ThemeConfig{}) || reset.Use != latest.Use {
		t.Fatalf("reset = %+v, err=%v", reset, err)
	}
}

func TestSaveThemeCancelledDoesNotWrite(t *testing.T) {
	dir := useTestDir(t)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if err := SaveTheme(ctx, ThemeConfig{Name: "dracula"}); err != context.Canceled {
		t.Fatalf("save with cancelled context = %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "config.yaml")); !os.IsNotExist(err) {
		t.Fatalf("cancelled operation wrote config: %v", err)
	}
}

func TestInvalidThemeColorDoesNotPreventConfigLoad(t *testing.T) {
	writeTestFile(t, "config.yaml", "use: mihomo\nkernels:\n  - name: mihomo\n    type: mihomo\ntheme:\n  name: custom\n  mode: nonsense\n  colors:\n    primary: invalid\n")
	got, err := Load()
	if err != nil || got.SelectedKernel() == nil || got.Theme.Colors["primary"] != "invalid" {
		t.Fatalf("appearance preference blocked kernel settings: %+v, %v", got, err)
	}
}

func TestMalformedThemeDoesNotBlockApplicationSettings(t *testing.T) {
	for _, theme := range []string{
		"theme: wrong-shape\n",
		"theme: [default, nord]\n",
		"theme:\n  name: [default]\n  mode: true\n  base: {}\n",
		"theme:\n  name: custom\n  colors: [red, blue]\n",
		"theme:\n  name: custom\n  colors:\n    primary: 123\n    border: {nested: color}\n    surface: '#123456'\n",
	} {
		t.Run(theme, func(t *testing.T) {
			writeTestFile(t, "config.yaml", "use: mihomo\nmirror: https://mirror.example\nkernels:\n  - name: mihomo\n    type: mihomo\n"+theme)
			cfg, err := Load()
			if err != nil || cfg.SelectedKernel() == nil || cfg.Mirror != "https://mirror.example" {
				t.Fatalf("malformed appearance blocked application settings: %+v, %v", cfg, err)
			}
			appearance, warning := LoadTheme()
			if warning == nil || !reflect.DeepEqual(appearance, cfg.Theme) {
				t.Fatalf("missing isolated TUI warning: appearance=%+v, warning=%v", appearance, warning)
			}
			if strings.Contains(theme, "surface:") && appearance.Colors["surface"] != "#123456" {
				t.Fatal("valid color was discarded with invalid siblings")
			}
		})
	}
}

func TestMalformedApplicationSettingsStillFail(t *testing.T) {
	for _, content := range []string{"kernels: wrong-shape\ntheme: wrong-shape\n", "kernels: [\ntheme: wrong-shape\n"} {
		t.Run(content, func(t *testing.T) {
			writeTestFile(t, "config.yaml", content)
			if _, err := Load(); err == nil {
				t.Fatal("malformed application config was silently accepted")
			}
		})
	}
}
