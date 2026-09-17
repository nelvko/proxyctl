package ui

import (
	"bytes"
	"strings"
	"testing"

	"github.com/nelvko/proxyctl/internal/download"
)

func TestReporterHonorsWriterAndOmitsPipeProgress(t *testing.T) {
	var out bytes.Buffer
	notify := Reporter(&out)
	notify(download.Event{Progress: true, Total: 100, Bytes: 50})
	if out.Len() != 0 {
		t.Fatal("progress polluted piped output", out.String())
	}
	notify(download.Event{Warning: true, Message: "could not download https://user:secret@example.com/token"})
	if !strings.Contains(out.String(), "could not download") || strings.Contains(out.String(), "secret") || strings.Contains(out.String(), "token") {
		t.Fatal(out.String())
	}
}
