package config

import (
	"os"
	"path/filepath"

	"github.com/spf13/viper"
	"go.yaml.in/yaml/v3"
)

type AppConfig struct {
	InitSystem string `mapstructure:"initsystem"`

	Kernel struct {
		Name       string `mapstructure:"name"`
		Bin        string `mapstructure:"bin"`
		ConfigDir  string `mapstructure:"configdir"`
		ConfigFile string `mapstructure:"configfile"`
	} `mapstructure:"kernel"`
}

const (
	AppName = "proxyctl"
)

var (
	AppConfigFile string
	AppCfg        = &AppConfig{}
	v             = viper.New()
)

func LoadAppConfig() error {
	if err := v.ReadInConfig(); err != nil {
		return err
	}
	if err := v.Unmarshal(AppCfg); err != nil {
		return err
	}
	return nil
}

func SaveAppConfig() error {
	f, err := os.Create(AppConfigFile)
	if err != nil {
		return err
	}
	defer f.Close()

	encoder := yaml.NewEncoder(f)
	defer encoder.Close()

	return encoder.Encode(AppCfg)
}

func init() {
	cfgDir, _ := os.UserConfigDir()
	AppConfigFile = filepath.Join(cfgDir, AppName, "config.yaml")
	v.SetConfigFile(AppConfigFile)
}
