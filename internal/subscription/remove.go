package subscription

import (
	"errors"
	"fmt"
	"os"
	"slices"
)

func (s *Service) Delete(profileName string, force bool) error {
	if err := s.ctx.Err(); err != nil {
		return err
	}
	tgt, err := s.Get(profileName)
	if err != nil {
		return err
	}
	selected := s.SelectedName() == profileName
	if selected && !force {
		return fmt.Errorf("profile %q is currently in use", profileName)
	}
	previous := *s.subConfig
	previous.Profiles = slices.Clone(previous.Profiles)
	var applied []byte
	var appliedErr error
	wasRunning := false
	runtimeChanged := false
	rollback := func(cause error) error {
		*s.subConfig = previous
		if runtimeChanged {
			restoreErr := restoreConfig(s.kernel.ConfigFile(), applied, appliedErr)
			if restoreErr == nil && wasRunning {
				restoreErr = s.kernel.Restart()
			}
			return errors.Join(cause, restoreErr)
		}
		return cause
	}
	if selected && s.kernel != nil {
		wasRunning, err = s.kernel.IsActive()
		if err != nil {
			return err
		}
		applied, appliedErr = os.ReadFile(s.kernel.ConfigFile())
		if appliedErr != nil && !errors.Is(appliedErr, os.ErrNotExist) {
			return appliedErr
		}
		if err := s.kernel.Stop(); err != nil {
			return err
		}
		runtimeChanged = true
		if err := os.Remove(s.kernel.ConfigFile()); err != nil && !errors.Is(err, os.ErrNotExist) {
			return rollback(err)
		}
	}
	if selected {
		s.subConfig.Use = ""
	}
	s.subConfig.Profiles = slices.DeleteFunc(s.subConfig.Profiles, func(p profile) bool { return p.Name == profileName })
	if err := s.save(); err != nil {
		return rollback(err)
	}
	if err := os.Remove(tgt.File); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("profile deleted but local file cleanup failed: %w", err)
	}
	return nil
}
