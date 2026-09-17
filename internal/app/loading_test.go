package app

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/nelvko/proxyctl/internal/config"
	"github.com/nelvko/proxyctl/internal/download"
)

func TestLoadDoesNotConfigureDownloads(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("AppData", t.TempDir())
	t.Setenv("GH_PROXY", "")
	existing := download.New([]string{"https://existing.example"}, nil, nil)
	if err := config.Save(&config.Config{Mirror: "https://saved.example"}); err != nil {
		t.Fatal(err)
	}
	a, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if got := existing.Mirrors(); !reflect.DeepEqual(got, []string{"https://existing.example"}) {
		t.Fatalf("loading changed HTTP runtime: %v", got)
	}
	if a.Subscriptions != nil {
		t.Fatal("loading eagerly read unrelated subscriptions")
	}
	downloads := a.downloader()
	if got := downloads.Mirrors(); !reflect.DeepEqual(got, []string{"https://saved.example"}) {
		t.Fatalf("runtime did not use loaded mirror: %v", got)
	}
}

func TestMetadataAssemblyIgnoresBrokenMainConfig(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("AppData", t.TempDir())
	if err := config.SaveSubscriptionConfig(&config.SubscriptionConfig{Profiles: []config.Profile{{Name: "office"}}}); err != nil {
		t.Fatal(err)
	}
	profilesDir, _ := config.ProfilesDir()
	if err := os.WriteFile(filepath.Join(filepath.Dir(profilesDir), "config.yaml"), []byte("broken: ["), 0600); err != nil {
		t.Fatal(err)
	}
	svc, err := LoadProfileMetadata(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Get("office"); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadRuntimeProfiles(t.Context(), nil); err == nil {
		t.Fatal("kernel-bound operation ignored broken main config")
	}
}
