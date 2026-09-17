package ui

import (
	"errors"
	"strings"
	"sync"

	"github.com/nelvko/proxyctl/internal/download"
	"github.com/nelvko/proxyctl/internal/redact"
)

// diagnostics collects warnings for a full-screen operation, without writing
// into its terminal. Progress is rendered by the TUI's own spinner.
type diagnostics struct {
	mu       sync.Mutex
	warnings []string
}

func (d *diagnostics) Report(event download.Event) {
	if !event.Warning || strings.TrimSpace(event.Message) == "" {
		return
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	d.warnings = append(d.warnings, redact.Error(errors.New(strings.TrimSpace(event.Message))).Error())
}
func (d *diagnostics) Text() string {
	d.mu.Lock()
	defer d.mu.Unlock()
	return strings.Join(d.warnings, "\n")
}
