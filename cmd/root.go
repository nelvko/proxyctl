package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"

	"charm.land/huh/v2"
	"github.com/nelvko/proxyctl/internal/app"
	"github.com/nelvko/proxyctl/internal/config"
	pkernel "github.com/nelvko/proxyctl/internal/kernel"
	"github.com/nelvko/proxyctl/internal/ui"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

var appState *app.App

// RootCmd represents the base command when called without any subcommands
var RootCmd = &cobra.Command{
	Use:   config.AppName,
	Short: "Terminal-native proxy manager",
	Long: `proxyctl installs and manages proxy kernels (mihomo today, more on
the way) as rootless user services, updates subscriptions on demand,
and wires the proxy into your shell.`,
	Example: `  # shell integration (once, in your shell rc):
  eval "$(proxyctl init zsh)"

  # enable / disable the proxy in the current shell:
  proxyctl on
  proxyctl off`,
}

// Command-group rules (stable under growth):
//   - default COMMANDS section: session actions (on/off/status) and domain
//     entry points (sub, and future node/tun/ui) — each new domain adds
//     exactly one line here, never a new group;
//   - KernelGroup: verbs managing the default object's lifecycle;
//   - ShellGroup: one-time shell integration and its helpers.
var (
	KernelGroup = &cobra.Group{ID: "kernel", Title: "Kernel Commands"}
	ShellGroup  = &cobra.Group{ID: "shell", Title: "Shell Integration"}
)

// App returns the loaded application for runtime-gated commands.
func App() *app.App {
	return appState
}

// PickKernel prompts for an installable kernel.
func PickKernel() (string, error) {
	if !isInteractive() {
		return "", fmt.Errorf("no terminal available, run `proxyctl kernel install <name>` instead")
	}
	implemented := pkernel.ImplementedNames()
	if len(implemented) == 0 {
		return "", errors.New("no installable kernel")
	}
	options := make([]huh.Option[string], 0, len(implemented))
	for _, name := range implemented {
		options = append(options, huh.NewOption(string(pkernel.FormatOf(name))+" config · "+name, name))
	}

	var name string
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("No kernel installed — pick one to download").
				Options(options...).
				Value(&name),
		),
	)
	if err := form.Run(); err != nil {
		return "", err
	}
	return name, nil
}

// needsRuntime reports whether the command operates on the active kernel.
// Commands annotated skipRuntime (the kernel group), the completion and
// help trees, shell-completion internals, and a bare invocation run
// without one.
func needsRuntime(root, cmd *cobra.Command) bool {
	switch cmd.Name() {
	// help, man (injected by fang), shell integration and completion
	// internals never need an active kernel.
	case "help", "man", "init", "__complete", "__completeNoDesc":
		return false
	}
	for c := cmd; c != nil; c = c.Parent() {
		if c.Name() == "completion" {
			return false
		}
		if c.Annotations["skipRuntime"] == "true" {
			return false
		}
	}
	return cmd != root
}

// ensureKernel offers an interactive picker when no kernel is installed,
// installs the pick and continues with the loaded app. Under the shell
// wrapper (eval'd stdout) an interactive TUI would be captured and lost, so
// it degrades to a plain hint there.
func ensureKernel(ctx context.Context, a *app.App) error {
	if !isInteractive() || os.Getenv("PROXYCTL_WRAPPED") != "" {
		return fmt.Errorf("no kernel installed, run `proxyctl kernel install` first")
	}
	name, err := PickKernel()
	if err != nil {
		return err
	}
	if err := a.InstallKernel(ctx, name); err != nil {
		return err
	}
	if _, err := a.Kernel(); err != nil {
		return err
	}
	appState = a
	ui.Ok(fmt.Sprintf("kernel %q installed", name))
	return nil
}

func isInteractive() bool {
	return term.IsTerminal(int(os.Stdin.Fd()))
}

// requireShellIntegration guards the eval-fed commands: when a human runs
// `proxyctl on` bare (no wrapper, stdout a terminal) the export statements
// would just scroll by without touching the environment — point at the
// integration instead, like conda does for `conda activate`.
func requireShellIntegration(cmdName string) error {
	if os.Getenv("PROXYCTL_WRAPPED") != "" {
		return nil
	}
	if term.IsTerminal(int(os.Stdout.Fd())) {
		shell := defaultShell()
		return fmt.Errorf("load the shell integration first:\n  %s\nthen re-run: proxyctl %s", integrationCmd(shell), cmdName)
	}
	return nil
}

// integrationCmd is the load command for the shell, as printed in guidance.
func integrationCmd(shell string) string {
	if shell == "fish" {
		return "proxyctl init fish | source"
	}
	return fmt.Sprintf("eval \"$(proxyctl init %s)\"", shell)
}

func init() {
	RootCmd.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		if !needsRuntime(RootCmd, cmd) {
			return nil
		}
		a, err := app.Load()
		if err != nil {
			return err
		}
		if _, err := a.Kernel(); err != nil {
			if errors.Is(err, config.ErrNoKernel) {
				return ensureKernel(cmd.Context(), a)
			}
			return err
		}
		appState = a
		return nil
	}
	RootCmd.AddGroup(KernelGroup, ShellGroup)
}
