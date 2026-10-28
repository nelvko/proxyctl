package ui

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDashboardControllerReadsRules(t *testing.T) {
	var gotMethod, gotPath, gotAuth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod, gotPath, gotAuth = r.Method, r.URL.Path, r.Header.Get("Authorization")
		_, _ = w.Write([]byte(`{"rules":[{"index":0,"type":"DOMAIN-SUFFIX","payload":"example.com","proxy":"Proxy","size":-1},{"type":"MATCH","payload":"","proxy":"DIRECT"}]}`))
	}))
	defer server.Close()

	client := &controllerClient{base: server.URL, secret: "test-secret", client: dashboardHTTPClient}
	rules, err := client.rules(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if gotMethod != http.MethodGet || gotPath != "/rules" || gotAuth != "Bearer test-secret" {
		t.Fatalf("rules request: method=%q path=%q auth=%q", gotMethod, gotPath, gotAuth)
	}
	if len(rules) != 2 || rules[0].Type != "DOMAIN-SUFFIX" || rules[0].Payload != "example.com" || rules[0].Proxy != "Proxy" || rules[1].Type != "MATCH" || rules[1].Proxy != "DIRECT" {
		t.Fatalf("rules = %#v", rules)
	}
}
