package config

import (
	"errors"
	"fmt"
	"io"

	"go.yaml.in/yaml/v3"
)

// ReadTheme reads a standalone theme document with the same theme: shape as
// config.yaml. Imports are strict; forgiving fallback remains a loading policy.
func ReadTheme(r io.Reader) (ThemeConfig, error) {
	var document struct {
		Theme yaml.Node `yaml:"theme"`
	}
	decoder := yaml.NewDecoder(r)
	decoder.KnownFields(true)
	if err := decoder.Decode(&document); err != nil {
		return ThemeConfig{}, fmt.Errorf("read theme: %w", err)
	}
	var extra yaml.Node
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			err = errors.New("expected one YAML document")
		}
		return ThemeConfig{}, fmt.Errorf("read theme: %w", err)
	}
	node := document.Theme
	if node.Kind != yaml.MappingNode || node.Tag != "!!map" {
		return ThemeConfig{}, errors.New("theme must be a mapping")
	}
	seen := make(map[string]bool)
	for i := 0; i < len(node.Content); i += 2 {
		key, value := node.Content[i], node.Content[i+1]
		if key.Tag != "!!str" || seen[key.Value] {
			return ThemeConfig{}, fmt.Errorf("invalid or duplicate theme field %q", key.Value)
		}
		seen[key.Value] = true
		switch key.Value {
		case "name", "mode", "base":
			if value.Kind != yaml.ScalarNode || value.Tag != "!!str" {
				return ThemeConfig{}, fmt.Errorf("theme.%s must be a string", key.Value)
			}
		case "colors":
			if value.Kind != yaml.MappingNode || value.Tag != "!!map" {
				return ThemeConfig{}, errors.New("theme.colors must be a mapping")
			}
			for j := 0; j < len(value.Content); j += 2 {
				colorKey, color := value.Content[j], value.Content[j+1]
				if colorKey.Tag != "!!str" || color.Kind != yaml.ScalarNode || color.Tag != "!!str" {
					return ThemeConfig{}, errors.New("theme.colors keys and values must be strings")
				}
			}
		default:
			return ThemeConfig{}, fmt.Errorf("unknown theme field %q", key.Value)
		}
	}
	var theme ThemeConfig
	if err := node.Decode(&theme); err != nil {
		return ThemeConfig{}, fmt.Errorf("decode theme: %w", err)
	}
	return theme, nil
}

// WriteTheme exports preferences, including partial overrides, rather than
// expanding terminal-dependent resolved colors or copying application state.
func WriteTheme(w io.Writer, theme ThemeConfig) error {
	encoder := yaml.NewEncoder(w)
	encoder.SetIndent(2)
	if err := encoder.Encode(struct {
		Theme ThemeConfig `yaml:"theme"`
	}{Theme: theme}); err != nil {
		return err
	}
	return encoder.Close()
}
