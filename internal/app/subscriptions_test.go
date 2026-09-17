package app

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/nelvko/proxyctl/internal/config"
)

func TestRemoveProfileLoadsOnlyRequiredState(t *testing.T) {
	for _, mode := range []string{"unselected-broken-config", "selected-without-force", "selected-force-no-kernel", "selected-force-broken-config"} {
		t.Run(mode, func(t *testing.T) {
			t.Setenv("HOME", t.TempDir())
			t.Setenv("XDG_CONFIG_HOME", t.TempDir())
			t.Setenv("AppData", t.TempDir())
			file := filepath.Join(t.TempDir(), "profile.yaml")
			if err := os.WriteFile(file, []byte("payload"), 0600); err != nil {
				t.Fatal(err)
			}
			selected := mode != "unselected-broken-config"
			cfg := &config.SubscriptionConfig{Profiles: []config.Profile{{Name: "work", File: file}}}
			if selected {
				cfg.Use = "work"
			}
			if err := config.SaveSubscriptionConfig(cfg); err != nil {
				t.Fatal(err)
			}
			if mode != "selected-force-no-kernel" {
				dir, err := config.ConfigDir()
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(dir, "config.yaml"), []byte("[invalid"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			force := mode == "selected-force-no-kernel" || mode == "selected-force-broken-config"
			err := WithStateLock(t.Context(), func() error { return RemoveProfile(t.Context(), "work", force, nil) })
			shouldFail := mode == "selected-without-force" || mode == "selected-force-broken-config"
			if (err != nil) != shouldFail {
				t.Fatalf("unexpected result: %v", err)
			}
			got, e := config.LoadSubscriptionConfig()
			if e != nil {
				t.Fatal(e)
			}
			_, fileErr := os.Stat(file)
			if shouldFail {
				if len(got.Profiles) != 1 || got.Use != "work" || fileErr != nil {
					t.Fatal("failed removal changed state")
				}
			} else if len(got.Profiles) != 0 || got.Use != "" || !os.IsNotExist(fileErr) {
				t.Fatal("removal left state or cache behind")
			}
		})
	}
}
