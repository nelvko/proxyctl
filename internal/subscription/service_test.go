package subscription

import (
	"context"
	"errors"
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
	stopped  bool
}

func (f *fakeKernel) ConfigFile() string { return f.cfgFile }

func (f *fakeKernel) ConfigFormat() kernel.ConfigFormat { return kernel.FormatClash }

func (f *fakeKernel) LatestArtifact() (*kernel.Artifact, error) {
	return nil, errors.New("not needed")
}

func (f *fakeKernel) InstalledVersion(ctx context.Context) (string, error) {
	return "", errors.New("not needed")
}

func (f *fakeKernel) IsActive() (bool, error) { return !f.stopped, nil }

func (f *fakeKernel) Restart() error {
	f.restarts++
	f.stopped = false
	return nil
}

func (f *fakeKernel) TestConfig(ctx context.Context, file string) error {
	b, err := os.ReadFile(file)
	if err != nil {
		return err
	}
	if !strings.Contains(string(b), "proxies:") {
		return &kernel.ConfigValidationError{Output: "invalid config: no proxies key"}
	}
	return nil
}

// newTestService wires a service against an isolated config root.
func newTestService(t *testing.T) (*Service, *fakeKernel) {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("AppData", t.TempDir())

	k := &fakeKernel{cfgFile: filepath.Join(t.TempDir(), "kernel-config.yaml")}
	return NewService(&config.SubscriptionConfig{}, k), k
}

func addTestProfile(svc *Service, draft *profile) error {
	if err := svc.Add(draft); err != nil {
		return err
	}
	if svc.SelectedName() == "" {
		_, err := svc.Use(draft.Name)
		return err
	}
	return nil
}

func TestUseRejectsOtherKernelFormat(t *testing.T) {
	svc, _ := newTestService(t)
	svc.subConfig.Profiles = []config.Profile{{Name: "json", Format: "sing-box", File: "/unused.json"}}
	if _, err := svc.Use("json"); err == nil || !strings.Contains(err.Error(), "sing-box") {
		t.Fatalf("error = %v", err)
	}
}

func (f *fakeKernel) Stop() error { f.stopped = true; return nil }

func writeFile(t *testing.T, path, content string) string {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func (f *fakeKernel) ProxyEndpoints() (kernel.Endpoints, error) {
	k, err := kernel.New(&config.KernelConfig{Name: "mihomo", ConfigFile: f.cfgFile})
	if err != nil {
		return kernel.Endpoints{}, err
	}
	return k.ProxyEndpoints()
}
