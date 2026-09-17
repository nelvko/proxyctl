package config

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"testing"
	"time"
)

func TestLockSerializesProcessesAndHonorsCancellation(t *testing.T) {
	d := useTestDir(t)
	unlock, err := Lock(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if unlock != nil {
			unlock()
		}
	}()
	child := exec.Command(os.Args[0], "-test.run=^TestLockChild$")
	child.Env = append(os.Environ(), "PROXYCTL_TEST_LOCK_DIR="+d)
	if out, err := child.CombinedOutput(); err != nil {
		t.Fatalf("child acquired held lock: %s %v", out, err)
	}
	unlock()
	unlock = nil
	release, err := Lock(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	release()
}
func TestLockChild(t *testing.T) {
	d := os.Getenv("PROXYCTL_TEST_LOCK_DIR")
	if d == "" {
		return
	}
	testDir = d
	ctx, cancel := context.WithTimeout(t.Context(), 100*time.Millisecond)
	defer cancel()
	release, err := Lock(ctx)
	if release != nil {
		release()
		t.Fatal("acquired parent lock")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected cancellable wait, got %v", err)
	}
}
