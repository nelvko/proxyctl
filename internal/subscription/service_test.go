package subscription

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nelvko/proxyctl/internal/config"
	"github.com/nelvko/proxyctl/internal/kernel"
	"github.com/nelvko/unisvc"
)

// fakeKernel stands in for a real kernel: the embedded unisvc.Service stays
// nil (tests never call its methods), config validation checks YAML syntax
// instead of exec'ing a binary, and service interactions are recorded.
type fakeKernel struct {
	unisvc.Service

	cfgFile  string
	restarts int
}

func (f *fakeKernel) ConfigFile() string                { return f.cfgFile }
func (f *fakeKernel) ConfigFormat() kernel.ConfigFormat { return kernel.FormatClash }
func (f *fakeKernel) LatestArtifact() (*kernel.Artifact, error) {
	return nil, errors.New("not needed")
}
func (f *fakeKernel) InstalledVersion() (string, error) {
	return "", errors.New("not needed")
}
func (f *fakeKernel) IsActive() (bool, error) { return true, nil }
func (f *fakeKernel) Restart() error {
	f.restarts++
	return nil
}

func (f *fakeKernel) TestConfig(file string) error {
	b, err := os.ReadFile(file)
	if err != nil {
		return err
	}
	if !strings.Contains(string(b), "proxies:") {
		return fmt.Errorf("invalid config: no proxies key")
	}
	return nil
}

// newTestService wires a service against an isolated config root.
func newTestService(t *testing.T) (*Service, *fakeKernel) {
	t.Helper()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	k := &fakeKernel{cfgFile: filepath.Join(t.TempDir(), "kernel-config.yaml")}
	return NewService(&config.SubscriptionConfig{}, k), k
}

func writeFile(t *testing.T, path, content string) string {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestUpdateRefreshesProfileFromFileSource(t *testing.T) {
	svc, _ := newTestService(t)

	src := filepath.Join(t.TempDir(), "sub.yaml")
	writeFile(t, src, "proxies: [a]\n")
	draft := &profile{Name: "p1", URL: "file://" + src}
	if err := svc.Add(draft); err != nil {
		t.Fatalf("Add() error = %v", err)
	}

	writeFile(t, src, "proxies: [b]\n")
	if err := svc.Update("p1"); err != nil {
		t.Fatalf("Update() error = %v", err)
	}

	got, err := os.ReadFile(draft.File)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "proxies: [b]\n" {
		t.Fatalf("profile file = %q, want the refreshed content", got)
	}
}

func TestUpdateReappliesActiveProfile(t *testing.T) {
	svc, k := newTestService(t)

	src := filepath.Join(t.TempDir(), "sub.yaml")
	writeFile(t, src, "proxies: [a]\n")
	draft := &profile{Name: "p1", URL: "file://" + src}
	if err := svc.Add(draft); err != nil {
		t.Fatalf("Add() error = %v", err)
	}
	k.restarts = 0 // Add auto-activates the first profile; start counting now

	writeFile(t, src, "proxies: [fresh]\n")
	if err := svc.Update("p1"); err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	if k.restarts == 0 {
		t.Fatal("active profile updated without re-applying it to the kernel")
	}
	got, _ := os.ReadFile(k.cfgFile)
	if string(got) != "proxies: [fresh]\n" {
		t.Fatalf("kernel config = %q, want the refreshed content", got)
	}
}

func TestUpdateInactiveProfileDoesNotRestartKernel(t *testing.T) {
	svc, k := newTestService(t)

	src := filepath.Join(t.TempDir(), "sub.yaml")
	writeFile(t, src, "proxies: [a]\n")
	first := &profile{Name: "p1", URL: "file://" + src}
	if err := svc.Add(first); err != nil {
		t.Fatalf("Add(first) error = %v", err)
	}
	secondSrc := filepath.Join(t.TempDir(), "sub2.yaml")
	writeFile(t, secondSrc, "proxies: [x]\n")
	second := &profile{Name: "p2", URL: "file://" + secondSrc}
	if err := svc.Add(second); err != nil {
		t.Fatalf("Add(second) error = %v", err)
	}
	k.restarts = 0

	writeFile(t, secondSrc, "proxies: [y]\n")
	if err := svc.Update("p2"); err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	if k.restarts != 0 {
		t.Fatalf("kernel restarted %d times for an inactive profile", k.restarts)
	}
}

func TestUpdateKeepsGoingWhenOneProfileFails(t *testing.T) {
	svc, _ := newTestService(t)

	goodSrc := filepath.Join(t.TempDir(), "good.yaml")
	good := &profile{Name: "good", URL: "file://" + writeFile(t, goodSrc, "proxies: [a]\n")}
	if err := svc.Add(good); err != nil {
		t.Fatalf("Add(good) error = %v", err)
	}
	// A profile whose source has disappeared since it was added.
	svc.subConfig.Profiles = append(svc.subConfig.Profiles, profile{
		Name: "bad", URL: "file:///nonexistent/sub.yaml", File: filepath.Join(t.TempDir(), "bad.yaml"),
	})

	err := svc.Update()
	if err == nil {
		t.Fatal("Update() succeeded with a broken source; want joined error")
	}
	if !strings.Contains(err.Error(), "bad") {
		t.Fatalf("error %v does not name the failing profile", err)
	}

	got, err := os.ReadFile(good.File)
	if err != nil || string(got) != "proxies: [a]\n" {
		t.Fatalf("good profile not updated: %q (%v)", got, err)
	}
}

func TestUpdateSkipsIdenticalContent(t *testing.T) {
	svc, k := newTestService(t)

	src := filepath.Join(t.TempDir(), "sub.yaml")
	writeFile(t, src, "proxies: [a]\n")
	draft := &profile{Name: "p1", URL: "file://" + src}
	if err := svc.Add(draft); err != nil {
		t.Fatalf("Add() error = %v", err)
	}
	k.restarts = 0 // Add auto-activates; start counting now

	if err := svc.Update("p1"); err != nil {
		t.Fatalf("Update() with unchanged source error = %v", err)
	}
	if k.restarts != 0 {
		t.Fatalf("kernel restarted %d times although the content did not change", k.restarts)
	}

	writeFile(t, src, "proxies: [changed]\n")
	if err := svc.Update("p1"); err != nil {
		t.Fatalf("Update() with changed source error = %v", err)
	}
	if k.restarts == 0 {
		t.Fatal("kernel not restarted after the active profile actually changed")
	}
}

func TestUpdateRejectsUnknownName(t *testing.T) {
	svc, _ := newTestService(t)
	if err := svc.Update("nope"); err == nil {
		t.Fatal("Update(unknown) succeeded; want error")
	}
}
