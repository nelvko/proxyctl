package app

import (
	"context"
	"errors"
	"fmt"

	"github.com/nelvko/proxyctl/internal/kernel"
)

// ControlKernel manages the selected shared service, without changing shell state.
func (a *App) ControlKernel(ctx context.Context, action string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	k, err := a.Kernel()
	if err != nil {
		return err
	}
	return controlKernel(ctx, k, action)
}

func controlKernel(ctx context.Context, k kernel.Kernel, action string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	switch action {
	case "stop":
		return k.Stop()
	case "start":
		on, err := k.IsActive()
		if err != nil || on {
			return err
		}
		if err := k.TestConfig(ctx, k.ConfigFile()); err != nil && !errors.Is(err, kernel.ErrNoValidation) {
			return err
		}
		return k.Start()
	case "restart":
		if err := k.TestConfig(ctx, k.ConfigFile()); err != nil && !errors.Is(err, kernel.ErrNoValidation) {
			return err
		}
		return k.Restart()
	default:
		return fmt.Errorf("unknown service action %q", action)
	}
}
