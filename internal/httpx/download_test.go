package httpx

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

func TestMirrorCandidates(t *testing.T) {
	const target = "https://github.com/MetaCubeX/mihomo/releases/download/v1.20.0/mihomo-linux-amd64-v1.20.0.gz"

	t.Run("no mirror configured returns only the direct URL", func(t *testing.T) {
		t.Setenv("GH_PROXY", "")
		SetMirrors()
		got, err := MirrorCandidates(target)
		if err != nil {
			t.Fatalf("MirrorCandidates() error = %v", err)
		}
		if len(got) != 1 || got[0] != target {
			t.Fatalf("MirrorCandidates() = %v, want [%q]", got, target)
		}
	})

	t.Run("mirror env prefixes the target, direct kept as fallback", func(t *testing.T) {
		t.Setenv("GH_PROXY", "https://mirror.example.com/gh")
		got, err := MirrorCandidates(target)
		if err != nil {
			t.Fatalf("MirrorCandidates() error = %v", err)
		}
		want := []string{"https://mirror.example.com/gh/" + target, target}
		if len(got) != 2 || got[0] != want[0] || got[1] != want[1] {
			t.Fatalf("MirrorCandidates() = %v, want %v", got, want)
		}
	})

	t.Run("trailing slash is not doubled", func(t *testing.T) {
		t.Setenv("GH_PROXY", "https://mirror.example.com/gh/")
		got, err := MirrorCandidates(target)
		if err != nil {
			t.Fatalf("MirrorCandidates() error = %v", err)
		}
		if got[0] != "https://mirror.example.com/gh/"+target {
			t.Fatalf("MirrorCandidates()[0] = %q, want single slash", got[0])
		}
	})

	t.Run("query and percent-escapes survive verbatim", func(t *testing.T) {
		t.Setenv("GH_PROXY", "https://mirror.example.com")
		const escaped = "https://github.com/a/b/releases/download/v1/f%20name.gz?download=1"
		got, err := MirrorCandidates(escaped)
		if err != nil {
			t.Fatalf("MirrorCandidates() error = %v", err)
		}
		if got[0] != "https://mirror.example.com/"+escaped {
			t.Fatalf("MirrorCandidates()[0] = %q, want verbatim %q", got[0], escaped)
		}
	})

	t.Run("comma-separated env yields every mirror then direct", func(t *testing.T) {
		t.Setenv("GH_PROXY", "https://m1.example.com, https://m2.example.com/gh")
		got, err := MirrorCandidates(target)
		if err != nil {
			t.Fatalf("MirrorCandidates() error = %v", err)
		}
		want := []string{
			"https://m1.example.com/" + target,
			"https://m2.example.com/gh/" + target,
			target,
		}
		if len(got) != len(want) {
			t.Fatalf("MirrorCandidates() = %v, want %v", got, want)
		}
		for i := range want {
			if got[i] != want[i] {
				t.Fatalf("MirrorCandidates()[%d] = %q, want %q", i, got[i], want[i])
			}
		}
	})

	t.Run("SetMirrors is used when the env is unset", func(t *testing.T) {
		t.Setenv("GH_PROXY", "")
		SetMirrors("https://cfg-mirror.example.com")
		t.Cleanup(func() { SetMirrors() })
		got, err := MirrorCandidates(target)
		if err != nil {
			t.Fatalf("MirrorCandidates() error = %v", err)
		}
		if got[0] != "https://cfg-mirror.example.com/"+target {
			t.Fatalf("MirrorCandidates()[0] = %q, want the configured mirror", got[0])
		}
	})

	t.Run("invalid mirror is an error", func(t *testing.T) {
		t.Setenv("GH_PROXY", "://not-a-url")
		if _, err := MirrorCandidates(target); err == nil {
			t.Fatal("MirrorCandidates() succeeded with an unparseable mirror; want error")
		}
	})

	t.Run("scheme-less mirror is an error", func(t *testing.T) {
		t.Setenv("GH_PROXY", "mirror.example.com/https://github.com")
		_, err := MirrorCandidates(target)
		if err == nil || !strings.Contains(err.Error(), "mirror") {
			t.Fatalf("error = %v, want an error naming the mirror", err)
		}
	})

	t.Run("non-http mirror scheme is an error", func(t *testing.T) {
		t.Setenv("GH_PROXY", "socks5://127.0.0.1:7890")
		if _, err := MirrorCandidates(target); err == nil {
			t.Fatal("MirrorCandidates() succeeded with a socks5 mirror; want error")
		}
	})

	t.Run("loopback mirror is skipped, direct remains", func(t *testing.T) {
		t.Setenv("GH_PROXY", "http://127.0.0.1:7890")
		got, err := MirrorCandidates(target)
		if err != nil {
			t.Fatalf("MirrorCandidates() error = %v", err)
		}
		if len(got) != 1 || got[0] != target {
			t.Fatalf("MirrorCandidates() = %v, want only the direct URL", got)
		}
	})
}

