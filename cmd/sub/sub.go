/*
Copyright © 2026 NAME HERE <EMAIL ADDRESS>
*/
package sub

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/nelvko/proxyctl/config"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// subCmd represents the sub command
var SubCmd = &cobra.Command{
	Use:   "sub",
	Short: "Manage Subscriptions",
	Long:  ``,
	RunE: func(cmd *cobra.Command, args []string) error {
		return tui()
	},
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		parent := cmd.Parent()
		if err := parent.PersistentPreRunE(parent, args); err != nil {
			return err
		}

		if err := v.ReadInConfig(); err != nil {
			return err
		}

		if err := v.Unmarshal(&cfg); err != nil {
			return err
		}
		return nil
	},
}

var (
	v   = viper.New()
	cfg Config

	profilesDir    string
	profilesConfig string
)

type profile struct {
	Name string `mapstructure:"name"`
	Url  string `mapstructure:"url"`
	File string `mapstructure:"file"`
}

type Config struct {
	Use   string    `mapstructure:"use"`
	Items []profile `mapstructure:"items"`
}

func saveSubConfig() error {
	v.Set("use", cfg.Use)
	v.Set("items", cfg.Items)
	return v.WriteConfig()
}

func init() {
	appConfigDir := filepath.Dir(config.AppConfigFile)
	profilesDir = filepath.Join(appConfigDir, "profiles")
	profilesConfig = filepath.Join(appConfigDir, "profiles.yaml")

	if _, err := os.Stat(profilesConfig); errors.Is(err, os.ErrNotExist) {
		os.MkdirAll(profilesDir, 0755)
		os.Create(profilesConfig)
	}

	v.SetConfigFile(profilesConfig)
}
