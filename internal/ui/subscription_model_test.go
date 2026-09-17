package ui

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"github.com/nelvko/proxyctl/internal/config"
	"github.com/nelvko/proxyctl/internal/subscription"
)

func TestTUISerializesMutatingActions(t *testing.T) {
	svc := subscription.NewService(&config.SubscriptionConfig{Profiles: []profile{{Name: "one"}}}, nil)
	m := initialModel(t.Context(), svc)
	next, operation := m.Update(tea.KeyPressMsg{Code: 'u'})
	m = next.(model)
	if operation == nil || !m.busy {
		t.Fatal("update did not enter busy state")
	}
	// Do not execute the operation: it remains in flight while keys arrive.
	for _, key := range []rune{'u', 'a', 'd', 'e', tea.KeyEnter, 'q'} {
		next, cmd := m.Update(tea.KeyPressMsg{Code: key})
		m = next.(model)
		if cmd != nil || !m.busy {
			t.Fatalf("key %q dispatched work during another operation", key)
		}
	}
	next, _ = m.Update(operationFinishedMsg{})
	m = next.(model)
	if m.busy {
		t.Fatal("completion did not release busy state")
	}
	next, operation = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if operation == nil || !next.(model).busy {
		t.Fatal("use did not start after update completed")
	}
}

func TestTUIFailedAddRetainsEditableDraft(t *testing.T) {
	for _, failure := range []error{errors.New("validation failed"), context.Canceled} {
		svc := subscription.NewService(&config.SubscriptionConfig{}, nil)
		m := initialModel(t.Context(), svc)
		m.width, m.height = 80, 24
		m.openAddForm()
		m.formDraft.URL = "https://example.com/private"
		m.formData.timeout = "45s"
		m.formData.headers = "X-Token: private"
		m.formData.advanced = true
		next, _ := m.Update(addSubmittedMsg{})
		m = next.(model)
		if !m.busy || m.formFocused || m.formDraft == nil {
			t.Fatal("submission discarded its draft")
		}
		oldForm := m.form
		next, _ = m.Update(operationFinishedMsg{err: failure})
		m = next.(model)
		if m.busy || !m.formFocused || m.formDraft == nil || m.form == oldForm {
			t.Fatal("failure did not reopen an editable form")
		}
		if m.formDraft.Name != "" || m.formDraft.URL != "https://example.com/private" || m.formData.timeout != "45s" || m.formData.headers != "X-Token: private" || !m.formData.advanced {
			t.Fatal("failure lost user input")
		}
		if m.err != nil {
			next, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
			m = next.(model)
		}
		next, _ = m.Update(addSubmittedMsg{})
		m = next.(model)
		if !m.busy {
			t.Fatal("retry did not submit")
		}
		next, _ = m.Update(operationFinishedMsg{added: "inferred"})
		m = next.(model)
		if m.formDraft != nil || m.form != nil {
			t.Fatal("successful submission retained old draft")
		}
		m.openAddForm()
		m.cancelForm()
		if m.formDraft != nil || m.formData != nil {
			t.Fatal("cancel retained old draft")
		}
	}
}

func TestMetadataFormPreservesInputOnCrossFieldValidationFailure(t *testing.T) {
	m := initialModel(t.Context(), subscription.NewService(&config.SubscriptionConfig{}, nil))
	m.width, m.height = 80, 24
	m.openAddForm()
	m.formDraft.URL = "https://example.com"
	m.formDraft.Download.UserAgent = "custom"
	m.formData.headers = "User-Agent: conflicting"
	next, _ := m.Update(addSubmittedMsg{})
	m = next.(model)
	if m.busy || m.err == nil || !m.formFocused || m.formData.headers != "User-Agent: conflicting" {
		t.Fatal("invalid settings lost instead of remaining editable")
	}
}

func TestMetadataFormDoesNotOverwriteConcurrentChanges(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("AppData", t.TempDir())
	original := profile{Name: "office", URL: "https://example.com", Download: config.DownloadConfig{Headers: map[string]string{"X-Token": "old"}}}
	svc := subscription.NewService(&config.SubscriptionConfig{Profiles: []profile{original}}, nil)
	draft := original
	f := newProfileForm(svc, &draft, original.Name)
	f.headers = "X-Token: edited"
	if err := f.apply(); err != nil {
		t.Fatal(err)
	}
	if reflect.DeepEqual(draft.Download.Headers, original.Download.Headers) || original.Download.Headers["X-Token"] != "old" {
		t.Fatal("form edited shared metadata")
	}
	concurrent := original
	concurrent.Download.UserAgent = "other-process"
	if err := svc.Set(original.Name, concurrent); err != nil {
		t.Fatal(err)
	}
	if err := saveFormProfile(svc, original, draft); err == nil {
		t.Fatal("stale form overwrote concurrent metadata")
	}
	current, _ := svc.Get(original.Name)
	if current.Download.UserAgent != "other-process" || current.Download.Headers["X-Token"] != "old" {
		t.Fatal("conflict modified saved metadata")
	}
}

func TestTUIHostMatchesDoNotHighlightTitle(t *testing.T) {
	targets := []string{"工作\x00alpha.example", "example\x00beta.test", "office\x00example.org"}
	ranks := filterProfiles("example", targets)
	if len(ranks) != 3 || ranks[0].Index != 1 || len(ranks[0].MatchedIndexes) == 0 {
		t.Fatalf("name matches should lead: %+v", ranks)
	}
	for _, rank := range ranks[1:] {
		if len(rank.MatchedIndexes) != 0 {
			t.Fatal("hostname match highlighted title")
		}
	}
	if len(filterProfiles("ALPHA", targets)) != 1 || len(filterProfiles("工作", targets)) != 1 || len(filterProfiles("missing", targets)) != 0 {
		t.Fatal("case/Unicode/empty search incorrect")
	}
	p := item(profile{Name: "office", URL: "https://user:pass@example.org/path-secret?token=query-secret"})
	if strings.Contains(p.FilterValue(), "secret") || strings.Contains(p.Description(), "secret") {
		t.Fatal("secret used for display/search")
	}
}

func TestTUIQuitAndCancellationBindings(t *testing.T) {
	svc := subscription.NewService(&config.SubscriptionConfig{Profiles: []profile{{Name: "one"}}}, nil)
	m := initialModel(t.Context(), svc)
	if key.Matches(tea.KeyPressMsg{Code: 'v'}, m.list.KeyMap.Quit) {
		t.Fatal("v still quits as select")
	}
	_, quit := m.Update(tea.KeyPressMsg{Code: 'q'})
	if quit == nil {
		t.Fatal("q did not quit")
	}
	if _, ok := quit().(tea.QuitMsg); !ok {
		t.Fatal("q returned a different action")
	}
	ctx, cancel := context.WithCancel(t.Context())
	m.busy, m.cancel = true, cancel
	next, _ := m.Update(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})
	m = next.(model)
	if ctx.Err() == nil || !m.busy {
		t.Fatal("ctrl+c must cancel and wait for operation completion")
	}
	next, _ = m.Update(operationFinishedMsg{err: context.Canceled})
	if next.(model).busy {
		t.Fatal("cancelled completion did not release busy state")
	}
}
