package ui

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestDashboardSettingsShowActiveConfigWithoutControllerSecret(t *testing.T) {
	m := newDashboardTestModel(context.Background())
	m.snapshot.kernel = "mihomo"
	m.snapshot.service = "运行中"
	m.snapshot.profile = "work"
	m.snapshot.configFile = "/tmp/mihomo/config.yaml"
	m.snapshot.proxy = "http://127.0.0.1:7890"
	m.snapshot.shell = "已启用"
	m.snapshot.controller = &controllerClient{base: "http://127.0.0.1:9090", secret: "top-secret"}
	view := strings.Join(m.settingsLines(24), "\n")
	for _, expected := range []string{"mihomo", "运行中", "work", "/tmp/mihomo/config.yaml", "http://127.0.0.1:7890", "控制 API    已配置", "proxyctl sub override edit --global"} {
		if !strings.Contains(view, expected) {
			t.Fatalf("settings missing %q: %q", expected, view)
		}
	}
	if strings.Contains(view, "top-secret") || strings.Contains(view, "9090") {
		t.Fatalf("controller credentials or address leaked: %q", view)
	}
}

func TestDashboardSettingsCompactAndRedactedWhenUnavailable(t *testing.T) {
	m := newDashboardTestModel(context.Background())
	m.snapshot.controllerHint = "控制 API 未启用；配置 http://user:pass@example.com/private?token=secret"
	m.snapshot.err = errors.New("read https://user:pass@example.com/private?token=secret failed")
	view := strings.Join(m.settingsLines(13), "\n")
	if !strings.Contains(view, "控制 API    未配置") || !strings.Contains(view, "状态读取失败") {
		t.Fatalf("missing unavailable status: %q", view)
	}
	if strings.Contains(view, "top-secret") || strings.Contains(view, "token=secret") || strings.Contains(view, "/private") || strings.Contains(view, "修改方式") {
		t.Fatalf("compact settings leaked or overflowed: %q", view)
	}
}
