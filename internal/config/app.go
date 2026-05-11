package config

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/spf13/viper"
	"go.yaml.in/yaml/v3"
)

var (
	AppConfigFile string
	AppConfigDir  string
)

var ErrInitRequired = errors.New("init first")

const (
	AppName = "proxyctl"
)

type AppConfig struct {
	Use string `mapstructure:"use"`
	// Kernel []KernelConfig `mapstructure:"kernel"`
	Kernel KernelConfig `mapstructure:"kernel"`
}

type KernelConfig struct {
	InitSystem string `mapstructure:"initsystem"`
	Name       string `mapstructure:"name"`
	Bin        string `mapstructure:"bin"`
	ConfigDir  string `mapstructure:"configdir"`
	ConfigFile string `mapstructure:"configfile"`
}

func LoadAppConfig() (*AppConfig, error) {
	if err := ensureAppConfig(); err != nil {
		return nil, err
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

func ensureAppConfig() error {
	if _, err := os.Stat(AppConfigFile); errors.Is(err, os.ErrNotExist) {
		return ErrInitRequired
	}
	return nil
}

func init() {
	cfgDir, _ := os.UserConfigDir()
	AppConfigDir = filepath.Join(cfgDir, AppName)
	AppConfigFile = filepath.Join(AppConfigDir, "config.yaml")
}
