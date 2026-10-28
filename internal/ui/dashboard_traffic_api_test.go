package ui

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestDashboardTrafficStreamReadsRatesAndStopsOnCancel(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/traffic" || r.Header.Get("Authorization") != "Bearer test-secret" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		_, _ = w.Write([]byte("bad json\n{\"up\":-1,\"down\":42}\n{\"up\":1024,\"down\":2048,\"upTotal\":9000}\n{\"up\":0,\"down\":0}\n"))
		w.(http.Flusher).Flush()
		<-r.Context().Done()
	}))
	defer server.Close()

	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
	defer cancel()
	client := &controllerClient{base: server.URL, secret: "test-secret", client: dashboardHTTPClient}
	rates := make(chan dashboardTraffic, 2)
	done := make(chan error, 1)
	go func() { done <- client.streamTraffic(ctx, rates) }()
	for _, want := range []dashboardTraffic{{Up: 1024, Down: 2048}, {Up: 0, Down: 0}} {
		select {
		case got := <-rates:
			if got != want {
				t.Fatalf("traffic = %+v, want %+v", got, want)
			}
		case <-ctx.Done():
			t.Fatal("timed out waiting for traffic sample")
		}
	}
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("stream after cancellation: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("traffic stream did not stop after cancellation")
	}
}
