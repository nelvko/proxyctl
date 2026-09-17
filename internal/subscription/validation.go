package subscription

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
)

// ValidateSourceURL checks supported remote and linked local sources.
func ValidateSourceURL(raw string) error {
	if raw == "" {
		return errors.New("source URL cannot be empty")
	}
	u, err := url.Parse(raw)
	if err != nil {
		return errors.New("invalid source URL")
	}
	switch u.Scheme {
	case "http", "https":
		if u.Hostname() == "" {
			return errors.New("HTTP source URL requires a hostname")
		}
		return nil
	case "file":
		if u.Host != "" && u.Host != "localhost" {
			return errors.New("file source must be local")
		}
		if !strings.HasPrefix(u.Path, "/") {
			return errors.New("file source requires an absolute path")
		}
		return nil
	case "":
		return errors.New("source URL must include http://, https://, or file://")
	default:
		return fmt.Errorf("unsupported URL scheme %q", u.Scheme)
	}
}
