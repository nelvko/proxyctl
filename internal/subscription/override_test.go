package subscription

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nelvko/proxyctl/internal/config"
	"github.com/nelvko/proxyctl/internal/kernel"
	"go.yaml.in/yaml/v3"
)

func saveTestOverride(t *testing.T, svc *Service, name string, global bool, content string) {
	t.Helper()
	session, err := svc.BeginOverrideEdit(name, global)
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	writeFile(t, session.Path(), content)
	if err := session.Commit(); err != nil {
		t.Fatal(err)
	}
}

func addOverrideProfile(t *testing.T, svc *Service, name, content string) profile {
	t.Helper()
	source := writeFile(t, filepath.Join(t.TempDir(), "source"), content)
	p := profile{Name: name, URL: "file://" + source}
	if err := svc.Add(&p); err != nil {
		t.Fatal(err)
	}
	return p
}

func overrideDocument(t *testing.T, path string) map[string]any {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	return doc
}

type overrideCheckingRuntime struct {
	*fakeKernel
	format kernel.ConfigFormat
	check  func(map[string]any) error
}

func (k *overrideCheckingRuntime) ConfigFormat() kernel.ConfigFormat {
	if k.format == "" {
		return kernel.FormatClash
	}
	return k.format
}

func (k *overrideCheckingRuntime) TestConfig(_ context.Context, path string) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var doc map[string]any
	if k.ConfigFormat() == kernel.FormatSingBox {
		err = json.Unmarshal(raw, &doc)
	} else {
		err = yaml.Unmarshal(raw, &doc)
	}
	if err != nil {
		return err
	}
	if k.check != nil {
		return k.check(doc)
	}
	return nil
}

func TestOverrideEditStagesAndUseAppliesWithoutChangingSource(t *testing.T) {
	svc, k := newTestService(t)
	raw := "proxies: [source]\nmixed-port: 7890\nmode: rule\n"
	p := addOverrideProfile(t, svc, "one", raw)
	if _, err := svc.Use(p.Name); err != nil {
		t.Fatal(err)
	}
	k.restarts = 0
	saveTestOverride(t, svc, "", true, "mixed-port: 7891\nmode: direct\n")
	saveTestOverride(t, svc, p.Name, false, "mixed-port: 7892\n")
	active, _ := os.ReadFile(k.cfgFile)
	if string(active) != raw || k.restarts != 0 {
		t.Fatal("saving overrides changed the running configuration")
	}
	if _, err := svc.Use(p.Name); err != nil {
		t.Fatal(err)
	}
	doc := overrideDocument(t, k.cfgFile)
	if doc["mixed-port"] != 7892 || doc["mode"] != "direct" || k.restarts != 1 {
		t.Fatalf("applied configuration: %+v, restarts=%d", doc, k.restarts)
	}
	cached, _ := os.ReadFile(p.File)
	if string(cached) != raw {
		t.Fatalf("rendering replaced the raw cache: %q", cached)
	}
}

func TestOverrideCommitKeepsEditedFilesPrivate(t *testing.T) {
	svc, _ := newTestService(t)
	p := addOverrideProfile(t, svc, "one", "proxies: []\n")
	session, err := svc.BeginOverrideEdit(p.Name, false)
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	writeFile(t, session.Path(), "secret: private\n")
	if err := os.Chmod(session.Path(), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := session.Commit(); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(p.File + ".override.yaml")
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("editor permissions exposed saved override: %v, %v", info, err)
	}
}

