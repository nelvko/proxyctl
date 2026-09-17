package subscription

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/nelvko/proxyctl/internal/config"
)

func TestDownloadNamingAndRequestMetadata(t *testing.T) {
	svc, _ := newTestService(t)
	var hits atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		if r.UserAgent() != "ProxyTest/1.0" || r.Header.Get("Authorization") != "Bearer private" {
			t.Errorf("request options not applied: %v", r.Header)
		}
		w.Header().Set("Content-Disposition", `attachment; filename="fallback.yaml"; filename*=UTF-8''%E5%B7%A5%E4%BD%9C.yaml`)
		_, _ = w.Write([]byte("proxies: []\n"))
	}))
	defer server.Close()
	for _, want := range []string{"工作", "工作-2", "explicit"} {
		draft := profile{URL: server.URL, Download: config.DownloadConfig{UserAgent: "ProxyTest/1.0", Headers: map[string]string{"authorization": "Bearer private"}}}
		if want == "explicit" {
			draft.Name = want
		}
		before := hits.Load()
		if err := svc.Add(&draft); err != nil {
			t.Fatal(err)
		}
		if draft.Name != want || hits.Load() != before+1 {
			t.Fatalf("name=%q, requests=%d", draft.Name, hits.Load()-before)
		}
		if filepath.Base(draft.File) == draft.Name+".yaml" {
			t.Fatal("cache identity depends on display name")
		}
	}
	if _, err := svc.Update(DownloadOptions{}, "工作"); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Get("工作"); err != nil {
		t.Fatal("update renamed profile")
	}
}

func TestSourceNameFallbackAndSanitization(t *testing.T) {
	for input, want := range map[string]string{
		"../../office.yaml": "office", `C:\private\work.yml`: "work", "\x1b bad.json ": "bad", ".": "", "": "", "../../../": "",
	} {
		if got := cleanName(input); got != want {
			t.Errorf("cleanName(%q)=%q, want %q", input, got, want)
		}
	}
	svc, _ := newTestService(t)
	source := writeFile(t, filepath.Join(t.TempDir(), "local.yml"), "proxies: []\n")
	draft := profile{URL: "file://" + source}
	if err := svc.Add(&draft); err != nil || draft.Name != "local" {
		t.Fatalf("local fallback: %q, %v", draft.Name, err)
	}
}

func TestKeepFailedOnlyRetainsValidatedDownloads(t *testing.T) {
	svc, _ := newTestService(t)
	source := writeFile(t, filepath.Join(t.TempDir(), "bad.yaml"), "invalid content")
	draft := profile{Name: "bad", URL: "file://" + source}
	err := svc.Add(&draft)
	if err == nil || !strings.Contains(err.Error(), "--keep-failed") {
		t.Fatalf("missing diagnostic hint: %v", err)
	}
	diagnostics, _ := config.DiagnosticsDir()
	if _, err := os.Stat(diagnostics); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("default retained diagnostic directory")
	}
	svc.KeepFailed = true
	err = svc.Add(&draft)
	if err == nil || !strings.Contains(err.Error(), "not applied") {
		t.Fatalf("missing preserved-path message: %v", err)
	}
	files, _ := os.ReadDir(diagnostics)
	if len(files) != 1 {
		t.Fatalf("diagnostics: %v", files)
	}
	file := filepath.Join(diagnostics, files[0].Name())
	if !strings.Contains(err.Error(), file) {
		t.Fatalf("error omitted actual path: %v", err)
	}
	info, _ := os.Stat(file)
	dirInfo, _ := os.Stat(diagnostics)
	if info.Mode().Perm() != 0o600 || dirInfo.Mode().Perm() != 0o700 {
		t.Fatal("diagnostics are not private")
	}
	data, _ := os.ReadFile(file)
	if string(data) != "invalid content" || len(svc.List()) != 0 {
		t.Fatal("invalid download was changed or registered")
	}
	source = writeFile(t, filepath.Join(t.TempDir(), "good.yaml"), "proxies: [old]\n")
	draft = profile{Name: "good", URL: "file://" + source}
	if err := svc.Add(&draft); err != nil {
		t.Fatal(err)
	}
	writeFile(t, source, "invalid update")
	if _, err := svc.Update(DownloadOptions{}, "good"); err == nil {
		t.Fatal("invalid update accepted")
	}
	current, _ := os.ReadFile(draft.File)
	if string(current) != "proxies: [old]\n" {
		t.Fatal("invalid update replaced cache")
	}
	draft = profile{Name: "missing", URL: "file:///nonexistent"}
	if err := svc.Add(&draft); err == nil || strings.Contains(err.Error(), "saved to") {
		t.Fatalf("missing file treated as validation failure: %v", err)
	}
	files, _ = os.ReadDir(diagnostics)
	if len(files) != 2 {
		t.Fatal("incomplete download retained")
	}
}

