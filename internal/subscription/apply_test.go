package subscription

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/nelvko/proxyctl/internal/config"
)

func TestUsePreservesStoppedService(t *testing.T) {
	svc, k := newTestService(t)
	k.stopped = true
	src := writeFile(t, filepath.Join(t.TempDir(), "source.yaml"), "proxies: [a]\n")
	if err := svc.Add(&profile{Name: "one", URL: "file://" + src}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Use("one"); err != nil {
		t.Fatal(err)
	}
	if !k.stopped || k.restarts != 0 || svc.SelectedName() != "one" {
		t.Fatal("use failed to preserve stopped state")
	}
	got, err := os.ReadFile(k.cfgFile)
	if err != nil || string(got) != "proxies: [a]\n" {
		t.Fatalf("staged config: %q %v", got, err)
	}
}

func TestUseRestoresStateWhenSaveFails(t *testing.T) {
	svc, k := newTestService(t)
	src := writeFile(t, filepath.Join(t.TempDir(), "source.yaml"), "proxies: [old]\n")
	if err := addTestProfile(svc, &profile{Name: "old", URL: "file://" + src}); err != nil {
		t.Fatal(err)
	}
	writeFile(t, src, "proxies: [new]\n")
	if err := svc.Add(&profile{Name: "new", URL: "file://" + src}); err != nil {
		t.Fatal(err)
	}
	dir, err := config.ProfilesDir()
	if err != nil {
		t.Fatal(err)
	}
	configFile := filepath.Join(filepath.Dir(dir), "subscriptions.yaml")
	if err := os.Remove(configFile); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(configFile, 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Use("new"); err == nil {
		t.Fatal("expected save failure")
	}
	got, err := os.ReadFile(k.cfgFile)
	if err != nil || string(got) != "proxies: [old]\n" || svc.SelectedName() != "old" {
		t.Fatalf("state not restored: %q %v %s", got, err, svc.SelectedName())
	}
}
