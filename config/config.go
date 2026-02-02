package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/nelvko/proxyctl/httpx"
	"github.com/spf13/viper"
)

type Config struct {
	InitSystem string `mapstructure:"initSystem"`

	Kernel struct {
		Name       string `mapstructure:"name"`
		BinPath    string `mapstructure:"binPath"`
		ConfigDir  string `mapstructure:"configDir"`
		ConfigFile string `mapstructure:"configFile"`
	} `mapstructure:"kernel"`

	GitHubProxy string `mapstructure:"githubProxy"`
}

var (
	myAppName    = "proxyctl"
	MyConfigDir  string
	MyConfigFile string
)

func init() {
	// if os.Geteuid() == 0 {
	MyConfigDir = filepath.Join("/etc", myAppName)
	MyConfigFile = filepath.Join(MyConfigDir, "config.yaml")
	// } else {
	// cfgDir, _ := os.UserConfigDir()
	// MyConfigDir = filepath.Join(cfgDir, myAppName)
	// MyConfigFile = filepath.Join(MyConfigDir, "config.yaml")
	// }
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	// viper.AddConfigPath(filepath.Join(MyConfigDir, myAppName))

	viper.AddConfigPath(filepath.Join("/etc", myAppName))
}

func Load() (*Config, error) {
	if err := viper.ReadInConfig(); err != nil {
		return nil, err
	}
	var cfg Config
	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func Save(cfg *Config) error {
	var configMap map[string]any

	data, err := json.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}
	if err := json.Unmarshal(data, &configMap); err != nil {
		return fmt.Errorf("failed to unmarshal config to map: %w", err)
	}

	for k, v := range configMap {
		viper.Set(k, v)
	}
	return viper.WriteConfig()
}

func Init() *Config {
	cfg := &Config{}
	cfg.GitHubProxy = os.Getenv(httpx.GH_PROXY)

	// if os.Geteuid() == 0 {
	// 	cfg.Kernel.BinPath = "/usr/local/bin"
	// } else {
	// 	home, _ := os.UserHomeDir()
	// 	cfg.Kernel.BinPath = filepath.Join(home, ".local", "bin")
	// }

	return cfg
}