func TestDownloadCancellationDoesNotCommitOrRetain(t *testing.T) {
	svc, _ := newTestService(t)
	started := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.(http.Flusher).Flush()
		close(started)
		<-r.Context().Done()
	}))
	defer server.Close()
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	svc = svc.WithContext(ctx)
	svc.KeepFailed = true
	done := make(chan error, 1)
	go func() { done <- svc.Add(&profile{URL: server.URL}) }()
	<-started
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("cancellation lost: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("download ignored cancellation")
	}
	if len(svc.List()) != 0 {
		t.Fatal("cancelled add committed")
	}
	diagnostics, _ := config.DiagnosticsDir()
	if _, err := os.Stat(diagnostics); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("cancelled download retained")
	}
}

func TestViaKernelRequiresRunningAndUsesAuthenticatedEndpoint(t *testing.T) {
	svc, k := newTestService(t)
	t.Setenv("HTTP_PROXY", "http://127.0.0.1:1")
	var hits atomic.Int32
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		if r.Header.Get("Proxy-Authorization") != "Basic dXNlcjpwYXNz" {
			t.Errorf("missing proxy authentication: %v", r.Header)
		}
		_, _ = w.Write([]byte("proxies: []\n"))
	}))
	defer proxy.Close()
	port := strings.Split(proxy.URL, ":")[2]
	writeFile(t, k.cfgFile, "port: "+port+"\nauthentication: [user:pass]\n")
	p := profile{Name: "via", URL: "http://source.invalid/secret?token=private", Download: config.DownloadConfig{DownloadVia: "kernel"}}
	k.stopped = true
	if err := svc.Add(&p); err == nil || hits.Load() != 0 {
		t.Fatal("stopped kernel did not fail closed")
	}
	k.stopped = false
	if err := svc.Add(&p); err != nil {
		t.Fatal(err)
	}
	if hits.Load() != 1 {
		t.Fatal("explicit route did not override environment")
	}
}

func TestInvalidMetadataFailsBeforeDownload(t *testing.T) {
	svc, _ := newTestService(t)
	for _, update := range []config.DownloadConfig{
		{Timeout: -time.Second}, {UserAgent: "bad\r\nheader"}, {Headers: map[string]string{"bad header": "x"}},
	} {
		if err := svc.Add(&profile{URL: "https://source.invalid/secret", Download: update}); err == nil {
			t.Fatalf("accepted invalid metadata: %+v", update)
		}
	}
}

func TestEmptyDownloadHasNoDiagnosticFile(t *testing.T) {
	svc, _ := newTestService(t)
	svc.KeepFailed = true
	source := writeFile(t, filepath.Join(t.TempDir(), "empty.yaml"), "")
	err := svc.Add(&profile{URL: "file://" + source})
	if err == nil || !strings.Contains(err.Error(), "empty") {
		t.Fatalf("empty download: %v", err)
	}
	diagnostics, _ := config.DiagnosticsDir()
	if _, err := os.Stat(diagnostics); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("empty diagnostic retained")
	}
}

type unavailableValidator struct{ *fakeKernel }

func (k unavailableValidator) TestConfig(context.Context, string) error { return os.ErrNotExist }

func TestUnavailableValidatorIsNotAnInvalidDownload(t *testing.T) {
	svc, k := newTestService(t)
	svc.kernel = unavailableValidator{k}
	svc.KeepFailed = true
	source := writeFile(t, filepath.Join(t.TempDir(), "source.yaml"), "proxies: []\n")
	err := svc.Add(&profile{URL: "file://" + source})
	if !errors.Is(err, os.ErrNotExist) || strings.Contains(err.Error(), "--keep-failed") {
		t.Fatalf("execution error misclassified: %v", err)
	}
	diagnostics, _ := config.DiagnosticsDir()
	if _, err := os.Stat(diagnostics); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("execution failure retained as invalid config")
	}
}
