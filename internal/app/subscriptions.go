package app

import (
	"context"
	"fmt"

	"github.com/nelvko/proxyctl/internal/config"
	"github.com/nelvko/proxyctl/internal/download"
	"github.com/nelvko/proxyctl/internal/subscription"
)

// LoadProfileMetadata reads subscription metadata without a main config or kernel.
func LoadProfileMetadata(ctx context.Context) (*subscription.Service, error) {
	cfg, err := config.LoadSubscriptionConfig()
	if err != nil {
		return nil, err
	}
	return subscription.NewService(cfg, nil).WithContext(ctx), nil
}

// LoadRuntimeProfiles assembles subscriptions with the selected kernel.
// For mutations, call this inside WithStateLock or use MutateRuntimeProfiles.
func LoadRuntimeProfiles(ctx context.Context, notify download.Notify) (*subscription.Service, error) {
	a, err := Load()
	if err != nil {
		return nil, err
	}
	a.Notify = notify
	svc, err := a.Profiles()
	if err != nil {
		return nil, err
	}
	return svc.WithContext(ctx), nil
}

// LoadSubscriptions loads subscription state only for operations that use it.
func (a *App) LoadSubscriptions() error {
	if a.Subscriptions != nil {
		return nil
	}
	cfg, err := config.LoadSubscriptionConfig()
	if err != nil {
		return fmt.Errorf("load subscription config: %w", err)
	}
	a.Subscriptions = cfg
	return nil
}

// Profiles returns the subscription service bound to the selected kernel.
func (a *App) Profiles() (*subscription.Service, error) {
	if err := a.LoadSubscriptions(); err != nil {
		return nil, err
	}
	k, err := a.Kernel()
	if err != nil {
		return nil, err
	}
	return subscription.NewService(a.Subscriptions, k), nil
}

// RemoveProfile removes metadata/cache and binds a runtime only when force-removing
// the selected profile. The caller owns the state lock.
func RemoveProfile(ctx context.Context, name string, force bool, notify download.Notify) error {
	svc, err := LoadProfileMetadata(ctx)
	if err != nil {
		return err
	}
	if force && svc.SelectedName() == name {
		a, err := Load()
		if err != nil {
			return err
		}
		a.Notify = notify
		if a.Config.SelectedKernel() != nil {
			svc, err = a.Profiles()
			if err != nil {
				return err
			}
			svc = svc.WithContext(ctx)
		}
	}
	return svc.Delete(name, force)
}

// MutateProfileMetadata reloads metadata inside the state lock, without a kernel.
func MutateProfileMetadata(ctx context.Context, run func(*subscription.Service) error) error {
	return WithStateLock(ctx, func() error {
		svc, err := LoadProfileMetadata(ctx)
		if err != nil {
			return err
		}
		return run(svc)
	})
}

// MutateRuntimeProfiles assembles fresh runtime-bound state inside the state lock.
func MutateRuntimeProfiles(ctx context.Context, notify download.Notify, run func(*subscription.Service) error) error {
	return WithStateLock(ctx, func() error {
		svc, err := LoadRuntimeProfiles(ctx, notify)
		if err != nil {
			return err
		}
		return run(svc)
	})
}
