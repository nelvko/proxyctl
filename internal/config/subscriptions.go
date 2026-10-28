package config

import (
	"errors"
	"fmt"
	"maps"
	"os"
	"time"

	"github.com/spf13/viper"
)

// SubscriptionConfig is exactly the in-memory image of subscriptions.yaml:
// the selected profile and the profile list. Same rule as Config —
// persisted fields only, nothing runtime-derived.
type SubscriptionConfig struct {
	Use         string            `mapstructure:"use"`
	UseByFormat map[string]string `mapstructure:"usebyformat"`
	Profiles    []Profile         `mapstructure:"profiles"`
}

type Profile struct {
	Name   string `mapstructure:"name"`
	URL    string `mapstructure:"url"`
	File   string `mapstructure:"file"`
	Format string `mapstructure:"format"`

	Download DownloadConfig `mapstructure:"download"`
}

// DownloadConfig holds saved request settings shared by add and update.
type DownloadConfig struct {
	Headers     map[string]string `mapstructure:"headers"`
	Timeout     time.Duration     `mapstructure:"timeout"`
	UserAgent   string            `mapstructure:"useragent"`
	DownloadVia string            `mapstructure:"downloadvia"`
	SkipCert    bool              `mapstructure:"skipcert"`
}

func LoadSubscriptionConfig() (*SubscriptionConfig, error) {
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
	if err := subV.UnmarshalExact(cfg); err != nil {
		if profiles, ok := subV.Get("profiles").([]any); ok {
			for _, entry := range profiles {
				if profile, ok := entry.(map[string]any); ok {
					if _, old := profile["update"]; old {
						return nil, fmt.Errorf("%s: profile key update: is obsolete; use download: for request settings and remove scheduling fields enable/interval/cron: %w", path, err)
					}
				}
			}
		}
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

// Clone returns independently editable subscription metadata.
func (p Profile) Clone() Profile {
	p.Download.Headers = maps.Clone(p.Download.Headers)
	return p
}
