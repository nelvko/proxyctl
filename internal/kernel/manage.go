package kernel

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/nelvko/proxyctl/internal/config"
)

// SetActive switches the active kernel and stops the previously active
// service. It refuses to switch between kernels with different config
// formats. Re-applying the subscription is left to the caller.
func SetActive(name string) error {
	appCfg, err := config.LoadAppConfig()
	if err != nil {
		return err
	}

	next := lookup(appCfg, name)
	if next == nil {
		return fmt.Errorf("kernel %q is not installed, run `proxyctl kernel install %s` first", name, name)
	}
	if appCfg.Use == name {
		return nil
	}

	if active := appCfg.ActiveKernel(); active != nil {
		if FormatOf(active.Name) != FormatOf(name) {
			return fmt.Errorf("kernel %q uses %s config format, incompatible with active kernel %q (%s)",
				name, FormatOf(name), active.Name, FormatOf(active.Name))
		}
		// Best-effort stop; tolerate kernels we can no longer construct.
		if kActive, err := New(active); err == nil {
			_ = kActive.Stop()
		}
	}

	appCfg.Use = name
	return config.SaveAppConfig(appCfg)
}

// Uninstall stops and removes the kernel's service, files and config entry.
// Kernels that can no longer be constructed (not supported yet) still get
// their files and config entry removed. If the kernel was active, no kernel
// remains active.
func Uninstall(name string) error {
	appCfg, err := config.LoadAppConfig()
	if err != nil {
		return err
	}

	kcfg := lookup(appCfg, name)
	if kcfg == nil {
		return fmt.Errorf("kernel %q is not installed", name)
	}
	if k, err := New(kcfg); err == nil {
		_ = k.Stop()
		if err := k.UnInstall(); err != nil {
			return err
		}
	}
	removeKernelFiles(*kcfg)

	appCfg.RemoveKernel(name)
	if appCfg.Use == name {
		appCfg.Use = ""
	}
	return config.SaveAppConfig(appCfg)
}

// removeKernelFiles deletes the kernel's files. Directories are only
// removed wholesale when they live under proxyctl's own roots — the config
// is user-editable, so a hand-written Bin like /usr/local/bin/mihomo must
// not nuke /usr/local/bin.
func removeKernelFiles(kcfg config.KernelConfig) {
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

// Upgrade replaces the kernel binary with the latest release. An empty name
// upgrades the active kernel. The service is restarted if it was running.
// The binary is replaced atomically (temp file + rename), so a failed
// download never breaks the running one.
func Upgrade(name string) error {
	appCfg, err := config.LoadAppConfig()
	if err != nil {
		return err
	}

	if name == "" {
		name = appCfg.Use
	}
	kcfg := lookup(appCfg, name)
	if kcfg == nil {
		return fmt.Errorf("kernel %q is not installed", name)
	}
	k, err := New(kcfg)
	if err != nil {
		return err
	}

	wasActive := false
	if appCfg.Use == name {
		if on, err := k.IsActive(); err == nil && on {
			wasActive = true
		}
	}

	if err := download(k, kcfg.Bin); err != nil {
		return err
	}
	if wasActive {
		return k.Restart()
	}
	return nil
}
