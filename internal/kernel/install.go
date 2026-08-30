package kernel

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/nelvko/proxyctl/internal/config"
	"github.com/nelvko/proxyctl/internal/httpx"
	"github.com/nelvko/unisvc"
)

const downloadTimeout = 150 * time.Second

// DefaultConfig returns the default on-disk layout for a kernel:
// binary under ~/.local/share/proxyctl/kernels/<name>/,
// config under ~/.config/proxyctl/<name>/.
func DefaultConfig(name string) config.KernelConfig {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	return config.KernelConfig{
		Name:       name,
		Bin:        filepath.Join(home, ".local", "share", config.AppName, "kernels", name, name),
		ConfigDir:  filepath.Join(home, ".config", config.AppName, name),
		ConfigFile: filepath.Join(home, ".config", config.AppName, name, "config.yaml"),
	}
}

// Install downloads the kernel, installs its user service and registers it
// in the app config. The first installed kernel becomes active.
func Install(name string) error {
	desc, ok := registry[name]
	if !ok {
		return fmt.Errorf("unknown kernel %q, available: %s", name, joinNames())
	}
	if !desc.ready {
		return fmt.Errorf("kernel %q is not supported yet", name)
	}

	kcfg := DefaultConfig(name)
	k, err := New(&kcfg)
	if err != nil {
		return err
	}
	// Fail fast before the long download if the init system can't host a
	// service at all (e.g. non-systemd Linux, macOS).
	if is := k.InitSystem(); is != unisvc.InitSystemd {
		return fmt.Errorf("service management is not available on %s (systemd only for now)", is)
	}

	if err := os.MkdirAll(kcfg.ConfigDir, 0o755); err != nil {
		return err
	}
	if _, err := os.Stat(kcfg.ConfigFile); errors.Is(err, os.ErrNotExist) {
		f, err := os.Create(kcfg.ConfigFile)
		if err != nil {
			return err
		}
		f.Close()
	}

	if err := download(k, kcfg.Bin); err != nil {
		return err
	}

	spec := unisvc.Spec{
		Command: kcfg.Bin,
		Args:    []string{"-d", kcfg.ConfigDir, "-f", kcfg.ConfigFile},
	}
	if err := installService(k, &spec); err != nil {
		return err
	}

	appCfg, err := config.LoadAppConfig()
	if err != nil {
		return err
	}
	appCfg.SetKernel(kcfg)
	if appCfg.Use == "" {
		appCfg.Use = kcfg.Name
	}
	return config.SaveAppConfig(appCfg)
}

// download fetches the kernel release and extracts the binary to bin.
func download(k Kernel, bin string) error {
	url, err := k.DownloadURL()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(bin), 0o755); err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), downloadTimeout)
	defer cancel()

	f, err := os.CreateTemp("", "proxyctl-kernel-*")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tmpName := f.Name()
	defer func() {
		f.Close()
		os.Remove(tmpName)
	}()

	if err := httpx.Download(ctx, url, f); err != nil {
		return err
	}
	if err := httpx.Ungzip(f, bin); err != nil {
		return fmt.Errorf("extract kernel: %w", err)
	}
	return os.Chmod(bin, 0o755)
}

// installService installs the service, replacing a previously installed
// unit so re-running install converges to the new spec instead of failing.
// A service that was running before is restarted afterwards.
func installService(k Kernel, spec *unisvc.Spec) error {
	wasActive := false
	if on, err := k.IsActive(); err == nil && on {
		wasActive = true
	}

	err := k.Install(spec)
	if errors.Is(err, unisvc.ErrAlreadyInstalled) {
		if err := k.UnInstall(); err != nil {
			return fmt.Errorf("reinstall service: %w", err)
		}
		err = k.Install(spec)
	}
	if err != nil {
		return err
	}
	if wasActive {
		return k.Start()
	}
	return nil
}

func joinNames() string {
	out := ""
	for i, name := range names {
		if i > 0 {
			out += ", "
		}
		out += name
	}
	return out
}
