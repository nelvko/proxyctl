package ui

import (
	"context"
	"fmt"
	"time"

	tea "charm.land/bubbletea/v2"
)

func (m *dashboardModel) stopTraffic() {
	if m.trafficCancel != nil {
		m.trafficCancel()
	}
	m.trafficCancel = nil
	m.trafficEvents = nil
	m.trafficGeneration++
	m.trafficAvailable = false
	m.traffic = dashboardTraffic{}
}

func (m *dashboardModel) startTraffic() tea.Cmd {
	if m.quitAfter || m.trafficCancel != nil || m.snapshot.controller == nil || m.snapshot.service != "运行中" {
		return nil
	}
	ctx, cancel := context.WithCancel(m.ctx)
	m.trafficCancel = cancel
	m.trafficAt = time.Now()
	m.trafficGeneration++
	generation := m.trafficGeneration
	events := make(chan dashboardTrafficMsg, 32)
	m.trafficEvents = events
	controller := m.snapshot.controller
	go func() {
		defer close(events)
		rates := make(chan dashboardTraffic, 32)
		finished := make(chan error, 1)
		go func() { finished <- controller.streamTraffic(ctx, rates); close(rates) }()
		for rate := range rates {
			select {
			case events <- dashboardTrafficMsg{generation: generation, rate: rate}:
			case <-ctx.Done():
				return
			}
		}
		err := <-finished
		if ctx.Err() == nil {
			select {
			case events <- dashboardTrafficMsg{generation: generation, err: err, done: true}:
			case <-ctx.Done():
			}
		}
	}()
	return nextDashboardTraffic(events, generation)
}

func nextDashboardTraffic(events <-chan dashboardTrafficMsg, generation int) tea.Cmd {
	return func() tea.Msg {
		event, ok := <-events
		if !ok {
			return dashboardTrafficMsg{generation: generation, done: true}
		}
		return event
	}
}

func dashboardRate(value int64) string {
	if value < 0 {
		value = 0
	}
	if value < 1024 {
		return fmt.Sprintf("%d B/s", value)
	}
	rate := float64(value)
	for i, unit := range [...]string{"KB/s", "MB/s", "GB/s", "TB/s"} {
		rate /= 1024
		if rate < 1024 || i == 3 {
			return fmt.Sprintf("%.1f %s", rate, unit)
		}
	}
	return "—"
}
