package ui

import (
	"context"
	"image/color"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/nelvko/proxyctl/internal/config"
)

func TestDashboardNavigationChangesWithWidth(t *testing.T) {
	m := newDashboardModel(context.Background())
	m.width, m.height, m.page = 80, 24, 3
	narrow := m.View().Content
	if !strings.Contains(narrow, "[4 连接]") || strings.Contains(narrow, "│") {
		t.Fatalf("narrow layout has wrong navigation: %q", narrow)
	}
	narrowLines := strings.Split(narrow, "\n")
	if len(narrowLines) != 24 || lipgloss.Width(narrowLines[0]) != 80 || lipgloss.Width(narrowLines[23]) != 80 || !strings.Contains(narrowLines[0], "/") || !strings.Contains(narrowLines[23], "/") {
		t.Fatalf("dashboard boundaries changed the screen size: %q", narrow)
	}
	if top, bottom := ansi.Strip(narrowLines[0]), ansi.Strip(narrowLines[23]); strings.Index(top, "proxyctl") <= 2 || !strings.HasPrefix(bottom, "  导航:") {
		t.Fatalf("boundary alignment: top=%q bottom=%q", top, bottom)
	}
	if strings.Contains(narrowLines[0], "未选择") || strings.Contains(narrowLines[23], "\x1b[48;") {
		t.Fatalf("dashboard boundary is too busy or footer has a background: top=%q bottom=%q", narrowLines[0], narrowLines[23])
	}
	if got := dashboardClickPage(80, 24, 12, 1); got != 1 {
		t.Fatalf("narrow mouse target = %d, want 1", got)
	}
	m.width = 120
	wide := m.View().Content
	if !strings.Contains(wide, "│") || !strings.Contains(wide, "4  连接") || !strings.Contains(wide, "\x1b[") {
		t.Fatalf("wide layout has wrong navigation: %q", wide)
	}
	wideLines := strings.Split(wide, "\n")
	if !strings.Contains(ansi.Strip(wideLines[0]), "连接") || strings.Contains(ansi.Strip(wideLines[0]), "proxyctl") ||
		strings.TrimSpace(strings.Split(ansi.Strip(wideLines[1]), "│")[0]) != "" ||
		!strings.Contains(ansi.Strip(wideLines[2]), "◆ proxyctl") {
		t.Fatalf("wide header and sidebar brand: %q %q %q", wideLines[0], wideLines[1], wideLines[2])
	}
	light, _ := m.Update(tea.BackgroundColorMsg{Color: color.White})
	if light.(dashboardModel).View().Content == wide {
		t.Fatal("terminal background did not change the palette")
	}
	if got := dashboardClickPage(120, 24, 2, 10); got != 3 {
		t.Fatalf("wide mouse target = %d, want 3", got)
	}
	if got := dashboardClickPage(35, 24, 18, 1); got != 3 {
		t.Fatalf("compact mouse target = %d, want 3", got)
	}
	m.width = 50
	if tabs := ansi.Strip(strings.Split(m.View().Content, "\n")[1]); !strings.Contains(tabs, "7设") {
		t.Fatalf("compact tabs hide settings: %q", tabs)
	}
	if got := dashboardClickPage(50, 24, 32, 1); got != 6 {
		t.Fatalf("compact settings mouse target = %d, want 6", got)
	}
	updated, _ := m.Update(tea.KeyPressMsg{Code: '2'})
	if got := updated.(dashboardModel).page; got != 1 {
		t.Fatalf("keyboard page = %d, want 1", got)
	}
	updated, _ = m.Update(tea.KeyPressMsg{Code: '7'})
	if got := updated.(dashboardModel).page; got != 6 {
		t.Fatalf("keyboard settings page = %d, want 6", got)
	}
}

func TestDashboardArrowsFollowNavigationFocus(t *testing.T) {
	m := newDashboardModel(context.Background())
	m.width, m.height, m.page = 120, 24, 2
	m.snapshot.profiles = []config.Profile{{Name: "one"}, {Name: "two"}}
	updated, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	m = updated.(dashboardModel)
	if m.page != 3 || !m.navFocused {
		t.Fatalf("sidebar arrow did not change page: page=%d focus=%v", m.page, m.navFocused)
	}
	updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyUp})
	m = updated.(dashboardModel)
	updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = updated.(dashboardModel)
	if m.page != 2 || m.navFocused {
		t.Fatalf("enter did not focus content: page=%d focus=%v", m.page, m.navFocused)
	}
	updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	m = updated.(dashboardModel)
	if m.page != 2 || m.profile != 1 {
		t.Fatalf("content arrow selected page=%d row=%d", m.page, m.profile)
	}
	updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEsc})
	m = updated.(dashboardModel)
	if !m.navFocused {
		t.Fatal("escape did not return to navigation")
	}
	m.width = 80
	updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyRight})
	if got := updated.(dashboardModel).page; got != 3 {
		t.Fatalf("top tab arrow selected page %d, want 3", got)
	}
}

