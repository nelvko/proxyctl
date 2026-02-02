package kernel

import (
	"github.com/nelvko/proxyctl/config"
	"github.com/nelvko/unisvc/service"
)

type Kernel = string

const (
	Clash   Kernel = "clash"
	Mihomo  Kernel = "mihomo"
	SingBox Kernel = "sing-box"
)

var AvailableKernel = []Kernel{Clash, Mihomo, SingBox}

func New(args ...Kernel) service.Service {
	var k Kernel
	if len(args) > 0 {
		k = args[0]
	} else {
		cfg, _ := config.Load()
		k = cfg.Kernel.Name
	}
	return service.New(k)
}
