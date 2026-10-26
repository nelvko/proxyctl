package subscription

import (
	"bytes"
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/nelvko/proxyctl/internal/kernel"
	"go.yaml.in/yaml/v3"
)

func TestMergeConfigPrecedenceAndReplacement(t *testing.T) {
	for _, format := range []kernel.ConfigFormat{kernel.FormatClash, kernel.FormatSingBox} {
		t.Run(string(format), func(t *testing.T) {
			base := []byte(`{"log":{"level":"info","disabled":false},"rules":["old","old2"],"keep":"source","remove":"source","toObject":false,"toScalar":{"old":true}}`)
			global := []byte(`{"log":{"level":"warn"},"rules":["global"],"new":"global"}`)
			profile := []byte(`{"log":{"level":"debug"},"rules":[],"remove":null,"new":"profile","toObject":{"new":true},"toScalar":false}`)
			got, err := mergeConfig(format, base, global, profile)
			if err != nil {
				t.Fatal(err)
			}
			var object map[string]any
			if err := yaml.Unmarshal(got, &object); err != nil {
				t.Fatal(err)
			}
			want := map[string]any{
				"log":   map[string]any{"level": "debug", "disabled": false},
				"rules": []any{}, "keep": "source", "remove": nil, "new": "profile",
				"toObject": map[string]any{"new": true}, "toScalar": false,
			}
			if !reflect.DeepEqual(object, want) {
				t.Fatalf("merged configuration = %#v; want %#v", object, want)
			}
			if format == kernel.FormatSingBox && !json.Valid(got) {
				t.Fatalf("sing-box output was not JSON: %s", got)
			}
		})
	}
}

func TestMergeConfigPreservesOriginalWithoutOverrides(t *testing.T) {
	for _, test := range []struct {
		format kernel.ConfigFormat
		base   string
	}{
		{kernel.FormatClash, "# user comment\nport: 7890\n"},
		{kernel.FormatSingBox, "{ \"inbounds\" : [] }\n"},
	} {
		for _, overlays := range [][][]byte{nil, {[]byte("{}")}, {[]byte("{}"), []byte("{}")}} {
			got, err := mergeConfig(test.format, []byte(test.base), overlays...)
			if err != nil || string(got) != test.base {
				t.Fatalf("format %s: unchanged configuration = %q, error %v", test.format, got, err)
			}
		}
	}
}

func TestMergeConfigLeavesUnmodifiedSourcesToKernelValidation(t *testing.T) {
	base := []byte("an adapter-specific source not handled by the merge parser")
	got, err := mergeConfig(kernel.FormatClash, base)
	if err != nil || !bytes.Equal(got, base) {
		t.Fatalf("source without overrides changed: %q, %v", got, err)
	}
}

func TestMergeConfigSupportsYAMLAnchorsAndMergeKeys(t *testing.T) {
	base := []byte("defaults: &defaults\n  server: example.com\n  port: 443\nproxy:\n  <<: *defaults\n  name: original\n")
	got, err := mergeConfig(kernel.FormatClash, base, []byte("proxy:\n  port: 8443\n"))
	if err != nil {
		t.Fatal(err)
	}
	var object map[string]any
	if err := yaml.Unmarshal(got, &object); err != nil {
		t.Fatal(err)
	}
	proxy := object["proxy"].(map[string]any)
	if proxy["server"] != "example.com" || proxy["port"] != 8443 || proxy["name"] != "original" {
		t.Fatalf("merged YAML alias lost inherited values: %#v", proxy)
	}
}

func TestMergeConfigPreservesJSONNumberPrecision(t *testing.T) {
	base := []byte(`{"integer":9007199254740993,"huge":123456789012345678901234567890,"fraction":0.123456789012345678901234567890,"exponent":1e100}`)
	got, err := mergeConfig(kernel.FormatSingBox, base, []byte(`{"enabled":true}`))
	if err != nil {
		t.Fatal(err)
	}
	decoder := json.NewDecoder(bytes.NewReader(got))
	decoder.UseNumber()
	var object map[string]any
	if err := decoder.Decode(&object); err != nil {
		t.Fatal(err)
	}
	for name, want := range map[string]string{
		"integer": "9007199254740993", "huge": "123456789012345678901234567890",
		"fraction": "0.123456789012345678901234567890", "exponent": "1e100",
	} {
		if got := object[name].(json.Number).String(); got != want {
			t.Fatalf("%s changed from %s to %s", name, want, got)
		}
	}
}

func TestMergeConfigRejectsAmbiguousAndInvalidDocuments(t *testing.T) {
	for _, test := range []struct {
		format kernel.ConfigFormat
		data   string
	}{
		{kernel.FormatClash, ""},
		{kernel.FormatClash, "null"},
		{kernel.FormatClash, "[]"},
		{kernel.FormatClash, "port: ["},
		{kernel.FormatClash, "port: 1\n---\nport: 2"},
		{kernel.FormatClash, "port: 1\nport: 2"},
		{kernel.FormatClash, "nested:\n  key: 1\n  key: 2"},
		{kernel.FormatClash, "nested:\n  1: value"},
		{kernel.FormatClash, "? [one, two]\n: value"},
		{kernel.FormatClash, "recursive: &self\n  child: *self"},
		{kernel.FormatSingBox, ""},
		{kernel.FormatSingBox, "null"},
		{kernel.FormatSingBox, "[]"},
		{kernel.FormatSingBox, "{invalid"},
		{kernel.FormatSingBox, "port: 7890"},
		{kernel.FormatSingBox, `{\"port\":1}`},
		{kernel.FormatSingBox, "{} {}"},
		{kernel.FormatSingBox, "{} trailing"},
		{kernel.FormatSingBox, `{"port":1,"port":2}`},
		{kernel.FormatSingBox, `{"nested":{"key":1,"key":2}}`},
		{kernel.FormatSingBox, `{"list":[{"key":1,"key":2}]}`},
		{kernel.FormatSingBox, `{"list":[}`},
		{kernel.ConfigFormat("custom"), "{}"},
	} {
		t.Run(string(test.format)+"/"+test.data, func(t *testing.T) {
			if _, err := mergeConfig(test.format, []byte(test.data), []byte("{}")); err == nil {
				t.Fatal("accepted an invalid source configuration")
			}
			if _, err := mergeConfig(test.format, []byte("{}"), []byte(test.data)); err == nil {
				t.Fatal("accepted an invalid override")
			}
		})
	}
}

func TestMergeConfigErrorsDoNotEchoConfigurationValues(t *testing.T) {
	for _, format := range []kernel.ConfigFormat{kernel.FormatClash, kernel.FormatSingBox} {
		for _, content := range []string{
			`{"private-token":"secret-token","private-token":"other-secret"}`,
			`{"private-token":"secret-token",broken`,
		} {
			_, err := mergeConfig(format, []byte("{}"), []byte(content))
			if err == nil {
				t.Fatal("expected invalid override")
			}
			if strings.Contains(err.Error(), "private-token") || strings.Contains(err.Error(), "secret-token") {
				t.Fatalf("error leaked configuration content: %v", err)
			}
		}
	}
}