func TestOverrideUpdateComparesEffectiveConfiguration(t *testing.T) {
	svc, k := newTestService(t)
	p := addOverrideProfile(t, svc, "one", "proxies: [source]\nmixed-port: 7890\n")
	saveTestOverride(t, svc, p.Name, false, "mixed-port: 7892\n")
	if _, err := svc.Use(p.Name); err != nil {
		t.Fatal(err)
	}
	k.restarts = 0
	if _, err := svc.Update(DownloadOptions{}, p.Name); err != nil || k.restarts != 0 {
		t.Fatalf("unchanged effective config restarted: %d, %v", k.restarts, err)
	}
	newRaw := "proxies: [source]\nmixed-port: 8000\n"
	writeFile(t, p.URL[len("file://"):], newRaw)
	results, err := svc.Update(DownloadOptions{}, p.Name)
	if err != nil || len(results) != 1 || results[0].Status != Updated || k.restarts != 0 {
		t.Fatalf("masked source change: %+v, restarts=%d, %v", results, k.restarts, err)
	}
	cached, _ := os.ReadFile(p.File)
	if string(cached) != newRaw {
		t.Fatalf("raw update lost: %q", cached)
	}
	saveTestOverride(t, svc, p.Name, false, "mixed-port: 7893\n")
	if _, err := svc.Update(DownloadOptions{}, p.Name); err != nil {
		t.Fatal(err)
	}
	if doc := overrideDocument(t, k.cfgFile); doc["mixed-port"] != 7893 || k.restarts != 1 {
		t.Fatalf("changed override was not applied: %+v, restarts=%d", doc, k.restarts)
	}
}

func TestGlobalOverridesAreIsolatedByFormat(t *testing.T) {
	svc, k := newTestService(t)
	clash := addOverrideProfile(t, svc, "clash", "proxies: []\n")
	saveTestOverride(t, svc, "", true, "mode: direct\n")
	svc.kernel = &overrideCheckingRuntime{fakeKernel: k, format: kernel.FormatSingBox}
	if patch, err := svc.ReadOverride("", true); err != nil || patch != nil {
		t.Fatalf("Clash global leaked into sing-box: %q, %v", patch, err)
	}
	saveTestOverride(t, svc, "", true, `{"log":{"level":"warn"}}`)
	box := addOverrideProfile(t, svc, "box", `{"inbounds":[]}`)
	if _, err := svc.Use(box.Name); err != nil {
		t.Fatal(err)
	}
	doc := overrideDocument(t, k.cfgFile)
	if _, leaked := doc["mode"]; leaked || doc["log"] == nil {
		t.Fatalf("wrong global format applied: %+v", doc)
	}
	svc.kernel = k
	if _, err := svc.Use(clash.Name); err != nil {
		t.Fatal(err)
	}
	doc = overrideDocument(t, k.cfgFile)
	if _, leaked := doc["log"]; leaked || doc["mode"] != "direct" {
		t.Fatalf("wrong global restored: %+v", doc)
	}
}

func TestOverrideSurvivesRenameAndIsRemovedWithProfile(t *testing.T) {
	svc, _ := newTestService(t)
	p := addOverrideProfile(t, svc, "old", "proxies: []\n")
	saveTestOverride(t, svc, p.Name, false, "mode: direct\n")
	p.Name = "new"
	if err := svc.Set("old", p); err != nil {
		t.Fatal(err)
	}
	metadata := NewService(svc.subConfig, nil)
	if patch, err := metadata.ReadOverride("new", false); err != nil || !bytes.Contains(patch, []byte("direct")) {
		t.Fatalf("rename lost the override or show requires runtime: %q, %v", patch, err)
	}
	if err := metadata.Delete("new", false); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{p.File, p.File + ".override.yaml"} {
		if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("profile deletion left %s: %v", path, err)
		}
	}
}

