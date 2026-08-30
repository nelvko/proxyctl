package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/viper"
	"go.yaml.in/yaml/v3"
)

var (
	AppConfigFile string
	AppConfigDir  string
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
	for i := range c.Kernels {
		if c.Kernels[i].Name == c.Use {
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
	// No config file yet means no kernel installed — not an error.
	if _, err := os.Stat(AppConfigFile); errors.Is(err, os.ErrNotExist) {
		return &AppConfig{}, nil
	}
	appV := viper.New()
	appV.SetConfigFile(AppConfigFile)
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
		return nil, fmt.Errorf("%s uses the old single-kernel format, remove it and run `proxyctl kernel install`", AppConfigFile)
	}
	return cfg, nil
}

func SaveAppConfig(cfg *AppConfig) error {
	if cfg == nil {
		return errors.New("app config is nil")
	}
	if err := os.MkdirAll(AppConfigDir, 0o755); err != nil {
		return err
	}
	f, err := os.Create(AppConfigFile)
	if err != nil {
		return err
	}
	defer f.Close()

	encoder := yaml.NewEncoder(f)
	defer encoder.Close()

	return encoder.Encode(cfg)
}

func init() {
	cfgDir, _ := os.UserConfigDir()
	AppConfigDir = filepath.Join(cfgDir, AppName)
	AppConfigFile = filepath.Join(AppConfigDir, "config.yaml")
}
