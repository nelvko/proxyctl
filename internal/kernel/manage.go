package kernel

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/nelvko/proxyctl/internal/config"
)

// ValidateRemoval checks ownership before the caller stops/removes the service.
func ValidateRemoval(cfg config.KernelConfig) error {
	_, err := removalPaths(cfg)
	return err
}

// RemoveKernelFiles recursively removes only this kernel's default directories.
// Custom config directories are retained; a custom binary is removed as a file.
func RemoveKernelFiles(cfg config.KernelConfig) error {
	paths, err := removalPaths(cfg)
	if err != nil {
		return err
	}
	for _, path := range paths {
		if err := os.RemoveAll(path); err != nil {
			return fmt.Errorf("remove %s: %w", path, err)
		}
	}
	if len(paths) == 0 || paths[len(paths)-1] != filepath.Dir(cfg.Bin) {
		info, err := os.Lstat(cfg.Bin)
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		if err != nil {
			return err
		}
		if info.IsDir() {
			return fmt.Errorf("kernel binary is a directory: %s", cfg.Bin)
		}
		return os.Remove(cfg.Bin)
	}
	return nil
}

func removalPaths(cfg config.KernelConfig) ([]string, error) {
	expected, err := config.DefaultKernelConfig(cfg.Name)
	if err != nil {
		return nil, err
	}
	root, err := config.ConfigDir()
	if err != nil {
		return nil, err
	}
	data, err := config.DataDir()
	if err != nil {
		return nil, err
	}
	var paths []string
	for _, item := range []struct{ path, owned, root string }{
		{cfg.ConfigDir, expected.ConfigDir, root},
		{filepath.Dir(cfg.Bin), filepath.Dir(expected.Bin), data},
	} {
		if !filepath.IsAbs(item.path) {
			return nil, fmt.Errorf("kernel path must be absolute: %q", item.path)
		}
		path := filepath.Clean(item.path)
		if path == item.owned {
			// Reject symlink components below the shared root: ownership is lexical,
			// but recursive deletion must never follow a redirected parent directory.
			rel, _ := filepath.Rel(item.root, path)
			current := item.root
			for _, part := range strings.Split(rel, string(filepath.Separator)) {
				current = filepath.Join(current, part)
				info, err := os.Lstat(current)
				if errors.Is(err, os.ErrNotExist) {
					break
				}
				if err != nil {
					return nil, err
				}
				if info.Mode()&os.ModeSymlink != 0 {
					return nil, fmt.Errorf("refusing symlinked kernel directory: %s", current)
				}
			}
			paths = append(paths, path)
		} else if within(root, path) || within(data, path) {
			return nil, fmt.Errorf("refusing to remove shared or another kernel's directory: %s", path)
		}
	}
	if !filepath.IsAbs(cfg.Bin) {
		return nil, fmt.Errorf("kernel binary must be absolute: %q", cfg.Bin)
	}
	if info, err := os.Lstat(cfg.Bin); err == nil && info.IsDir() {
		return nil, fmt.Errorf("kernel binary is a directory: %s", cfg.Bin)
	} else if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	return paths, nil
}

func within(root, path string) bool {
	rel, err := filepath.Rel(root, path)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}
