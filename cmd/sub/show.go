package sub

import (
	"errors"

	"github.com/nelvko/proxyctl/internal/app"
	"github.com/nelvko/proxyctl/internal/ui"
	"github.com/spf13/cobra"
)

func newShowCmd() *cobra.Command {
	var showURL bool
	cmd := &cobra.Command{
		Use: "show [name]", Short: "Show subscription metadata; defaults to the selected profile",
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			svc, err := app.LoadProfileMetadata(cmd.Context())
			if err != nil {
				return err
			}
			name := svc.SelectedName()
			if len(args) > 0 {
				name = args[0]
			}
			if name == "" {
				return errors.New("no profile is selected; specify a name from proxyctl sub list")
			}
			p, err := svc.Get(name)
			if err != nil {
				return err
			}
			cmd.Print(ui.ProfileDetails(p, svc.SelectedName(), showURL))
			return nil
		},
	}
	cmd.Flags().BoolVar(&showURL, "show-url", false, "Show the full source URL, including credentials and tokens")
	return cmd
}
