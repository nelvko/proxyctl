package kernel

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/nelvko/proxyctl/internal/config"
)

// RemoveKernelFiles deletes the kernel's on-disk footprint: its config
// directory (config.yaml, geodata, runtime cache — the whole kernel-owned
// directory, so files the kernel or the user dropped beside the config go
// too) and the binary. Directories are only removed wholesale when they
// live under proxyctl's own roots — the config is user-editable, so a
// hand-written Bin like /usr/local/bin/mihomo must not nuke /usr/local/bin.
// A missing file is not an error.
func RemoveKernelFiles(kcfg config.KernelConfig) error {
	if underManagedRoot(kcfg.ConfigDir) {
		if err := os.RemoveAll(kcfg.ConfigDir); err != nil {
			return fmt.Errorf("remove %s: %w", kcfg.ConfigDir, err)
		}
	}
	binDir := filepath.Dir(kcfg.Bin)
	if underManagedRoot(binDir) {
		if err := os.RemoveAll(binDir); err != nil {
			return fmt.Errorf("remove %s: %w", binDir, err)
		}
		return nil
	}
	if err := os.Remove(kcfg.Bin); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("remove %s: %w", kcfg.Bin, err)
	}
	return nil
}

func underManagedRoot(path string) bool {
	for _, root := range managedRoots() {
		rel, err := filepath.Rel(root, path)
		if err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return true
		}
	}
	return false
}

func managedRoots() []string {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}
	roots := []string{
		filepath.Join(home, ".local", "share", config.AppName),
	}
	// The config root DefaultConfig uses (os.UserConfigDir — ~/.config on
	// Linux, ~/Library/Application Support on macOS).
	if cfgDir, err := os.UserConfigDir(); err == nil {
		roots = append(roots, filepath.Join(cfgDir, config.AppName))
	}
	return roots
}
