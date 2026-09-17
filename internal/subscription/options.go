package subscription

import (
	"errors"
	"maps"
	"net/http"
	"strings"
	"time"
	"unicode"

	"github.com/nelvko/proxyctl/internal/config"
)

const DefaultDownloadTimeout = 10 * time.Second

// Keep the existing Go HTTP user agent, but make the request and display agree.
const DefaultDownloadUserAgent = "Go-http-client/1.1"

func DownloadTimeout(u config.DownloadConfig) time.Duration {
	if u.Timeout == 0 {
		return DefaultDownloadTimeout
	}
	return u.Timeout
}

func DownloadRoute(u config.DownloadConfig) string {
	if u.DownloadVia == "" {
		return "env"
	}
	return u.DownloadVia
}

func DownloadUserAgent(u config.DownloadConfig) string {
	if u.UserAgent != "" {
		return u.UserAgent
	}
	for name, value := range u.Headers {
		if strings.EqualFold(name, "User-Agent") {
			return value
		}
	}
	return DefaultDownloadUserAgent
}

func validHeaderName(name string) bool {
	return name != "" && !strings.ContainsFunc(name, func(r rune) bool {
		return r > 127 || !(unicode.IsLetter(r) || unicode.IsDigit(r) || strings.ContainsRune("!#$%&'*+-.^_`|~", r))
	})
}

func ValidateDownloadSettings(u config.DownloadConfig) error {
	if u.Timeout < 0 {
		return errors.New("download timeout cannot be negative")
	}
	switch DownloadRoute(u) {
	case "env", "direct", "kernel":
	default:
		return errors.New("download route must be env, direct or kernel")
	}
	if strings.ContainsAny(u.UserAgent, "\r\n") {
		return errors.New("user agent cannot contain newlines")
	}
	seen := make(map[string]bool)
	for name, value := range u.Headers {
		if !validHeaderName(name) || strings.ContainsAny(value, "\r\n") {
			return errors.New("invalid HTTP header name or value")
		}
		key := http.CanonicalHeaderKey(name)
		if seen[key] {
			return errors.New("duplicate HTTP header name (case insensitive)")
		}
		seen[key] = true
		if key == "User-Agent" && u.UserAgent != "" {
			return errors.New("use either user-agent or a User-Agent header, not both")
		}
	}
	return nil
}

// ParseHeaders accepts one Name: value per entry. Values may contain colons.
func ParseHeaders(entries []string) (map[string]string, error) {
	headers := make(map[string]string, len(entries))
	for _, entry := range entries {
		name, value, ok := strings.Cut(entry, ":")
		name = strings.TrimSpace(name)
		if !ok || !validHeaderName(name) || strings.ContainsAny(value, "\r\n") {
			return nil, errors.New("headers must use Name: value with no newlines")
		}
		key := http.CanonicalHeaderKey(name)
		if _, exists := headers[key]; exists {
			return nil, errors.New("duplicate HTTP header name (case insensitive)")
		}
		headers[key] = strings.TrimSpace(value)
	}
	return headers, nil
}

// Pointer fields distinguish omitted settings from explicit zero/false/empty.
type DownloadOptions struct {
	Route, UserAgent *string
	Timeout          *time.Duration
	Insecure         *bool
	Headers          map[string]string
	RemoveHeaders    []string
}

func (o DownloadOptions) Apply(u config.DownloadConfig) (config.DownloadConfig, error) {
	if o.Route != nil {
		u.DownloadVia = *o.Route
	}
	if o.UserAgent != nil {
		u.UserAgent = *o.UserAgent
	}
	if o.Timeout != nil {
		u.Timeout = *o.Timeout
	}
	if o.Insecure != nil {
		u.SkipCert = *o.Insecure
	}
	u.Headers = maps.Clone(u.Headers)
	for _, name := range o.RemoveHeaders {
		if !validHeaderName(name) {
			return u, errors.New("invalid header name to remove")
		}
		for existing := range u.Headers {
			if strings.EqualFold(existing, name) {
				delete(u.Headers, existing)
			}
		}
	}
	for name, value := range o.Headers {
		for existing := range u.Headers {
			if strings.EqualFold(existing, name) {
				delete(u.Headers, existing)
			}
		}
		if u.Headers == nil {
			u.Headers = make(map[string]string)
		}
		u.Headers[http.CanonicalHeaderKey(name)] = value
	}
	return u, ValidateDownloadSettings(u)
}
