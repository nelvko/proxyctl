package kernel

import (
	"fmt"
	"os/exec"

	"github.com/nelvko/proxyctl/config"
	"github.com/nelvko/unisvc"
)

type Clash struct {
	unisvc.Service
}

func (m Clash) TestConfig(configFile string) (bool, error) {
	cfg := config.Get()
	cmd := exec.Command(
		cfg.Kernel.Bin,
		"-t",
		"-f", configFile,
		"-d", cfg.Kernel.ConfigDir,
	)
	b, err := cmd.CombinedOutput()
	if err != nil {
		return false, fmt.Errorf("failed to test config: please check\n%s", b)
	}
	return true, nil
}

func (m Clash) latestVersion() (string, error) {
	return "v1.1.1", nil
}

func (m Clash) Upgrade() (bool, error) {
	return true, nil
}
