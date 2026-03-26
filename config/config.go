package config

import (
	"os"
	"path/filepath"

	"github.com/spf13/viper"
	"go.yaml.in/yaml/v3"
)

type config struct {
	InitSystem string `mapstructure:"initsystem"`

	Kernel struct {
		Name       string `mapstructure:"name"`
		Bin    string `mapstructure:"bin"`
		ConfigDir  string `mapstructure:"configdir"`
		ConfigFile string `mapstructure:"configfile"`
	} `mapstructure:"kernel"`
}

const (
	AppName = "proxyctl"
)

var (
	AppConfigFile string

	cfg config
	v   = viper.New()
)

func init() {
	cfgDir, _ := os.UserConfigDir()
	AppConfigFile = filepath.Join(cfgDir, AppName, "config.yaml")
	v.SetConfigFile(AppConfigFile)
}

func Load() error {
	if err := v.ReadInConfig(); err != nil {
		return err
	}
	return v.Unmarshal(&cfg)
}

func Get() *config {
	return &cfg
}

func Save() error {
	data, err := yaml.Marshal(&cfg)
	if err != nil {
		return err
	}
	return os.WriteFile(AppConfigFile, data, 0644)
}
