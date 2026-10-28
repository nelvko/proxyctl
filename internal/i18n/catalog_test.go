package i18n

import (
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

func TestResolveLocalePreferenceAndEnvironment(t *testing.T) {
	t.Setenv("LC_ALL", "")
	t.Setenv("LC_MESSAGES", "")
	t.Setenv("LANG", "en_US.UTF-8")
	if locale, err := Resolve(""); err != nil || locale != English {
		t.Fatalf("auto=%q,%v", locale, err)
	}
	if locale, err := Resolve("zh-CN"); err != nil || locale != Chinese {
		t.Fatalf("explicit=%q,%v", locale, err)
	}
	t.Setenv("LC_MESSAGES", "zh_CN.UTF-8")
	if locale, _ := Resolve("auto"); locale != Chinese {
		t.Fatal("LC_MESSAGES did not override LANG")
	}
	t.Setenv("LC_ALL", "en_GB.UTF-8")
	if locale, _ := Resolve(""); locale != English {
		t.Fatal("LC_ALL did not take precedence")
	}
	t.Setenv("LC_ALL", "C.UTF-8")
	if locale, _ := Resolve(""); locale != Chinese {
		t.Fatal("unsupported system locale did not use safe default")
	}
	if locale, err := Resolve("missing"); locale != Chinese || err == nil {
		t.Fatal("invalid explicit locale did not warn and fall back")
	}
}

func TestTextFallbackAndPerModelLocales(t *testing.T) {
	if Text(English, "common.cancel") != "Cancel" || Text(Chinese, "common.cancel") != "取消" {
		t.Fatal("locale lookup failed")
	}
	if Text("", "common.cancel") != "取消" || Text(Locale("missing"), "common.cancel") != "取消" {
		t.Fatal("unresolved locale failed fallback")
	}
	if Text(English, "missing.key") != "missing.key" {
		t.Fatal("unknown key should stay diagnosable")
	}
	if Text(English, "appearance.save_failed", "disk") != "Could not save appearance: disk" {
		t.Fatal("format substitution failed")
	}
}

func TestCatalogCompletenessAndPlaceholderParity(t *testing.T) {
	// Preserve argument identity as well as types, so indexed placeholders may
	// reorder translations without silently swapping differently typed values.
	placeholder := regexp.MustCompile(`%(?:\[([0-9]+)\])?[+#0 -]*(?:[0-9]+)?(?:\.[0-9]+)?([a-zA-Z%])`)
	types := func(value string) map[int]string {
		result := map[int]string{}
		next := 1
		for _, match := range placeholder.FindAllStringSubmatch(value, -1) {
			if match[2] == "%" {
				continue
			}
			if match[1] != "" {
				next, _ = strconv.Atoi(match[1])
			}
			if previous, ok := result[next]; ok && previous != match[2] {
				t.Errorf("argument %d has incompatible types in %q", next, value)
			}
			result[next] = match[2]
			next++
		}
		return result
	}
	seen := map[string]bool{}
	for _, catalog := range []map[string]Message{commonMessages, dashboardMessages, subscriptionMessages} {
		for key, message := range catalog {
			if seen[key] {
				t.Errorf("duplicate key %q", key)
			}
			seen[key] = true
			if strings.TrimSpace(message.Chinese) == "" || strings.TrimSpace(message.English) == "" {
				t.Errorf("missing locale for %q", key)
			}
			if !reflect.DeepEqual(types(message.Chinese), types(message.English)) {
				t.Errorf("placeholder mismatch for %q", key)
			}
		}
	}
}
