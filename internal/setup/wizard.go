package setup

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"charm.land/huh/v2"
	"github.com/nelvko/proxyctl/internal/config"
	"github.com/nelvko/proxyctl/internal/httpx"
	"github.com/nelvko/proxyctl/internal/kernel"
	"github.com/nelvko/proxyctl/internal/log"
	"github.com/nelvko/unisvc"
)

const defaultKernelName = "mihomo"

func CanWriteTo(path string) error {
	if len(path) == 0 {
		return fmt.Errorf("can't empty")

	}
	current := filepath.Clean(path)
	for {
		parent := filepath.Dir(current)
		if parent == current {
			break
		}
		_, err := os.Stat(parent)
		if os.IsNotExist(err) {
			current = parent
			continue
		}
		f, err := os.CreateTemp(parent, "test*")
		if err != nil {
			if os.IsPermission(err) {
				return fmt.Errorf("permission denied: unable to write to %s. Try running with sudo or change the path", path)
			}
			// e.g. a path component is a regular file (ENOTDIR):
			// report instead of looping forever.
			return fmt.Errorf("unable to write to %s: %w", path, err)
		}
		f.Close()
		os.Remove(f.Name())
		return nil
	}
	return nil
}

func kernelConfigForm(kernelCfg *config.KernelConfig) error {
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Select proxy kernel").
				Options(
					huh.NewOption(defaultKernelName, defaultKernelName),
				).
				Value(&kernelCfg.Name),

			huh.NewInput().
				Title("proxy kernel's binary").
				Value(&kernelCfg.Bin).
				Validate(CanWriteTo),

			huh.NewInput().
				Title("configuration directory").
				Value(&kernelCfg.ConfigDir).
				Validate(CanWriteTo),
		),
	)

	return form.Run()
}

func ConfirmWizard() (bool, error) {
	var confirm bool
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewConfirm().
				Title("🔧 Proxyctl is not initialized").
				Description("🤔 Would you like to run the wizard now?").
				Affirmative("🚀 Yes, let's go!").
				Negative("🚫 No, maybe later").
				Value(&confirm),
		),
	)
	if err := form.Run(); err != nil {
		return false, fmt.Errorf("failed to run program: %w", err)
	}
	return confirm, nil
}

func Wizard(yes bool) error {
	if !yes {
		confirm, err := ConfirmWizard()
		if err != nil {
			return err
		}
		if !confirm {
			fmt.Println("No problem! You can initialize whenever you're ready by running `proxyctl init`.")
			return nil
		}
	}

	appCfg := &config.AppConfig{
		Kernel: defaultKernelConfig(),
	}
	kernelCfg := &appCfg.Kernel
	if !yes {
		if err := kernelConfigForm(kernelCfg); err != nil {
			return err
		}
	}
	kernelCfg.ConfigFile = filepath.Join(kernelCfg.ConfigDir, "config.yaml")
	if err := os.MkdirAll(kernelCfg.ConfigDir, 0755); err != nil {
		return err
	}
	cfgFile, err := os.Create(kernelCfg.ConfigFile)
	if err != nil {
		return err
	}
	if err := cfgFile.Close(); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(kernelCfg.Bin), 0755); err != nil {
		return err
	}
	k, err := kernel.New(kernelCfg)
	if err != nil {
		return err
	}
	// Fail fast before the long download if the init system can't
	// host a service at all (e.g. non-systemd Linux, macOS).
	if is := k.InitSystem(); is != unisvc.InitSystemd {
		return fmt.Errorf("service management is not available on %s (systemd only for now)", is)
	}
	url, err := k.DownloadURL()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Second)
	defer cancel()

	f, err := os.CreateTemp("", filepath.Base(url))
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}

	defer func() {
		f.Close()
		os.Remove(f.Name())
	}()

	if err := httpx.Download(ctx, url, f); err != nil {
		return err
	}

	// if err := f.Sync(); err != nil {
	// 	return fmt.Errorf("failed to sync file: %w", err)
	// }
	// if _, err := f.Seek(0, io.SeekStart); err != nil {
	// 	return fmt.Errorf("failed to seek file: %w", err)
	// }

	if err := httpx.Ungzip(f, kernelCfg.Bin); err != nil {
		return fmt.Errorf("failed to extract file: %w", err)
	}
	if err := os.Chmod(kernelCfg.Bin, 0755); err != nil {
		return err
	}

	spec := unisvc.Spec{
		Command: kernelCfg.Bin,
		Args:    []string{"-d", kernelCfg.ConfigDir, "-f", kernelCfg.ConfigFile},
	}
	err = installService(k, &spec)
	if err != nil {
		return err
	}

	if err := config.SaveAppConfig(appCfg); err != nil {
		return fmt.Errorf("Failed to write configuration file: %w", err)
	}
	log.Ok("Successfully initialized", "✅")
	return nil
}

// installService installs the service, replacing a previously installed
// unit so that re-running init (e.g. `init --force` with new paths)
// converges to the new spec instead of failing.
func installService(k kernel.Kernel, spec *unisvc.Spec) error {
	err := k.Install(spec)
	if errors.Is(err, unisvc.ErrAlreadyInstalled) {
		if err := k.UnInstall(); err != nil {
			return fmt.Errorf("reinstall service: %w", err)
		}
		err = k.Install(spec)
	}
	return err
}

func defaultKernelConfig() config.KernelConfig {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		homeDir = "."
	}

	return config.KernelConfig{
		Name:      defaultKernelName,
		Bin:       filepath.Join(homeDir, ".local", "bin", defaultKernelName),
		ConfigDir: filepath.Join(homeDir, ".config", defaultKernelName),
	}
}
