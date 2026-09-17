// Package download retrieves remote content for subscriptions and kernel assets.
package download

import (
	"fmt"
	"net/http"
	"os"
	"slices"
	"strings"
	"sync"
)

// Event carries download progress or a diagnostic to the caller's presentation.
type Event struct {
	Message      string
	Warning      bool
	Total, Bytes int64
	Progress     bool
}

type Notify func(Event)

// Downloader owns one operation's transport, mirrors and diagnostics.
// Instances never change package defaults or another operation's client.
type Downloader struct {
	client                      *http.Client
	mirrors                     []string
	notify                      Notify
	warnLoopback, warnPlainHTTP sync.Once
}

func New(mirrors []string, client *http.Client, notify Notify) *Downloader {
	if env := os.Getenv("GH_PROXY"); env != "" {
		mirrors = strings.Split(env, ",")
	}
	if client == nil {
		client = http.DefaultClient
	}
	return &Downloader{client: client, mirrors: slices.DeleteFunc(slices.Clone(mirrors), func(m string) bool { return strings.TrimSpace(m) == "" }), notify: notify}
}

func (d *Downloader) Client() *http.Client { return d.client }

func (d *Downloader) Mirrors() []string { return slices.Clone(d.mirrors) }

func (d *Downloader) Report(event Event) {
	if d.notify != nil {
		d.notify(event)
	}
}

func (d *Downloader) Message(warning bool, format string, args ...any) {
	d.Report(Event{Message: fmt.Sprintf(format, args...), Warning: warning})
}

func (d *Downloader) Progress() func(int64, int64) {
	if d.notify == nil {
		return nil
	}
	return func(total, n int64) { d.Report(Event{Progress: true, Total: total, Bytes: n}) }
}

func (d *Downloader) WithClient(client *http.Client) *Downloader {
	return &Downloader{client: client, mirrors: slices.Clone(d.mirrors), notify: d.notify}
}
