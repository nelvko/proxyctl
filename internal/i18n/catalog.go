// Package i18n owns user-facing messages. Configuration keys, shortcuts and
// machine-readable values remain independent of the selected language.
package i18n

import (
	"fmt"
	"os"
	"strings"
)

type Locale string

const (
	Chinese Locale = "zh-CN"
	English Locale = "en"
)

type Message struct{ Chinese, English string }

// Resolve selects a supported locale once for a UI model. An explicit
// preference wins over the process locale; unknown values use a safe fallback.
func Resolve(preference string) (Locale, error) {
	if preference != "" && preference != "auto" {
		if locale, ok := parseLocale(preference); ok {
			return locale, nil
		}
		return Chinese, fmt.Errorf("unsupported language %q; using zh-CN", preference)
	}
	for _, key := range []string{"LC_ALL", "LC_MESSAGES", "LANG"} {
		if value := os.Getenv(key); value != "" {
			if locale, ok := parseLocale(value); ok {
				return locale, nil
			}
			// Locale precedence applies even when the selected locale is unsupported.
			return Chinese, nil
		}
	}
	return Chinese, nil
}

func parseLocale(value string) (Locale, bool) {
	value = strings.ToLower(strings.ReplaceAll(strings.TrimSpace(value), "_", "-"))
	value, _, _ = strings.Cut(value, ".")
	value, _, _ = strings.Cut(value, "@")
	if value == "en" || strings.HasPrefix(value, "en-") {
		return English, true
	}
	if value == "zh" || strings.HasPrefix(value, "zh-") {
		return Chinese, true
	}
	return Chinese, false
}

// Text falls back to Chinese for missing translations or an unresolved locale,
// and to the stable key for unknown messages. Models own locale state; there is
// no mutable process-wide translator during runtime preview.
func Text(locale Locale, key string, args ...any) string {
	message, ok := commonMessages[key]
	if !ok {
		message, ok = dashboardMessages[key]
	}
	if !ok {
		message, ok = subscriptionMessages[key]
	}
	if !ok {
		return key
	}
	value := message.Chinese
	if locale == English && message.English != "" {
		value = message.English
	}
	if value == "" {
		value = message.English
	}
	if len(args) > 0 {
		return fmt.Sprintf(value, args...)
	}
	return value
}

var commonMessages = map[string]Message{
	"common.confirm":            {"确认", "Confirm"},
	"common.cancel":             {"取消", "Cancel"},
	"common.close":              {"关闭", "Close"},
	"common.language":           {"语言", "Language"},
	"common.auto":               {"跟随系统", "System default"},
	"common.search":             {"搜索", "Search"},
	"common.choose":             {"请选择", "Choose an option"},
	"select.search_prompt":      {"搜索: ", "Search: "},
	"select.search_placeholder": {"输入关键字", "Type keywords"},
	"select.empty":              {"没有匹配的选项", "No matching options"},
	"select.help":               {"↑↓ 选择 · Enter 确认 · Esc 取消 · %d 项", "↑↓ Select · Enter Confirm · Esc Cancel · %d items"},
	"modal.scroll_hint":         {"↑↓ 滚动 · Esc 关闭", "↑↓ Scroll · Esc Close"},
	"theme.imported":            {"主题已导入，重新打开 TUI 后生效。", "Theme imported. Reopen the TUI to apply it."},
	"language.save_failed":      {"语言保存失败: %s", "Could not save language: %s"},
	"language.saved":            {"语言设置已保存", "Language preference saved"},
	"language.load_failed":      {"语言配置已回退: %s", "Language preference fell back: %s"},
}
