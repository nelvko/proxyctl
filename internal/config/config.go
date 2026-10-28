// Package config owns persisted settings, state paths and atomic file operations.
// Application workflows and runtime service assembly live in package app.
package config

import (
	"errors"
	"fmt"
	"os"

	"github.com/spf13/viper"
)

// ErrNoKernel means no usable kernel is configured yet.
var ErrNoKernel = errors.New("no kernel installed")

const (
	AppName = "proxyctl"
)

// Config is exactly the in-memory image of config.yaml: every field is
// persisted, and nothing runtime-derived belongs here. It holds proxyctl's
// own settings — which kernel is selected, what is installed, the download
// mirror. Per-subscription settings live in SubscriptionConfig; the
// per-invocation runtime assembly is app.App.
type Config struct {
	// Use is the name of the selected kernel.
	Use string `mapstructure:"use"`
	// Kernels holds the installed kernels.
	Kernels []KernelConfig `mapstructure:"kernels"`
	// Mirror is a GitHub mirror prefix (or comma-separated list) used to
	// accelerate kernel downloads; the GH_PROXY env variable overrides it.
	Mirror string `mapstructure:"mirror"`
	// Theme contains TUI appearance preferences; absent settings keep auto mode.
	Theme ThemeConfig `mapstructure:"theme" yaml:"theme,omitempty"`
	// Language is a presentation preference; empty follows the system locale.
	Language string `mapstructure:"language" yaml:"language,omitempty"`
}

// KernelConfig is one installed kernel's on-disk entry.
type KernelConfig struct {
	Name       string `mapstructure:"name"`
	Type       string `mapstructure:"type"`
	Bin        string `mapstructure:"bin"`
	ConfigDir  string `mapstructure:"configdir"`
	ConfigFile string `mapstructure:"configfile"`
	Source     string `mapstructure:"source"`
}

// SelectedKernel returns the config of the selected kernel, or nil.
func (c *Config) SelectedKernel() *KernelConfig {
	if c == nil {
		return nil
	}
	return c.KernelByName(c.Use)
}

// KernelByName returns the config of the named kernel, or nil.
func (c *Config) KernelByName(name string) *KernelConfig {
	if c == nil {
		return nil
	}
	for i := range c.Kernels {
		if c.Kernels[i].Name == name {
			return &c.Kernels[i]
		}
	}
	return nil
}

// SetKernel adds or replaces a kernel entry.
func (c *Config) SetKernel(kcfg KernelConfig) {
	for i := range c.Kernels {
		if c.Kernels[i].Name == kcfg.Name {
			c.Kernels[i] = kcfg
			return
		}
	}
	c.Kernels = append(c.Kernels, kcfg)
}

// RemoveKernel drops a kernel entry.
func (c *Config) RemoveKernel(name string) {
	for i := range c.Kernels {
		if c.Kernels[i].Name == name {
			c.Kernels = append(c.Kernels[:i], c.Kernels[i+1:]...)
			return
		}
	}
}

func Load() (*Config, error) {
	appV, path, err := readConfig()
	if err != nil {
		return nil, err
	}
	if appV == nil {
		return &Config{}, nil
	}
	// Decode appearance separately. A malformed optional theme must not make
	// kernel and subscription commands unusable; the TUI exposes its warning
	// through LoadTheme. Preserve errors from all other settings.
	settings := appV.AllSettings()
	delete(settings, "theme")
	delete(settings, "language")
	coreV := viper.New()
	if err := coreV.MergeConfigMap(settings); err != nil {
		return nil, err
	}
	cfg := &Config{}
	if err := coreV.Unmarshal(cfg); err != nil {
		return nil, err
	}
	cfg.Theme, _ = decodeTheme(appV.Get("theme"))
	cfg.Language, _ = decodeLanguage(appV.Get("language"))
	// The config predates the multi-kernel format; refuse loudly instead
	// of silently seeing zero kernels.
	if len(cfg.Kernels) == 0 && appV.IsSet("kernel") {
		return nil, fmt.Errorf("%s uses the old single-kernel format, remove it and run `proxyctl install`", path)
	}
	return cfg, nil
}

func readConfig() (*viper.Viper, string, error) {
	path, err := appConfigFile()
	if err != nil {
		return nil, "", err
	}
	// No config file yet means no kernel installed — not an error.
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		return nil, path, nil
	}
	appV := viper.New()
	appV.SetConfigFile(path)
	if err := appV.ReadInConfig(); err != nil {
		return nil, path, err
	}
	return appV, path, nil
}

func Save(cfg *Config) error {
	if cfg == nil {
		return errors.New("app config is nil")
	}
	path, err := appConfigFile()
	if err != nil {
		return err
	}
	return saveYAMLAtomic(path, cfg)
}
