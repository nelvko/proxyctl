package download

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestTLSOptionDoesNotMutateSharedClient(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte("ok")) }))
	defer server.Close()
	ctx, cancel := context.WithTimeout(t.Context(), 200*time.Millisecond)
	defer cancel()
	var out bytes.Buffer
	if _, err := New(nil, http.DefaultClient, nil).Fetch(ctx, server.URL, &out, nil, RequestOptions{}); err == nil {
		t.Fatal("untrusted certificate accepted by default")
	}
	out.Reset()
	if _, err := New(nil, http.DefaultClient, nil).Fetch(t.Context(), server.URL, &out, nil, RequestOptions{SkipCert: true}); err != nil {
		t.Fatal(err)
	}
	if out.String() != "ok" {
		t.Fatal("missing content")
	}
	transport := http.DefaultTransport.(*http.Transport)
	if transport.TLSClientConfig != nil && transport.TLSClientConfig.InsecureSkipVerify {
		t.Fatal("TLS setting leaked into shared transport")
	}
}
func TestRedirectDoesNotForwardCustomCredentialsToOtherOrigin(t *testing.T) {
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Subscription-Key") != "" {
			t.Error("credential crossed origins")
		}
		_, _ = w.Write([]byte("ok"))
	}))
	defer target.Close()
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Subscription-Key") != "secret" {
			t.Error("origin did not receive header")
		}
		http.Redirect(w, r, target.URL, http.StatusFound)
	}))
	defer origin.Close()
	var out bytes.Buffer
	if _, err := New(nil, http.DefaultClient, nil).Fetch(t.Context(), origin.URL, &out, nil, RequestOptions{Headers: http.Header{"X-Subscription-Key": []string{"secret"}}}); err != nil {
		t.Fatal(err)
	}
}
