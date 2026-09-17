package app

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/nelvko/proxyctl/internal/config"
	"github.com/nelvko/proxyctl/internal/kernel"
)

type testKernel struct {
	endpoints   kernel.Endpoints
	endpointErr error
	kernel.Kernel
	file                             string
	running                          bool
	starts, stops, restarts          int
	startErr, stopErr, validationErr error
}

func (k *testKernel) ProxyEndpoints() (kernel.Endpoints, error) { return k.endpoints, k.endpointErr }
func (k *testKernel) ConfigFile() string                        { return k.file }
func (k *testKernel) TestConfig(context.Context, string) error  { return k.validationErr }
func (k *testKernel) IsActive() (bool, error)                   { return k.running, nil }
func (k *testKernel) Start() error {
	k.starts++
	if k.startErr != nil {
		return k.startErr
	}
	k.running = true
	return nil
}
func (k *testKernel) Stop() error {
	k.stops++
	if k.stopErr != nil {
		return k.stopErr
	}
	k.running = false
	return nil
}
func (k *testKernel) Restart() error { k.restarts++; k.running = true; return nil }

func switchingApp(t *testing.T, running bool) (*App, *testKernel, *testKernel) {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("AppData", t.TempDir())
	dir := t.TempDir()
	source := filepath.Join(dir, "profile.yaml")
	if err := os.WriteFile(source, []byte("proxies: []\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	old := &testKernel{file: filepath.Join(dir, "old.yaml"), running: running}
	next := &testKernel{file: filepath.Join(dir, "next.yaml")}
	a := &App{
		Config:        &config.Config{Use: "mihomo", Kernels: []config.KernelConfig{{Name: "mihomo"}, {Name: "clash"}}},
		Subscriptions: &config.SubscriptionConfig{Use: "one", Profiles: []config.Profile{{Name: "one", File: source}}},
		newKernel: func(c *config.KernelConfig) (kernel.Kernel, error) {
			if c.Name == "mihomo" {
				return old, nil
			}
			return next, nil
		},
	}
	return a, old, next
}

func TestUseKernelPreservesRunningState(t *testing.T) {
	for _, running := range []bool{false, true} {
		a, old, next := switchingApp(t, running)
		if err := a.UseKernel(context.Background(), "clash"); err != nil {
			t.Fatal(err)
		}
		if a.Config.Use != "clash" || old.running || next.running != running {
			t.Fatalf("state: selection=%s old=%t next=%t", a.Config.Use, old.running, next.running)
		}
		cfg, err := config.Load()
		if err != nil || cfg.Use != "clash" {
			t.Fatalf("selection not saved: %+v %v", cfg, err)
		}
	}
}

func TestUseKernelValidatesBeforeStopping(t *testing.T) {
	a, old, next := switchingApp(t, true)
	next.validationErr = errors.New("invalid target config")
	if err := a.UseKernel(context.Background(), "clash"); err == nil {
		t.Fatal("expected validation failure")
	}
	if old.stops != 0 || !old.running || a.Config.Use != "mihomo" {
		t.Fatal("validation failure changed active state")
	}
}

func TestUseKernelPropagatesStopFailure(t *testing.T) {
	a, old, next := switchingApp(t, true)
	old.stopErr = errors.New("stop failed")
	if err := a.UseKernel(context.Background(), "clash"); !errors.Is(err, old.stopErr) {
		t.Fatalf("got %v", err)
	}
	if next.starts != 0 || a.Config.Use != "mihomo" {
		t.Fatal("switched despite stop failure")
	}
}

func TestUseKernelRestoresPreviousServiceOnStartFailure(t *testing.T) {
	a, old, next := switchingApp(t, true)
	next.startErr = errors.New("start failed")
	if err := a.UseKernel(context.Background(), "clash"); !errors.Is(err, next.startErr) {
		t.Fatalf("got %v", err)
	}
	if a.Config.Use != "mihomo" || !old.running || next.running {
		t.Fatal("previous running state was not restored")
	}
}

func TestServiceControlDoesNotLoadSubscriptions(t *testing.T) {
	a, old, _ := switchingApp(t, false)
	a.Subscriptions = nil
	if err := a.ControlKernel(context.Background(), "start"); err != nil {
		t.Fatal(err)
	}
	if err := a.ControlKernel(context.Background(), "start"); err != nil {
		t.Fatal(err)
	}
	if old.starts != 1 || a.Subscriptions != nil {
		t.Fatal("start not idempotent or loaded unrelated state")
	}
	if err := a.ControlKernel(context.Background(), "stop"); err != nil {
		t.Fatal(err)
	}
	if old.running {
		t.Fatal("stop left service running")
	}
}
