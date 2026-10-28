package ui

import (
	"errors"
	"fmt"
	"maps"
	"os"
	"slices"
	"strings"
	"time"

	"charm.land/bubbles/v2/key"
	"charm.land/huh/v2"
	"github.com/nelvko/proxyctl/internal/config"
	"github.com/nelvko/proxyctl/internal/i18n"
	"github.com/nelvko/proxyctl/internal/redact"
	"github.com/nelvko/proxyctl/internal/subscription"
)

// profileForm owns editable strings so invalid or failed submissions can be retried.
type profileForm struct {
	form             *huh.Form
	draft            *profile
	timeout, headers string
	advanced         bool
}

func newProfileForm(profiles *subscription.Service, draft *profile, originalName string) *profileForm {
	preference, _ := config.LoadLanguage()
	locale, _ := i18n.Resolve(preference)
	return newProfileFormLocale(profiles, draft, originalName, locale)
}

func newProfileFormLocale(profiles *subscription.Service, draft *profile, originalName string, locale i18n.Locale) *profileForm {
	tr := func(key string, args ...any) string { return i18n.Text(locale, key, args...) }
	f := &profileForm{draft: draft, timeout: draft.Download.Timeout.String()}
	var entries []string
	for name, value := range draft.Download.Headers {
		entries = append(entries, name+": "+value)
	}
	slices.Sort(entries)
	f.headers = strings.Join(entries, "\n")
	draft.Download.Headers = maps.Clone(draft.Download.Headers)
	draft.Download.DownloadVia = subscription.DownloadRoute(draft.Download)
	validateName := func(name string) error {
		if originalName != "" {
			if name == "" {
				return errors.New(tr("sub.form.name_empty"))
			}
			if name == originalName {
				return nil
			}
		}
		return profiles.CheckNameAvailable(name)
	}
	f.form = huh.NewForm(
		huh.NewGroup(
			huh.NewInput().Title(tr("sub.form.url")).Description(tr("sub.form.url_help")).Value(&draft.URL).Validate(subscription.ValidateSourceURL),
			huh.NewInput().Title(tr("sub.form.name")).Description(tr("sub.form.name_help")).Value(&draft.Name).Validate(validateName),
			huh.NewConfirm().Title(tr("sub.form.advanced")).Affirmative(tr("sub.yes")).Negative(tr("sub.no")).Value(&f.advanced),
		),
		huh.NewGroup(
			huh.NewSelect[string]().Title(tr("sub.form.route")).Options(huh.NewOption(tr("sub.form.route_env"), "env"), huh.NewOption(tr("sub.form.route_direct"), "direct"), huh.NewOption(tr("sub.form.route_kernel"), "kernel")).Value(&draft.Download.DownloadVia),
			huh.NewInput().Title(tr("sub.form.timeout")).Description(tr("sub.form.timeout_help", subscription.DefaultDownloadTimeout.String())).Value(&f.timeout).Validate(func(value string) error {
				d, err := time.ParseDuration(value)
				if err != nil {
					return errors.New(tr("sub.form.timeout_invalid"))
				}
				if d < 0 {
					return errors.New(tr("sub.form.timeout_negative"))
				}
				return nil
			}),
			huh.NewInput().Title(tr("sub.form.user_agent")).Description(tr("sub.form.user_agent_help")).Value(&draft.Download.UserAgent).Validate(func(value string) error {
				if strings.ContainsAny(value, "\r\n") {
					return errors.New(tr("sub.form.user_agent_invalid"))
				}
				return nil
			}),
			huh.NewConfirm().Title(tr("sub.form.skip_cert")).Description(tr("sub.form.skip_cert_help")).Affirmative(tr("sub.yes")).Negative(tr("sub.no")).Value(&draft.Download.SkipCert),
		).WithHideFunc(func() bool { return !f.advanced || strings.HasPrefix(draft.URL, "file://") }),
		huh.NewGroup(
			huh.NewText().Title(tr("sub.form.headers")).Description(tr("sub.form.headers_help")).Lines(4).Value(&f.headers).Validate(func(value string) error { _, err := parseFormHeaders(value); return err }),
		).WithHideFunc(func() bool { return !f.advanced || strings.HasPrefix(draft.URL, "file://") }),
	)
	themeConfig, _ := loadThemeConfig()
	f.form.WithTheme(profileHuhTheme(themeConfig)).WithKeyMap(profileFormKeyMap(locale))
	return f
}

