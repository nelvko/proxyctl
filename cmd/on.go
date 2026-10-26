package cmd

import (
	"github.com/nelvko/proxyctl/internal/env"
	"github.com/nelvko/proxyctl/internal/kernel"
	"github.com/nelvko/proxyctl/internal/redact"
	"github.com/spf13/cobra"
)

// onCmd represents the on command
func newOnCmd() *cobra.Command {
	command := &cobra.Command{
		Use:   "on",
		Short: "Enable the proxy environment in the current shell",
		Long: `Enable the proxy environment in the current shell.

Starts the kernel service if needed, then prints shell statements
that apply the proxy environment to stdout. With the shell
integration (eval "$(proxyctl init bash)"), they apply to the
current shell; the kernel config decides the actual address.
A subscription is optional. The initial configuration uses a local mixed
proxy with direct outbound connections.`,
		Args: cobra.NoArgs,
		RunE: mutation(func(cmd *cobra.Command, args []string) error {
			if err := requireShellIntegration(cmd.Name()); err != nil {
				return err
			}
			a, err := loadApp(cmd)
			if err != nil {
				return err
			}
			endpoints, err := a.StartProxy(cmd.Context())
			if err != nil {
				return err
			}
			e := env.ForProxy(endpoints.HTTP, endpoints.SOCKS)

			printShell(cmd, e.Export(env.Shell()))
			note := ""
			if k, err := a.Kernel(); err == nil {
				if initial, err := kernel.IsDefaultConfig(k); err == nil && initial {
					note = " (direct; default configuration)"
				}
			}
			cmd.PrintErrf("proxy on: %s%s\n", redact.URL(firstNonEmpty(e.HTTP, e.All)), note)
			return nil
		}),
	}
	return command
}

// printShell writes shell statements to stdout — the only thing allowed
// there for on/off commands.
func printShell(cmd *cobra.Command, lines []string) {
	for _, line := range lines {
		cmd.Println(line)
	}
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}
