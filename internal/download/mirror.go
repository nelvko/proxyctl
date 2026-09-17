package download

import (
	"fmt"
	"net"
	URL "net/url"
	"slices"
	"strings"
)

// MirrorCandidates returns the URLs to try for a GitHub artifact: each
// configured mirror's prefixed form, then the direct URL. The prefix is
// concatenated as a plain string on purpose: routing it through net/url
// would escape '?' and double-encode existing percent-escapes in the target.
// An invalid mirror value is a hard error — a typo must not silently swap
// the download source — while a loopback one (an HTTP proxy address, not a
// mirror prefix) is skipped with a warning.
func (d *Downloader) MirrorCandidates(raw string) ([]string, error) {
	list := d.Mirrors()
	candidates := make([]string, 0, len(list)+1)
	for _, m := range list {
		m = strings.TrimSpace(m)
		if m == "" {
			continue
		}
		u, err := URL.Parse(m)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
			return nil, fmt.Errorf("mirror %q is not an absolute http(s) URL (expected e.g. https://ghfast.top)", m)
		}
		if isLoopbackHost(u) {
			// A mirror is a URL prefix, not an HTTP proxy address: downloads
			// already honor HTTP_PROXY/HTTPS_PROXY, so after `proxyctl on`
			// they use the local kernel without one.
			d.warnLoopbackOnce("proxyctl: ignoring mirror %q: it looks like an HTTP proxy address, not a mirror prefix (downloads honor HTTP_PROXY)\n", m)
			continue
		}
		if u.Scheme == "http" {
			d.warnPlainHTTPOnce("proxyctl: warning: mirror %q is plain http; the transfer is not encrypted end-to-end\n", m)
		}
		prefixed := strings.TrimSuffix(m, "/") + "/" + raw
		if !slices.Contains(candidates, prefixed) {
			candidates = append(candidates, prefixed)
		}
	}
	// Direct last: a dead mirror must not sink the download when github.com
	// itself is reachable — and vice versa.
	if !slices.Contains(candidates, raw) {
		candidates = append(candidates, raw)
	}
	return candidates, nil
}

func (d *Downloader) warnLoopbackOnce(format string, args ...any) {
	d.warnLoopback.Do(func() { d.Message(true, format, args...) })
}

func (d *Downloader) warnPlainHTTPOnce(format string, args ...any) {
	d.warnPlainHTTP.Do(func() { d.Message(true, format, args...) })
}

// isLoopbackHost reports whether u points at the local machine — almost
// certainly the running kernel's inbound, which is an HTTP proxy rather than
// a prefix mirror.
func isLoopbackHost(u *URL.URL) bool {
	h := u.Hostname()
	if h == "localhost" {
		return true
	}
	ip := net.ParseIP(h)
	return ip != nil && ip.IsLoopback()
}
