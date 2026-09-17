package subscription

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/nelvko/proxyctl/internal/config"
)

func (s *Service) save() error {
	return config.SaveSubscriptionConfig(s.subConfig)
}

// writeAtomic stages private configuration beside its destination.
func writeAtomic(path string, data []byte) error {
	f, err := os.CreateTemp(filepath.Dir(path), ".config-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if _, err := f.Write(data); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(f.Name(), path)
}

func restoreConfig(path string, old []byte, oldErr error) error {
	if errors.Is(oldErr, os.ErrNotExist) {
		err := os.Remove(path)
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	if oldErr != nil {
		return oldErr
	}
	return writeAtomic(path, old)
}
