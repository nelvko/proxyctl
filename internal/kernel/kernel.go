package kernel

import (
	"fmt"

	"github.com/nelvko/proxyctl/internal/config"
	"github.com/nelvko/unisvc"
)

// ConfigFormat is the subscription config format a kernel understands.
type ConfigFormat string

const (
	FormatClash   ConfigFormat = "clash"
	FormatSingBox ConfigFormat = "sing-box"
)

type Kernel interface {
	unisvc.Service
	Manager
}

type Manager interface {
	// ConfigFile returns the kernel's active config file path.
	ConfigFile() string
	// ConfigFormat returns the subscription format the kernel understands.
	ConfigFormat() ConfigFormat
	TestConfig(configFile string) error
	DownloadURL() (string, error)
}

const (
	mihomo  = "mihomo"
	clash   = "clash"
	singbox = "sing-box"
)

type descriptor struct {
	format ConfigFormat
	// ready reports whether the kernel is implemented.
	ready bool
}

// names lists the known kernels, best-first.
var names = []string{mihomo, clash, singbox}

var registry = map[string]descriptor{
	mihomo:  {format: FormatClash, ready: true},
	clash:   {format: FormatClash},
	singbox: {format: FormatSingBox},
}

// Names returns the known kernel names.
func Names() []string {
	return append([]string(nil), names...)
}

// ReadyNames returns the implemented kernel names.
func ReadyNames() []string {
	ready := make([]string, 0, len(names))
	for _, name := range names {
		if registry[name].ready {
			ready = append(ready, name)
		}
	}
	return ready
}

// FormatOf returns the config format of a known kernel.
func FormatOf(name string) ConfigFormat {
	return registry[name].format
}

// Entry describes a kernel for listing.
type Entry struct {
	Name      string
	Format    ConfigFormat
	Installed bool
	Active    bool
	Ready     bool
}

func List() ([]Entry, error) {
	appCfg, err := config.LoadAppConfig()
	if err != nil {
		return nil, err
	}

	entries := make([]Entry, 0, len(names))
	for _, name := range names {
		desc := registry[name]
		entry := Entry{Name: name, Format: desc.format, Ready: desc.ready}
		for i := range appCfg.Kernels {
			if appCfg.Kernels[i].Name == name {
				entry.Installed = true
				entry.Active = appCfg.Use == name
			}
		}
		entries = append(entries, entry)
	}
	return entries, nil
}

func New(kernelCfg *config.KernelConfig) (Kernel, error) {
	switch kernelCfg.Name {
	case "", mihomo:
		// User scope: the kernel lives under $HOME, so installing and
		// operating the service needs no root.
		return &Mihomo{
			unisvc.New(mihomo, unisvc.WithScope(unisvc.ScopeUser)),
			*kernelCfg,
		}, nil
	case clash, singbox:
		return nil, fmt.Errorf("kernel %q is not supported yet", kernelCfg.Name)
	default:
		return nil, fmt.Errorf("unknown kernel %q, available: mihomo, clash, sing-box", kernelCfg.Name)
	}
}

// lookup returns the installed kernel config by name, or nil.
func lookup(appCfg *config.AppConfig, name string) *config.KernelConfig {
	for i := range appCfg.Kernels {
		if appCfg.Kernels[i].Name == name {
			return &appCfg.Kernels[i]
		}
	}
	return nil
}
