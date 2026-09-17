package subscription

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestAddOnlySavesProfile(t *testing.T) {
	svc, k := newTestService(t)
	k.stopped = true
	src := writeFile(t, filepath.Join(t.TempDir(), "source.yaml"), "proxies: [a]\n")
	if err := svc.Add(&profile{Name: "first", URL: "file://" + src}); err != nil {
		t.Fatal(err)
	}
	if svc.SelectedName() != "" || k.restarts != 0 || !k.stopped {
		t.Fatal("add selected a profile or changed runtime state")
	}
	if _, err := os.Stat(k.cfgFile); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("runtime file changed: %v", err)
	}
}

func TestAddRejectsPathTraversal(t *testing.T) {
	svc, _ := newTestService(t)
	for _, name := range []string{"../config", "../../outside", "nested/name", `nested\name`, "..", ".", " name", "name\n"} {
		if err := svc.Add(&profile{Name: name, URL: "file:///not-read"}); err == nil {
			t.Errorf("accepted %q", name)
		}
	}
	if len(svc.List()) != 0 {
		t.Fatal("invalid names changed state")
	}
}
