package ui

import (
	"context"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

func TestDashboardRulesBrowseAndRedact(t *testing.T) {
	m := newDashboardModel(context.Background())
	m.width, m.height, m.page = 80, 8, 4
	m.navFocused = false
	m.snapshot.rules = []dashboardRule{
		{Type: "DOMAIN", Payload: "one.example", Proxy: "DIRECT"},
		{Type: "DOMAIN-SUFFIX", Payload: "two.example", Proxy: "Proxy"},
		{Type: "DOMAIN", Payload: "https://user:pass@example.com/private?token=secret", Proxy: "Proxy"},
		{Type: "MATCH", Proxy: "DIRECT"},
	}
	updated, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	m = updated.(dashboardModel)
	if m.rule != 1 || m.page != 4 {
		t.Fatalf("rule cursor=%d page=%d", m.rule, m.page)
	}
	m.rule = 2
	view := ansi.Strip(m.View().Content)
	if !strings.Contains(view, "▶ 3  DOMAIN") || !strings.Contains(view, "example.com") {
		t.Fatalf("selected rule not visible: %q", view)
	}
	if strings.Contains(view, "token=secret") || strings.Contains(view, "/private") || strings.Contains(view, "user:pass") {
		t.Fatalf("rule content leaked credentials: %q", view)
	}
}

func TestDashboardRulesRefreshEveryFifthTick(t *testing.T) {
	m := newDashboardModel(context.Background())
	m.page, m.loading = dashboardRulesPage, false
	for i := 0; i < 4; i++ {
		updated, _ := m.Update(dashboardTickMsg{})
		m = updated.(dashboardModel)
		if m.loading {
			t.Fatalf("rules refreshed on tick %d", i+1)
		}
	}
	updated, _ := m.Update(dashboardTickMsg{})
	if !updated.(dashboardModel).loading {
		t.Fatal("rules did not refresh on fifth tick")
	}
}
