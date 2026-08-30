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
	Adapter
}

type Adapter interface {
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
	// implemented reports whether the kernel is implemented.
	implemented bool
}

// names lists the known kernels, best-first.
var names = []string{mihomo, clash, singbox}

var registry = map[string]descriptor{
	mihomo:  {format: FormatClash, implemented: true},
	clash:   {format: FormatClash},
	singbox: {format: FormatSingBox},
}

// Names returns the known kernel names.
func Names() []string {
	return append([]string(nil), names...)
}

// ImplementedNames returns the implemented kernel names.
func ImplementedNames() []string {
	implemented := make([]string, 0, len(names))
	for _, name := range names {
		if registry[name].implemented {
			implemented = append(implemented, name)
		}
	}
	return implemented
}

// Known reports whether name is a registered kernel.
func Known(name string) bool {
	_, ok := registry[name]
	return ok
}

// Implemented reports whether the kernel is implemented.
func Implemented(name string) bool {
	return registry[name].implemented
}

// FormatOf returns the config format of a known kernel.
func FormatOf(name string) ConfigFormat {
	return registry[name].format
}

// Entry describes a kernel for listing.
type Entry struct {
	Name        string
	Format      ConfigFormat
	Installed   bool
	Active      bool
	Implemented bool
}

// List builds the kernel overview from cfg without touching the system.
func List(cfg *config.AppConfig) []Entry {
	entries := make([]Entry, 0, len(names))
	for _, name := range names {
		desc := registry[name]
		entry := Entry{Name: name, Format: desc.format, Implemented: desc.implemented}
		if cfg.KernelByName(name) != nil {
			entry.Installed = true
			entry.Active = cfg.Use == name
		}
		entries = append(entries, entry)
	}
	return entries
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
