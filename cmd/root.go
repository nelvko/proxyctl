package cmd

import (
	"errors"
	"fmt"
	"os"

	"charm.land/huh/v2"
	"github.com/nelvko/proxyctl/internal/bootstrap"
	"github.com/nelvko/proxyctl/internal/config"
	pkernel "github.com/nelvko/proxyctl/internal/kernel"
	"github.com/nelvko/proxyctl/internal/log"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

var runtimeCtx *bootstrap.Runtime

// RootCmd represents the base command when called without any subcommands
var RootCmd = &cobra.Command{
	Use:   config.AppName,
	Short: "Go proxy elegantly",
}

// ManageGroupID returns the group ID used by management command groups.
func ManageGroupID() string {
	return manageGroup.ID
}

// Runtime returns the loaded runtime, or nil for commands without one.
func Runtime() *bootstrap.Runtime {
	return runtimeCtx
}

// PickKernel prompts for an installable kernel.
func PickKernel() (string, error) {
	if !isInteractive() {
		return "", fmt.Errorf("no terminal available, run `proxyctl kernel install <name>` instead")
	}
	ready := pkernel.ReadyNames()
	if len(ready) == 0 {
		return "", errors.New("no installable kernel")
	}
	options := make([]huh.Option[string], 0, len(ready))
	for _, name := range ready {
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

// bootstrapKernel offers an interactive picker when no kernel is installed,
// installs the pick and loads the runtime with it. Under the shell wrapper
// (eval'd stdout) an interactive TUI would be captured and lost, so it
// degrades to a plain hint there.
func bootstrapKernel() error {
	if !isInteractive() || os.Getenv("PROXYCTL_WRAPPED") != "" {
		return fmt.Errorf("no kernel installed, run `proxyctl kernel install` first")
	}
	name, err := PickKernel()
	if err != nil {
		return err
	}
	if err := pkernel.Install(name); err != nil {
		return err
	}
	rt, err := bootstrap.LoadRuntime()
	if err != nil {
		return err
	}
	runtimeCtx = rt
	log.Ok(fmt.Sprintf("kernel %q installed", name))
	return nil
}

func isInteractive() bool {
	return term.IsTerminal(int(os.Stdin.Fd()))
}

var manageGroup = &cobra.Group{ID: "manage", Title: "Management Commands"}

func init() {
	RootCmd.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		if !needsRuntime(RootCmd, cmd) {
			return nil
		}
		rt, err := bootstrap.LoadRuntime()
		if errors.Is(err, config.ErrNoKernel) {
			return bootstrapKernel()
		}
		if err != nil {
			return err
		}
		runtimeCtx = rt
		return nil
	}
	RootCmd.AddGroup(manageGroup)
}
