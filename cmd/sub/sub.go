package sub

import (
	"github.com/nelvko/proxyctl/cmd"
	"github.com/nelvko/proxyctl/internal/kernel"
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
		return profile.TUI()
	},
}

var (
	k kernel.Kernel
)

var interactive bool

func init() {
	subCmd.PersistentFlags().BoolVarP(&interactive, "interactive", "i", false, "enable interactive TUI mode")
	cmd.RootCmd.AddCommand(subCmd)
}
