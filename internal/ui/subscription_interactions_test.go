package ui

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
	"charm.land/huh/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/nelvko/proxyctl/internal/config"
	"github.com/nelvko/proxyctl/internal/i18n"
	"github.com/nelvko/proxyctl/internal/subscription"
)

func subscriptionInteractionModel(t *testing.T, count, width, height int) model {
	t.Helper()
	profiles := make([]profile, count)
	for i := range profiles {
		profiles[i] = profile{Name: fmt.Sprintf("profile-%02d", i), URL: fmt.Sprintf("https://node-%02d.example/source", i)}
	}
	m := initialModel(t.Context(), subscription.NewService(&config.SubscriptionConfig{Profiles: profiles}, nil))
	m.locale = i18n.English
	m.width, m.height = width, height
	m.layoutSubscriptions()
	return m
}

func subscriptionUpdate(m model, msg tea.Msg) model {
	next, _ := m.Update(msg)
	return next.(model)
}

func subscriptionActionClick(t *testing.T, m model, id string) (model, tea.Cmd) {
	t.Helper()
	for _, action := range m.layoutSubscriptions().actions {
		if action.id == id {
			next, cmd := m.Update(tea.MouseClickMsg{X: action.Bounds.X + 2, Y: action.Bounds.Y, Button: tea.MouseLeft})
			return next.(model), cmd
		}
	}
	t.Fatalf("action %s not visible", id)
	return m, nil
}

func TestSubscriptionMouseUsesFilteredPaginatedRows(t *testing.T) {
	m := subscriptionInteractionModel(t, 24, 80, 15)
	m.list.SetFilterText("profile-1")
	m.list.Select(4)
	layout := m.layoutSubscriptions()
	start, _ := m.list.Paginator.GetSliceBounds(len(m.list.VisibleItems()))
	if start == 0 {
		t.Fatal("fixture did not advance to another page")
	}
	want := m.list.VisibleItems()[start].(item).Name
	view := strings.Split(ansi.Strip(m.View().Content), "\n")
	if !strings.Contains(view[layout.list.Y], want) {
		t.Fatal("first visible row differs from native page")
	}
	m = subscriptionUpdate(m, tea.MouseClickMsg{X: 4, Y: layout.list.Y, Button: tea.MouseLeft})
	if m.list.SelectedItem().(item).Name != want || m.busy {
		t.Fatal("click did not select filtered page row, or applied it")
	}
	before := m.list.Index()
	m = subscriptionUpdate(m, tea.MouseWheelMsg{X: 4, Y: layout.list.Y, Button: tea.MouseWheelDown})
	if m.list.Index() != before+1 {
		t.Fatal("wheel did not move native cursor")
	}
	before = m.list.Index()
	m = subscriptionUpdate(m, tea.MouseWheelMsg{X: 4, Y: 0, Button: tea.MouseWheelDown})
	if m.list.Index() != before {
		t.Fatal("wheel outside the list changed selection")
	}
	m = subscriptionUpdate(m, tea.MouseMotionMsg{X: 4, Y: layout.list.Y})
	m.layoutSubscriptions()
	if m.hoverIndex != start || m.list.Index() != before {
		t.Fatal("hover changed keyboard selection")
	}
}

