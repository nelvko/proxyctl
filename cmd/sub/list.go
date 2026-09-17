package sub

import (
	"github.com/nelvko/proxyctl/internal/app"
	"github.com/nelvko/proxyctl/internal/redact"
	"github.com/spf13/cobra"
)

func newListCmd() *cobra.Command {
	return &cobra.Command{
		Aliases: []string{"ls"},
		Use:     "list", Short: "List subscription profiles", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			svc, err := app.LoadProfileMetadata(cmd.Context())
			if err != nil {
				return err
			}
			for _, p := range svc.List() {
				marker := " "
				if p.Name == svc.SelectedName() {
					marker = "*"
				}
				cmd.Printf("%s %s\t%s\n", marker, p.Name, redact.URL(p.URL))
			}
			return nil
		},
	}
}
