package kernel

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/nelvko/proxyctl/internal/config"
)

func defaultTestAdapter(format ConfigFormat, file string) Adapter {
	cfg := config.KernelConfig{ConfigFile: file}
	if format == FormatClash {
		return Mihomo{cfg: cfg}
	}
	return SingBox{cfg: cfg}
}

func TestEnsureDefaultConfigInitializesUsableLoopbackProxy(t *testing.T) {
	for _, format := range []ConfigFormat{FormatClash, FormatSingBox} {
		for _, existing := range []bool{false, true} {
			t.Run(string(format)+map[bool]string{false: "/missing", true: "/empty"}[existing], func(t *testing.T) {
				file := filepath.Join(t.TempDir(), "kernel", "config")
				if existing {
					if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
						t.Fatal(err)
					}
					if err := os.WriteFile(file, nil, 0o644); err != nil {
						t.Fatal(err)
					}
				}
				k := defaultTestAdapter(format, file)
				if err := EnsureDefaultConfig(k); err != nil {
					t.Fatal(err)
				}
				endpoints, err := k.ProxyEndpoints()
				if err != nil || endpoints.HTTP != "http://127.0.0.1:7890" || endpoints.SOCKS != "socks5h://127.0.0.1:7890" {
					t.Fatalf("generated endpoints: %+v, %v", endpoints, err)
				}
				if isDefault, err := IsDefaultConfig(k); err != nil || !isDefault {
					t.Fatalf("default recognition: %t, %v", isDefault, err)
				}
				before, err := os.Stat(file)
				if err != nil {
					t.Fatal(err)
				}
				if before.Mode().Perm() != 0o600 {
					t.Fatalf("generated permissions: %v", before.Mode())
				}
				if err := EnsureDefaultConfig(k); err != nil {
					t.Fatal(err)
				}
				after, err := os.Stat(file)
				if err != nil || !os.SameFile(before, after) {
					t.Fatalf("repeated initialization replaced file: %v", err)
				}
			})
		}
	}
}

func TestEnsureDefaultConfigPreservesAllNonemptyContent(t *testing.T) {
	for _, format := range []ConfigFormat{FormatClash, FormatSingBox} {
		for _, content := range []string{"{}\n", " ", "broken: [", "{invalid", "mixed-port: 1080\n"} {
			t.Run(string(format)+"/"+content, func(t *testing.T) {
				file := filepath.Join(t.TempDir(), "config")
				if err := os.WriteFile(file, []byte(content), 0o640); err != nil {
					t.Fatal(err)
				}
				k := defaultTestAdapter(format, file)
				if err := EnsureDefaultConfig(k); err != nil {
					t.Fatal(err)
				}
				raw, err := os.ReadFile(file)
				if err != nil || string(raw) != content {
					t.Fatalf("existing content changed: %q, %v", raw, err)
				}
				if isDefault, err := IsDefaultConfig(k); err != nil || isDefault {
					t.Fatalf("custom content marked default: %t, %v", isDefault, err)
				}
			})
		}
	}
}

func TestEnsureDefaultConfigPreservesSymlinks(t *testing.T) {
	for _, existing := range []bool{false, true} {
		dir := t.TempDir()
		target := filepath.Join(dir, "target")
		if existing {
			if err := os.WriteFile(target, nil, 0o600); err != nil {
				t.Fatal(err)
			}
		}
		file := filepath.Join(dir, "config")
		if err := os.Symlink(target, file); err != nil {
			t.Fatal(err)
		}
		if err := EnsureDefaultConfig(defaultTestAdapter(FormatClash, file)); err != nil {
			t.Fatal(err)
		}
		if got, err := os.Readlink(file); err != nil || got != target {
			t.Fatalf("symlink changed: %q, %v", got, err)
		}
		raw, err := os.ReadFile(target)
		if existing {
			if err != nil || len(raw) != 0 {
				t.Fatalf("symlink target overwritten: %q, %v", raw, err)
			}
		} else if !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("dangling symlink target created: %v", err)
		}
	}
}

func TestDefaultConfigReportsFileErrors(t *testing.T) {
	dir := t.TempDir()
	k := defaultTestAdapter(FormatClash, dir)
	if err := EnsureDefaultConfig(k); err == nil {
		t.Fatal("accepted a directory as a config file")
	}
	if _, err := IsDefaultConfig(k); err == nil {
		t.Fatal("reading a directory succeeded")
	}
	file := filepath.Join(dir, "config")
	if err := os.WriteFile(file, []byte("private"), 0); err != nil {
		t.Fatal(err)
	}
	k = defaultTestAdapter(FormatClash, file)
	if _, err := os.ReadFile(file); errors.Is(err, os.ErrPermission) {
		if err := EnsureDefaultConfig(k); !errors.Is(err, os.ErrPermission) {
			t.Fatalf("lost read permission error: %v", err)
		}
		if _, err := IsDefaultConfig(k); !errors.Is(err, os.ErrPermission) {
			t.Fatalf("lost read permission error: %v", err)
		}
	}
}

func TestDefaultConfigQueriesAndCustomAdaptersDoNotWrite(t *testing.T) {
	file := filepath.Join(t.TempDir(), "missing", "config")
	if _, err := IsDefaultConfig(defaultTestAdapter(FormatSingBox, file)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("missing config query: %v", err)
	}
	k := Custom{cfg: config.KernelConfig{ConfigFile: file}}
	if err := EnsureDefaultConfig(k); err != nil {
		t.Fatal(err)
	}
	if isDefault, err := IsDefaultConfig(k); err != nil || isDefault {
		t.Fatalf("custom adapter: %t, %v", isDefault, err)
	}
	if _, err := os.Stat(filepath.Dir(file)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("read or unknown adapter initialized directories: %v", err)
	}
}
