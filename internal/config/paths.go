package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// testDir lets in-package tests redirect the config root.
var testDir string

// dir resolves the proxyctl config root, failing loudly when the user
// config dir cannot be resolved (no HOME) instead of silently writing to
// a relative path.
func dir() (string, error) {
	if testDir != "" {
		return testDir, nil
	}
	d, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("resolve config dir (is HOME set?): %w", err)
	}
	if !filepath.IsAbs(d) {
		return "", fmt.Errorf("config directory must be absolute: %q", d)
	}
	return filepath.Join(d, AppName), nil
}

func appConfigFile() (string, error) {
	d, err := dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(d, "config.yaml"), nil
}

func subscriptionConfigFile() (string, error) {
	d, err := dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(d, "subscriptions.yaml"), nil
}

// ProfilesDir returns the directory holding subscription profile files.
func ProfilesDir() (string, error) {
	d, err := dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(d, "profiles"), nil
}

// DiagnosticsDir contains explicitly retained invalid downloads.
func DiagnosticsDir() (string, error) {
	d, err := dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(d, "diagnostics"), nil
}

// DataDir is the root of installed binaries. Path resolution never falls back
// to the working directory when the user's home is unavailable.
func DataDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home directory: %w", err)
	}
	if !filepath.IsAbs(home) {
		return "", fmt.Errorf("home directory must be absolute: %q", home)
	}
	return filepath.Join(home, ".local", "share", AppName), nil
}

// ConfigDir returns the shared settings root, not a kernel-owned directory.
func ConfigDir() (string, error) { return dir() }

func DefaultKernelConfig(name string) (KernelConfig, error) {
	if name == "" || name == "." || name == ".." || filepath.Base(name) != name || strings.ContainsAny(name, "\\/") || strings.ContainsFunc(name, func(r rune) bool { return !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '-') }) {
		return KernelConfig{}, fmt.Errorf("invalid kernel name %q", name)
	}
	switch name {
	case "profiles", "diagnostics", "kernels":
		return KernelConfig{}, fmt.Errorf("reserved kernel name %q", name)
	}
	root, err := dir()
	if err != nil {
		return KernelConfig{}, err
	}
	data, err := DataDir()
	if err != nil {
		return KernelConfig{}, err
	}
	cfgDir := filepath.Join(root, name)
	return KernelConfig{Name: name, Bin: filepath.Join(data, "kernels", name, name), ConfigDir: cfgDir, ConfigFile: filepath.Join(cfgDir, "config.yaml")}, nil
}
