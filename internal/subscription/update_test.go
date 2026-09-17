package subscription

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type failOnceRuntime struct {
	*fakeKernel
	failNext bool
}

func (k *failOnceRuntime) Restart() error {
	if k.failNext {
		k.failNext = false
		return errors.New("injected restart failure")
	}
	return k.fakeKernel.Restart()
}

func TestUpdateRetryRepairsFailedApplication(t *testing.T) {
	svc, k := newTestService(t)
	src := writeFile(t, filepath.Join(t.TempDir(), "source.yaml"), "proxies: [old]\n")
	if err := addTestProfile(svc, &profile{Name: "one", URL: "file://" + src}); err != nil {
		t.Fatal(err)
	}
	svc.kernel = &failOnceRuntime{fakeKernel: k, failNext: true}
	writeFile(t, src, "proxies: [new]\n")
	if _, err := svc.Update(DownloadOptions{}, "one"); err == nil {
		t.Fatal("expected restart failure")
	}
	old, err := os.ReadFile(k.cfgFile)
	if err != nil || string(old) != "proxies: [old]\n" {
		t.Fatalf("rollback: %q %v", old, err)
	}
	if _, err := svc.Update(DownloadOptions{}, "one"); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(k.cfgFile)
	if err != nil || string(got) != "proxies: [new]\n" {
		t.Fatalf("retry left stale config: %q %v", got, err)
	}
}

func TestUpdateRefreshesProfileFromFileSource(t *testing.T) {
	svc, _ := newTestService(t)

	src := filepath.Join(t.TempDir(), "sub.yaml")
	writeFile(t, src, "proxies: [a]\n")
	draft := &profile{Name: "p1", URL: "file://" + src}
	if err := addTestProfile(svc, draft); err != nil {
		t.Fatalf("Add() error = %v", err)
	}

	writeFile(t, src, "proxies: [b]\n")
	if _, err := svc.Update(DownloadOptions{}, "p1"); err != nil {
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
	if err := addTestProfile(svc, draft); err != nil {
		t.Fatalf("Add() error = %v", err)
	}
	k.restarts = 0 // Test setup selects the first profile; start counting now

	writeFile(t, src, "proxies: [fresh]\n")
	if _, err := svc.Update(DownloadOptions{}, "p1"); err != nil {
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
	if err := addTestProfile(svc, first); err != nil {
		t.Fatalf("Add(first) error = %v", err)
	}
	secondSrc := filepath.Join(t.TempDir(), "sub2.yaml")
	writeFile(t, secondSrc, "proxies: [x]\n")
	second := &profile{Name: "p2", URL: "file://" + secondSrc}
	if err := addTestProfile(svc, second); err != nil {
		t.Fatalf("Add(second) error = %v", err)
	}
	k.restarts = 0

	writeFile(t, secondSrc, "proxies: [y]\n")
	if _, err := svc.Update(DownloadOptions{}, "p2"); err != nil {
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
	if err := addTestProfile(svc, good); err != nil {
		t.Fatalf("Add(good) error = %v", err)
	}
	// A profile whose source has disappeared since it was added.
	svc.subConfig.Profiles = append(svc.subConfig.Profiles, profile{
		Name: "bad", URL: "file:///nonexistent/sub.yaml", File: filepath.Join(t.TempDir(), "bad.yaml"),
	})

	_, err := svc.Update(DownloadOptions{})
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
	if err := addTestProfile(svc, draft); err != nil {
		t.Fatalf("Add() error = %v", err)
	}
	k.restarts = 0 // Test setup selects the first profile; start counting now

	if _, err := svc.Update(DownloadOptions{}, "p1"); err != nil {
		t.Fatalf("Update() with unchanged source error = %v", err)
	}
	if k.restarts != 0 {
		t.Fatalf("kernel restarted %d times although the content did not change", k.restarts)
	}

	writeFile(t, src, "proxies: [changed]\n")
	if _, err := svc.Update(DownloadOptions{}, "p1"); err != nil {
		t.Fatalf("Update() with changed source error = %v", err)
	}
	if k.restarts == 0 {
		t.Fatal("kernel not restarted after the active profile actually changed")
	}
}

func TestUpdateRejectsUnknownName(t *testing.T) {
	svc, _ := newTestService(t)
	if _, err := svc.Update(DownloadOptions{}, "nope"); err == nil {
		t.Fatal("Update(unknown) succeeded; want error")
	}
}

// TestUpdateAppliesActiveDespiteOtherFailure pins the anti-stranding fix:
// when the active profile updated but another profile's source failed, the
// kernel must still be switched — otherwise it stays on the old
// subscription forever (the next update sees identical bytes).
func TestUpdateAppliesActiveDespiteOtherFailure(t *testing.T) {
	svc, k := newTestService(t)

	src := filepath.Join(t.TempDir(), "sub.yaml")
	writeFile(t, src, "proxies: [a]\n")
	active := &profile{Name: "active", URL: "file://" + src}
	if err := addTestProfile(svc, active); err != nil {
		t.Fatalf("Add(active) error = %v", err)
	}
	k.restarts = 0 // Test setup selects the first profile; start counting now
	svc.subConfig.Profiles = append(svc.subConfig.Profiles, profile{
		Name: "bad", URL: "file:///nonexistent/sub.yaml", File: filepath.Join(t.TempDir(), "bad.yaml"),
	})

	writeFile(t, src, "proxies: [fresh]\n")
	_, err := svc.Update(DownloadOptions{})
	if err == nil || !strings.Contains(err.Error(), "bad") {
		t.Fatalf("Update() error = %v; want a joined error naming the bad profile", err)
	}
	if k.restarts == 0 {
		t.Fatal("active profile update was not applied although another profile failed")
	}
	got, _ := os.ReadFile(k.cfgFile)
	if string(got) != "proxies: [fresh]\n" {
		t.Fatalf("kernel config = %q; want the updated active profile", got)
	}
}

// TestUpdateStagesConfigWhenKernelStopped pins that an update never
// resurrects a kernel the user deliberately stopped: new bytes go to the
// kernel config file, no restart.
func TestUpdateStagesConfigWhenKernelStopped(t *testing.T) {
	svc, k := newTestService(t)

	src := filepath.Join(t.TempDir(), "sub.yaml")
	writeFile(t, src, "proxies: [a]\n")
	if err := addTestProfile(svc, &profile{Name: "p1", URL: "file://" + src}); err != nil {
		t.Fatalf("Add() error = %v", err)
	}

	k.stopped = true
	k.restarts = 0
	writeFile(t, src, "proxies: [staged]\n")

	if _, err := svc.Update(DownloadOptions{}, "p1"); err != nil {
		t.Fatalf("Update() with a stopped kernel error = %v", err)
	}
	if k.restarts != 0 {
		t.Fatalf("kernel restarted %d times although it was stopped", k.restarts)
	}
	got, err := os.ReadFile(k.cfgFile)
	if err != nil || string(got) != "proxies: [staged]\n" {
		t.Fatalf("kernel config = %q (%v); want the staged update", got, err)
	}
}
