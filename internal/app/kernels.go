package app

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"os"

	"github.com/nelvko/proxyctl/internal/config"
	"github.com/nelvko/proxyctl/internal/kernel"
	"github.com/nelvko/proxyctl/internal/subscription"
	"github.com/nelvko/unisvc"
)

// InstallKernel installs the kernel binary, installs its user service and
// registers it. The first installed kernel becomes selected.
func (a *App) InstallKernel(ctx context.Context, name string, options ...config.KernelConfig) error {
	kcfg, err := config.DefaultKernelConfig(name)
	if err != nil {
		return err
	}
	if existing := a.Config.KernelByName(name); existing != nil {
		kcfg = *existing
	}
	if len(options) > 0 {
		kcfg = options[0]
	}
	if kcfg.Name != name {
		return fmt.Errorf("kernel name mismatch: %q != %q", kcfg.Name, name)
	}
	if kcfg.Type == "" && !kernel.Implemented(name) {
		return fmt.Errorf("kernel %q needs --bin for a local installation", name)
	}
	if kcfg.Type == "custom" && kcfg.Source == "" {
		return fmt.Errorf("kernel %q needs --bin", name)
	}
	k, err := a.makeKernel(&kcfg)
	if err != nil {
		return err
	}
	// Fail fast before the long download if the init system can't host a
	// service at all (e.g. non-systemd Linux, macOS).
	if is := k.InitSystem(); is != unisvc.InitSystemd {
		return fmt.Errorf("service management is not available on %s (systemd only for now)", is)
	}

	if err := ctx.Err(); err != nil {
		return err
	}
	if kcfg.Type != "custom" {
		if err := os.MkdirAll(kcfg.ConfigDir, 0o755); err != nil {
			return err
		}
	}
	if err := kernel.EnsureDefaultConfig(k); err != nil {
		return err
	}
	if kcfg.Source != "" {
		if err := kernel.InstallLocal(ctx, kcfg.Source, kcfg.Bin, kcfg.Type == "sing-box"); err != nil {
			return err
		}
	} else if err := a.downloadKernel(ctx, k, kcfg.Bin); err != nil && !errors.Is(err, kernel.ErrUpToDate) {
		return err
	}

	if err := kernel.InstallService(k, k.ServiceSpec()); err != nil {
		return err
	}

	a.Config.SetKernel(kcfg)
	if a.Config.Use == "" {
		a.Config.Use = kcfg.Name
	}
	return config.Save(a.Config)
}