func TestFailedProfileDeletionPreservesOverride(t *testing.T) {
	svc, _ := newTestService(t)
	p := addOverrideProfile(t, svc, "one", "proxies: []\n")
	saveTestOverride(t, svc, p.Name, false, "mode: direct\n")
	root, err := config.ConfigDir()
	if err != nil {
		t.Fatal(err)
	}
	metadataFile := filepath.Join(root, "subscriptions.yaml")
	if err := os.Remove(metadataFile); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(metadataFile, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := svc.Delete(p.Name, false); err == nil {
		t.Fatal("expected metadata save failure")
	}
	if patch, err := svc.ReadOverride(p.Name, false); err != nil || !bytes.Contains(patch, []byte("direct")) {
		t.Fatalf("failed deletion removed the override: %q, %v", patch, err)
	}
}

func TestUnknownOverrideFormatDoesNotPreventSourceCleanup(t *testing.T) {
	svc, _ := newTestService(t)
	p := addOverrideProfile(t, svc, "legacy", "proxies: []\n")
	svc.subConfig.Profiles[0].Format = "unknown-format"
	metadata := NewService(svc.subConfig, nil)
	if err := metadata.Delete(p.Name, false); err == nil {
		t.Fatal("expected a diagnostic for unknown override storage")
	}
	if _, err := os.Stat(p.File); !errors.Is(err, os.ErrNotExist) || len(metadata.List()) != 0 {
		t.Fatalf("unknown format prevented original cleanup: %v", err)
	}
}

func TestGlobalOverrideValidatesEveryAffectedProfileBeforeSaving(t *testing.T) {
	svc, k := newTestService(t)
	addOverrideProfile(t, svc, "first", "proxies: []\nname: first\n")
	addOverrideProfile(t, svc, "second", "proxies: []\nname: second\n")
	saveTestOverride(t, svc, "", true, "mixed-port: 7890\n")
	before, _ := svc.ReadOverride("", true)
	svc.kernel = &overrideCheckingRuntime{fakeKernel: k, check: func(doc map[string]any) error {
		if doc["name"] == "second" && doc["mixed-port"] == 7891 {
			return &kernel.ConfigValidationError{Output: "second rejects this port"}
		}
		return nil
	}}
	session, err := svc.BeginOverrideEdit("", true)
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	writeFile(t, session.Path(), "mixed-port: 7891\n")
	if err := session.Commit(); err == nil {
		t.Fatal("global override saved despite one invalid affected profile")
	}
	after, _ := svc.ReadOverride("", true)
	if !bytes.Equal(before, after) || k.restarts != 0 {
		t.Fatal("failed edit changed global override or restarted the service")
	}
}

func TestClearOverrideValidatesBeforeRemovalAndDoesNotApply(t *testing.T) {
	svc, k := newTestService(t)
	p := addOverrideProfile(t, svc, "one", "proxies: []\n")
	saveTestOverride(t, svc, p.Name, false, "required: true\n")
	svc.kernel = &overrideCheckingRuntime{fakeKernel: k, check: func(doc map[string]any) error {
		if doc["required"] != true {
			return &kernel.ConfigValidationError{Output: "required setting missing"}
		}
		return nil
	}}
	if _, err := svc.Use(p.Name); err != nil {
		t.Fatal(err)
	}
	k.restarts = 0
	active, _ := os.ReadFile(k.cfgFile)
	before, _ := svc.ReadOverride(p.Name, false)
	if err := svc.ClearOverride(p.Name, false); err == nil {
		t.Fatal("removed an override required for a valid final config")
	}
	after, _ := svc.ReadOverride(p.Name, false)
	if !bytes.Equal(before, after) {
		t.Fatal("failed clear removed the override")
	}
	svc.kernel = k
	if err := svc.ClearOverride(p.Name, false); err != nil {
		t.Fatal(err)
	}
	if patch, err := svc.ReadOverride(p.Name, false); err != nil || patch != nil {
		t.Fatalf("clear retained override: %q, %v", patch, err)
	}
	if err := svc.ClearOverride(p.Name, false); err != nil {
		t.Fatalf("clear was not idempotent: %v", err)
	}
	after, _ = os.ReadFile(k.cfgFile)
	if !bytes.Equal(active, after) || k.restarts != 0 {
		t.Fatal("clear changed the applied configuration")
	}
}

func TestOverrideApplicationFailureRollsBackAndUpdateRetries(t *testing.T) {
	svc, k := newTestService(t)
	p := addOverrideProfile(t, svc, "one", "proxies: [old]\n")
	saveTestOverride(t, svc, p.Name, false, "mixed-port: 7892\n")
	if _, err := svc.Use(p.Name); err != nil {
		t.Fatal(err)
	}
	active, _ := os.ReadFile(k.cfgFile)
	svc.kernel = &failOnceRuntime{fakeKernel: k, failNext: true}
	newRaw := "proxies: [new]\n"
	writeFile(t, p.URL[len("file://"):], newRaw)
	if _, err := svc.Update(DownloadOptions{}, p.Name); err == nil {
		t.Fatal("expected restart failure")
	}
	after, _ := os.ReadFile(k.cfgFile)
	cached, _ := os.ReadFile(p.File)
	if !bytes.Equal(active, after) || string(cached) != newRaw || svc.SelectedName() != p.Name {
		t.Fatalf("rollback lost applied config, new cache or selection: %q, %q", after, cached)
	}
	if _, err := svc.Update(DownloadOptions{}, p.Name); err != nil {
		t.Fatal(err)
	}
	doc := overrideDocument(t, k.cfgFile)
	proxies, _ := doc["proxies"].([]any)
	if doc["mixed-port"] != 7892 || len(proxies) != 1 || proxies[0] != "new" {
		t.Fatalf("unchanged download did not retry the final config: %+v", doc)
	}
}

func TestOverrideValidationUsesFinalContentForAddEditAndValidate(t *testing.T) {
	svc, k := newTestService(t)
	saveTestOverride(t, svc, "", true, "proxies: []\n")
	p := addOverrideProfile(t, svc, "one", "mixed-port: 7890\n")
	if err := svc.Validate(p.Name); err != nil {
		t.Fatalf("validated raw source instead of final config: %v", err)
	}
	edit, err := svc.BeginEdit(p.Name)
	if err != nil {
		t.Fatal(err)
	}
	defer edit.Close()
	writeFile(t, edit.Path(), "mixed-port: 7891\n")
	if err := edit.Commit(); err != nil {
		t.Fatalf("raw edit ignored the global override: %v", err)
	}
	if _, err := svc.Use(p.Name); err != nil {
		t.Fatal(err)
	}
	if doc := overrideDocument(t, k.cfgFile); doc["mixed-port"] != 7891 || doc["proxies"] == nil {
		t.Fatalf("final edited config: %+v", doc)
	}
}

func TestOverrideScopeAndReadOnlyMissingState(t *testing.T) {
	svc, _ := newTestService(t)
	root, err := config.ConfigDir()
	if err != nil {
		t.Fatal(err)
	}
	if patch, err := svc.ReadOverride("", true); err != nil || patch != nil {
		t.Fatalf("empty global state: %q, %v", patch, err)
	}
	if _, err := os.Stat(root); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("read-only query created storage: %v", err)
	}
	for _, scope := range []struct {
		name   string
		global bool
	}{{"", false}, {"both", true}} {
		if _, err := svc.ReadOverride(scope.name, scope.global); err == nil {
			t.Fatalf("accepted invalid scope: %+v", scope)
		}
	}
	metadata := NewService(svc.subConfig, nil)
	if _, err := metadata.ReadOverride("", true); err == nil {
		t.Fatal("resolved global format without runtime")
	}
	session, err := svc.BeginOverrideEdit("", true)
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	writeFile(t, session.Path(), "- not-an-object\n")
	if err := session.Commit(); err == nil {
		t.Fatal("accepted invalid global syntax without profiles")
	}
	if patch, err := svc.ReadOverride("", true); err != nil || patch != nil {
		t.Fatalf("invalid global edit was saved: %q, %v", patch, err)
	}
}

