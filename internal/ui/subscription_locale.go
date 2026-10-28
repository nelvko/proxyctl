package ui

import (
	"slices"
	"strings"

	"github.com/nelvko/proxyctl/internal/config"
	"github.com/nelvko/proxyctl/internal/i18n"
	"github.com/nelvko/proxyctl/internal/redact"
	"github.com/nelvko/proxyctl/internal/subscription"
)

func (m model) tr(key string, args ...any) string { return i18n.Text(m.locale, key, args...) }

// Translate typed results at the TUI boundary. The CLI's stable output and
// domain errors continue to use their existing format.
func (m model) application(state subscription.ApplicationState) string {
	switch state {
	case subscription.Applied:
		return m.tr("sub.application.applied")
	case subscription.Staged:
		return m.tr("sub.application.staged")
	case subscription.ApplicationFailed:
		return m.tr("sub.application.failed")
	default:
		return ""
	}
}

func (m model) profileUpdate(result subscription.UpdateResult) string {
	message := string(result.Status)
	switch result.Status {
	case subscription.Updated:
		message = m.tr("sub.update.updated")
	case subscription.Unchanged:
		message = m.tr("sub.update.unchanged")
	case subscription.UpdateFailed:
		message = m.tr("sub.update.failed")
	}
	if application := m.application(result.Application); application != "" {
		message += "; " + application
	}
	return message
}

// Details use saved metadata only and retain the CLI's credential redaction.
func (m model) profileDetails(p config.Profile, selected string) string {
	boolean := func(value bool) string {
		if value {
			return m.tr("sub.yes")
		}
		return m.tr("sub.no")
	}
	format := p.Format
	if format == "" {
		format = "clash"
	}
	out := m.tr("sub.detail.metadata", p.Name, format, boolean(p.Name == selected), redact.URL(p.URL), p.File)
	if strings.HasPrefix(p.URL, "file://") {
		return out + m.tr("sub.detail.file_source")
	}
	route := "sub.detail.route_invalid"
	switch subscription.DownloadRoute(p.Download) {
	case "env":
		route = "sub.detail.route_env"
	case "direct":
		route = "sub.detail.route_direct"
	case "kernel":
		route = "sub.detail.route_kernel"
	}
	agent := redact.Text(subscription.DownloadUserAgent(p.Download))
	if p.Download.UserAgent == "" {
		agent += m.tr("sub.detail.default_agent")
		for key := range p.Download.Headers {
			if strings.EqualFold(key, "User-Agent") {
				agent = m.tr("sub.detail.header_agent")
			}
		}
	}
	headers := make([]string, 0, len(p.Download.Headers))
	for key := range p.Download.Headers {
		headers = append(headers, redact.Text(key))
	}
	slices.Sort(headers)
	return out + m.tr("sub.detail.download", subscription.DownloadTimeout(p.Download), m.tr(route), agent, boolean(!p.Download.SkipCert), headers)
}
