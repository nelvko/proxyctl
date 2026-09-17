package sub

import (
	"github.com/nelvko/proxyctl/internal/app"
	"github.com/nelvko/proxyctl/internal/ui"
	"github.com/spf13/cobra"
)

func newEditCmd() *cobra.Command {
	var editor string
	var apply bool
	cmd := &cobra.Command{
		Use: "edit [name]", Short: "Edit and validate a local subscription profile",
		Long: `Edit a local profile using --editor or $EDITOR. Without a name, select one
interactively. Valid edits are saved but not applied unless --apply is given.
An update from the source URL replaces local edits.`,
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
			if err := ui.Edit(cmd.Context(), svc, name, editor, cmd.InOrStdin(), cmd.OutOrStdout(), cmd.ErrOrStderr()); err != nil {
				return err
			}
			if apply {
				result, err := svc.Use(name)
				if err != nil {
					return err
				}
				cmd.Printf("profile %q edited and selected; %s\n", name, ui.Application(result))
			} else {
				cmd.Printf("profile %q edited; run `proxyctl sub use %s` to apply\n", name, profileNameArg(name))
			}
			return nil
		}),
	}
	cmd.Flags().StringVarP(&editor, "editor", "e", "", "Editor command")
	cmd.Flags().BoolVar(&apply, "apply", false, "Select and apply the edited profile, preserving service running state")
	return cmd
}
