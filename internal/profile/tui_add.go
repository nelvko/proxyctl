package profile

import (
	"errors"
	"fmt"
	"net/url"

	"charm.land/huh/v2"
	"charm.land/lipgloss/v2"
	"github.com/nelvko/proxyctl/internal/config"
)

var descStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#767676"))

func initialForm(option *config.Profile) *huh.Form {
	return huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Source URL").
				Description(descStyle.Render("Supports http://, https://, and file:// URLs")).
				Value(&option.URL).
				Validate(validateSourceURL),
			huh.NewInput().
				Title("Profile Name").
				Description(descStyle.Render("Optional; defaults to the current Unix timestamp")).
				Validate(checkUniqueName).
				Value(&option.Name),
		),
	)
}

func validateSourceURL(raw string) error {
	if raw == "" {
		return errors.New("source URL cannot be empty")
	}
	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("invalid source URL: %w", err)
	}
	switch u.Scheme {
	case "http", "https", "file":
		return nil
	case "":
		return errors.New("source URL must include http://, https://, or file://")
	default:
		return fmt.Errorf("unsupported URL scheme %q", u.Scheme)
	}
}

func TuiAdd(option *config.Profile) error {
	if err := initialForm(option).Run(); err != nil {
		return err
	}
	return nil
}
