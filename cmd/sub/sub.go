package sub

import (
	rootcmd "github.com/nelvko/proxyctl/cmd"
	"github.com/nelvko/proxyctl/internal/profile"
	"github.com/spf13/cobra"
)

// subCmd represents the sub command
var subCmd = &cobra.Command{
	Use:   "sub",
	Short: "Manage subscription profiles",
	Long: `Manage subscription profiles.

Run without a subcommand to open the TUI.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return profile.TUI(profiles())
	},
}

var interactive bool

// profiles returns the subscription service bound to the active kernel.
func profiles() *profile.Service {
	svc, err := rootcmd.App().Profiles()
	if err != nil {
		return nil
	}
	return svc
}

func init() {
	subCmd.PersistentFlags().BoolVarP(&interactive, "interactive", "i", false, "enable interactive TUI mode")
	rootcmd.RootCmd.AddCommand(subCmd)
}
