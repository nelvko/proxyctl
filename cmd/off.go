package cmd

import (
	"github.com/nelvko/proxyctl/internal/app"
	"github.com/nelvko/proxyctl/internal/env"
	"github.com/spf13/cobra"
)

// offCmd represents the off command
func newOffCmd() *cobra.Command {
	var stop bool
	command := &cobra.Command{
		Use:   "off",
		Short: "Disable the proxy environment in the current shell",
		Long: `Disable the proxy environment in the current shell.

Print shell statements that remove the proxy environment from the current
shell. The shared kernel service keeps running by default.

Use --stop to also stop the selected kernel's shared service. This affects
other shells using that service, without clearing their environment variables.
The current shell is cleared even if stopping fails; the command still returns
the stop error.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := requireShellIntegration(cmd.Name()); err != nil {
				return err
			}
			printShell(cmd, env.ProxyEnv{}.Unset(env.Shell()))
			cmd.PrintErrln("proxy off")
			if !stop {
				return nil
			}
			return app.WithStateLock(cmd.Context(), func() error {
				a, err := loadApp(cmd)
				if err != nil {
					return err
				}
				selected := a.Config.SelectedKernel()
				if selected == nil {
					cmd.PrintErrln("no kernel selected; nothing to stop")
					return nil
				}
				k, err := a.Kernel()
				if err != nil {
					return err
				}
				running, err := k.IsActive()
				if err != nil {
					return err
				}
				if !running {
					cmd.PrintErrf("kernel %q: already stopped\n", selected.Name)
					return nil
				}
				if err := a.ControlKernel(cmd.Context(), "stop"); err != nil {
					return err
				}
				cmd.PrintErrf("kernel %q: stopped\n", selected.Name)
				return nil
			})
		},
	}
	command.Flags().BoolVar(&stop, "stop", false, "Also stop the selected kernel's shared service")
	return command
}
