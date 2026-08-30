package kernel

import (
	"fmt"

	pkernel "github.com/nelvko/proxyctl/internal/kernel"
	"github.com/spf13/cobra"
)

// listCmd represents the list command
var listCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List kernels",
	Long:    `List known kernels with install and active status.`,
	Args:    cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		entries, err := pkernel.List()
		if err != nil {
			return err
		}
		for _, e := range entries {
			marker := " "
			state := "not implemented"
			switch {
			case e.Installed && e.Active:
				state = "installed, active"
			case e.Installed:
				state = "installed"
			case e.Ready:
				state = "available"
			}
			if e.Active {
				marker = "*"
			}
			fmt.Printf("%s %s\t%s\t%s\n", marker, e.Name, e.Format, state)
		}
		return nil
	},
}

func init() {
	kernelCmd.AddCommand(listCmd)
}
