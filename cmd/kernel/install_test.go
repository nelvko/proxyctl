package kernel

import (
	"os"
	"path/filepath"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/huh/v2"
	"github.com/nelvko/proxyctl/internal/config"
)

func TestLocalKernelConfig(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("AppData", t.TempDir())
	binary := filepath.Join(t.TempDir(), "kernel")
	if err := os.WriteFile(binary, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name, kind, wantKind, file string
	}{
		{"mycore", "", "custom", "config.yaml"},
		{"mihomo", "", "mihomo", "config.yaml"},
		{"sing-box", "", "sing-box", "config.json"},
		{"myfork", "mihomo", "mihomo", "config.yaml"},
		{"myfork", "sing-box", "sing-box", "config.json"},
		{"mihomo", "custom", "custom", "config.yaml"},
	} {
		t.Run(test.name+"/"+test.kind, func(t *testing.T) {
			cfg, err := localKernelConfig(test.name, binary, test.kind)
			if err != nil {
				t.Fatal(err)
			}
			if cfg.Name != test.name || cfg.Source != binary || cfg.Bin == binary || cfg.Type != test.wantKind || filepath.Base(cfg.ConfigFile) != test.file {
				t.Fatalf("wrong installation settings: %+v", cfg)
			}
		})
	}
	for _, test := range []struct{ name, path, kind string }{
		{"../escape", binary, "custom"},
		{"profiles", binary, "custom"},
		{"mycore", "", "custom"},
		{"mycore", t.TempDir(), "custom"},
		{"mycore", binary + ".missing", "custom"},
		{"mycore", binary, "unsupported"},
	} {
		if _, err := localKernelConfig(test.name, test.path, test.kind); err == nil {
			t.Fatalf("accepted invalid installation settings: %+v", test)
		}
	}
}

func TestInstallSelectionCancellation(t *testing.T) {
	for _, code := range []rune{'q', tea.KeyEscape} {
		var name string
		form := newInstallForm(&config.Config{}, &name)
		form.Init()
		form.Update(tea.KeyPressMsg{Code: code})
		if form.State != huh.StateAborted {
			t.Fatalf("key %q did not cancel the selection", code)
		}
	}
}
