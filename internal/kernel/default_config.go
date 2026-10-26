package kernel

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func defaultConfig(format ConfigFormat) string {
	switch format {
	case FormatClash:
		return "mixed-port: 7890\nallow-lan: false\nbind-address: 127.0.0.1\nmode: direct\n"
	case FormatSingBox:
		return `{
  "inbounds": [{"type": "mixed", "listen": "127.0.0.1", "listen_port": 7890}],
  "outbounds": [{"type": "direct", "tag": "direct"}],
  "route": {"final": "direct"}
}
`
	default:
		return ""
	}
}

// EnsureDefaultConfig initializes missing configs and repairs the empty
// placeholders created by older installations. Nonempty files and symlinks are
// preserved. The caller must hold the state lock.
func EnsureDefaultConfig(k Adapter) error {
	content := defaultConfig(k.ConfigFormat())
	if content == "" {
		return nil
	}
	path := k.ConfigFile()
	info, err := os.Lstat(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("inspect kernel config: %w", err)
	}
	if err == nil {
		if info.Mode()&os.ModeSymlink != 0 {
			return nil
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("kernel config is not a regular file: %s", path)
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read kernel config: %w", err)
		}
		if len(raw) != 0 {
			return nil
		}
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("initialize kernel config: %w", err)
	}
	if err := writeStagedFile(path, strings.NewReader(content), 0o600); err != nil {
		return fmt.Errorf("initialize kernel config: %w", err)
	}
	return nil
}

// IsDefaultConfig reports whether the active file still contains proxyctl's
// generated direct configuration. It does not infer the mode of custom configs
// or modify files.
func IsDefaultConfig(k Adapter) (bool, error) {
	content := defaultConfig(k.ConfigFormat())
	if content == "" {
		return false, nil
	}
	raw, err := os.ReadFile(k.ConfigFile())
	if err != nil {
		return false, fmt.Errorf("read kernel config: %w", err)
	}
	return string(raw) == content, nil
}
