package config

import (
	"os"
	"path/filepath"

	"go.yaml.in/yaml/v3"
)

// saveYAMLAtomic writes v to a sibling temp file (0600) and renames it
// into place, so a crash never truncates the config, and encode/flush
// errors surface instead of being swallowed by a deferred Close.
func saveYAMLAtomic(path string, v any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".tmp*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer func() {
		if tmpName != "" {
			os.Remove(tmpName)
		}
	}()

	encoder := yaml.NewEncoder(tmp)
	if err := encoder.Encode(v); err != nil {
		tmp.Close()
		return err
	}
	if err := encoder.Close(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpName, path); err != nil {
		return err
	}
	tmpName = ""
	return nil
}
