package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/viper"
	"go.yaml.in/yaml/v3"
)

// ErrNoKernel means no usable kernel is configured yet.
var ErrNoKernel = errors.New("no kernel installed")

const (
	AppName = "proxyctl"
)

type AppConfig struct {
	// Use is the name of the active kernel.
	Use string `mapstructure:"use"`
	// Kernels holds the installed kernels.
	Kernels []KernelConfig `mapstructure:"kernels"`
}

type KernelConfig struct {
	Name       string `mapstructure:"name"`
	Bin        string `mapstructure:"bin"`
	ConfigDir  string `mapstructure:"configdir"`
	ConfigFile string `mapstructure:"configfile"`
}

// ActiveKernel returns the config of the active kernel, or nil.
func (c *AppConfig) ActiveKernel() *KernelConfig {
	if c == nil {
		return nil
	}
	return c.KernelByName(c.Use)
}

// KernelByName returns the config of the named kernel, or nil.
func (c *AppConfig) KernelByName(name string) *KernelConfig {
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
func (c *AppConfig) SetKernel(kcfg KernelConfig) {
	for i := range c.Kernels {
		if c.Kernels[i].Name == kcfg.Name {
			c.Kernels[i] = kcfg
			return
		}
	}
	c.Kernels = append(c.Kernels, kcfg)
}

// RemoveKernel drops a kernel entry.
func (c *AppConfig) RemoveKernel(name string) {
	for i := range c.Kernels {
		if c.Kernels[i].Name == name {
			c.Kernels = append(c.Kernels[:i], c.Kernels[i+1:]...)
			return
		}
	}
}

func LoadAppConfig() (*AppConfig, error) {
	path, err := appConfigFile()
	if err != nil {
		return nil, err
	}
	// No config file yet means no kernel installed — not an error.
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		return &AppConfig{}, nil
	}
	appV := viper.New()
	appV.SetConfigFile(path)
	if err := appV.ReadInConfig(); err != nil {
		return nil, err
	}
	cfg := &AppConfig{}
	if err := appV.Unmarshal(cfg); err != nil {
		return nil, err
	}
	// The config predates the multi-kernel format; refuse loudly instead
	// of silently seeing zero kernels.
	if len(cfg.Kernels) == 0 && appV.IsSet("kernel") {
		return nil, fmt.Errorf("%s uses the old single-kernel format, remove it and run `proxyctl kernel install`", path)
	}
	return cfg, nil
}

func SaveAppConfig(cfg *AppConfig) error {
	if cfg == nil {
		return errors.New("app config is nil")
	}
	path, err := appConfigFile()
	if err != nil {
		return err
	}
	return saveYAMLAtomic(path, cfg)
}

// saveYAMLAtomic writes v to a sibling temp file (0600) and renames it
// into place, so a crash never truncates the config, and encode/flush
// errors surface instead of being swallowed by a deferred Close.
func saveYAMLAtomic(path string, v any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".tmp*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer func() {
		if tmpName != "" {
			os.Remove(tmpName)
		}
	}()

	encoder := yaml.NewEncoder(tmp)
	if err := encoder.Encode(v); err != nil {
		tmp.Close()
		return err
	}
	if err := encoder.Close(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpName, path); err != nil {
		return err
	}
	tmpName = ""
	return nil
}
