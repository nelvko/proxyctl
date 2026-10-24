package cmd

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nelvko/proxyctl/internal/config"
	"github.com/nelvko/proxyctl/internal/env"
)

func TestOffStopWithoutSelectedKernel(t *testing.T) {
	isolatedConfig(t)
	output, stderr, err := runCommand("off", "--stop")
	if err != nil || !strings.Contains(output, "unset HTTP_PROXY\n") || strings.Contains(output, "export ") || !strings.Contains(stderr, "nothing to stop") {
		t.Fatalf("off --stop: output=%q stderr=%q err=%v", output, stderr, err)
	}
}

func TestOffStopRetainsCleanupOnFailure(t *testing.T) {
	for _, test := range []string{"config", "kernel"} {
		t.Run(test, func(t *testing.T) {
			dir := isolatedConfig(t)
			if test == "config" {
				if err := os.MkdirAll(dir, 0o700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(dir, "config.yaml"), []byte("broken: ["), 0o600); err != nil {
					t.Fatal(err)
				}
			} else if err := config.Save(&config.Config{Use: "clash", Kernels: []config.KernelConfig{{Name: "clash"}}}); err != nil {
				t.Fatal(err)
			}
			output, _, err := runCommand("off", "--stop")
			if err == nil || !strings.Contains(output, "unset HTTP_PROXY\n") || strings.Contains(output, "export ") {
				t.Fatalf("stop failure lost error or cleanup: output=%q err=%v", output, err)
			}
			if _, _, err := runCommand("off", "--stop=false"); err != nil {
				t.Fatalf("--stop=false loaded broken runtime state: %v", err)
			}
		})
	}
}

func TestOffStopHelpDoesNotRunCleanup(t *testing.T) {
	isolatedConfig(t)
	output, _, err := runCommand("off", "--stop", "--help")
	if err != nil || !strings.Contains(output, "--stop") || strings.Contains(output, "unset HTTP_PROXY") {
		t.Fatalf("help executed off: output=%q err=%v", output, err)
	}
}

func TestShellWrappersApplyOffStopCleanupAndPreserveFailure(t *testing.T) {
	isolatedConfig(t)
	for _, shell := range []string{"bash", "zsh", "fish"} {
		t.Run(shell, func(t *testing.T) {
			path, err := exec.LookPath(shell)
			if err != nil {
				t.Skip(err)
			}
			dir := t.TempDir()
			cleanup := strings.Join(env.ProxyEnv{}.Unset(shell), "; ")
			unexpected := "export REVIEW_PROXY=wrong"
			if shell == "fish" {
				unexpected = "set -gx REVIEW_PROXY wrong"
			}
			fake := "#!/bin/sh\ncase \"$*\" in\n" +
				" 'off --stop'|'off --stop=1'|'off --stop=t'|'off --stop=T'|'off --stop=true'|'off --stop=TRUE'|'off --stop=True'|'off --stop --'|'off --stop=false --stop') printf '%s\\n' '" + cleanup + "'; exit 7;;\n" +
				" 'off --stop=0'|'off --stop=f'|'off --stop=F'|'off --stop=false'|'off --stop=FALSE'|'off --stop=False'|'off --stop --stop=false') printf '%s\\n' '" + cleanup + "';;\n" +
				" *) printf '%s\\n' '" + unexpected + "';;\nesac\n"
			if err := os.WriteFile(filepath.Join(dir, "proxyctl"), []byte(fake), 0o700); err != nil {
				t.Fatal(err)
			}
			script := posixScript(shell) + `
for option in --stop --stop=1 --stop=t --stop=T --stop=true --stop=TRUE --stop=True; do
    export HTTP_PROXY=enabled http_proxy=enabled
    proxyctl off "$option"
    [ "$?" = 7 ] || exit 1
    [ -z "${HTTP_PROXY+x}${http_proxy+x}" ] || exit 2
done
export HTTP_PROXY=enabled http_proxy=enabled
proxyctl off --stop=false --stop
[ "$?" = 7 ] || exit 3
[ -z "${HTTP_PROXY+x}${http_proxy+x}" ] || exit 4
export HTTP_PROXY=enabled http_proxy=enabled
proxyctl off --stop --
[ "$?" = 7 ] || exit 5
[ -z "${HTTP_PROXY+x}${http_proxy+x}" ] || exit 6
for option in --stop=0 --stop=f --stop=F --stop=false --stop=FALSE --stop=False; do
    proxyctl off "$option" || exit 5
done
proxyctl off --stop --stop=false || exit 6
export REVIEW_PROXY=enabled
proxyctl off --stop --help >/dev/null || exit 7
proxyctl off --help --stop >/dev/null || exit 8
proxyctl off --stop invalid >/dev/null || exit 9
[ "$REVIEW_PROXY" = enabled ] || exit 10
`
			args := []string{"-c", script}
			if shell == "fish" {
				script = fishScript() + `
for option in --stop --stop=1 --stop=t --stop=T --stop=true --stop=TRUE --stop=True
    set -gx HTTP_PROXY enabled
    set -gx http_proxy enabled
    proxyctl off $option
    test $status -eq 7; or exit 1
    not set -q HTTP_PROXY; or exit 2
    not set -q http_proxy; or exit 3
end
set -gx HTTP_PROXY enabled
set -gx http_proxy enabled
proxyctl off --stop=false --stop
test $status -eq 7; or exit 4
not set -q HTTP_PROXY; or exit 5
not set -q http_proxy; or exit 6
set -gx HTTP_PROXY enabled
set -gx http_proxy enabled
proxyctl off --stop --
test $status -eq 7; or exit 7
not set -q HTTP_PROXY; or exit 8
not set -q http_proxy; or exit 9
for option in --stop=0 --stop=f --stop=F --stop=false --stop=FALSE --stop=False
    proxyctl off $option; or exit 7
end
proxyctl off --stop --stop=false; or exit 8
set -gx REVIEW_PROXY enabled
proxyctl off --stop --help >/dev/null; or exit 9
proxyctl off --help --stop >/dev/null; or exit 10
proxyctl off --stop invalid >/dev/null; or exit 11
test "$REVIEW_PROXY" = enabled; or exit 12
`
				args = []string{"--no-config", "-c", script}
			}
			command := exec.Command(path, args...)
			command.Env = append(os.Environ(), "PATH="+dir+":"+os.Getenv("PATH"))
			if output, err := command.CombinedOutput(); err != nil {
				t.Fatalf("off --stop integration: %s %v", output, err)
			}
		})
	}
}
