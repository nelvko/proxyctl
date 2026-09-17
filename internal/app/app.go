// Package app assembles the proxyctl state (kernels + subscriptions) and
// owns the use cases that span the kernel, service and profile domains.
// Commands stay thin: parse arguments, call a use case, print.
package app

import (
	"fmt"
	"strings"

	"github.com/nelvko/proxyctl/internal/config"
	"github.com/nelvko/proxyctl/internal/download"
	"github.com/nelvko/proxyctl/internal/kernel"
)

// App is the loaded application state. Zero kernels is a loadable, valid
// state — lifecycle commands operate on it directly.
type App struct {
	Config        *config.Config
	Subscriptions *config.SubscriptionConfig
	Notify        download.Notify
	newKernel     func(*config.KernelConfig) (kernel.Kernel, error)
}

func Load() (*App, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, fmt.Errorf("load app config: %w", err)
	}
	return &App{Config: cfg}, nil
}

// Kernel returns the selected kernel, or config.ErrNoKernel.
func (a *App) Kernel() (kernel.Kernel, error) {
	kcfg := a.Config.SelectedKernel()
	if kcfg == nil {
		return nil, fmt.Errorf("%w; run `proxyctl install <name>` or `proxyctl use <name>`", config.ErrNoKernel)
	}
	return a.makeKernel(kcfg)
}

func (a *App) makeKernel(cfg *config.KernelConfig) (kernel.Kernel, error) {
	if a.newKernel != nil {
		return a.newKernel(cfg)
	}
	return kernel.NewWithDownloads(cfg, a.downloader())
}

func (a *App) downloader() *download.Downloader {
	return download.New(strings.Split(a.Config.Mirror, ","), nil, a.Notify)
}
