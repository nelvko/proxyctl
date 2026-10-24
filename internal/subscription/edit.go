package subscription

import (
	"os"
	"path/filepath"
)

// EditSession stages edits separately so invalid or abandoned edits cannot
// replace a valid subscription. Both CLI and TUI use this transaction.
type EditSession struct {
	service   *Service
	profile   profile
	temporary string
}

func (s *Service) BeginEdit(profileName string) (*EditSession, error) {
	if err := s.requireRuntime(); err != nil {
		return nil, err
	}
	p, err := s.Get(profileName)
	if err != nil {
		return nil, err
	}
	if err := s.requireCompatible(p); err != nil {
		return nil, err
	}
	original, err := os.ReadFile(p.File)
	if err != nil {
		return nil, err
	}
	f, err := os.CreateTemp(filepath.Dir(p.File), ".edit-*"+filepath.Ext(p.File))
	if err != nil {
		return nil, err
	}
	session := &EditSession{service: s, profile: p, temporary: f.Name()}
	if _, err := f.Write(original); err != nil {
		f.Close()
		session.Close()
		return nil, err
	}
	if err := f.Close(); err != nil {
		session.Close()
		return nil, err
	}

	return session, nil
}

func (e *EditSession) Path() string { return e.temporary }

func (e *EditSession) Close() { _ = os.Remove(e.temporary) }

func (e *EditSession) Commit() error {
	if err := e.service.kernel.TestConfig(e.service.ctx, e.temporary); err != nil {
		return err
	}
	if err := e.service.ctx.Err(); err != nil {
		return err
	}
	return os.Rename(e.temporary, e.profile.File)
}