func TestDashboardSidebarRatesAdaptToHeight(t *testing.T) {
	m := newDashboardModel(context.Background())
	m.width, m.height = dashboardWideWidth, 30
	m.snapshot.kernel = "mihomo"
	m.snapshot.service = "运行中"
	m.snapshot.profile = "很长的订阅名称会被截断"
	m.snapshot.shell = "已启用"
	m.traffic = dashboardTraffic{Up: 41472, Down: 128}
	m.trafficAvailable = true
	lines := strings.Split(m.View().Content, "\n")
	if len(lines) != 30 {
		t.Fatalf("line count = %d, want 30", len(lines))
	}
	for i, line := range lines {
		if lipgloss.Width(line) != dashboardWideWidth {
			t.Fatalf("line %d width = %d, want %d", i, lipgloss.Width(line), dashboardWideWidth)
		}
	}
	left := func(y int) string { return strings.Split(ansi.Strip(lines[y]), "│")[0] }
	if strings.TrimSpace(left(1)) != "" ||
		!strings.Contains(left(2), "◆ proxyctl") ||
		!strings.Contains(left(16), "7  设置") ||
		!strings.Contains(left(26), "↑ 40.5 KB/s") ||
		!strings.Contains(left(27), "↓ 128 B/s") ||
		strings.TrimSpace(left(28)) != "" {
		t.Fatalf("sidebar status placement: %q", lines)
	}
	for _, y := range []int{25, 26, 27, 28} {
		if strings.Contains(left(y), "内核") || strings.Contains(left(y), "mihomo") || strings.Contains(left(y), "订阅") || strings.Contains(left(y), "shell") {
			t.Fatalf("sidebar contains details outside rates: %q", left(y))
		}
	}
	if got := dashboardClickPage(dashboardWideWidth, 30, 2, 26); got != -1 {
		t.Fatalf("status area switched to page %d", got)
	}
	m.height = 24
	compactLines := strings.Split(ansi.Strip(m.View().Content), "\n")
	if !strings.Contains(compactLines[20], "↑ 40.5 KB/s") || !strings.Contains(compactLines[21], "↓ 128 B/s") ||
		strings.TrimSpace(strings.Split(compactLines[22], "│")[0]) != "" {
		t.Fatalf("compact sidebar status or bottom gap: %q", compactLines)
	}
	m.trafficAvailable = false
	if unavailable := ansi.Strip(m.View().Content); !strings.Contains(unavailable, "↑ —") || !strings.Contains(unavailable, "↓ —") {
		t.Fatalf("unavailable traffic was not marked: %q", unavailable)
	}
	m.height = 20
	if strings.Contains(ansi.Strip(m.View().Content), "──────────────") {
		t.Fatal("status block overlaps navigation on short terminals")
	}
	m.height, m.width = 24, dashboardWideWidth-1
	if strings.Contains(ansi.Strip(m.View().Content), "当前状态") {
		t.Fatal("status block appeared in top-tab layout")
	}
}

