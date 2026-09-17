package ui

import (
	"fmt"
	"slices"
	"strings"

	"github.com/nelvko/proxyctl/internal/config"
	"github.com/nelvko/proxyctl/internal/redact"
	"github.com/nelvko/proxyctl/internal/subscription"
)

// ProfileDetails renders saved metadata only; it never probes a service or source.
func ProfileDetails(p config.Profile, selected string, showURL bool) string {
	source := redact.URL(p.URL)
	if showURL {
		source = p.URL
	}
	out := fmt.Sprintf("name:       %s\nselected:   %t\nsource:     %s\ncache:      %s\n", p.Name, p.Name == selected, source, p.File)
	if strings.HasPrefix(p.URL, "file://") {
		return out + "source type: linked local file (update reads the original path again)\ndownload:   HTTP settings do not apply to file sources\n"
	}
	route := map[string]string{
		"env":    "environment proxy settings (direct when none apply)",
		"direct": "direct (no explicit proxy; OS routing still applies)",
		"kernel": "running kernel (required)",
	}[subscription.DownloadRoute(p.Download)]
	if route == "" {
		route = "invalid saved download route"
	}
	ua := redact.Text(subscription.DownloadUserAgent(p.Download))
	if p.Download.UserAgent == "" {
		ua += " (default; not customized)"
		for key := range p.Download.Headers {
			if strings.EqualFold(key, "User-Agent") {
				ua = "configured through request header (value hidden)"
			}
		}
	}
	headers := make([]string, 0, len(p.Download.Headers))
	for key := range p.Download.Headers {
		headers = append(headers, redact.Text(key))
	}
	slices.Sort(headers)
	return out + fmt.Sprintf("timeout:    %s (download including retries; excludes validation)\nroute:      %s (configured policy)\nuser-agent: %s\nverify TLS: %t (HTTPS subscription download only)\nheaders:    %v\n", subscription.DownloadTimeout(p.Download), route, ua, !p.Download.SkipCert, headers)
}

func Application(state subscription.ApplicationState) string {
	switch state {
	case subscription.Applied:
		return "applied; kernel restarted"
	case subscription.Staged:
		return "staged; kernel remains stopped"
	case subscription.ApplicationFailed:
		return "application failed; cache retained for retry"
	default:
		return ""
	}
}
func ProfileUpdate(result subscription.UpdateResult) string {
	message := string(result.Status)
	if application := Application(result.Application); application != "" {
		message += "; " + application
	}
	return message
}
