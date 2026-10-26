package subscription

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/nelvko/proxyctl/internal/config"
	"github.com/nelvko/proxyctl/internal/kernel"
)

func profileFormat(p profile) kernel.ConfigFormat {
	if p.Format == "" {
		return kernel.FormatClash
	}
	return kernel.ConfigFormat(p.Format)
}

func overrideExtension(format kernel.ConfigFormat) (string, error) {
	switch format {
	case kernel.FormatClash:
		return ".yaml", nil
	case kernel.FormatSingBox:
		return ".json", nil
	default:
		return "", fmt.Errorf("unsupported override format %q", format)
	}
}

func globalOverridePath(format kernel.ConfigFormat) (string, error) {
	ext, err := overrideExtension(format)
	if err != nil {
		return "", err
	}
	dir, err := config.ConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "overrides", string(format)+ext), nil
}

func profileOverridePath(p profile) (string, error) {
	ext, err := overrideExtension(profileFormat(p))
	if err != nil {
		return "", err
	}
	if p.File == "" {
		return "", errors.New("profile has no cached configuration")
	}
	return p.File + ".override" + ext, nil
}

func (s *Service) overrideTarget(name string, global bool) (string, kernel.ConfigFormat, error) {
	if global && name != "" || !global && name == "" {
		return "", "", errors.New("specify a profile name or --global, exclusively")
	}
	if global {
		if err := s.requireRuntime(); err != nil {
			return "", "", err
		}
		format := s.kernel.ConfigFormat()
		path, err := globalOverridePath(format)
		return path, format, err
	}
	p, err := s.Get(name)
	if err != nil {
		return "", "", err
	}
	path, err := profileOverridePath(p)
	return path, profileFormat(p), err
}

func readOptional(path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	return data, err
}

// ReadOverride returns the saved patch, or nil when none is configured.
// A profile's override can be inspected without a selected kernel.
func (s *Service) ReadOverride(name string, global bool) ([]byte, error) {
	path, _, err := s.overrideTarget(name, global)
	if err != nil {
		return nil, err
	}
	return readOptional(path)
}

type overrideChange struct {
	path string
	data []byte // nil removes this layer for candidate validation
}

func (s *Service) renderConfig(p profile, raw []byte, candidate *overrideChange) ([]byte, error) {
	global, err := globalOverridePath(profileFormat(p))
	if err != nil {
		return nil, err
	}
	paths := []string{global}
	if p.File != "" {
		path, err := profileOverridePath(p)
		if err != nil {
			return nil, err
		}
		paths = append(paths, path)
	}
	var patches [][]byte
	for _, path := range paths {
		var data []byte
		if candidate != nil && candidate.path == path {
			data = candidate.data
		} else if data, err = readOptional(path); err != nil {
			return nil, fmt.Errorf("read override: %w", err)
		}
		if data != nil {
			patches = append(patches, data)
		}
	}
	data, err := mergeConfig(profileFormat(p), raw, patches...)
	if err != nil {
		return nil, &kernel.ConfigValidationError{Output: err.Error()}
	}
	return data, nil
}

// validatedConfig builds an effective configuration without changing the source
// cache or active file. The private temporary is removed on every exit path.
func (s *Service) validatedConfig(p profile, source string, candidate *overrideChange) ([]byte, error) {
	if err := s.ctx.Err(); err != nil {
		return nil, err
	}
	raw, err := os.ReadFile(source)
	if err != nil {
		return nil, err
	}
	data, err := s.renderConfig(p, raw, candidate)
	if err != nil {
		return nil, err
	}
	ext, err := overrideExtension(profileFormat(p))
	if err != nil {
		return nil, err
	}
	f, err := os.CreateTemp(filepath.Dir(source), ".effective-*"+ext)
	if err != nil {
		return nil, err
	}
	defer os.Remove(f.Name())
	_, writeErr := f.Write(data)
	if err := errors.Join(writeErr, f.Close()); err != nil {
		return nil, err
	}
	if err := s.kernel.TestConfig(s.ctx, f.Name()); err != nil {
		var invalid *kernel.ConfigValidationError
		if errors.As(err, &invalid) {
			return nil, &kernel.ConfigValidationError{Output: strings.ReplaceAll(invalid.Output, f.Name(), "effective configuration")}
		}
		return nil, err
	}
	return data, s.ctx.Err()
}

