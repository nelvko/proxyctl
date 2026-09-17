package sub

import (
	"github.com/nelvko/proxyctl/internal/app"
	"github.com/nelvko/proxyctl/internal/ui"
	"github.com/spf13/cobra"
)

func newUseCmd() *cobra.Command {
	return &cobra.Command{
		Use: "use [name]", Short: "Select and apply a subscription profile",
		Long: `Select a profile and apply its configuration. A running kernel is restarted;
a stopped kernel stays stopped. Run proxyctl on to enable the proxy in your shell.
Without a name, select a profile interactively.`,
		Args: cobra.MaximumNArgs(1),
		RunE: mutation(func(cmd *cobra.Command, args []string) error {
			svc, err := app.LoadRuntimeProfiles(cmd.Context(), ui.Reporter(cmd.ErrOrStderr()))
			if err != nil {
				return err
			}
			name, err := selectProfile(cmd, args, svc)
			if err != nil {
				return cancelled(cmd, err)
			}
			result, err := svc.Use(name)
			if err != nil {
				return err
			}
			cmd.Printf("profile %q selected; %s\n", name, ui.Application(result))
			return nil
		}),
	}
}
