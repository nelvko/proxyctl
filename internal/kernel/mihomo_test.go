package kernel

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/nelvko/proxyctl/internal/config"
)

func TestParseVersion(t *testing.T) {
	const output = "Mihomo Meta v1.19.30 linux amd64 with go1.26.6 Sun Aug 16 10:02:25 UTC 2026\nUse tags: with_gvisor\n"

	if got := parseVersion(output); got != "v1.19.30" {
		t.Fatalf("parseVersion() = %q, want %q", got, "v1.19.30")
	}
	if got := parseVersion("mihomo unknown version"); got != "" {
		t.Fatalf("parseVersion() = %q, want empty", got)
	}
}

func TestConfigValidationHonorsCancellation(t *testing.T) {
	binary := filepath.Join(t.TempDir(), "kernel")
	if err := os.WriteFile(binary, []byte("#!/bin/sh\nexec sleep 30\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	m := Mihomo{cfg: config.KernelConfig{Bin: binary}}
	ctx, cancel := context.WithTimeout(t.Context(), 100*time.Millisecond)
	defer cancel()
	before := time.Now()
	err := m.TestConfig(ctx, "/unused")
	if !errors.Is(err, context.DeadlineExceeded) || time.Since(before) > 2*time.Second {
		t.Fatalf("validation did not cancel promptly: %v", err)
	}
}

func TestMihomoServiceSpec(t *testing.T) {
	m := Mihomo{cfg: config.KernelConfig{Bin: "/custom dir/mihomo", ConfigDir: "/config dir", ConfigFile: "/config dir/custom.yaml"}}
	spec := m.ServiceSpec()
	if spec.Command != m.cfg.Bin || !reflect.DeepEqual(spec.Args, []string{"-d", m.cfg.ConfigDir, "-f", m.cfg.ConfigFile}) {
		t.Fatalf("wrong service invocation: %+v", spec)
	}
}