func TestSubscriptionToolbarKeepsKeyboardAndModalFocus(t *testing.T) {
	m := subscriptionInteractionModel(t, 20, 80, 18)
	m.list.SetFilterText("profile-1")
	m.list.Select(2)
	index, page, filter := m.list.Index(), m.list.Paginator.Page, m.list.FilterValue()
	m = subscriptionUpdate(m, tea.KeyPressMsg{Code: tea.KeyTab})
	if m.toolbarFocus != 0 {
		t.Fatal("tab did not focus toolbar")
	}
	m = subscriptionUpdate(m, tea.KeyPressMsg{Code: tea.KeyEsc})
	if m.toolbarFocus != -1 {
		t.Fatal("escape did not return focus to list")
	}
	m, _ = subscriptionActionClick(t, m, "help")
	if !m.dialog.Active || m.busy {
		t.Fatal("help did not open modal")
	}
	m = subscriptionUpdate(m, tea.KeyPressMsg{Code: tea.KeyDown})
	m = subscriptionUpdate(m, tea.MouseWheelMsg{X: m.dialog.bodyBounds.X, Y: m.dialog.bodyBounds.Y, Button: tea.MouseWheelDown})
	m = subscriptionUpdate(m, tea.KeyPressMsg{Code: tea.KeyEsc})
	if m.dialog.Active || m.list.Index() != index || m.list.Paginator.Page != page || m.list.FilterValue() != filter || m.toolbarFocus != -1 {
		t.Fatal("modal damaged underlying list/filter/focus")
	}
	m, _ = subscriptionActionClick(t, m, "remove")
	if !m.dialog.Active || m.removeTarget == nil || m.busy {
		t.Fatal("remove did not require confirmation")
	}
	target := *m.removeTarget
	m = subscriptionUpdate(m, tea.KeyPressMsg{Code: tea.KeyEnter})
	if m.dialog.Active || m.removeTarget != nil || m.busy || len(m.profiles.List()) != 20 {
		t.Fatal("default Enter must cancel removal")
	}
	m, _ = subscriptionActionClick(t, m, "remove")
	m = subscriptionUpdate(m, tea.KeyPressMsg{Code: tea.KeyRight})
	next, operation := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = next.(model)
	if !m.busy || operation == nil || m.dialog.Active || !reflect.DeepEqual(target, profile(m.list.SelectedItem().(item))) {
		t.Fatal("explicit confirmation did not dispatch original target")
	}
	// Do not run the filesystem operation. Cancellation must wait for completion.
	if m.cancel != nil {
		m.cancel()
	}
}

func TestSubscriptionMouseBusyCancelAndDisabledActions(t *testing.T) {
	m := subscriptionInteractionModel(t, 0, 80, 24)
	m, cmd := subscriptionActionClick(t, m, "use")
	if cmd != nil || m.busy || m.err != nil {
		t.Fatal("disabled action reacted")
	}
	ctx, cancel := context.WithCancel(t.Context())
	m.busy, m.cancel = true, cancel
	m, _ = subscriptionActionClick(t, m, "cancel")
	if ctx.Err() == nil || !m.busy {
		t.Fatal("mouse cancel must cancel context and wait")
	}
	m = subscriptionUpdate(m, operationFinishedMsg{err: context.Canceled})
	if m.busy {
		t.Fatal("completion did not release busy state")
	}
}

func TestSubscriptionMouseFormNextHonorsHuhValidation(t *testing.T) {
	m := subscriptionInteractionModel(t, 0, 80, 24)
	m, _ = subscriptionActionClick(t, m, "add")
	if !m.formFocused {
		t.Fatal("mouse add did not open form")
	}
	field := m.form.GetFocusedField()
	m, cmd := subscriptionActionClick(t, m, "next")
	if m.form.GetFocusedField() != field || m.busy {
		t.Fatal("mouse Next bypassed required source validation")
	}
	if cmd != nil {
		// Native validation returns no next-field command for an empty source.
		if msg := cmd(); msg != nil {
			t.Fatalf("invalid input advanced form: %T", msg)
		}
	}
	m.formDraft.URL = "https://example.com/draft"
	m, _ = subscriptionActionClick(t, m, "cancel")
	if m.formFocused || m.formDraft != nil || m.formData != nil {
		t.Fatal("mouse cancel retained form state")
	}
}

func TestSubscriptionResizeExactGridAndNoHiddenClicks(t *testing.T) {
	m := subscriptionInteractionModel(t, 20, 80, 24)
	m.list.SetFilterText("profile-1")
	m.list.Select(5)
	for _, size := range [][2]int{{1, 1}, {4, 3}, {18, 8}, {40, 12}, {120, 36}, {80, 24}} {
		m = subscriptionUpdate(m, tea.WindowSizeMsg{Width: size[0], Height: size[1]})
		view := ansi.Strip(m.View().Content)
		lines := strings.Split(view, "\n")
		if len(lines) != size[1] {
			t.Fatalf("%v height %d", size, len(lines))
		}
		for _, line := range lines {
			if ansi.StringWidth(line) != size[0] {
				t.Fatalf("%v width: %q", size, line)
			}
		}
		before := m.list.Index()
		m = subscriptionUpdate(m, tea.MouseClickMsg{X: size[0], Y: size[1], Button: tea.MouseLeft})
		if m.list.Index() != before || m.busy {
			t.Fatal("outside click changed state")
		}
		if m.list.FilterState() != list.FilterApplied || m.list.FilterValue() != "profile-1" {
			t.Fatal("resize discarded filter")
		}
	}
}