// Keep Huh's navigation/validation behavior; translate only its help labels.
func profileFormKeyMap(locale i18n.Locale) *huh.KeyMap {
	km := huh.NewDefaultKeyMap()
	km.Quit = key.NewBinding(key.WithKeys("esc", "ctrl+c"), key.WithHelp("esc", i18n.Text(locale, "sub.cancel")))
	for _, binding := range []struct {
		binding *key.Binding
		key     string
	}{
		{&km.Input.AcceptSuggestion, "sub.form.complete"},
		{&km.Input.Next, "sub.next"}, {&km.Input.Prev, "sub.previous"}, {&km.Input.Submit, "sub.form.submit"},
		{&km.Text.Next, "sub.next"}, {&km.Text.Prev, "sub.previous"}, {&km.Text.Submit, "sub.form.submit"},
		{&km.Text.NewLine, "sub.form.newline"}, {&km.Text.Editor, "sub.form.editor"},
		{&km.Select.Next, "sub.form.select"}, {&km.Select.Prev, "sub.previous"}, {&km.Select.Submit, "sub.form.submit"},
		{&km.Select.Up, "sub.up"}, {&km.Select.Down, "sub.down"}, {&km.Select.Left, "sub.form.left"}, {&km.Select.Right, "sub.form.right"},
		{&km.Select.Filter, "sub.filter"}, {&km.Select.SetFilter, "sub.form.set_filter"}, {&km.Select.ClearFilter, "sub.form.clear_filter"},
		{&km.Select.HalfPageUp, "sub.form.half_page_up"}, {&km.Select.HalfPageDown, "sub.form.half_page_down"},
		{&km.Select.GotoTop, "sub.form.first"}, {&km.Select.GotoBottom, "sub.form.last"},
		{&km.Confirm.Next, "sub.next"}, {&km.Confirm.Prev, "sub.previous"}, {&km.Confirm.Submit, "sub.form.submit"},
		{&km.Confirm.Toggle, "sub.form.toggle"}, {&km.Confirm.Accept, "sub.yes"}, {&km.Confirm.Reject, "sub.no"},
	} {
		binding.binding.SetHelp(binding.binding.Help().Key, i18n.Text(locale, binding.key))
	}
	return km
}

func parseFormHeaders(value string) (map[string]string, error) {
	var lines []string
	for _, line := range strings.Split(value, "\n") {
		if strings.TrimSpace(line) != "" {
			lines = append(lines, line)
		}
	}
	return subscription.ParseHeaders(lines)
}

func (f *profileForm) apply() error {
	timeout, err := time.ParseDuration(f.timeout)
	if err != nil {
		return err
	}
	headers, err := parseFormHeaders(f.headers)
	if err != nil {
		return err
	}
	next := *f.draft
	next.Download.Timeout, next.Download.Headers = timeout, headers
	if err := subscription.ValidateSourceURL(next.URL); err != nil {
		return err
	}
	if err := subscription.ValidateDownloadSettings(next.Download); err != nil {
		return err
	}
	*f.draft = next
	return nil
}

func PromptAdd(profiles *subscription.Service, draft *profile) error {
	return promptProfile(profiles, draft, "")
}

func PromptModify(profiles *subscription.Service, draft *profile) error {
	return promptProfile(profiles, draft, draft.Name)
}

func promptProfile(profiles *subscription.Service, draft *profile, original string) error {
	f := newProfileForm(profiles, draft, original)
	for {
		if err := f.form.Run(); err != nil {
			return err
		}
		if err := f.apply(); err != nil {
			fmt.Fprintln(os.Stderr, redact.Error(err))
			previous := f
			f = newProfileForm(profiles, draft, original)
			f.timeout, f.headers, f.advanced = previous.timeout, previous.headers, previous.advanced
			continue
		}
		return nil
	}
}
