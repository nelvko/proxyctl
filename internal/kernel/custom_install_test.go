package kernel

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/nelvko/proxyctl/internal/config"
)

func TestInstallLocal(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "source")
	bin := filepath.Join(dir, "installed", "core")
	if err := os.WriteFile(src, []byte("new binary"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(bin), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(bin, []byte("old binary"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := InstallLocal(context.Background(), src, bin, false); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(bin); string(got) != "new binary" {
		t.Fatalf("installed binary = %q", got)
	}
	if got, _ := os.ReadFile(src); string(got) != "new binary" {
		t.Fatalf("source binary = %q", got)
	}
	if info, err := os.Stat(bin); err != nil || info.Mode().Perm() != 0o755 {
		t.Fatalf("installed mode = %v, %v", info, err)
	}
	if err := InstallLocal(context.Background(), filepath.Join(dir, "missing"), bin, false); err == nil {
		t.Fatal("accepted missing source")
	}
	if got, _ := os.ReadFile(bin); string(got) != "new binary" {
		t.Fatalf("failed copy replaced binary: %q", got)
	}
}

func TestInstallLocalSingBoxCompanion(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "sing-box")
	if err := os.WriteFile(src, []byte("binary"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "libcronet.so"), []byte("library"), 0o644); err != nil {
		t.Fatal(err)
	}
	bin := filepath.Join(dir, "installed", "sing-box")
	if err := InstallLocal(context.Background(), src, bin, true); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(filepath.Join(filepath.Dir(bin), "libcronet.so")); string(got) != "library" {
		t.Fatalf("companion = %q", got)
	}
}

func TestCustomHasBareServiceSpec(t *testing.T) {
	c := Custom{cfg: config.KernelConfig{Bin: "/tmp/core"}}
	if spec := c.ServiceSpec(); spec.Command != "/tmp/core" {
		t.Fatalf("command = %q", spec.Command)
	}
	if !errors.Is(c.TestConfig(context.Background(), ""), ErrNoValidation) {
		t.Fatal("custom kernel unexpectedly validates config")
	}
	if _, err := c.ProxyEndpoints(); err == nil {
		t.Fatal("custom kernel unexpectedly has a proxy endpoint")
	}
}

func TestTarGzExtractsExecutableAndCompanion(t *testing.T) {
	dir := t.TempDir()
	var data bytes.Buffer
	gz := gzip.NewWriter(&data)
	tw := tar.NewWriter(gz)
	for _, item := range []struct{ name, body string }{{"release/libcronet.so", "library"}, {"release/core", "binary"}} {
		if err := tw.WriteHeader(&tar.Header{Name: item.name, Mode: 0o644, Size: int64(len(item.body)), Typeflag: tar.TypeReg}); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write([]byte(item.body)); err != nil {
			t.Fatal(err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	archive := filepath.Join(dir, "core.tar.gz")
	if err := os.WriteFile(archive, data.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(archive)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	bin := filepath.Join(dir, "installed", "core")
	if err := os.MkdirAll(filepath.Dir(bin), 0o755); err != nil {
		t.Fatal(err)
	}
	art := &Artifact{Archive: "tar.gz", Entry: "release/core", Extra: []string{"release/libcronet.so"}}
	if err := extractExtraFiles(f, bin, art); err != nil {
		t.Fatal(err)
	}
	if err := extractArtifact(f, bin, art); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(bin); string(got) != "binary" {
		t.Fatalf("binary = %q", got)
	}
	if got, _ := os.ReadFile(filepath.Join(filepath.Dir(bin), "libcronet.so")); string(got) != "library" {
		t.Fatalf("companion = %q", got)
	}
	art.Entry = "release/missing"
	if err := extractArtifact(f, bin, art); err == nil {
		t.Fatal("accepted missing executable")
	}
	if got, _ := os.ReadFile(bin); string(got) != "binary" {
		t.Fatal("missing executable replaced binary")
	}
}
