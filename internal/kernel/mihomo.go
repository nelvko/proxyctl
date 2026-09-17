package kernel

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/nelvko/proxyctl/internal/config"
	"github.com/nelvko/proxyctl/internal/download"
	"github.com/nelvko/unisvc"
)

type Mihomo struct {
	unisvc.Service

	cfg       config.KernelConfig
	downloads *download.Downloader
}

// ServiceSpec describes how this adapter starts its binary.
func (m Mihomo) ServiceSpec() *unisvc.Spec {
	return &unisvc.Spec{Command: m.cfg.Bin, Args: []string{"-d", m.cfg.ConfigDir, "-f", m.cfg.ConfigFile}}
}

func (m Mihomo) ConfigFile() string { return m.cfg.ConfigFile }

func (m Mihomo) ConfigFormat() ConfigFormat { return FormatClash }

// configTestTimeout bounds a config test: a validating mihomo may fetch
// missing geodata, which on a blocked route would otherwise hang sub
// add/use/update forever.
const configTestTimeout = 60 * time.Second

// TestConfig tests the configuration file for Mihomo by executing the Mihomo binary.
// It captures the output and returns an error if the test fails.
func (m Mihomo) TestConfig(ctx context.Context, configFile string) error {
	// A config referencing geodata makes `mihomo -t` fetch any missing file
	// itself — on a blocked route that hangs until the timeout below. Seed
	// first so the test runs on local files.
	m.ensureGeodata(ctx, configFile)

	ctx, cancel := context.WithTimeout(ctx, configTestTimeout)
	defer cancel()
	cmd := exec.CommandContext(
		ctx,
		m.cfg.Bin,
		"-t",
		"-f", configFile,
		"-d", m.cfg.ConfigDir,
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		if ctx.Err() != nil {
			return fmt.Errorf("validate configuration: %w", ctx.Err())
		}
		var exitErr *exec.ExitError
		if !errors.As(err, &exitErr) {
			return fmt.Errorf("run kernel validation: %w", err)
		}
		return &ConfigValidationError{Output: strings.TrimSpace(string(out))}
	}
	return nil
}

func (m Mihomo) downloader() *download.Downloader {
	if m.downloads != nil {
		return m.downloads
	}
	return download.New(nil, nil, nil)
}
