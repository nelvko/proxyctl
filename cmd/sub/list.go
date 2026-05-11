package sub

import (
	"fmt"

	rootcmd "github.com/nelvko/proxyctl/cmd"
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
		profiles := rootcmd.Runtime().Profiles
		if interactive {
			return profile.TUI(profiles)
		}
		current := profiles.CurrentName()
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
