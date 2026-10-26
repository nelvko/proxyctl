package kernel

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/nelvko/proxyctl/internal/config"
)

func TestRemovalOwnership(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	cfg, err := config.DefaultKernelConfig("mihomo")
	if err != nil {
		t.Fatal(err)
	}
	root, _ := config.ConfigDir()
	profiles := filepath.Join(root, "profiles")
	overrides := filepath.Join(root, "overrides")
	for _, dir := range []string{cfg.ConfigDir, filepath.Dir(cfg.Bin), profiles, overrides} {
		if err := os.MkdirAll(dir, 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "keep"), []byte("data"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	for _, path := range []string{root, profiles, overrides, filepath.Join(root, "clash")} {
		bad := cfg
		bad.ConfigDir = path
		if err := RemoveKernelFiles(bad); err == nil {
			t.Fatalf("accepted unowned directory %s", path)
		}
		if _, err := os.Stat(filepath.Join(cfg.ConfigDir, "keep")); err != nil {
			t.Fatalf("partially deleted before validation: %v", err)
		}
	}
	reserved := cfg
	reserved.Name, reserved.ConfigDir = "overrides", overrides
	if err := ValidateRemoval(reserved); err == nil {
		t.Fatal("accepted a kernel claiming the shared override directory")
	}
	if err := RemoveKernelFiles(cfg); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(profiles, "keep")); err != nil {
		t.Fatal("deleted subscriptions", err)
	}
	if _, err := os.Stat(filepath.Join(overrides, "keep")); err != nil {
		t.Fatal("deleted global overrides", err)
	}
	if _, err := os.Stat(cfg.ConfigDir); !os.IsNotExist(err) {
		t.Fatal("kernel directory remains", err)
	}
	if err := RemoveKernelFiles(cfg); err != nil {
		t.Fatal("missing files must be idempotent", err)
	}
}

func TestRemovalRejectsSymlinkedParent(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	cfg, err := config.DefaultKernelConfig("mihomo")
	if err != nil {
		t.Fatal(err)
	}
	outside := t.TempDir()
	data, _ := config.DataDir()
	if err := os.MkdirAll(data, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(data, "kernels")); err != nil {
		t.Fatal(err)
	}
	if err := RemoveKernelFiles(cfg); err == nil {
		t.Fatal("accepted redirected kernel parent")
	}
}