func TestOverrideValidationDiagnosticsRetainOriginalDownload(t *testing.T) {
	for _, raw := range []string{"proxies: [", "proxies: []\n"} {
		t.Run(raw, func(t *testing.T) {
			svc, k := newTestService(t)
			saveTestOverride(t, svc, "", true, "mode: direct\n")
			svc.kernel = &overrideCheckingRuntime{fakeKernel: k, check: func(map[string]any) error {
				return &kernel.ConfigValidationError{Output: "rejected effective config"}
			}}
			svc.KeepFailed = true
			source := writeFile(t, filepath.Join(t.TempDir(), "invalid.yaml"), raw)
			err := svc.Add(&profile{Name: "invalid", URL: "file://" + source})
			if err == nil || !strings.Contains(err.Error(), "original download") || strings.Contains(err.Error(), ".effective-") {
				t.Fatalf("diagnostic did not identify retained input: %v", err)
			}
			dir, _ := config.DiagnosticsDir()
			files, err := os.ReadDir(dir)
			if err != nil || len(files) != 1 {
				t.Fatalf("retained diagnostics: %v, %v", files, err)
			}
			data, err := os.ReadFile(filepath.Join(dir, files[0].Name()))
			if err != nil || string(data) != raw || len(svc.List()) != 0 {
				t.Fatalf("diagnostic replaced original download with a merged file: %q, %v", data, err)
			}
		})
	}
}
