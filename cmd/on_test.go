package cmd

import (
	"errors"
	"reflect"
	"strings"
	"testing"
)

func TestShellCandidates(t *testing.T) {
	t.Parallel()

	got := shellCandidates("/bin/zsh")
	want := []string{"/bin/zsh", "/bin/bash", "bash", "zsh", "/bin/sh", "sh"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("shellCandidates() = %v, want %v", got, want)
	}
}

func TestFindShellPathFallsBack(t *testing.T) {
	t.Parallel()

	var seen []string
	got, err := findShellPath(func(file string) (string, error) {
		seen = append(seen, file)
		if file == "bash" {
			return "/usr/bin/bash", nil
		}
		return "", errors.New("not found")
	}, []string{"/bad/shell", "bash", "sh"})
	if err != nil {
		t.Fatalf("findShellPath() returned error: %v", err)
	}
	if got != "/usr/bin/bash" {
		t.Fatalf("findShellPath() = %q, want %q", got, "/usr/bin/bash")
	}
	wantSeen := []string{"/bad/shell", "bash"}
	if !reflect.DeepEqual(seen, wantSeen) {
		t.Fatalf("lookup order = %v, want %v", seen, wantSeen)
	}
}

func TestFindShellPathReportsCandidates(t *testing.T) {
	t.Parallel()

	_, err := findShellPath(func(file string) (string, error) {
		return "", errors.New("missing")
	}, []string{"bash", "sh"})
	if err == nil {
		t.Fatal("findShellPath() error = nil, want non-nil")
	}
	if !strings.Contains(err.Error(), "bash, sh") {
		t.Fatalf("findShellPath() error = %q, want candidate list", err)
	}
}
