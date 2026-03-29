package sub

import (
	"errors"
	"os"
	"path/filepath"
	"time"

	"github.com/nelvko/proxyctl/config"
	"github.com/spf13/viper"
	"go.yaml.in/yaml/v3"
)

var (
	v = viper.New()

	subCfg        = &subConfig{}
	subDir        string
	subConfigFile string
)

type subConfig struct {
	Use      string    `mapstructure:"use"`
	Profiles []profile `mapstructure:"profiles"`
}
type profile struct {
	Name string `mapstructure:"name"`
	URL  string `mapstructure:"url"`
	File string `mapstructure:"file"`

	Update updateConfig `mapstructure:"update"`
}
type updateConfig struct {
	Enable bool `mapstructure:"enable"`

	Headers map[string]string `mapstructure:"headers"`

	Timeout  time.Duration `mapstructure:"timeout"`
	Interval time.Duration `mapstructure:"interval"`

	Cron      string `mapstructure:"cron"`
	UserAgent string `mapstructure:"UserAgent"`

	UseProxy bool
	SkipCert bool
}

func loadSubConfig() error {
	if err := v.ReadInConfig(); err != nil {
		return err
	}
	if err := v.Unmarshal(subCfg); err != nil {
		return err
	}
	return nil
}

func saveSubConfig() error {
	f, err := os.Create(subConfigFile)
	if err != nil {
		return err
	}
	defer f.Close()

	encoder := yaml.NewEncoder(f)
	defer encoder.Close()

	return encoder.Encode(subCfg)
}

func init() {
	subDir = filepath.Join(config.AppConfigDir, "profiles")
	subConfigFile = filepath.Join(config.AppConfigDir, "profiles.yaml")
	if _, err := os.Stat(config.AppConfigFile); err == nil {
		if _, err := os.Stat(subConfigFile); errors.Is(err, os.ErrNotExist) {
			os.MkdirAll(subDir, 0o755)
			os.WriteFile(subConfigFile, nil, 0o666)
		}
	}
	v.SetConfigFile(subConfigFile)
}
