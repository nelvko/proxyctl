package bootstrap

import (
	"fmt"

	"github.com/nelvko/proxyctl/internal/config"
	"github.com/nelvko/proxyctl/internal/kernel"
	"github.com/nelvko/proxyctl/internal/profile"
)

type Runtime struct {
	Kernel   kernel.Kernel
	Profiles *profile.Service
}

func LoadRuntime() (*Runtime, error) {
	appCfg, err := config.LoadAppConfig()
	if err != nil {
		return nil, fmt.Errorf("load app config: %w", err)
	}
	kcfg := appCfg.ActiveKernel()
	if kcfg == nil {
		return nil, config.ErrNoKernel
	}

	subCfg, err := config.LoadSubConfig()
	if err != nil {
		return nil, fmt.Errorf("load sub config: %w", err)
	}
	k, err := kernel.New(kcfg)
	if err != nil {
		return nil, err
	}
	return &Runtime{
		Kernel:   k,
		Profiles: profile.NewService(subCfg, k),
	}, nil
}
