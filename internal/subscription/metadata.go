package subscription

import (
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"
	"unicode"

	"github.com/nelvko/proxyctl/internal/config"
)

// Set replaces mutable metadata while keeping the cached content's identity.
func (s *Service) Set(name string, next config.Profile) error {
	current, err := s.Get(name)
	if err != nil {
		return err
	}
	if next.Name == "" {
		return errors.New("profile name cannot be empty")
	}
	if next.Name != name {
		if err := s.CheckNameAvailable(next.Name); err != nil {
			return err
		}
	}
	if err := validateProfile(next); err != nil {
		return err
	}
	if err := s.ctx.Err(); err != nil {
		return err
	}
	next.File = current.File
	old := *s.subConfig
	old.Profiles = slices.Clone(old.Profiles)
	old.UseByFormat = maps.Clone(old.UseByFormat)
	for i := range s.subConfig.Profiles {
		if s.subConfig.Profiles[i].Name == name {
			s.subConfig.Profiles[i] = next.Clone()
			break
		}
	}
	if s.subConfig.Use == name {
		s.subConfig.Use = next.Name
	}
	for format, selected := range s.subConfig.UseByFormat {
		if selected == name {
			s.subConfig.UseByFormat[format] = next.Name
		}
	}
	if err := s.save(); err != nil {
		*s.subConfig = old
		return err
	}
	return nil
}

func (s *Service) CheckNameAvailable(name string) error {
	if name == "" {
		return nil
	}
	if name == "." || name == ".." || strings.ContainsAny(name, "/\\") || strings.TrimSpace(name) != name || strings.ContainsFunc(name, unicode.IsControl) {
		return errors.New("profile name must be a single filename without path separators or surrounding whitespace")
	}
	ok := slices.ContainsFunc(s.subConfig.Profiles, func(p profile) bool {
		return p.Name == name
	})
	if ok {
		return fmt.Errorf("profile %q already exists", name)
	}
	return nil
}
