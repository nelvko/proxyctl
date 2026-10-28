package ui

import (
	"strings"

	"github.com/nelvko/proxyctl/internal/i18n"
)

func (m dashboardModel) tr(key string, args ...any) string { return i18n.Text(m.locale, key, args...) }

// Existing line renderers share translated templates. Match their fixed prefix
// instead of assuming one language when assigning heading and status styles.
func (m dashboardModel) hasMessagePrefix(value string, keys ...string) bool {
	for _, key := range keys {
		prefix, _, _ := strings.Cut(m.tr(key), "%")
		if strings.HasPrefix(strings.TrimSpace(value), strings.TrimSpace(prefix)) {
			return true
		}
	}
	return false
}

func (m dashboardModel) pageNames() []string {
	keys := []string{"page.overview", "page.nodes", "page.subscriptions", "page.connections", "page.rules", "page.logs", "page.settings"}
	names := make([]string, len(keys))
	for i, key := range keys {
		names[i] = m.tr(key)
	}
	return names
}

// Snapshot state remains stable for controller/lifecycle comparisons; only its
// visible labels are translated.
func (m dashboardModel) displayState(value string) string {
	keys := map[string]string{"读取中": "state.loading", "未选择": "state.unselected", "未启用": "state.disabled", "已启用": "state.enabled", "运行中": "state.running", "已停止": "state.stopped", "未知": "state.unknown", "不可用": "state.unavailable", "已配置": "state.configured", "未配置": "state.unconfigured"}
	if key, ok := keys[value]; ok {
		return m.tr(key)
	}
	return value
}

func (m dashboardModel) displayHint(value string) string {
	if m.locale != i18n.English {
		return value
	}
	replacements := []struct{ source, key string }{
		{"未选择内核；先运行 proxyctl install 或 proxyctl use", "hint.no_kernel"},
		{"控制 API 未启用；在生效的内核配置中设置本地 external-controller / clash_api", "hint.no_api"},
		{"内核未运行；启动后可查看节点、连接、规则和实时日志", "hint.not_running"},
	}
	for _, replacement := range replacements {
		if value == replacement.source {
			return m.tr(replacement.key)
		}
	}
	for _, prefix := range []struct{ source, key string }{{"读取节点失败: ", "hint.nodes_failed"}, {"读取连接失败: ", "hint.connections_failed"}, {"读取规则失败: ", "hint.rules_failed"}} {
		if tail, ok := strings.CutPrefix(value, prefix.source); ok {
			return m.tr(prefix.key, tail)
		}
	}
	return value
}