func TestSubscriptionFeedbackIgnoresStaleExpiry(t *testing.T) {
	m := subscriptionInteractionModel(t, 1, 80, 24)
	m.successMessage("old")
	old := subscriptionFeedbackExpiredMsg(m.feedbackID)
	m.pendingMessage("current")
	m = subscriptionUpdate(m, old)
	if m.feedback != "current" {
		t.Fatal("stale timer cleared a new message")
	}
}

func TestSubscriptionReverseTabEntersLastEnabledAction(t *testing.T) {
	m := subscriptionInteractionModel(t, 0, 80, 24)
	m = subscriptionUpdate(m, tea.KeyPressMsg{Code: tea.KeyTab, Mod: tea.ModShift})
	actions := m.layoutSubscriptions().actions
	if m.toolbarFocus < 0 || actions[m.toolbarFocus].id != "quit" {
		t.Fatal("shift+tab did not enter last enabled action")
	}
}

func TestSubscriptionErrorModalKeyboardScrollRetainsDraft(t *testing.T) {
	m := subscriptionInteractionModel(t, 1, 80, 24)
	m.openAddForm()
	m.formDraft.URL = "https://example.com/draft"
	m.errorMessage(strings.Repeat("diagnostic line\n", 50))
	m = subscriptionUpdate(m, tea.KeyPressMsg{Code: tea.KeyPgDown})
	if m.err == nil || !m.dialog.Active || m.dialog.viewport.YOffset() == 0 {
		t.Fatal("long error could not scroll by keyboard")
	}
	m = subscriptionUpdate(m, tea.KeyPressMsg{Code: tea.KeyEnter})
	if m.err != nil || m.dialog.Active || !m.formFocused || m.formDraft.URL != "https://example.com/draft" {
		t.Fatal("acknowledgement damaged form draft")
	}
}

// Run native Huh transitions, ignoring cursor animation after its first tick.
// Stop before dispatching the actual subscription operation to the filesystem.
func processHuhTestCommands(t *testing.T, m *model, command tea.Cmd) bool {
	t.Helper()
	queue := []tea.Cmd{command}
	for steps := 0; len(queue) > 0 && steps < 60; steps++ {
		cmd := queue[0]
		queue = queue[1:]
		if cmd == nil {
			continue
		}
		msg := cmd()
		switch message := msg.(type) {
		case tea.BatchMsg:
			queue = append(queue, message...)
		case addSubmittedMsg:
			return true
		default:
			if typ := reflect.TypeOf(msg); typ != nil && typ.PkgPath() == "charm.land/huh/v2" {
				queue = append(queue, m.handleForm(msg))
			}
		}
	}
	return false
}

func TestSubscriptionMouseFinalFormFieldSubmits(t *testing.T) {
	m := subscriptionInteractionModel(t, 0, 80, 24)
	m.openAddForm()
	m.formDraft.URL = "https://example.com/source"
	m.formData.advanced = true
	m.form.UpdateFieldPositions()
	// Navigate setup to the final request-headers group using native messages.
	processHuhTestCommands(t, &m, m.form.NextGroup())
	processHuhTestCommands(t, &m, m.form.NextGroup())
	if _, ok := m.form.GetFocusedField().(*huh.Text); !ok {
		t.Fatal("fixture did not reach final field")
	}
	m, cmd := subscriptionActionClick(t, m, "next")
	if !processHuhTestCommands(t, &m, cmd) || m.form.State != huh.StateCompleted {
		t.Fatal("mouse Next did not submit final valid Huh field")
	}
	if m.busy {
		t.Fatal("test executed actual subscription operation")
	}
}
