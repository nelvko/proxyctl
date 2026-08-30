package kernel

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/nelvko/proxyctl/internal/config"
)

// RemoveKernelFiles deletes the kernel's files. Directories are only
// removed wholesale when they live under proxyctl's own roots — the config
// is user-editable, so a hand-written Bin like /usr/local/bin/mihomo must
// not nuke /usr/local/bin.
func RemoveKernelFiles(kcfg config.KernelConfig) {
	if underManagedRoot(kcfg.ConfigDir) {
		_ = os.RemoveAll(kcfg.ConfigDir)
	}
	binDir := filepath.Dir(kcfg.Bin)
	if underManagedRoot(binDir) {
		_ = os.RemoveAll(binDir)
	} else {
		_ = os.Remove(kcfg.Bin)
	}
}

func underManagedRoot(path string) bool {
	for _, root := range managedRoots() {
		rel, err := filepath.Rel(root, path)
		if err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return true
		}
	}
	return false
}

func managedRoots() []string {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}
	return []string{
		filepath.Join(home, ".local", "share", config.AppName),
		filepath.Join(home, ".config", config.AppName),
	}
}
