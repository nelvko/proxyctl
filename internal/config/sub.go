package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/viper"
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

type SubscriptionConfig struct {
	Use      string    `mapstructure:"use"`
	Profiles []Profile `mapstructure:"profiles"`
}

type Profile struct {
	Name string `mapstructure:"name"`
	URL  string `mapstructure:"url"`
	File string `mapstructure:"file"`

	Update UpdateConfig `mapstructure:"update"`
}

type UpdateConfig struct {
	Enable bool `mapstructure:"enable"`

	Headers map[string]string `mapstructure:"headers"`

	Timeout  time.Duration `mapstructure:"timeout"`
	Interval time.Duration `mapstructure:"interval"`

	Cron      string `mapstructure:"cron"`
	UserAgent string `mapstructure:"useragent"`

	UseProxy bool `mapstructure:"useproxy"`
	SkipCert bool `mapstructure:"skipcert"`
}

func LoadSubscriptionConfig() (*SubscriptionConfig, error) {
	if err := ensureProfilesDir(); err != nil {
		return nil, err
	}
	path, err := subscriptionConfigFile()
	if err != nil {
		return nil, err
	}
	if info, err := os.Stat(path); errors.Is(err, os.ErrNotExist) || (err == nil && info.Size() == 0) {
		return &SubscriptionConfig{}, nil
	} else if err != nil {
		return nil, err
	}
	subV := viper.New()
	subV.SetConfigFile(path)
	if err := subV.ReadInConfig(); err != nil {
		return nil, err
	}
	cfg := &SubscriptionConfig{}
	if err := subV.Unmarshal(cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

func SaveSubscriptionConfig(cfg *SubscriptionConfig) error {
	if cfg == nil {
		return errors.New("subscription config is nil")
	}
	path, err := subscriptionConfigFile()
	if err != nil {
		return err
	}
	return saveYAMLAtomic(path, cfg)
}

func ensureProfilesDir() error {
	dir, err := ProfilesDir()
	if err != nil {
		return err
	}
	return os.MkdirAll(dir, 0o755)
}
