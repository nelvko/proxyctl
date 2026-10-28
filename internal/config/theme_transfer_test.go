package config

import (
	"bytes"
	"errors"
	"reflect"
	"strings"
	"testing"
)

func TestThemeDocumentRoundTripPreservesPreferences(t *testing.T) {
	for _, theme := range []ThemeConfig{{}, {Name: "nord", Mode: "light"}, {Name: "custom", Base: "tokyo-night", Mode: "dark", Colors: map[string]string{"primary": "#abcdef", "foreground": "#123456"}}} {
		var output bytes.Buffer
		if err := WriteTheme(&output, theme); err != nil {
			t.Fatal(err)
		}
		got, err := ReadTheme(&output)
		if err != nil || !reflect.DeepEqual(got, theme) {
			t.Fatalf("roundtrip: got %+v, want %+v, err=%v", got, theme, err)
		}
	}
}

func TestThemeDocumentRejectsInvalidShapesAndUnknownFields(t *testing.T) {
	for _, input := range []string{
		"", "name: nord\n", "theme: nord\n", "theme: [nord]\n", "theme: null\n",
		"theme: {}\nuse: mihomo\n", "theme:\n  mode: true\n", "theme:\n  name: 42\n",
		"theme:\n  typo: dark\n", "theme:\n  colors: red\n", "theme:\n  colors:\n    primary: 42\n",
		"theme:\n  colors:\n    primary: {nested: value}\n", "theme:\n  colors:\n    primary: '#123456'\n    primary: '#abcdef'\n",
		"theme:\n  name: nord\n  name: dracula\n", "theme: {}\n---\ntheme: {}\n", "theme: {}\n---\n[\n",
	} {
		t.Run(input, func(t *testing.T) {
			if got, err := ReadTheme(strings.NewReader(input)); err == nil {
				t.Fatalf("invalid document accepted: %+v", got)
			}
		})
	}
}

type themeFailWriter struct{}

func (themeFailWriter) Write([]byte) (int, error) { return 0, errors.New("write failed") }

func TestThemeDocumentPropagatesWriteErrors(t *testing.T) {
	if err := WriteTheme(themeFailWriter{}, ThemeConfig{}); err == nil {
		t.Fatal("failed export was reported as successful")
	}
}
