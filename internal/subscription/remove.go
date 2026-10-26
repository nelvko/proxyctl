package subscription

import (
	"errors"
	"fmt"
	"maps"
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
	for _, name := range s.subConfig.UseByFormat {
		if name == profileName {
			selected = true
		}
	}
	if selected && !force {
		return fmt.Errorf("profile %q is currently in use", profileName)
	}
	previous := *s.subConfig
	previous.Profiles = slices.Clone(previous.Profiles)
	previous.UseByFormat = maps.Clone(previous.UseByFormat)
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
	if s.SelectedName() == profileName && s.kernel != nil {
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
		if s.subConfig.Use == profileName {
			s.subConfig.Use = ""
		}
		for format, name := range s.subConfig.UseByFormat {
			if name == profileName {
				delete(s.subConfig.UseByFormat, format)
			}
		}
	}
	s.subConfig.Profiles = slices.DeleteFunc(s.subConfig.Profiles, func(p profile) bool { return p.Name == profileName })
	if err := s.save(); err != nil {
		return rollback(err)
	}
	var cleanup []error
	paths := []string{tgt.File}
	if patch, err := profileOverridePath(tgt); err != nil {
		cleanup = append(cleanup, err)
	} else {
		paths = append(paths, patch)
	}
	for _, path := range paths {
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			cleanup = append(cleanup, err)
		}
	}
	if err := errors.Join(cleanup...); err != nil {
		return fmt.Errorf("profile deleted but local file cleanup failed: %w", err)
	}
	return nil
}
