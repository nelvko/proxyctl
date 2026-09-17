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
	return p, nil
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
