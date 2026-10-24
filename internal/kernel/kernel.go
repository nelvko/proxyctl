package kernel

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/nelvko/proxyctl/internal/config"
	"github.com/nelvko/proxyctl/internal/download"
	"github.com/nelvko/unisvc"
)

// ErrUpToDate reports that the installed kernel already is the latest
// release; callers treat it as success — the download step is done, but
// install still converges the service setup afterwards.
var ErrUpToDate = errors.New("kernel is already the latest version")

// ConfigValidationError means the kernel ran and rejected the candidate.
// Execution failures and cancellation are returned separately.
type ConfigValidationError struct{ Output string }

func (e *ConfigValidationError) Error() string { return "test config: " + e.Output }

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

// Artifact describes a kernel release asset: where the bytes come from and,
// when known, what they must hash to.
type Artifact struct {
	// URL is the direct github.com download URL; mirrors are prefixed onto
	// it at download time.
	URL string
	// Version is the release tag, e.g. "v1.19.30" — the same format the
	// installed binary reports.
	Version string
	// SHA256 pins the artifact (hex, from the GitHub API); empty means the
	// download cannot be verified.
	SHA256 string
	Size   int64
	// FromAPI reports whether the metadata came from the GitHub API (as
	// opposed to the version.txt fallback). An API answer without a digest
	// is unexpected and fails closed when mirrors are in play.
	FromAPI bool
	Archive string
	Entry   string
	Extra   []string
}

type Adapter interface {
	ServiceSpec() *unisvc.Spec
	ProxyEndpoints() (Endpoints, error)
	// ConfigFile returns the kernel's active config file path.
	ConfigFile() string
	// ConfigFormat returns the subscription format the kernel understands.
	ConfigFormat() ConfigFormat
	TestConfig(ctx context.Context, configFile string) error
	// LatestArtifact describes the latest release's asset.
	LatestArtifact(ctx context.Context, downloads *download.Downloader) (*Artifact, error)
	// InstalledVersion returns the release version of the on-disk binary.
	InstalledVersion(ctx context.Context) (string, error)
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
var names = []string{mihomo, singbox, clash}

var registry = map[string]descriptor{
	mihomo:  {format: FormatClash, implemented: true},
	singbox: {format: FormatSingBox, implemented: true},
	clash:   {format: FormatClash},
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
	Selected    bool
	Implemented bool
}

// List builds the kernel overview from cfg without touching the system.
func List(cfg *config.Config) []Entry {
	entries := make([]Entry, 0, len(names))
	for _, name := range names {
		desc := registry[name]
		entry := Entry{Name: name, Format: desc.format, Implemented: desc.implemented}
		if cfg.KernelByName(name) != nil {
			entry.Installed = true
			entry.Selected = cfg.Use == name
			if installed := cfg.KernelByName(name); installed.Type != "" {
				entry.Format = FormatOf(installed.Type)
			}
		}
		entries = append(entries, entry)
	}
	for _, item := range cfg.Kernels {
		if !Known(item.Name) {
			entries = append(entries, Entry{Name: item.Name, Format: FormatOf(item.Type), Installed: true, Selected: cfg.Use == item.Name, Implemented: true})
		}
	}
	return entries
}

func New(kernelCfg *config.KernelConfig) (Kernel, error) {
	return NewWithDownloads(kernelCfg, download.New(nil, nil, nil))
}

func NewWithDownloads(kernelCfg *config.KernelConfig, downloads *download.Downloader) (Kernel, error) {
	kind := kernelCfg.Type
	if kind == "" {
		kind = kernelCfg.Name
	}
	if kind == "custom" {
		return &Custom{Service: unisvc.New(kernelCfg.Name, unisvc.WithScope(unisvc.ScopeUser)), cfg: *kernelCfg}, nil
	}
	switch kind {
	case "", mihomo:
		// User scope: the kernel lives under $HOME, so installing and
		// operating the service needs no root.
		return &Mihomo{
			unisvc.New(kernelCfg.Name, unisvc.WithScope(unisvc.ScopeUser)),
			*kernelCfg,
			downloads,
		}, nil
	case singbox:
		return &SingBox{Service: unisvc.New(kernelCfg.Name, unisvc.WithScope(unisvc.ScopeUser)), cfg: *kernelCfg}, nil
	case clash:
		return nil, fmt.Errorf("kernel %q is not supported yet", kernelCfg.Name)
	default:
		return nil, fmt.Errorf("unknown kernel %q, known kernels: %s (implemented: %s)", kernelCfg.Name, strings.Join(Names(), ", "), strings.Join(ImplementedNames(), ", "))
	}
}
