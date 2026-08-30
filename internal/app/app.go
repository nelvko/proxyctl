// Package app assembles the proxyctl state (kernels + subscriptions) and
// owns the use cases that span the kernel, service and profile domains.
// Commands stay thin: parse arguments, call a use case, print.
package app

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/nelvko/proxyctl/internal/config"
	"github.com/nelvko/proxyctl/internal/kernel"
	"github.com/nelvko/proxyctl/internal/subscription"
	"github.com/nelvko/unisvc"
)

// App is the loaded application state. Zero kernels is a loadable, valid
// state — lifecycle commands operate on it directly.
type App struct {
	Cfg          *config.AppConfig
	Subscription *config.SubscriptionConfig
}

func Load() (*App, error) {
	cfg, err := config.LoadAppConfig()
	if err != nil {
		return nil, fmt.Errorf("load app config: %w", err)
	}
	subs, err := config.LoadSubscriptionConfig()
	if err != nil {
		return nil, fmt.Errorf("load subscription config: %w", err)
	}
	return &App{Cfg: cfg, Subscription: subs}, nil
}

// SaveConfig persists the app config (not the subscription state).
func (a *App) Save() error {
	return config.SaveAppConfig(a.Cfg)
}

// Kernel returns the active kernel, or config.ErrNoKernel.
func (a *App) Kernel() (kernel.Kernel, error) {
	kcfg := a.Cfg.ActiveKernel()
	if kcfg == nil {
		return nil, config.ErrNoKernel
	}
	return kernel.New(kcfg)
}

// Profiles returns the subscription service bound to the active kernel.
func (a *App) Profiles() (*subscription.Service, error) {
	k, err := a.Kernel()
	if err != nil {
		return nil, err
	}
	return subscription.NewService(a.Subscription, k), nil
}

// InstallKernel downloads the kernel, installs its user service and
// registers it. The first installed kernel becomes active.
func (a *App) InstallKernel(name string) error {
	if !kernel.Known(name) {
		return fmt.Errorf("unknown kernel %q, implemented: %s", name, strings.Join(kernel.ImplementedNames(), ", "))
	}
	if !kernel.Implemented(name) {
		return fmt.Errorf("kernel %q is not supported yet", name)
	}

	kcfg := kernel.DefaultConfig(name)
	k, err := kernel.New(&kcfg)
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

	if err := kernel.Download(k, kcfg.Bin); err != nil {
		return err
	}

	spec := unisvc.Spec{
		Command: kcfg.Bin,
		Args:    []string{"-d", kcfg.ConfigDir, "-f", kcfg.ConfigFile},
	}
	if err := kernel.InstallService(k, &spec); err != nil {
		return err
	}

	a.Cfg.SetKernel(kcfg)
	if a.Cfg.Use == "" {
		a.Cfg.Use = kcfg.Name
	}
	return a.Save()
}

// UseKernel switches the active kernel and re-applies the current
// subscription on it. Re-invoking with the same name retries a failed
// re-apply.
func (a *App) UseKernel(name string) error {
	if a.Cfg.KernelByName(name) == nil {
		return fmt.Errorf("kernel %q is not installed, run `proxyctl kernel install %s` first", name, name)
	}

	// Stop the previously active kernel whenever it is not the target —
	// also on the idempotent retry path, so a failed first switch (e.g.
	// Stop failed and the old kernel still holds the port) can recover.
	if prev := a.Cfg.ActiveKernel(); prev != nil && prev.Name != name {
		if kPrev, err := kernel.New(prev); err == nil {
			_ = kPrev.Stop()
		}
	}

	if a.Cfg.Use != name {
		if active := a.Cfg.ActiveKernel(); active != nil {
			if kernel.FormatOf(active.Name) != kernel.FormatOf(name) {
				return fmt.Errorf("kernel %q uses %s config format, incompatible with active kernel %q (%s)",
					name, kernel.FormatOf(name), active.Name, kernel.FormatOf(active.Name))
			}
		}
		a.Cfg.Use = name
		if err := a.Save(); err != nil {
			return err
		}
	}

	// Re-apply the current subscription on the (new) kernel.
	svc, err := a.Profiles()
	if err != nil {
		return err
	}
	p, err := svc.Active()
	switch {
	case errors.Is(err, subscription.ErrNoActiveProfile):
		return nil
	case err != nil:
		return fmt.Errorf("kernel switched but the current subscription is unusable: %w", err)
	}
	if err := svc.Use(p.Name); err != nil {
		return fmt.Errorf("kernel switched but subscription re-apply failed: %w", err)
	}
	return nil
}

// UninstallKernel stops and removes the kernel's service, files and
// config entry. Kernels that can no longer be constructed still get their
// files and entry removed. If it was active, no kernel remains active.
func (a *App) UninstallKernel(name string) error {
	kcfg := a.Cfg.KernelByName(name)
	if kcfg == nil {
		return fmt.Errorf("kernel %q is not installed", name)
	}
	if k, err := kernel.New(kcfg); err == nil {
		_ = k.Stop()
		if err := k.Uninstall(); err != nil {
			return err
		}
	} else {
		// Kernels we can no longer construct still get their stale unit
		// removed.
		_ = unisvc.New(kcfg.Name, unisvc.WithScope(unisvc.ScopeUser)).Uninstall()
	}
	kernel.RemoveKernelFiles(*kcfg)

	a.Cfg.RemoveKernel(name)
	if a.Cfg.Use == name {
		a.Cfg.Use = ""
	}
	return a.Save()
}

// UpgradeKernel replaces the kernel binary with the latest release. An
// empty name upgrades the active kernel. The service is restarted if it
// was running.
func (a *App) UpgradeKernel(name string) error {
	if name == "" {
		name = a.Cfg.Use
	}
	kcfg := a.Cfg.KernelByName(name)
	if kcfg == nil {
		return fmt.Errorf("kernel %q is not installed", name)
	}
	k, err := kernel.New(kcfg)
	if err != nil {
		return err
	}

	wasRunning := false
	if a.Cfg.Use == name {
		on, err := k.IsActive()
		if err != nil {
			return err
		}
		wasRunning = on
	}

	if err := kernel.Download(k, kcfg.Bin); err != nil {
		return err
	}
	if wasRunning {
		return k.Restart()
	}
	return nil
}
