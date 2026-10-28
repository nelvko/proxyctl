package ui

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDashboardConnectionMetricsPreserveKnownZeroAndMissingValues(t *testing.T) {
	for _, tc := range []struct {
		name, body                    string
		listKnown, upKnown, downKnown bool
		count                         int
		up, down                      int64
		memory                        *int64
	}{
		{name: "known zero", body: `{"connections":[],"uploadTotal":0,"downloadTotal":0,"memory":0}`, listKnown: true, upKnown: true, downKnown: true, memory: metricPointer(0)},
		{name: "null empty list", body: `{"connections":null,"uploadTotal":12,"downloadTotal":34,"memory":56}`, listKnown: true, upKnown: true, downKnown: true, up: 12, down: 34, memory: metricPointer(56)},
		{name: "active list", body: `{"connections":[{"id":"one"},{"id":"two"}],"uploadTotal":1024,"downloadTotal":2048,"memory":4096}`, listKnown: true, upKnown: true, downKnown: true, count: 2, up: 1024, down: 2048, memory: metricPointer(4096)},
		{name: "all missing", body: `{}`},
		{name: "partial fields", body: `{"uploadTotal":17}`, upKnown: true, up: 17},
		{name: "null metrics", body: `{"connections":[],"uploadTotal":null,"downloadTotal":null,"memory":null}`, listKnown: true},
		{name: "negative metrics", body: `{"connections":[],"uploadTotal":-1,"downloadTotal":-2,"memory":-3}`, listKnown: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/connections" || r.Method != http.MethodGet {
					t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
				}
				_, _ = io.WriteString(w, tc.body)
			}))
			defer server.Close()
			controller := &controllerClient{base: server.URL, client: server.Client()}
			metrics, err := controller.connections(t.Context())
			if err != nil {
				t.Fatal(err)
			}
			if metrics.Available != tc.listKnown || metrics.UploadTotalKnown != tc.upKnown || metrics.DownloadTotalKnown != tc.downKnown {
				t.Fatalf("metric availability = list:%v up:%v down:%v", metrics.Available, metrics.UploadTotalKnown, metrics.DownloadTotalKnown)
			}
			if len(metrics.Connections) != tc.count || metrics.UploadTotal != tc.up || metrics.DownloadTotal != tc.down {
				t.Fatalf("wrong decoded metrics: %+v", metrics)
			}
			if (metrics.Memory == nil) != (tc.memory == nil) || tc.memory != nil && *metrics.Memory != *tc.memory {
				t.Fatalf("memory=%v, expected %v", metrics.Memory, tc.memory)
			}
		})
	}
}

func TestDashboardMalformedConnectionMetricsDoNotBecomeKnownZero(t *testing.T) {
	for _, body := range []string{
		`{"connections":{}}`,
		`{"connections":[],"uploadTotal":"0"}`,
		`{"connections":[],"downloadTotal":1.5}`,
		`{"connections":[],"memory":"4096"}`,
		`{"connections":[],"memory":9223372036854775808}`,
		`invalid`,
	} {
		t.Run(body, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = io.WriteString(w, body) }))
			defer server.Close()
			controller := &controllerClient{base: server.URL, client: server.Client()}
			metrics, err := controller.connections(t.Context())
			if err == nil || metrics.Available || metrics.UploadTotalKnown || metrics.DownloadTotalKnown || metrics.Memory != nil {
				t.Fatalf("malformed metrics acquired a valid reading: %+v, %v", metrics, err)
			}
		})
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNotFound) }))
	defer server.Close()
	controller := &controllerClient{base: server.URL, client: server.Client()}
	metrics, err := controller.connections(t.Context())
	if err == nil || metrics.Available || metrics.Memory != nil {
		t.Fatalf("unavailable endpoint acquired a valid reading: %+v, %v", metrics, err)
	}
}

func TestDashboardConnectionMetricsClearValuesOmittedByLaterResponse(t *testing.T) {
	var metrics dashboardConnections
	if err := json.Unmarshal([]byte(`{"connections":[],"uploadTotal":1,"downloadTotal":2,"memory":3}`), &metrics); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(`{}`), &metrics); err != nil {
		t.Fatal(err)
	}
	if metrics.Available || metrics.UploadTotalKnown || metrics.DownloadTotalKnown || metrics.Memory != nil || metrics.UploadTotal != 0 || metrics.DownloadTotal != 0 {
		t.Fatalf("missing fields retained stale values: %+v", metrics)
	}
}

func metricPointer(value int64) *int64 { return &value }
