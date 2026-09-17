package subscription

import (
	"bytes"
	"errors"
	"fmt"
	"net/url"
	"os"
	"slices"

	"github.com/nelvko/proxyctl/internal/config"
)

// UpdateResult reports cache refresh and application separately for one profile.
type UpdateResult struct {
	Name        string
	Status      UpdateStatus
	Application ApplicationState
	Err         error
}

// Update refreshes named profiles, or all profiles if no names are supplied.
// Request overrides do not change saved settings. A changed or out-of-sync
// selected profile is applied; errors are collected without stopping other targets.
func (s *Service) Update(options DownloadOptions, names ...string) ([]UpdateResult, error) {
	if err := s.requireRuntime(); err != nil {
		return nil, err
	}
	targets, err := s.resolveTargets(names)
	if err != nil {
		return nil, err
	}
	var results []UpdateResult
	var errs []error
	for _, p := range targets {
		if err := s.ctx.Err(); err != nil {
			errs = append(errs, err)
			break
		}
		requestProfile := p
		requestProfile.Download, err = options.Apply(p.Download)
		var changed bool
		if err == nil {
			changed, err = s.updateProfile(requestProfile)
		}
		result := UpdateResult{Name: p.Name, Status: Unchanged}
		if changed {
			result.Status = Updated
		}
		if err == nil && s.SelectedName() == p.Name && (changed || contentChanged(p.File, s.kernel.ConfigFile())) {
			result.Application, err = s.Use(p.Name)
			if err != nil {
				result.Application = ApplicationFailed
			}
		}
		if err != nil {
			result.Status = UpdateFailed
			result.Err = err
			errs = append(errs, fmt.Errorf("%s: %w", p.Name, err))
		}
		results = append(results, result)
	}
	return results, errors.Join(errs...)
}

func (s *Service) resolveTargets(names []string) ([]profile, error) {
	if len(names) == 0 {
		return s.List(), nil
	}
	targets := make([]profile, 0, len(names))
	for _, name := range names {
		p, err := s.Get(name)
		if err != nil {
			return nil, err
		}
		if !slices.ContainsFunc(targets, func(existing profile) bool { return existing.Name == p.Name }) {
			targets = append(targets, p)
		}
	}
	return targets, nil
}

// updateProfile re-fetches one profile and swaps its file in atomically,
// after the kernel accepts the new config. It reports whether the content
// actually changed; identical content leaves the file — and the kernel —
// untouched.
func (s *Service) updateProfile(p profile) (bool, error) {
	u, err := url.Parse(p.URL)
	if err != nil {
		return false, err
	}
	profilesDir, err := config.ProfilesDir()
	if err != nil {
		return false, err
	}
	if err := os.MkdirAll(profilesDir, 0o755); err != nil {
		return false, err
	}
	tmpFile, err := os.CreateTemp(profilesDir, ".profile-*")
	if err != nil {
		return false, err
	}
	tmpName := tmpFile.Name()
	defer func() {
		tmpFile.Close()
		if tmpName != "" {
			_ = os.Remove(tmpName)
		}
	}()

	if _, err := s.fetchSource(u, tmpFile, p); err != nil {
		return false, err
	}
	if err := s.validateDownload(tmpName); err != nil {
		return false, err
	}
	if err := s.ctx.Err(); err != nil {
		return false, err
	}
	if !contentChanged(tmpName, p.File) {
		return false, nil
	}
	if err := tmpFile.Close(); err != nil {
		return false, err
	}
	if err := os.Rename(tmpName, p.File); err != nil {
		return false, err
	}
	tmpName = ""
	return true, nil
}

// contentChanged reports whether tmp differs from the profile's current
// file; a missing or unreadable current file counts as changed.
func contentChanged(tmp, cur string) bool {
	fresh, err := os.ReadFile(tmp)
	if err != nil {
		return true
	}
	current, err := os.ReadFile(cur)
	if err != nil {
		return true
	}
	return !bytes.Equal(fresh, current)
}

type UpdateStatus string

const (
	Unchanged    UpdateStatus = "unchanged"
	Updated      UpdateStatus = "updated"
	UpdateFailed UpdateStatus = "failed"
)
