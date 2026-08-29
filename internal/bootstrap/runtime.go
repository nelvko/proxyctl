package bootstrap

import (
	"fmt"

	"github.com/nelvko/proxyctl/internal/config"
	"github.com/nelvko/proxyctl/internal/kernel"
	"github.com/nelvko/proxyctl/internal/profile"
)

type Runtime struct {
	// AppConfig *config.AppConfig
	// SubConfig *config.SubConfig

	Kernel   kernel.Kernel
	Profiles *profile.Service
}

func LoadRuntime() (*Runtime, error) {
	appCfg, err := config.LoadAppConfig()
	if err != nil {
		return nil, fmt.Errorf("load app config: %w", err)
	}

	subCfg, err := config.LoadSubConfig()
	if err != nil {
		return nil, fmt.Errorf("load sub config: %w", err)
	}
	k, err := kernel.New(&appCfg.Kernel)
	if err != nil {
		return nil, err
	}
	rt := &Runtime{
		// AppConfig: appCfg,
		// SubConfig: subCfg,
		Kernel:   k,
		Profiles: profile.NewService(appCfg, subCfg, k),
	}
	return rt, nil
}
