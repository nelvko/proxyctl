package kernel

import (
	"fmt"

	"github.com/nelvko/proxyctl/internal/config"
	"github.com/nelvko/unisvc"
)

const (
	mihomo  = "mihomo"
	clash   = "clash"
	singbox = "sing-box"
)

type Kernel interface {
	unisvc.Service
	Manager
}

type Manager interface {
	TestConfig(configFile string) error
	DownloadURL() (string, error)
	Upgrade() error
}

func New(kernelCfg *config.KernelConfig) (Kernel, error) {
	switch kernelCfg.Name {
	case "", mihomo:
		return &Mihomo{
			// User scope: mihomo lives in $HOME, no root needed to
			// install or operate the service.
			unisvc.New(mihomo, unisvc.WithScope(unisvc.ScopeUser)),
			*kernelCfg,
		}, nil
	default:
		return nil, fmt.Errorf("unsupported kernel %q", kernelCfg.Name)
	}
}
