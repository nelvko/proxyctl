package app

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/nelvko/proxyctl/internal/config"
	"github.com/nelvko/proxyctl/internal/subscription"
)

func TestOperationAssemblyReadsStateAfterLock(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("AppData", t.TempDir())
	dir := t.TempDir()
	if err := config.Save(&config.Config{Use: "mihomo", Kernels: []config.KernelConfig{{Name: "mihomo", Bin: filepath.Join(dir, "mihomo"), ConfigDir: dir, ConfigFile: filepath.Join(dir, "runtime.yaml")}}}); err != nil {
		t.Fatal(err)
	}
	if err := config.SaveSubscriptionConfig(&config.SubscriptionConfig{Profiles: []config.Profile{{Name: "old"}}}); err != nil {
		t.Fatal(err)
	}
	unlock, err := config.Lock(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	defer unlock()
	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
	defer cancel()
	done := make(chan error, 1)
	go func() {
		err := MutateRuntimeProfiles(ctx, nil, func(svc *subscription.Service) error {
			_, err := svc.Get("new")
			return err
		})
		done <- err
	}()
	if err := config.SaveSubscriptionConfig(&config.SubscriptionConfig{Profiles: []config.Profile{{Name: "new"}}}); err != nil {
		t.Fatal(err)
	}
	unlock()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-ctx.Done():
		t.Fatal("operation failed to acquire lock and read fresh state")
	}
}

func TestAssemblyFailureReleasesOperationLock(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("AppData", t.TempDir())
	err := MutateRuntimeProfiles(t.Context(), nil, func(*subscription.Service) error { t.Fatal("unexpected callback"); return nil })
	if !errors.Is(err, config.ErrNoKernel) {
		t.Fatalf("unexpected assembly result: %v", err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()
	unlock, err := config.Lock(ctx)
	if err != nil {
		t.Fatalf("failed assembly leaked lock: %v", err)
	}
	unlock()
}
