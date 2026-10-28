package ui

import (
	"runtime"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/nelvko/proxyctl/internal/config"
	"github.com/nelvko/proxyctl/internal/i18n"
	"github.com/nelvko/proxyctl/internal/subscription"
)

func TestProfileFormExplicitLocaleAndConfirmationLabels(t *testing.T) {
	for _, test := range []struct {
		locale         i18n.Locale
		title, yes, no string
	}{
		{i18n.Chinese, "来源 URL", "是", "否"},
		{i18n.English, "Source URL", "Yes", "No"},
	} {
		t.Run(string(test.locale), func(t *testing.T) {
			draft := profile{URL: "https://example.com", Name: "office"}
			f := newProfileFormLocale(subscription.NewService(&config.SubscriptionConfig{}, nil), &draft, "", test.locale)
			f.form.WithWidth(80).WithHeight(24)
			f.form.Init()
			view := ansi.Strip(f.form.View())
			for _, text := range []string{test.title, test.yes, test.no} {
				if !strings.Contains(view, text) {
					t.Fatalf("locale %q form missing %q: %q", test.locale, text, view)
				}
			}
			if draft.URL != "https://example.com" || draft.Name != "office" || f.advanced {
				t.Fatal("localizing a form changed its editable values")
			}
		})
	}
	// Each form owns its help translations; creating a Chinese form cannot
	// change navigation or labels in an English form.
	english, chinese := profileFormKeyMap(i18n.English), profileFormKeyMap(i18n.Chinese)
	if english.Input.Next.Help().Desc != "Next" || chinese.Input.Next.Help().Desc != "下一步" {
		t.Fatal("form help did not use its own locale")
	}
	if strings.Join(english.Input.Next.Keys(), ",") != "enter,tab" || strings.Join(chinese.Quit.Keys(), ",") != "esc,ctrl+c" {
		t.Fatal("translation changed form key bindings")
	}
}

func TestProfileFormWrapperUsesSavedLanguage(t *testing.T) {
	if runtime.GOOS == "darwin" {
		t.Skip("macOS ignores XDG_CONFIG_HOME; run this isolated persistence test on Linux or Windows")
	}
	root := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", root)
	t.Setenv("AppData", root)
	t.Setenv("LC_ALL", "zh_CN.UTF-8")
	if err := config.SaveLanguage(t.Context(), "en"); err != nil {
		t.Fatal(err)
	}
	svc := subscription.NewService(&config.SubscriptionConfig{}, nil)
	draft := profile{}
	f := newProfileForm(svc, &draft, "")
	f.form.WithWidth(80).WithHeight(24)
	f.form.Init()
	if view := ansi.Strip(f.form.View()); !strings.Contains(view, "Source URL") {
		t.Fatalf("CLI prompt did not use saved language: %q", view)
	}
	f = newProfileFormLocale(svc, &draft, "", i18n.Chinese)
	f.form.WithWidth(80).WithHeight(24)
	f.form.Init()
	if view := ansi.Strip(f.form.View()); !strings.Contains(view, "来源 URL") {
		t.Fatalf("saved language overrode explicit TUI locale: %q", view)
	}
}

func TestTUIProfileDetailsLocalizedAndRedacted(t *testing.T) {
	p := profile{Name: "office", URL: "https://user:password@example.org/private-path?token=private-token", Download: config.DownloadConfig{Headers: map[string]string{"Authorization": "private-secret", "User-Agent": "private-agent"}}}
	for _, test := range []struct {
		locale                i18n.Locale
		label, hidden, source string
	}{
		{i18n.Chinese, "当前使用：是", "值已隐藏", "关联的本地文件"},
		{i18n.English, "Selected:   Yes", "value hidden", "linked local file"},
	} {
		m := model{locale: test.locale}
		view := m.profileDetails(p, "office")
		for _, secret := range []string{"password", "private-path", "private-token", "private-secret", "private-agent"} {
			if strings.Contains(view, secret) {
				t.Fatalf("locale %q details leaked %q: %q", test.locale, secret, view)
			}
		}
		for _, expected := range []string{test.label, test.hidden, subscription.DefaultDownloadTimeout.String(), "Authorization"} {
			if !strings.Contains(view, expected) {
				t.Fatalf("locale %q details missing %q: %q", test.locale, expected, view)
			}
		}
		local := p
		local.URL = "file:///private/source.yaml"
		view = m.profileDetails(local, "")
		if !strings.Contains(view, test.source) || strings.Contains(view, "User-Agent") || strings.Contains(view, "Authorization") {
			t.Fatalf("local source displayed HTTP preferences: %q", view)
		}
	}
}

func TestTUIResultTranslationLeavesCLIOutputStable(t *testing.T) {
	result := subscription.UpdateResult{Status: subscription.Updated, Application: subscription.Staged}
	if got := (model{locale: i18n.Chinese}).profileUpdate(result); !strings.Contains(got, "已更新") || !strings.Contains(got, "已暂存") {
		t.Fatal(got)
	}
	if got := (model{locale: i18n.English}).profileUpdate(result); got != "updated; staged; kernel remains stopped" {
		t.Fatal(got)
	}
	if got := ProfileUpdate(result); got != "updated; staged; kernel remains stopped" {
		t.Fatalf("CLI output changed: %q", got)
	}
	if got := (model{locale: i18n.Chinese}).application(subscription.ApplicationState("future")); got != "" {
		t.Fatalf("unknown application state acquired a false status: %q", got)
	}
}