func TestUseLocalKernel(t *testing.T) {
	t.Cleanup(func() { client = http.DefaultClient })

	t.Run("without shell proxy the kernel inbound takes over", func(t *testing.T) {
		for _, k := range []string{"HTTP_PROXY", "http_proxy", "HTTPS_PROXY", "https_proxy"} {
			t.Setenv(k, "")
		}
		UseLocalKernel("127.0.0.1:7890")
		if Client() == http.DefaultClient {
			t.Fatal("Client() still the default; want the kernel inbound transport")
		}
	})

	t.Run("an explicit shell proxy wins", func(t *testing.T) {
		client = http.DefaultClient
		t.Setenv("HTTP_PROXY", "http://127.0.0.1:8888")
		UseLocalKernel("127.0.0.1:7890")
		if Client() != http.DefaultClient {
			t.Fatal("UseLocalKernel overrode an explicit shell proxy")
		}
	})

	t.Run("empty address is a no-op", func(t *testing.T) {
		client = http.DefaultClient
		UseLocalKernel("")
		if Client() != http.DefaultClient {
			t.Fatal("UseLocalKernel(\"\") changed the client")
		}
	})
}

// shrinkTimings collapses the retry policy so failure-path tests stay fast.
func shrinkTimings(t *testing.T) {
	t.Helper()
	origAttempts, origBackoff, origStall := downloadAttempts, retryBackoff, stallWindow
	downloadAttempts, retryBackoff, stallWindow = 3, time.Millisecond, 50*time.Millisecond
	t.Cleanup(func() {
		downloadAttempts, retryBackoff, stallWindow = origAttempts, origBackoff, origStall
	})
}

func TestDownloadRetriesTransientFailures(t *testing.T) {
	shrinkTimings(t)

	hits := 0
	srv := newTestServer(func() (int, string) {
		hits++
		if hits < 3 {
			return 502, ""
		}
		return 200, "payload"
	})
	defer srv.Close()

	f := tempFile(t)
	n, err := Download(t.Context(), srv.URL, f, nil)
	if err != nil {
		t.Fatalf("Download() error = %v", err)
	}
	if hits != 3 {
		t.Fatalf("server hit %d times, want 3 (two 502s then a 200)", hits)
	}
	if n != int64(len("payload")) {
		t.Fatalf("Download() copied %d bytes, want %d", n, len("payload"))
	}
	if got := readAll(t, f); got != "payload" {
		t.Fatalf("content = %q, want %q", got, "payload")
	}
}

func TestDownloadDoesNotRetryClientErrors(t *testing.T) {
	shrinkTimings(t)

	hits := 0
	srv := newTestServer(func() (int, string) {
		hits++
		return 404, ""
	})
	defer srv.Close()

	f := tempFile(t)
	_, err := Download(t.Context(), srv.URL, f, nil)
	if err == nil {
		t.Fatal("Download() succeeded on a 404; want error")
	}
	if !strings.Contains(err.Error(), srv.URL) {
		t.Fatalf("error %q does not name the URL", err)
	}
	if hits != 1 {
		t.Fatalf("server hit %d times, want 1 (404 is not retryable)", hits)
	}
}

