package subscription

import (
	"errors"
	"fmt"
	"maps"
	"os"
	"slices"
)

func (s *Service) Use(profileName string) (ApplicationState, error) {
	if err := s.requireRuntime(); err != nil {
		return "", err
	}
	i := slices.IndexFunc(s.subConfig.Profiles, func(p profile) bool {
		return p.Name == profileName
	})
	if i == -1 {
		return "", fmt.Errorf("can't find %s profile", profileName)
	}
	if err := s.requireCompatible(s.subConfig.Profiles[i]); err != nil {
		return "", err
	}

	useFile := s.subConfig.Profiles[i].File
	kernelCfg := s.kernel.ConfigFile()
	if err := s.kernel.TestConfig(s.ctx, useFile); err != nil {
		return "", err
	}

	newBytes, err := os.ReadFile(useFile)
	if err != nil {
		return "", err
	}
	// Keep the old bytes so a failed restart can restore them — otherwise
	// the kernel runs the new subscription while subscriptions.yaml still
	// points at the old one.
	running, err := s.kernel.IsActive()
	if err != nil {
		return "", err
	}
	old, oldErr := os.ReadFile(kernelCfg)
	if oldErr != nil && !errors.Is(oldErr, os.ErrNotExist) {
		return "", oldErr
	}
	if err := s.ctx.Err(); err != nil {
		return "", err
	}
	if err := writeAtomic(kernelCfg, newBytes); err != nil {
		return "", err
	}

	if running {
		if err := s.commitUse(kernelCfg, old, oldErr); err != nil {
			return "", err
		}
	}
	previousName := s.subConfig.Use
	previousByFormat := maps.Clone(s.subConfig.UseByFormat)
	s.subConfig.Use = profileName
	if s.subConfig.UseByFormat == nil {
		s.subConfig.UseByFormat = make(map[string]string)
	}
	s.subConfig.UseByFormat[string(s.kernel.ConfigFormat())] = profileName
	if err := s.save(); err != nil {
		s.subConfig.Use = previousName
		s.subConfig.UseByFormat = previousByFormat
		restoreErr := restoreConfig(kernelCfg, old, oldErr)
		if restoreErr == nil && running {
			restoreErr = s.kernel.Restart()
		}
		return "", errors.Join(fmt.Errorf("persist subscription selection: %w", err), restoreErr)
	}
	if running {
		return Applied, nil
	}
	return Staged, nil
}

// commitUse restarts the kernel and verifies it came back up, rolling the
// kernel config back on failure.
func (s *Service) commitUse(kernelCfg string, old []byte, oldErr error) error {
	var switchErr error
	if err := s.kernel.Restart(); err != nil {
		switchErr = err
	} else if active, err := s.kernel.IsActive(); err != nil {
		switchErr = err
	} else if !active {
		switchErr = errors.New("kernel failed to become active after restart")
	}
	if switchErr == nil {
		return nil
	}

	if err := restoreConfig(kernelCfg, old, oldErr); err == nil {
		if rerr := s.kernel.Restart(); rerr != nil {
			return fmt.Errorf("switch failed: %v (kernel config restored, restore restart failed: %v, run `proxyctl on`)", switchErr, rerr)
		}
		return fmt.Errorf("switch failed: %v (kernel config restored)", switchErr)
	}
	return fmt.Errorf("switch failed: %v (kernel config left switched; restore %s manually)", switchErr, kernelCfg)
}

type ApplicationState string

const (
	Applied           ApplicationState = "applied"
	Staged            ApplicationState = "staged"
	ApplicationFailed ApplicationState = "failed"
)
