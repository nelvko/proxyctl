package subscription

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/nelvko/proxyctl/internal/config"
)

func TestDeleteSelectedProfileRequiresForceAndClearsRuntime(t *testing.T) {
	svc, k := newTestService(t)
	src := writeFile(t, filepath.Join(t.TempDir(), "source.yaml"), "proxies: []\n")
	if err := addTestProfile(svc, &profile{Name: "one", URL: "file://" + src}); err != nil {
		t.Fatal(err)
	}
	if err := svc.Delete("one", false); err == nil {
		t.Fatal("deleted selected profile without force")
	}
	if k.stopped {
		t.Fatal("rejected deletion stopped kernel")
	}
	if err := svc.Delete("one", true); err != nil {
		t.Fatal(err)
	}
	if !k.stopped || svc.SelectedName() != "" || len(svc.List()) != 0 {
		t.Fatal("forced deletion left active state")
	}
	if _, err := os.Stat(k.cfgFile); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("applied config remains: %v", err)
	}
}

func TestDeleteRestoresSelectedProfileWhenSaveFails(t *testing.T) {
	svc, k := newTestService(t)
	src := writeFile(t, filepath.Join(t.TempDir(), "source.yaml"), "proxies: [old]\n")
	if err := addTestProfile(svc, &profile{Name: "one", URL: "file://" + src}); err != nil {
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
	if err := svc.Delete("one", true); err == nil {
		t.Fatal("expected save failure")
	}
	if k.stopped || svc.SelectedName() != "one" || len(svc.List()) != 1 {
		t.Fatal("deletion failure did not restore selected profile and service")
	}
	got, err := os.ReadFile(k.cfgFile)
	if err != nil || string(got) != "proxies: [old]\n" {
		t.Fatalf("runtime config not restored: %q %v", got, err)
	}
}
