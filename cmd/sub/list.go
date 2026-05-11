package sub

import (
	"github.com/nelvko/proxyctl/internal/profile"
	"github.com/spf13/cobra"
)

// lsCmd represents the ls command
var lsCmd = &cobra.Command{
	Use:     "list [name]",
	Aliases: []string{"ls"},
	Short:   "List subscription profiles",
	Long:    ``,
	RunE: func(cmd *cobra.Command, args []string) error {
		if interactive {
			return profile.TUI()
		}
		// todo list profiles
		return nil
	},
}

func init() {
	subCmd.AddCommand(lsCmd)
}
