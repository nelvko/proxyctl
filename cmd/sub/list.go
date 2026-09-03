package sub

import (
	"fmt"

	"github.com/nelvko/proxyctl/internal/subscription"
	"github.com/spf13/cobra"
)

// lsCmd represents the ls command
var lsCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List subscription profiles",
	Long:    ``,
	RunE: func(cmd *cobra.Command, args []string) error {
		profiles, err := profiles()
		if err != nil {
			return err
		}
		if interactive {
			return subscription.TUI(profiles)
		}
		current := profiles.ActiveName()
		for _, p := range profiles.List() {
			marker := " "
			if p.Name == current {
				marker = "*"
			}
			fmt.Printf("%s %s\t%s\n", marker, p.Name, p.URL)
		}
		return nil
	},
}

func init() {
	subCmd.AddCommand(lsCmd)
}
