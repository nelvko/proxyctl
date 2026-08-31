package httpx

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	URL "net/url"
	"os"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// configuredMirrors holds mirror prefixes from the --mirror flag or the
// config file; the GH_PROXY environment variable overrides both.
var configuredMirrors []string

// SetMirrors records mirror prefixes to try in front of direct GitHub
// downloads; empty entries are dropped.
func SetMirrors(mirrors ...string) {
	configuredMirrors = slices.DeleteFunc(slices.Clone(mirrors), func(m string) bool {
		return strings.TrimSpace(m) == ""
	})
}

// client serves all proxyctl downloads. It stays http.DefaultClient (which
// honors HTTP(S)_PROXY) until the managed kernel's inbound is registered.
var client = http.DefaultClient

// Client returns the http.Client used for downloads, so callers resolving
// metadata (e.g. the GitHub API) share the same transport policy.
func Client() *http.Client {
	return client
}

// UseLocalKernel routes downloads through the managed kernel's inbound at
// addr — applied when the kernel is running but the shell carries no proxy
// environment of its own (fresh SSH, cron, scripts). An explicit shell
// proxy always wins over the managed kernel.
func UseLocalKernel(addr string) {
	if addr == "" {
		return
	}
	for _, k := range []string{"HTTP_PROXY", "http_proxy", "HTTPS_PROXY", "https_proxy"} {
		if os.Getenv(k) != "" {
			return
		}
	}
	t := http.DefaultTransport.(*http.Transport).Clone()
	t.Proxy = http.ProxyURL(&URL.URL{Scheme: "http", Host: addr})
	client = &http.Client{Transport: t}
}

// Mirrors returns the effective mirror list: the comma-separated GH_PROXY
// environment variable when set, else the configured one.
func Mirrors() []string {
	if env := os.Getenv("GH_PROXY"); env != "" {
		return strings.Split(env, ",")
	}
	return configuredMirrors
}

