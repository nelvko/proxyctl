package ui

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/nelvko/proxyctl/internal/download"
	"github.com/nelvko/proxyctl/internal/redact"
	"golang.org/x/term"
)

// Reporter renders diagnostics to the command's writer. Pipes receive messages
// but no progress; each operation owns its own progress throttle.
func Reporter(out io.Writer) download.Notify {
	terminal := false
	if f, ok := out.(*os.File); ok {
		terminal = term.IsTerminal(int(f.Fd()))
	}
	var mu sync.Mutex
	var last time.Time
	return func(event download.Event) {
		mu.Lock()
		defer mu.Unlock()
		if event.Progress {
			if !terminal || time.Since(last) < time.Second {
				return
			}
			last = time.Now()
			if event.Total > 0 {
				fmt.Fprintf(out, "%.1f / %.1f MiB\n", float64(event.Bytes)/(1<<20), float64(event.Total)/(1<<20))
			} else {
				fmt.Fprintf(out, "%.1f MiB\n", float64(event.Bytes)/(1<<20))
			}
			return
		}
		if event.Message != "" {
			fmt.Fprintln(out, redact.Error(errors.New(strings.TrimSpace(event.Message))))
		}
	}
}
