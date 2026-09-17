package subscription

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEditValidatesBeforeReplacingAndDoesNotApply(t *testing.T) {
	svc, k := newTestService(t)
	src := writeFile(t, filepath.Join(t.TempDir(), "source.yaml"), "proxies: [old]\n")
	if err := addTestProfile(svc, &profile{Name: "one", URL: "file://" + src}); err != nil {
		t.Fatal(err)
	}
	k.restarts = 0
	edit, err := svc.BeginEdit("one")
	if err != nil {
		t.Fatal(err)
	}
	defer edit.Close()
	writeFile(t, edit.temporary, "invalid content")
	if err := edit.Commit(); err == nil {
		t.Fatal("invalid edit accepted")
	}
	p, _ := svc.Get("one")
	old, err := os.ReadFile(p.File)
	if err != nil || string(old) != "proxies: [old]\n" {
		t.Fatalf("invalid edit replaced profile: %q %v", old, err)
	}
	writeFile(t, edit.temporary, "proxies: [edited]\n")
	if err := edit.Commit(); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(k.cfgFile)
	if err != nil || string(got) != "proxies: [old]\n" || k.restarts != 0 {
		t.Fatalf("edit unexpectedly applied: %q %v", got, err)
	}
}