// MirrorCandidates returns the URLs to try for a GitHub artifact: each
// configured mirror's prefixed form, then the direct URL. The prefix is
// concatenated as a plain string on purpose: routing it through net/url
// would escape '?' and double-encode existing percent-escapes in the target.
// An invalid mirror value is a hard error — a typo must not silently swap
// the download source — while a loopback one (an HTTP proxy address, not a
// mirror prefix) is skipped with a warning.
func MirrorCandidates(raw string) ([]string, error) {
	list := Mirrors()
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
			warnOnce("proxyctl: ignoring mirror %q: it looks like an HTTP proxy address, not a mirror prefix (downloads honor HTTP_PROXY)\n", m)
			continue
		}
		if u.Scheme == "http" {
			warnOnce("proxyctl: warning: mirror %q is plain http; the transfer is not encrypted end-to-end\n", m)
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

// warnGHProxy prints a mirror notice to stderr at most once per process:
// a single command may resolve several proxied URLs (version probe + asset).
var warnGHProxy sync.Once

func warnOnce(format string, args ...any) {
	warnGHProxy.Do(func() {
		fmt.Fprintf(os.Stderr, format, args...)
	})
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

// Retry policy for Download. Vars rather than consts so tests can shrink
// the timings.
var (
	downloadAttempts = 3
	retryBackoff     = time.Second
	stallWindow      = 30 * time.Second
	// minTransferRate is the pace a single attempt must sustain: the attempt
	// budget scales with Content-Length but never drops below budgetFloor.
	minTransferRate = int64(64 << 10) // bytes per second
	budgetFloor     = 90 * time.Second
)

// statusError reports a non-2xx response and carries the URL, which the bare
// net/http error never mentions.
type statusError struct {
	code   int
	status string
	url    string
}

func (e *statusError) Error() string {
	return fmt.Sprintf("unexpected status %s for %s", e.status, e.url)
}

// Download fetches url into dst, retrying transient failures (network errors,
// stalls, 5xx/429) up to downloadAttempts times, and returns the number of
// bytes copied. A transfer that stops making progress for stallWindow fails
// even when the context budget is far from exhausted. Between attempts dst is
// rewound when it implements Reset() error or is an *os.File; other writers
// get a single attempt. onProgress, when non-nil, receives the response's
// Content-Length (-1 when unknown) and the byte count so far as the copy
// proceeds.
func Download(ctx context.Context, url string, dst io.Writer, onProgress func(total, n int64)) (int64, error) {
	var lastErr error
	for attempt := 1; attempt <= downloadAttempts; attempt++ {
		if attempt > 1 {
			select {
			case <-ctx.Done():
				return 0, ctx.Err()
			case <-time.After(retryBackoff << (attempt - 1)):
			}
		}

		n, err := fetch(ctx, url, dst, onProgress)
		if err == nil {
			return n, nil
		}
		lastErr = err
		if !retryable(err) || ctx.Err() != nil {
			return n, err
		}
		if !resetDst(dst) {
			return n, err // cannot rewind dst for a clean retry
		}
	}
	return 0, fmt.Errorf("download failed after %d attempts: %w", downloadAttempts, lastErr)
}

// resetDst rewinds dst for a clean retry when it knows how; it reports
// whether the retry can proceed.
func resetDst(dst io.Writer) bool {
	switch v := dst.(type) {
	case interface{ Reset() error }:
		return v.Reset() == nil
	case *os.File:
		if _, err := v.Seek(0, io.SeekStart); err != nil {
			return false
		}
		return v.Truncate(0) == nil
	}
	return false
}

// retryable reports whether another attempt is worth it: server-side status
// codes yes, definitive client-side ones (404, 403, …) no.
func retryable(err error) bool {
	var se *statusError
	if errors.As(err, &se) {
		return se.code >= 500 || se.code == http.StatusTooManyRequests
	}
	return true // network error, reset, stall
}

func fetch(ctx context.Context, url string, dst io.Writer, onProgress func(total, n int64)) (int64, error) {
	// Two watchdogs bound a single attempt independently of the overall
	// context budget: stallWindow of total silence, and a size-aware budget
	// so a dribbling connection cannot hold the download forever.
	stallCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	arm := time.AfterFunc(stallWindow, cancel)
	defer arm.Stop()
	var slow atomic.Bool

	req, err := http.NewRequestWithContext(stallCtx, http.MethodGet, url, nil)
	if err != nil {
		return 0, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return 0, classifyTransferErr(ctx, stallCtx, &slow, time.Duration(0), err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return 0, &statusError{code: resp.StatusCode, status: resp.Status, url: url}
	}

	total := resp.ContentLength
	budget := budgetFloor
	if sized := time.Duration(total / minTransferRate * int64(time.Second)); sized > budget {
		budget = sized
	}
	budgetTimer := time.AfterFunc(budget, func() {
		slow.Store(true)
		cancel()
	})
	defer budgetTimer.Stop()

	cw := &countingWriter{dst: dst, onProgress: onProgress, total: total}
	if _, err := io.Copy(cw, &watchdogReader{r: resp.Body, timer: arm}); err != nil {
		return cw.n, classifyTransferErr(ctx, stallCtx, &slow, budget, err)
	}
	return cw.n, nil
}

// classifyTransferErr names the real cause of a failed transfer: the overall
// budget running out, a dribble below the minimum rate, or a full stall.
func classifyTransferErr(ctx, stallCtx context.Context, slow *atomic.Bool, budget time.Duration, err error) error {
	switch {
	case ctx.Err() != nil:
		return ctx.Err() // overall budget exhausted (or caller canceled)
	case slow.Load():
		return fmt.Errorf("transfer too slow (exceeded %s budget, below %d KiB/s): %w", budget, minTransferRate>>10, err)
	case stallCtx.Err() != nil:
		return fmt.Errorf("stalled for %s: %w", stallWindow, err)
	}
	return err
}

// countingWriter forwards to dst while tracking the byte count and reporting
// progress.
type countingWriter struct {
	dst        io.Writer
	onProgress func(total, n int64)
	total      int64
	n          int64
}

func (c *countingWriter) Write(p []byte) (int, error) {
	n, err := c.dst.Write(p)
	c.n += int64(n)
	if c.onProgress != nil {
		c.onProgress(c.total, c.n)
	}
	return n, err
}

// watchdogReader re-arms the stall watchdog on every read so only genuine
// silence triggers it.
type watchdogReader struct {
	r     io.Reader
	timer *time.Timer
}

func (w *watchdogReader) Read(p []byte) (int, error) {
	n, err := w.r.Read(p)
	if n > 0 {
		w.timer.Reset(stallWindow)
	}
	return n, err
}
