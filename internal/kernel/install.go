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

// Download fetches the kernel release and extracts the binary to bin,
// replacing it atomically.
func Download(k Kernel, bin string) error {
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

// InstallService installs the service, replacing a previously installed
// unit so re-running install converges to the new spec instead of failing.
// A service that was running before is restarted afterwards.
func InstallService(k Kernel, spec *unisvc.Spec) error {
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
