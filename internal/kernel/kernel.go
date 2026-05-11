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
			unisvc.New(mihomo),
			*kernelCfg,
		}, nil
	default:
		return nil, fmt.Errorf("unsupported kernel %q", kernelCfg.Name)
	}
}
