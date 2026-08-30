package config

import (
	"errors"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/viper"
	"go.yaml.in/yaml/v3"
)

var (
	SubDir        string
	subConfigFile string
)

type SubConfig struct {
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
	UserAgent string `mapstructure:"UserAgent"`

	UseProxy bool
	SkipCert bool
}

func LoadSubConfig() (*SubConfig, error) {
	if err := ensureSubConfig(); err != nil {
		return nil, err
	}
	info, err := os.Stat(subConfigFile)
	if errors.Is(err, os.ErrNotExist) {
		return &SubConfig{}, nil
	}
	if err != nil {
		return nil, err
	}
	if info.Size() == 0 {
		return &SubConfig{}, nil
	}
	subV := viper.New()
	subV.SetConfigFile(subConfigFile)
	if err := subV.ReadInConfig(); err != nil {
		return nil, err
	}
	cfg := &SubConfig{}
	if err := subV.Unmarshal(cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

func SaveSubConfig(cfg *SubConfig) error {
	if cfg == nil {
		return errors.New("sub config is nil")
	}
	if err := ensureSubConfig(); err != nil {
		return err
	}
	f, err := os.Create(subConfigFile)
	if err != nil {
		return err
	}
	defer f.Close()

	encoder := yaml.NewEncoder(f)
	defer encoder.Close()

	return encoder.Encode(cfg)
}

func ensureSubConfig() error {
	return os.MkdirAll(SubDir, 0o755)
}

func init() {
	SubDir = filepath.Join(AppConfigDir, "profiles")
	subConfigFile = filepath.Join(AppConfigDir, "profiles.yaml")
}
