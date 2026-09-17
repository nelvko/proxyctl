package subscription

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/nelvko/proxyctl/internal/config"
)

func TestMetadataServiceRejectsRuntimeOperations(t *testing.T) {
	svc := NewService(&config.SubscriptionConfig{Profiles: []config.Profile{{Name: "one"}}}, nil)
	if err := svc.Add(&profile{Name: "new", URL: "https://example.com"}); !errors.Is(err, ErrNoRuntime) {
		t.Fatalf("Add: %v", err)
	}
	if _, err := svc.Use("one"); !errors.Is(err, ErrNoRuntime) {
		t.Fatalf("Use: %v", err)
	}
	if _, err := svc.BeginEdit("one"); !errors.Is(err, ErrNoRuntime) {
		t.Fatalf("Edit: %v", err)
	}
	if _, err := svc.Update(DownloadOptions{}, "one"); !errors.Is(err, ErrNoRuntime) {
		t.Fatalf("Update: %v", err)
	}
}

func TestProfileHeadersAreOwnedSnapshots(t *testing.T) {
	svc, _ := newTestService(t)
	source := writeFile(t, filepath.Join(t.TempDir(), "source.yaml"), "proxies: []\n")
	draft := profile{Name: "one", URL: "file://" + source, Download: config.DownloadConfig{Headers: map[string]string{"X-Test": "saved"}}}
	if err := svc.Add(&draft); err != nil {
		t.Fatal(err)
	}
	draft.Download.Headers["X-Test"] = "draft mutation"
	read, _ := svc.Get("one")
	read.Download.Headers["X-Test"] = "read mutation"
	listed := svc.List()
	listed[0].Download.Headers["X-Test"] = "list mutation"
	current, _ := svc.Get("one")
	if current.Download.Headers["X-Test"] != "saved" {
		t.Fatal("caller changed service state", current)
	}
	current.Download.Headers["X-Test"] = "modified"
	if err := svc.Set("one", current); err != nil {
		t.Fatal(err)
	}
	current.Download.Headers["X-Test"] = "after save"
	saved, _ := svc.Get("one")
	if saved.Download.Headers["X-Test"] != "modified" {
		t.Fatal("Set retained caller map", saved)
	}
}
