package download

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"sync/atomic"
	"time"

	"github.com/nelvko/proxyctl/internal/redact"
)

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
	return fmt.Sprintf("unexpected status %s for %s", e.status, redact.URL(e.url))
}

// RequestOptions configures one download without changing the shared client.
type RequestOptions struct {
	Headers    http.Header
	UserAgent  string
	SkipCert   bool
	OnResponse func(http.Header)
}

// Fetch downloads url into dst, retrying transient failures (network errors,
// stalls, 5xx/429) up to downloadAttempts times, and returns the number of
// bytes copied. A transfer that stops making progress for stallWindow fails
// even when the context budget is far from exhausted. Between attempts dst is
// rewound when it implements Reset() error or is an *os.File; other writers
// get a single attempt. onProgress, when non-nil, receives the response's
// Content-Length (-1 when unknown) and the byte count so far as the copy
// proceeds.
func (d *Downloader) Fetch(ctx context.Context, url string, dst io.Writer, onProgress func(total, n int64), opts RequestOptions) (int64, error) {
	local := *d.client
	if opts.SkipCert {
		transport := local.Transport
		if transport == nil {
			transport = http.DefaultTransport
		}
		base, ok := transport.(*http.Transport)
		if !ok {
			return 0, errors.New("TLS options require an HTTP transport")
		}
		t := base.Clone()
		if t.TLSClientConfig == nil {
			t.TLSClientConfig = &tls.Config{}
		} else {
			t.TLSClientConfig = t.TLSClientConfig.Clone()
		}
		t.TLSClientConfig.InsecureSkipVerify = true // explicit per-profile setting
		local.Transport = t
		defer t.CloseIdleConnections()
	}
	previousRedirect := local.CheckRedirect
	local.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if len(via) > 0 && (req.URL.Host != via[0].URL.Host || req.URL.Scheme != via[0].URL.Scheme) {
			for key := range opts.Headers {
				req.Header.Del(key)
			}
		}
		if previousRedirect != nil {
			return previousRedirect(req, via)
		}
		if len(via) >= 10 {
			return errors.New("stopped after 10 redirects")
		}
		return nil
	}
	cl := &local
	var lastErr error
	for attempt := 1; attempt <= downloadAttempts; attempt++ {
		if attempt > 1 {
			select {
			case <-ctx.Done():
				return 0, ctx.Err()
			case <-time.After(retryBackoff << (attempt - 1)):
			}
		}

		n, err := fetch(cl, ctx, url, dst, onProgress, opts)
		if err == nil {
			return n, nil
		}
		lastErr = redact.Error(err)
		if !retryable(err) || ctx.Err() != nil {
			return n, redact.Error(err)
		}
		if !resetDst(dst) {
			return n, redact.Error(err) // cannot rewind dst for a clean retry
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

func fetch(cl *http.Client, ctx context.Context, url string, dst io.Writer, onProgress func(total, n int64), opts RequestOptions) (int64, error) {
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
	req.Header = opts.Headers.Clone()
	if req.Header == nil {
		req.Header = make(http.Header)
	}
	if opts.UserAgent != "" {
		req.Header.Set("User-Agent", opts.UserAgent)
	}
	resp, err := cl.Do(req)
	if err != nil {
		return 0, classifyTransferErr(ctx, stallCtx, &slow, time.Duration(0), err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return 0, &statusError{code: resp.StatusCode, status: resp.Status, url: url}
	}

	if opts.OnResponse != nil {
		opts.OnResponse(resp.Header.Clone())
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
