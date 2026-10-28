package ui

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/nelvko/proxyctl/internal/config"
)

// Theme describes shared visual roles, rather than individual components.
// All colors use #RRGGBB; Bubble Tea handles terminal color-depth conversion.
type Theme struct {
	Background string
	Surface    string
	Foreground string
	Muted      string
	Disabled   string
	Primary    string
	Border     string
	Success    string
	Warning    string
	Error      string
	Selected   string
	Hover      string
}

type themeOption struct{ Name, Label string }

func builtInThemes() []themeOption {
	return []themeOption{
		{"default", "Default"},
		{"catppuccin-mocha", "Catppuccin Mocha"},
		{"tokyo-night", "Tokyo Night"},
		{"dracula", "Dracula"},
		{"nord", "Nord"},
	}
}

func defaultThemeConfig() config.ThemeConfig { return config.ThemeConfig{} }

func loadThemeConfig() (config.ThemeConfig, error) {
	return config.LoadTheme()
}

func saveThemeConfig(ctx context.Context, cfg config.ThemeConfig) error {
	return config.SaveTheme(ctx, cfg)
}

var themeColorPattern = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

// ValidateThemeConfig is the strict import/export boundary. Runtime loading
// still resolves usable fallbacks, while imports must never persist typos.
func ValidateThemeConfig(cfg config.ThemeConfig) error {
	_, err := resolveTheme(cfg, true)
	if cfg.Name != "custom" && (cfg.Base != "" || len(cfg.Colors) > 0) {
		// Preserve inactive custom preferences on round-trip, but validate them
		// before a later switch to Custom can make them active.
		_, customErr := resolveTheme(config.ThemeConfig{Name: "custom", Base: cfg.Base, Colors: cfg.Colors}, true)
		err = errors.Join(err, customErr)
	}
	return err
}

// resolveTheme always returns a complete palette. Warnings report invalid
// preferences without making the rest of the application unusable.
func resolveTheme(cfg config.ThemeConfig, darkBG bool) (Theme, error) {
	var warnings []error
	mode := cfg.Mode
	if mode == "" {
		mode = "auto"
	}
	if mode != "auto" && mode != "dark" && mode != "light" {
		warnings = append(warnings, fmt.Errorf("未知主题模式 %q，已使用 auto", mode))
		mode = "auto"
	}
	name := cfg.Name
	if name == "" {
		name = "default"
	}
	base := name
	if name == "custom" {
		base = cfg.Base
		if base == "" {
			base = "default"
		}
	}
	palette, ok := darkTheme(base)
	if !ok {
		warnings = append(warnings, fmt.Errorf("未知主题 %q，已使用 default", base))
		base = "default"
		palette, _ = darkTheme(base)
	}
	// An explicit palette is intentional. Only an unspecified palette follows
	// background detection, unless the user explicitly selects light mode.
	light := mode == "light" || mode == "auto" && cfg.Name == "" && !darkBG
	if light {
		palette = lightTheme(base)
	}
	// Overrides apply to custom only, so switching to a built-in is predictable.
	if name == "custom" {
		tokens := map[string]*string{
			"background": &palette.Background, "surface": &palette.Surface,
			"foreground": &palette.Foreground, "muted": &palette.Muted,
			"disabled": &palette.Disabled, "primary": &palette.Primary,
			"border": &palette.Border, "success": &palette.Success,
			"warning": &palette.Warning, "error": &palette.Error,
			"selected": &palette.Selected, "hover": &palette.Hover,
		}
		keys := make([]string, 0, len(cfg.Colors))
		for key := range cfg.Colors {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			token, exists := tokens[key]
			if !exists {
				warnings = append(warnings, fmt.Errorf("未知主题颜色 %q，已忽略", key))
				continue
			}
			value := cfg.Colors[key]
			if !themeColorPattern.MatchString(value) {
				warnings = append(warnings, fmt.Errorf("主题颜色 %q 须使用 #RRGGBB，已继承基础主题", key))
				continue
			}
			*token = strings.ToUpper(value)
		}
	}
	return palette, errors.Join(warnings...)
}

func darkTheme(name string) (Theme, bool) {
	switch name {
	case "default":
		return Theme{
			Background: "#141626", Surface: "#1C1E32", Foreground: "#C7CBE0",
			Muted: "#999AB5", Disabled: "#75758F", Primary: "#A78BFA",
			Border: "#34304F", Success: "#4ADE80", Warning: "#FBBF24", Error: "#FB7185",
			Selected: "#302843", Hover: "#25243B",
		}, true
	case "catppuccin-mocha":
		return Theme{
			Background: "#1E1E2E", Surface: "#313244", Foreground: "#CDD6F4",
			Muted: "#A6ADC8", Disabled: "#7F849C", Primary: "#CBA6F7",
			Border: "#45475A", Success: "#A6E3A1", Warning: "#F9E2AF", Error: "#F38BA8",
			Selected: "#45405B", Hover: "#363447",
		}, true
	case "tokyo-night":
		return Theme{
			Background: "#1A1B26", Surface: "#24283B", Foreground: "#C0CAF5",
			Muted: "#A9B1D6", Disabled: "#737AA2", Primary: "#BB9AF7",
			Border: "#3B4261", Success: "#9ECE6A", Warning: "#E0AF68", Error: "#F7768E",
			Selected: "#34304F", Hover: "#292B41",
		}, true
	case "dracula":
		return Theme{
			Background: "#282A36", Surface: "#343746", Foreground: "#F8F8F2",
			Muted: "#B5B9D4", Disabled: "#8188A8", Primary: "#BD93F9",
			Border: "#44475A", Success: "#50FA7B", Warning: "#F1FA8C", Error: "#FF6E6E",
			Selected: "#49405D", Hover: "#3B3B4E",
		}, true
	case "nord":
		return Theme{
			Background: "#2E3440", Surface: "#3B4252", Foreground: "#ECEFF4",
			Muted: "#B8C5D9", Disabled: "#8793A8", Primary: "#88C0D0",
			Border: "#4C566A", Success: "#A3BE8C", Warning: "#EBCB8B", Error: "#F08C96",
			Selected: "#434F63", Hover: "#394251",
		}, true
	default:
		return Theme{}, false
	}
}

// The dark-only named schemes have light adaptations with the same accent
// family and dark status colors for readable contrast on a light background.
func lightTheme(name string) Theme {
	palette := Theme{
		Background: "#F6F5FA", Surface: "#ECEAF3", Foreground: "#28263D",
		Muted: "#625D78", Disabled: "#89839B", Primary: "#6940BB",
		Border: "#D7D2E3", Success: "#16713C", Warning: "#925C00", Error: "#B4233F",
		Selected: "#DED5EF", Hover: "#E8E2F1",
	}
	switch name {
	case "catppuccin-mocha":
		palette.Primary = "#8839EF"
	case "tokyo-night":
		palette.Primary = "#5A4A9B"
	case "dracula":
		palette.Primary = "#7540B7"
	case "nord":
		palette.Background, palette.Surface = "#ECEFF4", "#E5E9F0"
		palette.Primary, palette.Selected, palette.Hover = "#3B687C", "#CDDDE5", "#DEE7EC"
	}
	return palette
}
