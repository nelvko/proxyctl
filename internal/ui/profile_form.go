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
				return errors.New("profile name cannot be empty")
			}
			if name == originalName {
				return nil
			}
		}
		return profiles.CheckNameAvailable(name)
	}
	f.form = huh.NewForm(
		huh.NewGroup(
			huh.NewInput().Title("Source URL").Description("HTTP(S) or file://; file updates read the original path again").Value(&draft.URL).Validate(subscription.ValidateSourceURL),
			huh.NewInput().Title("Profile Name").Description("Optional on add; inferred from response or source · esc cancels").Value(&draft.Name).Validate(validateName),
			huh.NewConfirm().Title("Edit download settings?").Value(&f.advanced),
		),
		huh.NewGroup(
			huh.NewSelect[string]().Title("Download route").Options(huh.NewOption("Environment proxy settings", "env"), huh.NewOption("Direct (no explicit proxy)", "direct"), huh.NewOption("Running kernel (required)", "kernel")).Value(&draft.Download.DownloadVia),
			huh.NewInput().Title("Timeout").Description("Includes retries, excludes validation; 0 uses "+subscription.DefaultDownloadTimeout.String()).Value(&f.timeout).Validate(func(value string) error {
				d, err := time.ParseDuration(value)
				if err != nil {
					return errors.New("use a duration such as 30s or 1m; 0 restores the default")
				}
				if d < 0 {
					return errors.New("timeout cannot be negative")
				}
				return nil
			}),
			huh.NewInput().Title("User-Agent").Description("Empty restores the default").Value(&draft.Download.UserAgent).Validate(func(value string) error {
				if strings.ContainsAny(value, "\r\n") {
					return errors.New("user agent cannot contain newlines")
				}
				return nil
			}),
			huh.NewConfirm().Title("Skip HTTPS certificate verification?").Description("Subscription download only; normally leave off").Value(&draft.Download.SkipCert),
		).WithHideFunc(func() bool { return !f.advanced || strings.HasPrefix(draft.URL, "file://") }),
		huh.NewGroup(
			huh.NewText().Title("Request headers").Description("One Name: value per line; clear a line to remove its header").Lines(4).Value(&f.headers).Validate(func(value string) error { _, err := parseFormHeaders(value); return err }),
		).WithHideFunc(func() bool { return !f.advanced || strings.HasPrefix(draft.URL, "file://") }),
	)
	return f
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
	km := huh.NewDefaultKeyMap()
	km.Quit = key.NewBinding(key.WithKeys("esc", "ctrl+c"), key.WithHelp("esc", "cancel"))
	f := newProfileForm(profiles, draft, original)
	for {
		if err := f.form.WithKeyMap(km).Run(); err != nil {
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
