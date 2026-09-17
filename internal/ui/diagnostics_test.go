package ui

import (
	"strings"
	"testing"

	"github.com/nelvko/proxyctl/internal/download"
)

func TestDiagnosticsKeepWarningsAndRedactSources(t *testing.T) {
	var d diagnostics
	d.Report(download.Event{Message: "progress status"})
	d.Report(download.Event{Progress: true, Bytes: 100})
	d.Report(download.Event{Warning: true, Message: "fetch https://user:password@example.com/private-token failed"})
	text := d.Text()
	if strings.Contains(text, "progress") || strings.Contains(text, "password") || strings.Contains(text, "private-token") || !strings.Contains(text, "failed") {
		t.Fatal(text)
	}
}