// Validate checks the effective configuration used by Use and kernel switching.
func (s *Service) Validate(name string) error {
	if err := s.requireRuntime(); err != nil {
		return err
	}
	p, err := s.Get(name)
	if err != nil {
		return err
	}
	if err := s.requireCompatible(p); err != nil {
		return err
	}
	_, err = s.validatedConfig(p, p.File, nil)
	return err
}

func (s *Service) validateOverride(name string, global bool, change overrideChange) error {
	_, format, err := s.overrideTarget(name, global)
	if err != nil {
		return err
	}
	// Partial patches need only syntax validation when no matching profiles exist.
	if change.data != nil {
		if _, err := mergeConfig(format, []byte("{}"), change.data); err != nil {
			return err
		}
	}
	profiles := s.Compatible()
	if !global {
		p, err := s.Get(name)
		if err != nil {
			return err
		}
		if err := s.requireCompatible(p); err != nil {
			return err
		}
		profiles = []profile{p}
	}
	for _, p := range profiles {
		if _, err := s.validatedConfig(p, p.File, &change); err != nil {
			return fmt.Errorf("profile %q: %w", p.Name, err)
		}
	}
	return s.ctx.Err()
}

// OverrideEditSession keeps abandoned or invalid edits away from saved patches.
type OverrideEditSession struct {
	service   *Service
	name      string
	global    bool
	path      string
	temporary string
}

func (s *Service) BeginOverrideEdit(name string, global bool) (*OverrideEditSession, error) {
	if err := s.requireRuntime(); err != nil {
		return nil, err
	}
	if err := s.ctx.Err(); err != nil {
		return nil, err
	}
	path, format, err := s.overrideTarget(name, global)
	if err != nil {
		return nil, err
	}
	if !global {
		p, _ := s.Get(name)
		if err := s.requireCompatible(p); err != nil {
			return nil, err
		}
	}
	data, err := readOptional(path)
	if err != nil {
		return nil, err
	}
	if data == nil {
		data = []byte("{}\n")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, err
	}
	ext, _ := overrideExtension(format)
	f, err := os.CreateTemp(filepath.Dir(path), ".override-*"+ext)
	if err != nil {
		return nil, err
	}
	session := &OverrideEditSession{service: s, name: name, global: global, path: path, temporary: f.Name()}
	_, writeErr := f.Write(data)
	if err := errors.Join(writeErr, f.Close()); err != nil {
		session.Close()
		return nil, err
	}
	return session, nil
}

func (e *OverrideEditSession) Path() string { return e.temporary }
func (e *OverrideEditSession) Close()       { _ = os.Remove(e.temporary) }

func (e *OverrideEditSession) Commit() error {
	data, err := os.ReadFile(e.temporary)
	if err != nil {
		return err
	}
	if err := e.service.validateOverride(e.name, e.global, overrideChange{e.path, data}); err != nil {
		return err
	}
	if err := os.Chmod(e.temporary, 0o600); err != nil {
		return err
	}
	return os.Rename(e.temporary, e.path)
}

// ClearOverride validates the remaining layers, then removes only the patch.
// The active configuration and service remain unchanged until explicit Use.
func (s *Service) ClearOverride(name string, global bool) error {
	if err := s.requireRuntime(); err != nil {
		return err
	}
	path, _, err := s.overrideTarget(name, global)
	if err != nil {
		return err
	}
	data, err := readOptional(path)
	if err != nil || data == nil {
		return err
	}
	if err := s.validateOverride(name, global, overrideChange{path: path}); err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}
