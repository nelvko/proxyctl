package kernel

import (
	"github.com/nelvko/proxyctl/config"
	"github.com/nelvko/unisvc"
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

func New(args ...string) (Kernel, error) {
	kernelName := config.Get().Kernel.Name
	if len(args) > 0 {
		kernelName = string(args[0])
	}
	return &Mihomo{
		unisvc.New(kernelName),
	}, nil
}
