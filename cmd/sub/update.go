package sub

import (
	"errors"

	"github.com/nelvko/proxyctl/internal/app"
	"github.com/nelvko/proxyctl/internal/ui"
	"github.com/spf13/cobra"
)

func newUpdateCmd() *cobra.Command {
	var all, keepFailed bool
	var download downloadFlags
	cmd := &cobra.Command{
		Use: "update [name...]", Short: "Refresh the selected or explicitly named subscription profiles",
		Long: `Refresh named profiles, or the selected profile when no names are supplied.
Use --all to update every profile compatible with the selected kernel. Names and --all are mutually exclusive.
Validated downloads replace local edits. The selected profile is applied when
necessary; running kernels restart and stopped kernels remain stopped.
Download flags override saved settings for this invocation only, including --all.
file:// sources are read again from the original file path.`,
		Args: func(cmd *cobra.Command, args []string) error {
			if all && len(args) > 0 {
				return errors.New("--all cannot be combined with profile names")
			}
			return nil
		},
		RunE: mutation(func(cmd *cobra.Command, args []string) error {
			options, err := download.options(cmd)
			if err != nil {
				return err
			}
			svc, err := app.LoadRuntimeProfiles(cmd.Context(), ui.Reporter(cmd.ErrOrStderr()))
			if err != nil {
				return err
			}
			svc.KeepFailed = keepFailed
			if len(svc.List()) == 0 {
				return errors.New("no profiles; run proxyctl sub add <url> first")
			}
			if all && len(svc.Compatible()) == 0 {
				return errors.New("no profiles are compatible with the selected kernel")
			}
			if len(args) == 0 && !all {
				if svc.SelectedName() == "" {
					return errors.New("no profile is selected; specify a name or use --all")
				}
				args = []string{svc.SelectedName()}
			}
			results, err := svc.Update(options, args...)
			for _, result := range results {
				cmd.Printf("profile %q: %s\n", result.Name, ui.ProfileUpdate(result))
			}
			return err
		}),
	}
	cmd.Flags().BoolVar(&all, "all", false, "Update every profile compatible with the selected kernel")
	cmd.Flags().BoolVar(&keepFailed, "keep-failed", false, "Keep complete downloads that fail validation in diagnostics/; remove them manually after inspection")
	download.bind(cmd, true)
	return cmd
}
