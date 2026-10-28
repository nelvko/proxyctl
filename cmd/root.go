package cmd

import (
	"fmt"
	"os"

	kernelcmd "github.com/nelvko/proxyctl/cmd/kernel"
	"github.com/nelvko/proxyctl/cmd/sub"
	"github.com/nelvko/proxyctl/internal/config"
	"github.com/nelvko/proxyctl/internal/ui"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

// NewRootCmd builds an independent command tree. Dependencies are loaded
// by the operation that needs them; help and shell cleanup need no state.
func NewRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   config.AppName,
		Short: "Manage proxy kernels, subscriptions and the current shell",
		Long: `Manage proxy kernels as user services, refresh subscription profiles,
and enable the proxy in individual shells. Service operations affect all
shells. The on command also starts the service when needed; off clears the
calling shell's proxy environment and can also stop the service with --stop.`,
		Example: `  proxyctl install mihomo
  proxyctl sub add <url> --name work --use
  eval "$(proxyctl init zsh)"
  proxyctl on
  proxyctl off`,
		SilenceUsage:  true,
		SilenceErrors: true,
		Args:          cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			out, fileOutput := cmd.OutOrStdout().(*os.File)
			if !term.IsTerminal(int(os.Stdin.Fd())) || !fileOutput || !term.IsTerminal(int(out.Fd())) {
				return cmd.Help()
			}
			return ui.RunDashboard(cmd.Context())
		},
	}
	root.SetOut(os.Stdout)
	root.SetErr(os.Stderr)
	root.AddGroup(
		&cobra.Group{ID: "kernel", Title: "Kernel Management"},
		&cobra.Group{ID: "service", Title: "Service Control"},
		&cobra.Group{ID: "shell", Title: "Shell Integration"},
	)
	root.AddCommand(kernelcmd.NewCommands()...)
	root.AddCommand(sub.NewCommand(), newOnCmd(), newOffCmd(), newStatusCmd(), newEnvCmd(), newInitCmd())
	wrapCommandErrors(root)
	return root
}

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