func TestDashboardNavigationFitsShortAndTinyTerminals(t *testing.T) {
	m := newDashboardModel(context.Background())
	m.page, m.width, m.height = dashboardSettingsPage, 120, 16
	shortLines := strings.Split(ansi.Strip(m.View().Content), "\n")
	if len(shortLines) != 16 || !strings.Contains(shortLines[1], "[7 设置]") || strings.Contains(shortLines[1], "│") {
		t.Fatalf("short wide terminal did not use top tabs: %q", shortLines)
	}
	if got := dashboardClickPage(120, 16, 2, 15); got != -1 {
		t.Fatalf("footer click switched to page %d", got)
	}
	if got := dashboardClickPage(120, 16, 51, 1); got != dashboardSettingsPage {
		t.Fatalf("short terminal settings tab = %d", got)
	}
	m.height = 18
	if tabs := strings.Split(ansi.Strip(m.View().Content), "\n")[1]; !strings.Contains(tabs, "[7 设置]") {
		t.Fatalf("18-row terminal did not keep top tabs: %q", tabs)
	}
	m.height = 19
	if brand := strings.Split(ansi.Strip(m.View().Content), "\n")[2]; !strings.Contains(brand, "◆ proxyctl") {
		t.Fatalf("19-row terminal did not switch to sidebar: %q", brand)
	}
	m.width, m.height = 30, 24
	if tabs := strings.Split(ansi.Strip(m.View().Content), "\n")[1]; !strings.Contains(tabs, "[7]") {
		t.Fatalf("tiny tabs hide selected page: %q", tabs)
	}
	if got := dashboardClickPage(30, 24, 19, 1); got != dashboardSettingsPage {
		t.Fatalf("tiny settings tab = %d", got)
	}
	m.width = 20
	if tabs := strings.Split(ansi.Strip(m.View().Content), "\n")[1]; !strings.Contains(tabs, "1234567") {
		t.Fatalf("bare digit tabs hide pages: %q", tabs)
	}
	if got := dashboardClickPage(20, 24, 6, 1); got != dashboardSettingsPage {
		t.Fatalf("bare digit settings tab = %d", got)
	}
}

func TestDashboardControllerReadsConfigAndSelectsNode(t *testing.T) {
	var gotAuth, gotPath, gotBody string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth, gotPath = r.Header.Get("Authorization"), r.URL.EscapedPath()
		if r.Method == http.MethodPut {
			body, _ := io.ReadAll(r.Body)
			gotBody = string(body)
			w.WriteHeader(http.StatusNoContent)
			return
		}
		_, _ = w.Write([]byte(`{"proxies":{"Group A":{"name":"Group A","type":"Selector","now":"Node 1","all":["Node 1","Node 2"]}}}`))
	}))
	defer server.Close()
	address := strings.TrimPrefix(server.URL, "http://")
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("external-controller: "+address+"\nsecret: top-secret\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	controller, err := controllerFromConfig(&config.KernelConfig{Name: "mihomo", ConfigFile: path})
	if err != nil {
		t.Fatal(err)
	}
	groups, _, err := controller.proxies(t.Context())
	if err != nil || len(groups) != 1 || groups[0].Now != "Node 1" || gotAuth != "Bearer top-secret" {
		t.Fatalf("read groups: %#v, auth=%q, err=%v", groups, gotAuth, err)
	}
	if err := controller.selectProxy(t.Context(), "Group A", "Node 2"); err != nil {
		t.Fatal(err)
	}
	if gotPath != "/proxies/Group%20A" || gotBody != `{"name":"Node 2"}` {
		t.Fatalf("selection path=%q body=%q", gotPath, gotBody)
	}
}

func TestDashboardControllerCanBeAbsent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte(`{"outbounds":[{"type":"direct"}]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	controller, err := controllerFromConfig(&config.KernelConfig{Name: "sing-box", ConfigFile: path})
	if err != nil || controller != nil {
		t.Fatalf("controller=%v err=%v", controller, err)
	}
}

func TestDashboardLogStreamRedactsURLs(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("{\"type\":\"info\",\"payload\":\"fetch https://user:pass@example.com/private?token=secret\"}\n"))
	}))
	defer server.Close()
	controller := &controllerClient{base: server.URL, client: dashboardHTTPClient}
	lines := make(chan string, 1)
	if err := controller.streamLogs(t.Context(), lines); err != nil {
		t.Fatal(err)
	}
	line := <-lines
	if !strings.Contains(line, "example.com") || strings.Contains(line, "private") || strings.Contains(line, "secret") {
		t.Fatalf("unsafe log line: %q", line)
	}
	if got := dashboardLine("one\ntwo", 12, false); strings.Contains(got, "\n") {
		t.Fatalf("control character entered layout: %q", got)
	}
}

func TestDashboardLinePreservesGraphemeClusters(t *testing.T) {
	for _, test := range []struct {
		value string
		width int
		want  string
	}{
		{"👨‍👩‍👦 node", 2, "👨‍👩‍👦"},
		{"👨‍👩‍👦 node", 1, " "},
		{"✈️ node", 2, "✈️"},
	} {
		got := dashboardLine(test.value, test.width, false)
		if got != test.want || lipgloss.Width(got) != test.width {
			t.Errorf("dashboardLine(%q, %d) = %q (width %d), want %q", test.value, test.width, got, lipgloss.Width(got), test.want)
		}
	}
}