// UseKernel selects a kernel and applies the selected subscription. A running
// session moves to the new kernel; a stopped session stays stopped. Failure
// restores the previous selection and restarts its service when necessary.
func (a *App) UseKernel(ctx context.Context, name string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	target := a.Config.KernelByName(name)
	if target == nil {
		return fmt.Errorf("kernel %q is not installed; run `proxyctl install %s`", name, name)
	}
	next, err := a.makeKernel(target)
	if err != nil {
		return err
	}
	if err := a.LoadSubscriptions(); err != nil {
		return err
	}
	previousName := a.Config.Use
	previous := a.Config.SelectedKernel()
	previousUse := a.Subscriptions.Use
	previousSelections := maps.Clone(a.Subscriptions.UseByFormat)
	var old kernel.Kernel
	if previous != nil {
		old, err = a.makeKernel(previous)
		if err != nil {
			return err
		}
	}
	if a.Subscriptions.UseByFormat == nil {
		a.Subscriptions.UseByFormat = make(map[string]string)
	}
	if old != nil && previousUse != "" {
		a.Subscriptions.UseByFormat[string(old.ConfigFormat())] = previousUse
	}
	selected := a.Subscriptions.UseByFormat[string(next.ConfigFormat())]
	if old == nil && previousUse != "" {
		for _, p := range a.Subscriptions.Profiles {
			format := p.Format
			if format == "" {
				format = string(kernel.FormatClash)
			}
			if p.Name == previousUse && format == string(next.ConfigFormat()) {
				selected = previousUse
				break
			}
		}
	}
	a.Subscriptions.Use = selected
	restoreSelection := func() {
		a.Subscriptions.Use = previousUse
		a.Subscriptions.UseByFormat = maps.Clone(previousSelections)
	}
	svc := subscription.NewService(a.Subscriptions, next).WithContext(ctx)
	profile, profileErr := svc.Selected()
	if profileErr != nil && !errors.Is(profileErr, subscription.ErrNoSelectedProfile) {
		restoreSelection()
		return profileErr
	}
	if profileErr == nil {
		if err := next.TestConfig(ctx, profile.File); err != nil {
			restoreSelection()
			return err
		}
	} else {
		if err := kernel.EnsureDefaultConfig(next); err != nil {
			restoreSelection()
			return err
		}
		if err := next.TestConfig(ctx, next.ConfigFile()); err != nil && !errors.Is(err, kernel.ErrNoValidation) {
			restoreSelection()
			return err
		}
	}
	if err := ctx.Err(); err != nil {
		restoreSelection()
		return err
	}
	wasRunning := false
	if previous != nil && previousName != name {
		wasRunning, err = old.IsActive()
		if err != nil {
			restoreSelection()
			return err
		}
		if wasRunning {
			if err := old.Stop(); err != nil {
				restoreSelection()
				return err
			}
		}
	}
	rollback := func(cause error) error {
		a.Config.Use = previousName
		restoreSelection()
		if err := config.Save(a.Config); err != nil {
			cause = errors.Join(cause, err)
		}
		if err := config.SaveSubscriptionConfig(a.Subscriptions); err != nil {
			cause = errors.Join(cause, err)
		}
		if old != nil && wasRunning {
			if err := next.Stop(); err != nil {
				return errors.Join(cause, fmt.Errorf("stop target kernel during rollback: %w", err))
			}
			if err := old.Start(); err != nil {
				return errors.Join(cause, fmt.Errorf("restore previous kernel: %w", err))
			}
		}
		return cause
	}
	if profileErr == nil {
		if _, err := svc.Use(profile.Name); err != nil {
			return rollback(err)
		}
	}
	if wasRunning {
		if err := next.Start(); err != nil {
			return rollback(err)
		}
		on, err := next.IsActive()
		if err != nil {
			return rollback(err)
		}
		if !on {
			return rollback(errors.New("target kernel failed to become active"))
		}
	}
	a.Config.Use = name
	if err := config.Save(a.Config); err != nil {
		return rollback(err)
	}
	if err := config.SaveSubscriptionConfig(a.Subscriptions); err != nil {
		return rollback(err)
	}
	return nil
}

// UninstallKernel stops and removes the kernel's service, files and
// config entry. Kernels that can no longer be constructed still get their
// files and entry removed. If it was selected, no kernel remains selected.
func (a *App) UninstallKernel(name string) error {
	kcfg := a.Config.KernelByName(name)
	if kcfg == nil {
		return fmt.Errorf("kernel %q is not installed", name)
	}
	if err := kernel.ValidateRemoval(*kcfg); err != nil {
		return err
	}
	if k, err := a.makeKernel(kcfg); err == nil {
		// Propagate: a failed stop leaves a process holding the port while
		// the unit and binary vanish below — a ghost. systemctl stop on an
		// already-stopped unit succeeds, so this only fails on real errors.
		if err := k.Stop(); err != nil {
			return err
		}
		if err := k.Uninstall(); err != nil {
			return err
		}
	} else {
		// Kernels we can no longer construct still get their stale unit
		// removed.
		_ = unisvc.New(kcfg.Name, unisvc.WithScope(unisvc.ScopeUser)).Uninstall()
	}
	if err := kernel.RemoveKernelFiles(*kcfg); err != nil {
		return err
	}

	a.Config.RemoveKernel(name)
	if a.Config.Use == name {
		a.Config.Use = ""
	}
	return config.Save(a.Config)
}

// UpgradeKernel downloads the latest built-in release or recopies the saved
// local binary. An empty name selects the current kernel. A running service
// is restarted after the binary changes.
func (a *App) UpgradeKernel(ctx context.Context, name string) error {
	if name == "" {
		name = a.Config.Use
	}
	kcfg := a.Config.KernelByName(name)
	if kcfg == nil {
		return fmt.Errorf("kernel %q is not installed", name)
	}
	k, err := a.makeKernel(kcfg)
	if err != nil {
		return err
	}

	wasRunning := false
	if a.Config.Use == name {
		on, err := k.IsActive()
		if err != nil {
			return err
		}
		wasRunning = on
	}

	if kcfg.Source != "" {
		if err := kernel.InstallLocal(ctx, kcfg.Source, kcfg.Bin, kcfg.Type == "sing-box"); err != nil {
			return err
		}
	} else if err := a.downloadKernel(ctx, k, kcfg.Bin); err != nil {
		return err
	}
	if wasRunning {
		return k.Restart()
	}
	return nil
}