func TestDownloadDetectsStall(t *testing.T) {
	shrinkTimings(t)
	downloadAttempts = 1

	// Sends a header and one byte, then goes silent until the client hangs up.
	srv := newStallingServer()
	defer srv.Close()

	f := tempFile(t)
	_, err := Download(t.Context(), srv.URL, f, nil)
	if err == nil {
		t.Fatal("Download() succeeded on a stalled transfer; want error")
	}
	if !strings.Contains(err.Error(), "stalled") {
		t.Fatalf("error = %v, want a stall error", err)
	}
}

func TestDownloadKillsSlowDribble(t *testing.T) {
	shrinkTimings(t)
	downloadAttempts = 1
	origRate, origFloor := minTransferRate, budgetFloor
	minTransferRate, budgetFloor = 1<<30, 150*time.Millisecond
	t.Cleanup(func() { minTransferRate, budgetFloor = origRate, origFloor })

	// Dribbles one byte every 20ms: flowing, so the stall watchdog never
	// fires, but far below the budget's implied rate.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "1000")
		for i := 0; i < 1000; i++ {
			_, _ = w.Write([]byte("x"))
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
			select {
			case <-time.After(20 * time.Millisecond):
			case <-r.Context().Done():
				return
			}
		}
	}))
	defer srv.Close()

	f := tempFile(t)
	_, err := Download(t.Context(), srv.URL, f, nil)
	if err == nil {
		t.Fatal("Download() waited out a dribble; want error")
	}
	if !strings.Contains(err.Error(), "too slow") {
		t.Fatalf("error = %v, want a too-slow budget error", err)
	}
}

func TestDownloadRewindsFileBetweenAttempts(t *testing.T) {
	shrinkTimings(t)

	hits := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits++
		if hits == 1 {
			// Announce more than we send: the attempt fails mid-transfer
			// after bytes have already reached dst.
			w.Header().Set("Content-Length", "100")
			_, _ = w.Write([]byte("first-attempt-body"))
			return
		}
		_, _ = w.Write([]byte("ok"))
	}))
	defer srv.Close()

	f := tempFile(t)
	if _, err := Download(t.Context(), srv.URL, f, nil); err != nil {
		t.Fatalf("Download() error = %v", err)
	}
	if hits != 2 {
		t.Fatalf("server hit %d times, want 2 (truncation then success)", hits)
	}
	// A shorter second payload must not leave the first attempt's tail behind.
	if got := readAll(t, f); got != "ok" {
		t.Fatalf("content = %q, want %q (retry must rewind dst)", got, "ok")
	}
}

func TestDownloadReportsProgress(t *testing.T) {
	srv := newTestServer(func() (int, string) { return 200, "hello world" })
	defer srv.Close()

	f := tempFile(t)
	var lastTotal, lastN int64
	calls := 0
	_, err := Download(t.Context(), srv.URL, f, func(total, n int64) {
		calls++
		lastTotal, lastN = total, n
	})
	if err != nil {
		t.Fatalf("Download() error = %v", err)
	}
	if lastTotal != int64(len("hello world")) {
		t.Fatalf("progress total = %d, want %d", lastTotal, len("hello world"))
	}
	if lastN != int64(len("hello world")) {
		t.Fatalf("progress n = %d, want %d", lastN, len("hello world"))
	}
	if calls == 0 {
		t.Fatal("progress callback never invoked")
	}
}

func tempFile(t *testing.T) *os.File {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "download-*")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { f.Close() })
	return f
}

func readAll(t *testing.T, f *os.File) string {
	t.Helper()
	if _, err := f.Seek(0, 0); err != nil {
		t.Fatal(err)
	}
	b := make([]byte, 4096)
	n, err := f.Read(b)
	if err != nil && n == 0 {
		t.Fatal(err)
	}
	return string(b[:n])
}

// newTestServer serves one (status, body) pair per request, produced by next.
func newTestServer(next func() (int, string)) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		code, body := next()
		w.WriteHeader(code)
		if body != "" {
			_, _ = w.Write([]byte(body))
		}
	}))
}

// newStallingServer announces a body it never finishes sending.
func newStallingServer() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "1024")
		_, _ = w.Write([]byte("x"))
		<-r.Context().Done() // hold the connection open, sending nothing
	}))
}
