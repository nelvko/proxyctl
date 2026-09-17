package sub

import (
	"github.com/nelvko/proxyctl/internal/app"
	"github.com/nelvko/proxyctl/internal/ui"
	"github.com/spf13/cobra"
)

func newRemoveCmd() *cobra.Command {
	var force bool
	cmd := &cobra.Command{
		Aliases: []string{"rm", "delete", "del"},
		Use:     "remove [name]", Short: "Remove a subscription registration and its cached file",
		Long: `Remove a locally managed profile; the remote or file URL source is unchanged. Without a name, select one interactively.
Removing the selected profile requires --force, which also stops the selected
kernel and removes its applied configuration. Shell variables are unchanged;
run proxyctl off in shells that no longer need the proxy.`,
		Args: cobra.MaximumNArgs(1),
		RunE: mutation(func(cmd *cobra.Command, args []string) error {
			svc, err := app.LoadProfileMetadata(cmd.Context())
			if err != nil {
				return err
			}
			name, err := selectProfile(cmd, args, svc)
			if err != nil {
				return cancelled(cmd, err)
			}
			if err := app.RemoveProfile(cmd.Context(), name, force, ui.Reporter(cmd.ErrOrStderr())); err != nil {
				return err
			}
			cmd.Printf("profile %q removed\n", name)
			return nil
		}),
	}
	cmd.Flags().BoolVarP(&force, "force", "f", false, "Allow removing the selected profile; stop the kernel and remove its applied config")
	return cmd
}
