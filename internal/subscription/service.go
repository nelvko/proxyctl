package subscription

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/nelvko/proxyctl/internal/config"
	"github.com/nelvko/proxyctl/internal/kernel"
)

// ErrNoSelectedProfile means no subscription has been chosen yet — a normal
// state, unlike a dangling Use pointing at a deleted profile.
var ErrNoSelectedProfile = errors.New("no profile selected")

type profile = config.Profile

// Runtime contains only the kernel operations used by subscriptions.
type Runtime interface {
	ConfigFormat() kernel.ConfigFormat
	ProxyEndpoints() (kernel.Endpoints, error)
	ConfigFile() string
	TestConfig(context.Context, string) error
	IsActive() (bool, error)
	Restart() error
	Stop() error
}

type Service struct {
	subConfig  *config.SubscriptionConfig
	kernel     Runtime
	ctx        context.Context
	KeepFailed bool
}

func NewService(subCfg *config.SubscriptionConfig, k Runtime) *Service {
	return &Service{
		subConfig: subCfg,
		kernel:    k,
		ctx:       context.Background(),
	}
}

// WithContext binds one operation to its caller. Copies share persisted state;
// callers serialize mutations, while cancellation belongs to this operation.
func (s *Service) WithContext(ctx context.Context) *Service {
	next := *s
	next.ctx = ctx
	return &next
}

func (s *Service) List() []profile {
	profiles := make([]profile, len(s.subConfig.Profiles))
	for i, p := range s.subConfig.Profiles {
		profiles[i] = p.Clone()
	}
	return profiles
}

// Compatible returns profiles understood by the selected kernel.
func (s *Service) Compatible() []profile {
	if s.kernel == nil {
		return s.List()
	}
	var profiles []profile
	for _, p := range s.List() {
		if s.requireCompatible(p) == nil {
			profiles = append(profiles, p)
		}
	}
	return profiles
}

func (s *Service) SelectedName() string {
	return s.subConfig.Use
}

func (s *Service) Get(name string) (profile, error) {
	i := slices.IndexFunc(s.subConfig.Profiles, func(p profile) bool {
		return p.Name == name
	})
	if i == -1 {
		return profile{}, fmt.Errorf("profile %q not found\nUse `proxyctl sub list` to see available profiles", name)
	}
	return s.subConfig.Profiles[i].Clone(), nil
}

func (s *Service) Selected() (profile, error) {
	if s.subConfig.Use == "" {
		return profile{}, ErrNoSelectedProfile
	}
	p, err := s.Get(s.subConfig.Use)
	if err != nil {
		return profile{}, fmt.Errorf("failed to get current profile: %w", err)
	}
	if s.kernel != nil {
		if err := s.requireCompatible(p); err != nil {
			return profile{}, err
		}
	}
	return p, nil
}

func (s *Service) requireCompatible(p profile) error {
	format := p.Format
	if format == "" {
		format = string(kernel.FormatClash)
	}
	if current := string(s.kernel.ConfigFormat()); current != format {
		return fmt.Errorf("profile %q uses %s configuration; selected kernel uses %s", p.Name, format, current)
	}
	return nil
}

// ErrNoRuntime indicates an operation requiring validation/application was
// attempted on a metadata-only service.
var ErrNoRuntime = errors.New("operation requires a selected kernel")

func (s *Service) requireRuntime() error {
	if s.kernel == nil {
		return ErrNoRuntime
	}
	return nil
}
