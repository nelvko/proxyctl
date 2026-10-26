package sub

import (
	"bytes"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/nelvko/proxyctl/internal/config"
)

func overrideTestState(t *testing.T) string {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("AppData", t.TempDir())
	dir, err := config.ConfigDir()
	if err != nil {
		t.Fatal(err)
	}
	return dir
}

func runOverrideCommand(t *testing.T, args ...string) (string, error) {
	t.Helper()
	cmd := NewCommand()
	var output, stderr bytes.Buffer
	cmd.SetOut(&output)
	cmd.SetErr(&stderr)
	cmd.SetIn(strings.NewReader(""))
	cmd.SetContext(t.Context())
	cmd.SilenceErrors, cmd.SilenceUsage = true, true
	cmd.SetArgs(args)
	err := cmd.Execute()
	return output.String(), err
}

func overrideTestFixture(t *testing.T, profiles bool) (active, cache, binary string) {
	t.Helper()
	overrideTestState(t)
	dir := t.TempDir()
	binary = filepath.Join(dir, "kernel")
	if err := os.WriteFile(binary, []byte("#!/bin/sh\nif [ \"$1\" = -t ]; then exit 0; fi\nexit 91\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	active = filepath.Join(dir, "active.yaml")
	cache = filepath.Join(dir, "profile.yaml")
	for _, path := range []string{active, cache} {
		if err := os.WriteFile(path, []byte("mixed-port: 7890\nproxies: []\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := config.Save(&config.Config{Use: "mihomo", Kernels: []config.KernelConfig{{Name: "mihomo", Bin: binary, ConfigDir: dir, ConfigFile: active}}}); err != nil {
		t.Fatal(err)
	}
	subs := &config.SubscriptionConfig{}
	if profiles {
		subs.Use = "work"
		subs.Profiles = []config.Profile{{Name: "work", File: cache, Format: "clash"}}
	}
	if err := config.SaveSubscriptionConfig(subs); err != nil {
		t.Fatal(err)
	}
	return active, cache, binary
}

func overrideTestEditor(t *testing.T, content string, exitCode int) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "editor")
	script := "#!/bin/sh\ncat > \"$1\" <<'PROXYCTL_OVERRIDE'\n" + content + "PROXYCTL_OVERRIDE\nexit " + strconv.Itoa(exitCode) + "\n"
	if err := os.WriteFile(path, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestOverrideRequiresExplicitExclusiveTargetBeforeLoading(t *testing.T) {
	for _, action := range []string{"edit", "show", "clear"} {
		for _, target := range [][]string{nil, {""}, {"--global", "work"}, {"work", "backup"}, {"--global=false"}} {
			t.Run(action+"/"+strings.Join(target, "_"), func(t *testing.T) {
				dir := overrideTestState(t)
				if err := os.MkdirAll(dir, 0o700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(dir, "config.yaml"), []byte("broken: ["), 0o600); err != nil {
					t.Fatal(err)
				}
				args := append([]string{"override", action}, target...)
				if _, err := runOverrideCommand(t, args...); err == nil || strings.Contains(err.Error(), "config") {
					t.Fatalf("target validation ran after loading state: args=%v err=%v", args, err)
				}
				if _, err := os.Stat(filepath.Join(dir, "state.lock")); !os.IsNotExist(err) {
					t.Fatalf("invalid target acquired state lock: %v", err)
				}
			})
		}
	}
}

func TestOverrideEditShowClearLeaveSourceAndActiveConfigUnchanged(t *testing.T) {
	active, cache, _ := overrideTestFixture(t, true)
	before, err := os.ReadFile(active)
	if err != nil {
		t.Fatal(err)
	}
	patch := "mixed-port: 7891\nsecret: explicit-secret-value\n"
	editor := overrideTestEditor(t, patch, 0)
	output, err := runOverrideCommand(t, "override", "edit", "work", "-e", editor)
	if err != nil || !strings.Contains(output, "sub use work") || !strings.Contains(output, "active configuration unchanged") {
		t.Fatalf("edit: output=%q err=%v", output, err)
	}
	output, err = runOverrideCommand(t, "override", "show", "work")
	if err != nil || output != patch {
		t.Fatalf("explicit show changed the saved patch: output=%q err=%v", output, err)
	}
	output, err = runOverrideCommand(t, "override", "clear", "work")
	if err != nil || !strings.Contains(output, "cleared") || !strings.Contains(output, "sub use work") {
		t.Fatalf("clear: output=%q err=%v", output, err)
	}
	output, err = runOverrideCommand(t, "override", "show", "work")
	if err != nil || !strings.Contains(output, "no override") {
		t.Fatalf("cleared override remains: output=%q err=%v", output, err)
	}
	for _, path := range []string{active, cache} {
		if got, err := os.ReadFile(path); err != nil || !bytes.Equal(got, before) {
			t.Fatalf("override mutated source/active file %s: %q %v", path, got, err)
		}
	}
}

func TestOverrideEditorAndValidationFailurePreserveSavedPatch(t *testing.T) {
	overrideTestFixture(t, true)
	original := "mixed-port: 7891\n"
	if _, err := runOverrideCommand(t, "override", "edit", "work", "--editor", overrideTestEditor(t, original, 0)); err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		content string
		exit    int
	}{{"mixed-port: 7892\n", 7}, {"mixed-port: [\n", 0}} {
		if _, err := runOverrideCommand(t, "override", "edit", "work", "--editor", overrideTestEditor(t, test.content, test.exit)); err == nil {
			t.Fatalf("invalid/failed edit was saved: %+v", test)
		}
		output, err := runOverrideCommand(t, "override", "show", "work")
		if err != nil || output != original {
			t.Fatalf("failed edit replaced saved patch: output=%q err=%v", output, err)
		}
	}
}

func TestOverrideShowProfileNeedsOnlyMetadata(t *testing.T) {
	overrideTestFixture(t, true)
	patch := "mixed-port: 7891\n"
	if _, err := runOverrideCommand(t, "override", "edit", "work", "--editor", overrideTestEditor(t, patch, 0)); err != nil {
		t.Fatal(err)
	}
	dir, err := config.ConfigDir()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "config.yaml"), []byte("broken: ["), 0o600); err != nil {
		t.Fatal(err)
	}
	output, err := runOverrideCommand(t, "override", "show", "work")
	if err != nil || output != patch {
		t.Fatalf("profile override show required runtime: output=%q err=%v", output, err)
	}
}

func TestGlobalOverrideWithoutProfilesReceivesSyntaxValidation(t *testing.T) {
	_, _, binary := overrideTestFixture(t, false)
	// No profile exists, so the kernel validator must not be invoked yet.
	if err := os.WriteFile(binary, []byte("#!/bin/sh\nexit 91\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	patch := "mixed-port: 7891\n"
	output, err := runOverrideCommand(t, "override", "edit", "--global", "--editor", overrideTestEditor(t, patch, 0))
	if err != nil || !strings.Contains(output, "syntax checked") || !strings.Contains(output, "sub use <name>") {
		t.Fatalf("global edit without profiles: output=%q err=%v", output, err)
	}
	output, err = runOverrideCommand(t, "override", "show", "--global")
	if err != nil || output != patch {
		t.Fatalf("global show: output=%q err=%v", output, err)
	}
	if _, err := runOverrideCommand(t, "override", "clear", "--global"); err != nil {
		t.Fatalf("global clear without profiles: %v", err)
	}
}
