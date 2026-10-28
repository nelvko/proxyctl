package config

import (
	"context"
	"errors"
	"fmt"
)

// ThemeConfig reuses config.yaml. Color keys are semantic tokens, and missing
// tokens inherit the selected base palette. Validation belongs to the UI so an
// invalid appearance preference cannot prevent CLI kernel operations.
type ThemeConfig struct {
	Name   string            `mapstructure:"name" yaml:"name,omitempty"`
	Mode   string            `mapstructure:"mode" yaml:"mode,omitempty"`
	Base   string            `mapstructure:"base" yaml:"base,omitempty"`
	Colors map[string]string `mapstructure:"colors" yaml:"colors,omitempty"`
}

// LoadTheme reports appearance-only decode warnings without preventing Load
// from returning usable application settings. Invalid fields inherit defaults.
func LoadTheme() (ThemeConfig, error) {
	v, _, err := readConfig()
	if err != nil || v == nil {
		return ThemeConfig{}, err
	}
	return decodeTheme(v.Get("theme"))
}

func decodeTheme(raw any) (ThemeConfig, error) {
	if raw == nil {
		return ThemeConfig{}, nil
	}
	values, ok := raw.(map[string]any)
	if !ok {
		return ThemeConfig{}, errors.New("theme must be a mapping; using default appearance")
	}
	var theme ThemeConfig
	var warnings []error
	for _, field := range []struct {
		name  string
		value *string
	}{{"name", &theme.Name}, {"mode", &theme.Mode}, {"base", &theme.Base}} {
		if value, exists := values[field.name]; exists {
			if text, ok := value.(string); ok {
				*field.value = text
			} else {
				warnings = append(warnings, fmt.Errorf("theme.%s must be a string; using default", field.name))
			}
		}
	}
	if rawColors, exists := values["colors"]; exists && rawColors != nil {
		colors, ok := rawColors.(map[string]any)
		if !ok {
			warnings = append(warnings, errors.New("theme.colors must be a mapping; inheriting base colors"))
		} else {
			for key, value := range colors {
				if text, ok := value.(string); ok {
					if theme.Colors == nil {
						theme.Colors = make(map[string]string)
					}
					theme.Colors[key] = text
				} else {
					warnings = append(warnings, fmt.Errorf("theme.colors.%s must be a string; inheriting base color", key))
				}
			}
		}
	}
	return theme, errors.Join(warnings...)
}

// SaveTheme reloads inside the shared state lock so a TUI opened before a CLI
// mutation cannot overwrite kernel or mirror changes when saving appearance.
func SaveTheme(ctx context.Context, theme ThemeConfig) error {
	unlock, err := Lock(ctx)
	if err != nil {
		return err
	}
	defer unlock()
	cfg, err := Load()
	if err != nil {
		return err
	}
	cfg.Theme = theme
	return Save(cfg)
}
