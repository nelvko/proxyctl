package sub

import (
	"github.com/spf13/cobra"
)

// lsCmd represents the ls command
var lsCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List subscription profiles",
	Long: ``,
	RunE: func(cmd *cobra.Command, args []string) error {
		if interactive {
			return tui()
		}
		// todo list profiles
		return nil
	},
}

func init() {
	subCmd.AddCommand(lsCmd)
}
