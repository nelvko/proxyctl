package subscription

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/nelvko/proxyctl/internal/config"
)

func TestSetRenamesSelectionWithoutChangingCache(t *testing.T) {
	svc, k := newTestService(t)
	source := writeFile(t, filepath.Join(t.TempDir(), "source.yaml"), "proxies: [old]\n")
	if err := addTestProfile(svc, &profile{Name: "old", URL: "file://" + source}); err != nil {
		t.Fatal(err)
	}
	before, _ := svc.Get("old")
	next := before
	next.Name = "new"
	next.Download.UserAgent = "NewAgent"
	next.Download.DownloadVia = "kernel"
	restarts := k.restarts
	if err := svc.Set("old", next); err != nil {
		t.Fatal(err)
	}
	next, _ = svc.Get("new")
	if next.File != before.File || svc.SelectedName() != "new" || restarts != k.restarts {
		t.Fatal("metadata changed runtime/cache or lost selection")
	}
	writeFile(t, source, "proxies: [other]\n")
	if err := svc.Add(&profile{Name: "old", URL: "file://" + source}); err != nil {
		t.Fatal(err)
	}
	content, _ := os.ReadFile(next.File)
	if string(content) != "proxies: [old]\n" {
		t.Fatal("reusing old name overwrote renamed cache")
	}
	next.Name = "old"
	if err := svc.Set("new", next); err == nil {
		t.Fatal("duplicate rename accepted")
	}
	next.Name = "new"
	next.Download.UserAgent = ""
	next.Download.DownloadVia = "env"
	if err := svc.Set("new", next); err != nil {
		t.Fatal(err)
	}
	saved, _ := config.LoadSubscriptionConfig()
	if saved.Profiles[0].Download.UserAgent != "" || saved.Profiles[0].Download.DownloadVia != "env" {
		t.Fatal("explicit empty/false not saved")
	}
}

func TestSetRestoresMetadataWhenSaveFails(t *testing.T) {
	svc, _ := newTestService(t)
	source := writeFile(t, filepath.Join(t.TempDir(), "source.yaml"), "proxies: []\n")
	if err := svc.Add(&profile{Name: "old", URL: "file://" + source}); err != nil {
		t.Fatal(err)
	}
	next, _ := svc.Get("old")
	next.Name = "new"
	dir, _ := config.ProfilesDir()
	file := filepath.Join(filepath.Dir(dir), "subscriptions.yaml")
	if err := os.Remove(file); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(file, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := svc.Set("old", next); err == nil {
		t.Fatal("expected persistence failure")
	}
	if _, err := svc.Get("old"); err != nil {
		t.Fatal("metadata not restored")
	}
	if _, err := svc.Get("new"); err == nil {
		t.Fatal("failed rename retained")
	}
}
