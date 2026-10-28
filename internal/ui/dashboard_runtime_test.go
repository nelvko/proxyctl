package ui

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/nelvko/proxyctl/internal/config"
	"github.com/nelvko/proxyctl/internal/kernel"
	"github.com/nelvko/unisvc"
)

type dashboardRuntimeTestKernel struct {
	kernel.Kernel
	endpoints   kernel.Endpoints
	endpointErr error
	initSystem  unisvc.InitSystem
	version     func(context.Context) (string, error)
}

func (k dashboardRuntimeTestKernel) ProxyEndpoints() (kernel.Endpoints, error) {
	return k.endpoints, k.endpointErr
}
func (k dashboardRuntimeTestKernel) InitSystem() unisvc.InitSystem { return k.initSystem }
func (k dashboardRuntimeTestKernel) InstalledVersion(ctx context.Context) (string, error) {
	return k.version(ctx)
}

func TestDashboardRuntimeInfoReadsAdapterWithoutQueryingVersion(t *testing.T) {
	calls := 0
	k := dashboardRuntimeTestKernel{
		endpoints:  kernel.Endpoints{HTTP: "http://user:password@127.0.0.1:7890", SOCKS: "socks5://user:password@127.0.0.1:7891"},
		initSystem: unisvc.InitSystemd,
		version:    func(context.Context) (string, error) { calls++; return "v1.2.3", nil },
	}
	cfg := &config.KernelConfig{Name: "work", Type: "mihomo", Bin: filepath.Join(t.TempDir(), "not-installed")}
	info := readDashboardRuntimeInfo(cfg, k)
	if calls != 0 || info.KernelType != "mihomo" || info.Platform != runtime.GOOS+"/"+runtime.GOARCH || info.Service != "systemd" {
		t.Fatalf("unexpected runtime observation: %+v, version calls=%d", info, calls)
	}
	if strings.Contains(info.HTTP+info.SOCKS, "user") || strings.Contains(info.HTTP+info.SOCKS, "password") || !strings.Contains(info.HTTP, "127.0.0.1:7890") || !strings.Contains(info.SOCKS, "127.0.0.1:7891") {
		t.Fatal("runtime endpoints leaked credentials or lost their actual ports")
	}
	if info.ServiceAvailable != (runtime.GOOS == "linux") {
		t.Fatal("unsupported platform acquired a working user service backend")
	}
	var state dashboardVersionState
	command := state.Refresh(t.Context(), info)
	if command == nil || calls != 0 {
		t.Fatal("version query was not deferred to a command")
	}
	if !state.Apply(command().(dashboardVersionMsg)) || calls != 1 || state.Value != "v1.2.3" {
		t.Fatal("deferred installed-version query failed")
	}
	failure := errors.New("unreadable active config")
	k.endpointErr, k.initSystem = failure, unisvc.InitUnknown
	info = readDashboardRuntimeInfo(cfg, k)
	if !errors.Is(info.EndpointErr, failure) || info.HTTP != "" || info.SOCKS != "" || info.ServiceAvailable {
		t.Fatal("failed endpoint observation produced usable values")
	}
	if info := readDashboardRuntimeInfo(nil, nil); info.Platform == "" || info.VersionKey != "" || info.versionLoader != nil {
		t.Fatal("missing kernel lost platform information or scheduled a version query")
	}
}

func TestDashboardVersionKeyTracksKernelAndBinaryChanges(t *testing.T) {
	path := filepath.Join(t.TempDir(), "kernel")
	cfg := config.KernelConfig{Name: "one", Type: "mihomo", Bin: path}
	missing := dashboardVersionKey(cfg)
	if missing == "" || missing != dashboardVersionKey(cfg) {
		t.Fatal("unavailable binary did not retain a stable identity")
	}
	if err := os.WriteFile(path, []byte("first"), 0o600); err != nil {
		t.Fatal(err)
	}
	first := dashboardVersionKey(cfg)
	if first == missing {
		t.Fatal("installing a binary did not invalidate its version")
	}
	if err := os.WriteFile(path, []byte("longer replacement"), 0o600); err != nil {
		t.Fatal(err)
	}
	replaced := dashboardVersionKey(cfg)
	if replaced == first {
		t.Fatal("replacing a binary did not invalidate its version")
	}
	cfg.Name = "two"
	if dashboardVersionKey(cfg) == replaced {
		t.Fatal("changing the selected kernel did not change its identity")
	}
}

func TestDashboardVersionCacheRejectsStaleResultsAndCachesFailures(t *testing.T) {
	var state dashboardVersionState
	calls := 0
	loader := func(ctx context.Context) (string, error) {
		calls++
		if _, ok := ctx.Deadline(); !ok {
			t.Fatal("installed-version query has no deadline")
		}
		return "v2", nil
	}
	a := dashboardRuntimeInfo{VersionKey: "a", versionLoader: loader}
	b := dashboardRuntimeInfo{VersionKey: "b", versionLoader: loader}
	oldA := state.Refresh(t.Context(), a)
	if !state.Loading || state.Loaded || state.Refresh(t.Context(), a) != nil || calls != 0 {
		t.Fatal("periodic refresh repeated an in-flight query")
	}
	oldB := state.Refresh(t.Context(), b)
	newA := state.Refresh(t.Context(), a)
	if state.Apply(oldA().(dashboardVersionMsg)) || state.Apply(oldB().(dashboardVersionMsg)) {
		t.Fatal("stale query replaced the current binary's version")
	}
	if !state.Apply(newA().(dashboardVersionMsg)) || state.Value != "v2" || state.Loading || !state.Loaded || state.Refresh(t.Context(), a) != nil {
		t.Fatal("completed version was not cached")
	}
	state.Invalidate()
	failure := errors.New("version process failed")
	a.versionLoader = func(context.Context) (string, error) { return "incorrect", failure }
	command := state.Refresh(t.Context(), a)
	if !state.Apply(command().(dashboardVersionMsg)) || !errors.Is(state.Err, failure) || state.Value != "" || !state.Loaded || state.Refresh(t.Context(), a) != nil {
		t.Fatal("failed version query was not cached as unavailable")
	}
	state.Invalidate()
	if state.Refresh(t.Context(), a) == nil {
		t.Fatal("explicit invalidation did not allow a retry")
	}
	state.Refresh(t.Context(), dashboardRuntimeInfo{})
	if state.Key != "" || state.Value != "" || state.Loaded || state.Loading {
		t.Fatal("clearing kernel selection retained a prior version")
	}
}

func TestDashboardVersionCancellationDoesNotCallLoader(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	called := false
	info := dashboardRuntimeInfo{VersionKey: "a", versionLoader: func(context.Context) (string, error) { called = true; return "v1", nil }}
	var state dashboardVersionState
	command := state.Refresh(ctx, info)
	if !state.Apply(command().(dashboardVersionMsg)) || called || !errors.Is(state.Err, context.Canceled) || state.Value != "" || state.Loading {
		t.Fatal("cancelled version query executed or produced a version")
	}
}
