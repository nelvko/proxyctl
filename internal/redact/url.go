// Package redact removes sensitive URL components and unsafe control characters from human-readable output.
package redact

import (
	"net/url"
	"regexp"
	"strings"
	"unicode"
)

// URL hides userinfo, paths, query strings and fragments. Subscription tokens
// commonly live in paths as well as query parameters.
func URL(raw string) string {
	if raw == "" {
		return ""
	}
	u, err := url.Parse(raw)
	if err != nil || u.Scheme == "" {
		return "[redacted URL]"
	}
	if u.Scheme == "file" {
		return "file://[local source]"
	}
	if u.Host == "" {
		return "[redacted URL]"
	}
	result := u.Scheme + "://" + u.Host
	if u.Path != "" && u.Path != "/" {
		result += "/…"
	}
	if u.RawQuery != "" || u.ForceQuery {
		result += "?[redacted]"
	}
	if u.Fragment != "" {
		result += "#[redacted]"
	}
	return clean(result)
}

var urlPattern = regexp.MustCompile(`(?:https?|socks5h?|file)://[^\s"'<>]+`)

func Text(value string) string { return clean(urlPattern.ReplaceAllStringFunc(value, URL)) }
func clean(value string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsControl(r) && r != '\n' && r != '\t' {
			return -1
		}
		return r
	}, value)
}

type safeError struct{ cause error }

func (e safeError) Error() string { return Text(e.cause.Error()) }
func (e safeError) Unwrap() error { return e.cause }
func Error(err error) error {
	if err == nil {
		return nil
	}
	return safeError{err}
}
