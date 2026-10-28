package ui

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/nelvko/proxyctl/internal/config"
	"github.com/nelvko/proxyctl/internal/kernel"
	"github.com/nelvko/proxyctl/internal/redact"
	"github.com/nelvko/unisvc"
)

// Runtime details are observed from the selected adapter, not inferred from
// the optional Clash controller or from the presence of a subscription.
type dashboardRuntimeInfo struct {
	KernelType, HTTP, SOCKS, Platform string
	Service                           string
	ServiceAvailable                  bool
	EndpointErr                       error
	VersionKey                        string
	versionLoader                     func(context.Context) (string, error)
}

func readDashboardRuntimeInfo(cfg *config.KernelConfig, k kernel.Kernel) dashboardRuntimeInfo {
	info := dashboardRuntimeInfo{Platform: runtime.GOOS + "/" + runtime.GOARCH}
	if cfg == nil {
		return info
	}
	info.KernelType = cfg.Type
	if info.KernelType == "" {
		info.KernelType = cfg.Name
	}
	if k == nil {
		return info
	}
	initSystem := k.InitSystem()
	info.Service = string(initSystem)
	// The current unisvc dependency only implements Linux/systemd. Its other
	// detected init-system names use UnsupportedService, not working backends.
	info.ServiceAvailable = runtime.GOOS == "linux" && initSystem == unisvc.InitSystemd
	endpoints, err := k.ProxyEndpoints()
	if err != nil {
		info.EndpointErr = err
	} else {
		info.HTTP, info.SOCKS = redact.URL(endpoints.HTTP), redact.URL(endpoints.SOCKS)
	}
	info.VersionKey = dashboardVersionKey(*cfg)
	info.versionLoader = k.InstalledVersion
	return info
}

// File metadata makes upgrades/reinstalls refresh the cached disk version.
// A missing or unreadable binary still has a stable key, so a failed query
// does not spawn another process on each periodic status refresh.
func dashboardVersionKey(cfg config.KernelConfig) string {
	key := cfg.Name + "\x00" + cfg.Type + "\x00" + cfg.Bin
	if stat, err := os.Stat(cfg.Bin); err == nil {
		return fmt.Sprintf("%s\x00%d\x00%d", key, stat.Size(), stat.ModTime().UnixNano())
	}
	return key + "\x00unavailable"
}

type dashboardVersionState struct {
	Key             string
	Value           string
	Err             error
	Loading, Loaded bool
	generation      uint64
}

type dashboardVersionMsg struct {
	key        string
	generation uint64
	value      string
	err        error
}

// Refresh schedules at most one disk-version query for a selected binary.
// The command captures no mutable state; only Apply changes the UI model.
func (s *dashboardVersionState) Refresh(ctx context.Context, info dashboardRuntimeInfo) tea.Cmd {
	if info.VersionKey == "" || info.versionLoader == nil {
		if s.Key != "" {
			s.Invalidate()
		}
		return nil
	}
	if s.Key == info.VersionKey {
		return nil
	}
	s.Invalidate()
	s.Key, s.Loading = info.VersionKey, true
	key, generation, load := s.Key, s.generation, info.versionLoader
	return func() tea.Msg {
		queryCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()
		result := dashboardVersionMsg{key: key, generation: generation}
		if err := queryCtx.Err(); err != nil {
			result.err = err
			return result
		}
		result.value, result.err = load(queryCtx)
		if err := queryCtx.Err(); err != nil {
			result.value, result.err = "", err
		}
		return result
	}
}

func (s *dashboardVersionState) Apply(msg dashboardVersionMsg) bool {
	if !s.Loading || msg.key != s.Key || msg.generation != s.generation {
		return false
	}
	s.Loading, s.Loaded, s.Err = false, true, msg.err
	s.Value = ""
	if msg.err == nil {
		s.Value = singleLine(redact.Text(strings.TrimSpace(msg.value)))
	}
	return true
}

// Invalidate permits an explicit refresh while rejecting an older in-flight
// result, including when the user switches A -> B -> A before it completes.
func (s *dashboardVersionState) Invalidate() {
	s.generation++
	s.Key, s.Value, s.Err, s.Loading, s.Loaded = "", "", nil, false, false
}
