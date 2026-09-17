package ui

import (
	"os"
	"path/filepath"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/nelvko/proxyctl/internal/app"
	"github.com/nelvko/proxyctl/internal/config"
)

// Run only the command batch for one UI action, without starting a terminal.
func finishTestOperation(t *testing.T, operation tea.Cmd) operationFinishedMsg {
	t.Helper()
	if operation == nil {
		t.Fatal("action did not dispatch an operation")
	}
	batch, ok := operation().(tea.BatchMsg)
	if !ok {
		t.Fatal("expected an asynchronous operation batch")
	}
	for _, cmd := range batch {
		if result, ok := cmd().(operationFinishedMsg); ok {
			return result
		}
	}
	t.Fatal("operation did not finish")
	return operationFinishedMsg{}
}

func TestMetadataActionsIgnoreBrokenKernelConfig(t *testing.T) {
	for _, action := range []string{"modify", "remove"} {
		t.Run(action, func(t *testing.T) {
			t.Setenv("HOME", t.TempDir())
			t.Setenv("XDG_CONFIG_HOME", t.TempDir())
			t.Setenv("AppData", t.TempDir())
			file := filepath.Join(t.TempDir(), "profile.yaml")
			if err := os.WriteFile(file, []byte("original"), 0600); err != nil {
				t.Fatal(err)
			}
			original := profile{Name: "work", URL: "https://example.com", File: file}
			if err := config.SaveSubscriptionConfig(&config.SubscriptionConfig{Profiles: []profile{original}}); err != nil {
				t.Fatal(err)
			}
			dir, err := config.ConfigDir()
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, "config.yaml"), []byte("[invalid"), 0600); err != nil {
				t.Fatal(err)
			}
			svc, err := app.LoadProfileMetadata(t.Context())
			if err != nil {
				t.Fatal(err)
			}
			original, err = svc.Get("work")
			if err != nil {
				t.Fatal(err)
			}
			m := initialModel(t.Context(), svc)
			m.width, m.height = 80, 24
			var operation tea.Cmd
			if action == "modify" {
				draft := original
				m.formOriginal, m.formDraft = &original, &draft
				m.openProfileForm()
				m.formDraft.Name = "renamed"
				_, operation = m.Update(addSubmittedMsg{})
			} else {
				m.handleList(tea.KeyPressMsg{Code: 'd'})
				operation = m.handleList(tea.KeyPressMsg{Code: 'd'})
			}
			result := finishTestOperation(t, operation)
			if result.err != nil {
				t.Fatal(result.err)
			}
			saved, err := config.LoadSubscriptionConfig()
			if err != nil {
				t.Fatal(err)
			}
			if action == "modify" {
				if len(saved.Profiles) != 1 || saved.Profiles[0].Name != "renamed" {
					t.Fatal("metadata not saved")
				}
				data, err := os.ReadFile(file)
				if err != nil || string(data) != "original" {
					t.Fatal("metadata edit changed cache")
				}
			} else if len(saved.Profiles) != 0 {
				t.Fatal("profile not removed")
			}
		})
	}
}
