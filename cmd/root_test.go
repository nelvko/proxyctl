package cmd

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nelvko/proxyctl/internal/config"
)

func isolatedConfig(t *testing.T) string {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("AppData", t.TempDir())
	t.Setenv("PROXYCTL_WRAPPED", "1")
	t.Setenv("PROXYCTL_SHELL", "bash")
	root, err := os.UserConfigDir()
	if err != nil {
		t.Fatal(err)
	}
	return filepath.Join(root, config.AppName)
}

func runCommand(args ...string) (string, string, error) {
	root := NewRootCmd()
	var out, stderr bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&stderr)
	root.SetArgs(args)
	err := root.Execute()
	return out.String(), stderr.String(), err
}

func TestRootWithoutTerminalShowsHelp(t *testing.T) {
	isolatedConfig(t)
	out, _, err := runCommand()
	if err != nil || !strings.Contains(out, "Manage proxy kernels") {
		t.Fatalf("root help: %q %v", out, err)
	}
}

func TestShellCleanupIgnoresBrokenState(t *testing.T) {
	dir := isolatedConfig(t)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"config.yaml", "subscriptions.yaml"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("broken: ["), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	for _, args := range [][]string{{"off"}, {"env", "--unset"}} {
		out, _, err := runCommand(args...)
		if err != nil {
			t.Fatalf("%v: %v", args, err)
		}
		if !strings.Contains(out, "unset HTTP_PROXY\n") || strings.Contains(out, "export ") {
			t.Fatalf("%v: %q", args, out)
		}
	}
}

func TestQueriesLoadOnlyTheirOwnState(t *testing.T) {
	dir := isolatedConfig(t)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	subPath := filepath.Join(dir, "subscriptions.yaml")
	if err := os.WriteFile(subPath, []byte("broken: ["), 0o600); err != nil {
		t.Fatal(err)
	}
	if out, _, err := runCommand("list"); err != nil || !strings.Contains(out, "mihomo") {
		t.Fatalf("kernel list: %q %v", out, err)
	}
	if err := os.Remove(subPath); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "config.yaml"), []byte("broken: ["), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := runCommand("sub", "list"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "profiles")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("query created profiles directory: %v", err)
	}
}

func TestCommandArgumentsAreValidatedBeforeLoading(t *testing.T) {
	isolatedConfig(t)
	cases := [][]string{
		{"status", "extra"}, {"on", "extra"}, {"off", "extra"}, {"env", "extra"},
		{"sub", "typo"}, {"sub", "list", "extra"}, {"start", "extra"},
		{"use"}, {"install", "a", "b"},
		{"kernel", "install", "mihomo"}, {"kernel", "list"}, {"kernel", "use", "mihomo"},
		{"sub", "delete", "a", "b"}, {"sub", "use", "a", "--interactive"},
	}
	for _, args := range cases {
		if _, _, err := runCommand(args...); err == nil {
			t.Errorf("%v accepted invalid arguments", args)
		} else if strings.Contains(err.Error(), "no kernel") {
			t.Errorf("%v loaded runtime before validation: %v", args, err)
		}
	}
}

func TestCommandInstancesDoNotShareFlags(t *testing.T) {
	isolatedConfig(t)
	if _, _, err := runCommand("env", "--unset"); err != nil {
		t.Fatal(err)
	}
	if out, _, err := runCommand("env"); err == nil || out != "" {
		t.Fatalf("unset leaked into another command tree: %q %v", out, err)
	}
}

