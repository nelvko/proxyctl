package subscription

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"sync/atomic"
	"testing"
	"time"

	"github.com/nelvko/proxyctl/internal/config"
)

func TestDownloadRouteSelectsExplicitTransport(t *testing.T) {
	svc, _ := newTestService(t)
	var directHits, proxyHits atomic.Int32
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		directHits.Add(1)
		if r.UserAgent() != DefaultDownloadUserAgent {
			t.Errorf("unexpected default UA: %q", r.UserAgent())
		}
		_, _ = w.Write([]byte("proxies: []\n"))
	}))
	defer origin.Close()
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		proxyHits.Add(1)
		_, _ = w.Write([]byte("proxies: []\n"))
	}))
	defer proxy.Close()
	// Inject a proxy policy even for loopback, which ProxyFromEnvironment bypasses.
	original := http.DefaultTransport
	transport := original.(*http.Transport).Clone()
	proxyURL, _ := url.Parse(proxy.URL)
	transport.Proxy = http.ProxyURL(proxyURL)
	http.DefaultTransport = transport
	t.Cleanup(func() { http.DefaultTransport = original; transport.CloseIdleConnections() })
	for _, route := range []string{"direct", "env"} {
		if err := svc.Add(&profile{Name: route, URL: origin.URL, Download: config.DownloadConfig{DownloadVia: route}}); err != nil {
			t.Fatal(err)
		}
	}
	if directHits.Load() != 1 || proxyHits.Load() != 1 {
		t.Fatalf("direct=%d proxy=%d", directHits.Load(), proxyHits.Load())
	}
}

func TestUpdateRequestOverridesNeverPersist(t *testing.T) {
	svc, _ := newTestService(t)
	var lastAgent, lastHeader atomic.Value
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		lastAgent.Store(r.UserAgent())
		lastHeader.Store(r.Header.Get("X-Token"))
		_, _ = w.Write([]byte("proxies: []\n"))
	}))
	defer server.Close()
	for _, name := range []string{"one", "two"} {
		if err := svc.Add(&profile{Name: name, URL: server.URL, Download: config.DownloadConfig{UserAgent: "saved", Headers: map[string]string{"x-token": "saved-token"}}}); err != nil {
			t.Fatal(err)
		}
	}
	before, _ := config.LoadSubscriptionConfig()
	agent, route, insecure, timeout := "temporary", "direct", true, time.Second
	o := DownloadOptions{UserAgent: &agent, Route: &route, Insecure: &insecure, Timeout: &timeout, Headers: map[string]string{"X-Token": "temporary-token"}}
	results, err := svc.Update(o)
	if err != nil || len(results) != 2 || lastAgent.Load() != agent || lastHeader.Load() != "temporary-token" {
		t.Fatalf("overrides: %+v %v %q %q", results, err, lastAgent.Load(), lastHeader.Load())
	}
	after, _ := config.LoadSubscriptionConfig()
	if !reflect.DeepEqual(before, after) || !reflect.DeepEqual(svc.subConfig, after) {
		t.Fatal("temporary settings changed saved/in-memory metadata")
	}
	agent = ""
	results, err = svc.Update(DownloadOptions{UserAgent: &agent, RemoveHeaders: []string{"X-TOKEN"}}, "one")
	if err != nil || lastAgent.Load() != DefaultDownloadUserAgent || lastHeader.Load() != "" {
		t.Fatalf("clear: %+v %v %q %q", results, err, lastAgent.Load(), lastHeader.Load())
	}
	if _, err := svc.Update(DownloadOptions{}, "one"); err != nil || lastAgent.Load() != "saved" || lastHeader.Load() != "saved-token" {
		t.Fatal("subsequent update did not inherit saved settings")
	}
}

func TestTemporaryTLSOverrideCanRestoreVerification(t *testing.T) {
	svc, _ := newTestService(t)
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte("proxies: []\n")) }))
	defer server.Close()
	p := profile{Name: "tls", URL: server.URL, Download: config.DownloadConfig{SkipCert: true, Timeout: 100 * time.Millisecond}}
	if err := svc.Add(&p); err != nil {
		t.Fatal(err)
	}
	verify := false
	if _, err := svc.Update(DownloadOptions{Insecure: &verify}, "tls"); err == nil {
		t.Fatal("explicit false did not restore TLS verification")
	}
	if _, err := svc.Update(DownloadOptions{}, "tls"); err != nil {
		t.Fatalf("temporary verification changed saved insecure setting: %v", err)
	}
}

func TestDownloadBudgetIncludesResponseBodyAndRetryDelay(t *testing.T) {
	for _, retry := range []bool{false, true} {
		t.Run(map[bool]string{false: "slow body", true: "retry delay"}[retry], func(t *testing.T) {
			svc, _ := newTestService(t)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if retry {
					w.WriteHeader(http.StatusServiceUnavailable)
					return
				}
				w.WriteHeader(http.StatusOK)
				w.(http.Flusher).Flush()
				<-r.Context().Done()
			}))
			defer server.Close()
			err := svc.Add(&profile{URL: server.URL, Download: config.DownloadConfig{Timeout: 30 * time.Millisecond}})
			if !errors.Is(err, context.DeadlineExceeded) || len(svc.List()) != 0 {
				t.Fatalf("deadline not enforced: %v", err)
			}
		})
	}
}
