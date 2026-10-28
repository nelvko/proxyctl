package ui

import (
	"context"
	"testing"
	"time"
)

func TestDashboardTrafficRateFormattingAndExpiry(t *testing.T) {
	for _, test := range []struct {
		bytesPerSecond int64
		want           string
	}{
		{0, "0 B/s"},
		{128, "128 B/s"},
		{41472, "40.5 KB/s"},
		{2 << 20, "2.0 MB/s"},
	} {
		if got := dashboardRate(test.bytesPerSecond); got != test.want {
			t.Errorf("dashboardRate(%d) = %q, want %q", test.bytesPerSecond, got, test.want)
		}
	}
	m := newDashboardTestModel(context.Background())
	m.trafficAvailable = true
	m.trafficAt = time.Now().Add(-6 * time.Second)
	canceled := false
	m.trafficCancel = func() { canceled = true }
	updated, _ := m.Update(dashboardTickMsg{})
	m = updated.(dashboardModel)
	if !canceled || m.trafficAvailable || m.trafficCancel != nil {
		t.Fatalf("stale traffic remained visible: canceled=%v available=%v", canceled, m.trafficAvailable)
	}
}

func TestDashboardTrafficIgnoresOldStreamSamples(t *testing.T) {
	m := newDashboardTestModel(context.Background())
	m.trafficGeneration = 2
	m.trafficEvents = make(chan dashboardTrafficMsg)
	updated, _ := m.Update(dashboardTrafficMsg{generation: 1, rate: dashboardTraffic{Up: 99}})
	m = updated.(dashboardModel)
	if m.trafficAvailable {
		t.Fatal("stale traffic sample was accepted")
	}
	updated, _ = m.Update(dashboardTrafficMsg{generation: 2, rate: dashboardTraffic{Up: 128, Down: 256}})
	m = updated.(dashboardModel)
	if !m.trafficAvailable || m.traffic.Up != 128 || m.traffic.Down != 256 {
		t.Fatalf("current traffic sample was not accepted: %+v", m.traffic)
	}
}