func TestAddDoesNotSelectOrContactService(t *testing.T) {
	isolatedConfig(t)
	binary := filepath.Join(t.TempDir(), "kernel")
	if err := os.WriteFile(binary, []byte("#!/bin/sh\nexit 0\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	runtimeFile := filepath.Join(t.TempDir(), "config.yaml")
	if err := config.Save(&config.Config{Use: "mihomo", Kernels: []config.KernelConfig{{Name: "mihomo", Bin: binary, ConfigFile: runtimeFile}}}); err != nil {
		t.Fatal(err)
	}
	src := filepath.Join(t.TempDir(), "source.yaml")
	if err := os.WriteFile(src, []byte("proxies: []\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	names := []string{"one", "two", "work profile", "-work's $(exit 9); `exit 8`"}
	for _, name := range names {
		out, _, err := runCommand("sub", "add", "file://"+src, "--name", name)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(out, "No profile is selected.\nNext: proxyctl sub use ") || !strings.HasSuffix(out, "Then: proxyctl on\n") {
			t.Fatalf("missing next steps: %q", out)
		}
		for _, shell := range []string{"bash", "zsh", "fish"} {
			path, err := exec.LookPath(shell)
			if err != nil {
				t.Logf("skipping hint check for unavailable %s", shell)
				continue
			}
			args := strings.Split(strings.Split(out, "Next: proxyctl sub use ")[1], "\n")[0]
			// Parse the suggested arguments in a real shell without invoking proxyctl.
			command := exec.Command(path, "-c", "printf '%s\\n' "+args)
			got, err := command.CombinedOutput()
			want := name + "\n"
			if strings.HasPrefix(name, "-") {
				want = "--\n" + want
			}
			if err != nil || string(got) != want {
				t.Fatalf("%s hint for %q: %q, %v", shell, name, got, err)
			}
		}
	}
	cfg, err := config.LoadSubscriptionConfig()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Use != "" || len(cfg.Profiles) != len(names) {
		t.Fatalf("add changed selection or leaked state: %+v", cfg)
	}
	if _, err := os.Stat(runtimeFile); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("add wrote runtime configuration: %v", err)
	}
	cfg.Use = "one"
	if err := config.SaveSubscriptionConfig(cfg); err != nil {
		t.Fatal(err)
	}
	out, _, err := runCommand("sub", "add", "file://"+src, "--name", "another")
	if err != nil || out != "profile \"another\" added\n" {
		t.Fatalf("add with existing selection: %q, %v", out, err)
	}
	cfg, err = config.LoadSubscriptionConfig()
	if err != nil || cfg.Use != "one" {
		t.Fatalf("add changed existing selection: %+v, %v", cfg, err)
	}
	out, _, err = runCommand("sub", "add", "file://"+src, "--name", "another")
	if err == nil || out != "" {
		t.Fatalf("failed add printed success or next steps: %q, %v", out, err)
	}
}

func TestPOSIXWrapperOnlyEvaluatesSuccessfulBareActions(t *testing.T) {
	isolatedConfig(t)
	for _, shell := range []string{"bash", "zsh"} {
		t.Run(shell, func(t *testing.T) {
			path, err := exec.LookPath(shell)
			if err != nil {
				t.Skip(err)
			}
			dir := t.TempDir()
			fake := `#!/bin/sh
case "$*" in
 on) printf '%s\n' 'export REVIEW_PROXY=enabled';;
 off) printf '%s\n' 'export REVIEW_PROXY=wrong'; exit 7;;
 *) printf '%s\n' 'help is not shell code';;
esac
`
			if err := os.WriteFile(filepath.Join(dir, "proxyctl"), []byte(fake), 0o700); err != nil {
				t.Fatal(err)
			}
			script := posixScript(shell) + `
proxyctl on || exit 1
[ "$REVIEW_PROXY" = enabled ] || exit 2
proxyctl off
[ "$?" = 7 ] || exit 3
[ "$REVIEW_PROXY" = enabled ] || exit 4
proxyctl on --help >/dev/null || exit 5
[ "$REVIEW_PROXY" = enabled ] || exit 6
`
			command := exec.Command(path, "-c", script)
			command.Env = append(os.Environ(), "PATH="+dir+":"+os.Getenv("PATH"))
			if out, err := command.CombinedOutput(); err != nil {
				t.Fatalf("wrapper: %s %v", out, err)
			}
		})
	}
}

func TestFishWrapperOnlyEvaluatesSuccessfulBareActions(t *testing.T) {
	isolatedConfig(t)
	path, err := exec.LookPath("fish")
	if err != nil {
		t.Skip(err)
	}
	dir := t.TempDir()
	fake := `#!/bin/sh
case "$*" in
 on) printf '%s\n' 'set -gx REVIEW_PROXY enabled';;
 off) printf '%s\n' 'set -gx REVIEW_PROXY wrong'; exit 7;;
 *) printf '%s\n' 'help is not shell code';;
esac
`
	if err := os.WriteFile(filepath.Join(dir, "proxyctl"), []byte(fake), 0o700); err != nil {
		t.Fatal(err)
	}
	script := fishScript() + `
proxyctl on; or exit 1
test "$REVIEW_PROXY" = enabled; or exit 2
proxyctl off
test $status -eq 7; or exit 3
test "$REVIEW_PROXY" = enabled; or exit 4
proxyctl on --help >/dev/null; or exit 5
test "$REVIEW_PROXY" = enabled; or exit 6
`
	command := exec.Command(path, "--no-config", "-c", script)
	command.Env = append(os.Environ(), "PATH="+dir+":"+os.Getenv("PATH"))
	if out, err := command.CombinedOutput(); err != nil {
		t.Fatalf("wrapper: %s %v", out, err)
	}
}

func TestSubscriptionAliases(t *testing.T) {
	for _, args := range [][]string{
		{"subscription", "list"}, {"sub", "ls"}, {"subscription", "ls"},
		{"sub", "del", "work"}, {"subscription", "del", "work"},
	} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			isolatedConfig(t)
			file := filepath.Join(t.TempDir(), "work.yaml")
			if err := os.WriteFile(file, []byte("proxies: []\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			cfg := &config.SubscriptionConfig{Profiles: []config.Profile{{Name: "work", URL: "file://" + file, File: file}}}
			if err := config.SaveSubscriptionConfig(cfg); err != nil {
				t.Fatal(err)
			}
			out, _, err := runCommand(args...)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(out, "work") {
				t.Fatalf("missing profile in output: %q", out)
			}
			if args[1] == "del" {
				saved, err := config.LoadSubscriptionConfig()
				if err != nil {
					t.Fatal(err)
				}
				if len(saved.Profiles) != 0 {
					t.Fatal("alias did not delete the profile")
				}
				if _, err := os.Stat(file); !errors.Is(err, os.ErrNotExist) {
					t.Fatalf("profile file remains: %v", err)
				}
			}
		})
	}
}
