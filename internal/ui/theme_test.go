package ui

import (
	"math"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/nelvko/proxyctl/internal/config"
)

func TestThemeModeAndExplicitPalette(t *testing.T) {
	dark, _ := resolveTheme(defaultThemeConfig(), true)
	light, _ := resolveTheme(defaultThemeConfig(), false)
	if dark.Background == light.Background {
		t.Fatal("auto mode did not follow terminal background")
	}
	for _, name := range []string{"default", "catppuccin-mocha", "tokyo-night", "dracula", "nord"} {
		explicitDark, err := resolveTheme(config.ThemeConfig{Name: name}, true)
		explicitOnLight, errLight := resolveTheme(config.ThemeConfig{Name: name}, false)
		if err != nil || errLight != nil || explicitDark != explicitOnLight {
			t.Fatalf("explicit %s was overridden by detection: %v %v", name, err, errLight)
		}
		forcedLight, err := resolveTheme(config.ThemeConfig{Name: name, Mode: "light"}, true)
		if err != nil || forcedLight.Background == explicitDark.Background {
			t.Fatalf("light mode did not override %s: %v", name, err)
		}
	}
	forcedDark, err := resolveTheme(config.ThemeConfig{Mode: "dark"}, false)
	if err != nil || forcedDark != dark {
		t.Fatalf("explicit dark ignored: %+v, %v", forcedDark, err)
	}
}

func TestThemeCustomInheritsBaseAndValidatesEachToken(t *testing.T) {
	base, _ := resolveTheme(config.ThemeConfig{Name: "nord"}, true)
	cfg := config.ThemeConfig{Name: "custom", Base: "nord", Colors: map[string]string{"primary": "#abcdef", "background": "#123", "oops": "#123456"}}
	got, err := resolveTheme(cfg, true)
	if err == nil || !strings.Contains(err.Error(), "background") || !strings.Contains(err.Error(), "oops") {
		t.Fatalf("missing warnings: %v", err)
	}
	if got.Primary != "#ABCDEF" || got.Background != base.Background || got.Surface != base.Surface {
		t.Fatalf("partial overrides lost inherited tokens: %+v", got)
	}
	if cfg.Colors["primary"] != "#abcdef" {
		t.Fatal("resolution mutated persisted config")
	}
	// Selecting a built-in must ignore an old custom color map.
	cfg.Name = "nord"
	got, err = resolveTheme(cfg, true)
	if err != nil || got != base {
		t.Fatalf("custom colors leaked into built-in: %+v, %v", got, err)
	}
}

func TestInvalidThemeFallsBackWithoutIncompletePalette(t *testing.T) {
	for _, cfg := range []config.ThemeConfig{{Name: "missing"}, {Name: "custom", Base: "missing"}, {Mode: "missing"}} {
		got, err := resolveTheme(cfg, true)
		want, _ := resolveTheme(defaultThemeConfig(), true)
		if err == nil || got != want {
			t.Fatalf("invalid config %+v gave %+v, err=%v", cfg, got, err)
		}
	}
	for _, value := range []string{"red", "#fff", "#12345678", "#gggggg", "\x1b[31m", " #abcdef"} {
		got, err := resolveTheme(config.ThemeConfig{Name: "custom", Colors: map[string]string{"primary": value}}, true)
		want, _ := resolveTheme(defaultThemeConfig(), true)
		if err == nil || got != want {
			t.Fatalf("invalid color %q escaped fallback: %+v, %v", value, got, err)
		}
	}
}

func TestThemeImportValidationIncludesInactiveOverrides(t *testing.T) {
	for _, cfg := range []config.ThemeConfig{
		{Name: "nord", Base: "missing"},
		{Name: "nord", Colors: map[string]string{"primary": "bad"}},
		{Colors: map[string]string{"unknown": "#123456"}},
	} {
		if err := ValidateThemeConfig(cfg); err == nil {
			t.Fatalf("strict import validation accepted %+v", cfg)
		}
	}
	if err := ValidateThemeConfig(config.ThemeConfig{Name: "nord", Base: "default", Colors: map[string]string{"primary": "#123456"}}); err != nil {
		t.Fatalf("valid inactive preferences cannot round-trip: %v", err)
	}
}

func TestBuiltInThemeRegistryAndReadableText(t *testing.T) {
	var names []string
	for _, option := range builtInThemes() {
		names = append(names, option.Name)
		for _, mode := range []string{"dark", "light"} {
			theme, err := resolveTheme(config.ThemeConfig{Name: option.Name, Mode: mode}, true)
			if err != nil {
				t.Fatal(err)
			}
			v := reflect.ValueOf(theme)
			for i := range v.NumField() {
				if !themeColorPattern.MatchString(v.Field(i).String()) {
					t.Fatalf("incomplete %s/%s token %s", option.Name, mode, v.Type().Field(i).Name)
				}
			}
			for _, value := range []string{theme.Foreground, theme.Muted, theme.Primary, theme.Success, theme.Warning, theme.Error} {
				if ratio := themeContrast(value, theme.Background); ratio < 4.5 {
					t.Errorf("%s/%s color %s contrast %.2f < 4.5", option.Name, mode, value, ratio)
				}
			}
			if ratio := themeContrast(theme.Foreground, theme.Selected); ratio < 4.5 {
				t.Errorf("%s/%s selected text contrast %.2f < 4.5", option.Name, mode, ratio)
			}
		}
	}
	want := []string{"default", "catppuccin-mocha", "tokyo-night", "dracula", "nord"}
	if !reflect.DeepEqual(names, want) {
		t.Fatalf("unstable registry names: %v", names)
	}
}

func themeContrast(a, b string) float64 {
	luminance := func(hex string) float64 {
		rgb, _ := strconv.ParseUint(hex[1:], 16, 32)
		linear := func(value uint64) float64 {
			c := float64(value&255) / 255
			if c <= 0.04045 {
				return c / 12.92
			}
			return math.Pow((c+0.055)/1.055, 2.4)
		}
		return 0.2126*linear(rgb>>16) + 0.7152*linear(rgb>>8) + 0.0722*linear(rgb)
	}
	x, y := luminance(a), luminance(b)
	return (max(x, y) + 0.05) / (min(x, y) + 0.05)
}
