package app

import (
	"errors"
	"fmt"

	"github.com/nelvko/proxyctl/internal/config"
	"github.com/nelvko/proxyctl/internal/kernel"
)

var AppCtx *Runtime

type Runtime struct {
	AppConfig *config.AppConfig
	SubConfig *config.SubConfig

	// Profile *config.Profile

	Kernel kernel.Kernel
}

func LoadRuntime() (*Runtime, error) {
	appCfg, err := config.LoadAppConfig()
	if err != nil {
		if errors.Is(err, config.ErrInitRequired) {
			if err := kernel.Wizard(false); err != nil {
				return nil, err
			}
			appCfg, err = config.LoadAppConfig()
		}
		if err != nil {
			return nil, fmt.Errorf("load app config: %w", err)
		}
	}

	subCfg, err := config.LoadSubConfig()
	if err != nil {
		return nil, fmt.Errorf("load sub config: %w", err)
	}
	k, err := kernel.New(&appCfg.Kernel)
	if err != nil {
		return nil, err
	}
	AppCtx = &Runtime{
		AppConfig: appCfg,
		SubConfig: subCfg,
		Kernel:    k,
	}
	return AppCtx, nil
}

func (rt *Runtime) SaveAppConfig() error {
	if rt == nil {
		return errors.New("runtime is nil")
	}
	return config.SaveAppConfig(rt.AppConfig)
}

func (rt *Runtime) SaveSubConfig() error {
	if rt == nil {
		return errors.New("runtime is nil")
	}
	return config.SaveSubConfig(rt.SubConfig)
}
