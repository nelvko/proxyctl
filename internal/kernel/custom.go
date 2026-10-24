package kernel

import (
	"context"
	"errors"

	"github.com/nelvko/proxyctl/internal/config"
	"github.com/nelvko/proxyctl/internal/download"
	"github.com/nelvko/unisvc"
)

// A custom binary can be installed and managed as a user service. Proxy and
// subscription behavior requires a known adapter.
type Custom struct {
	unisvc.Service
	cfg config.KernelConfig
}

var ErrNoValidation = errors.New("this binary has no config validator; use --type mihomo or --type sing-box for a compatible binary")

func (c Custom) ServiceSpec() *unisvc.Spec  { return &unisvc.Spec{Command: c.cfg.Bin} }
func (c Custom) ConfigFile() string         { return c.cfg.ConfigFile }
func (c Custom) ConfigFormat() ConfigFormat { return "" }
func (c Custom) ProxyEndpoints() (Endpoints, error) {
	return Endpoints{}, errors.New("this binary has no proxy adapter; use --type mihomo or --type sing-box for a compatible binary")
}
func (c Custom) TestConfig(context.Context, string) error { return ErrNoValidation }
func (c Custom) LatestArtifact(context.Context, *download.Downloader) (*Artifact, error) {
	return nil, errors.New("this binary was installed locally")
}
func (c Custom) InstalledVersion(context.Context) (string, error) {
	return "", errors.New("this binary was installed locally")
}
