package kernel

import (
	pkernel "github.com/nelvko/proxyctl/internal/kernel"
	"github.com/spf13/cobra"
)

// listCmd lists kernels with install and selection status.
func newListCmd() *cobra.Command {
	command := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List kernels",
		Long:    `List known kernels with install and selection status.`,
		Args:    cobra.NoArgs,

		RunE: func(cmd *cobra.Command, args []string) error {
			a, err := loadApp(cmd)
			if err != nil {
				return err
			}
			for _, e := range pkernel.List(a.Config) {
				marker := " "
				state := "not implemented"
				switch {
				case e.Installed && e.Selected:
					state = "installed, selected"
				case e.Installed:
					state = "installed"
				case e.Implemented:
					state = "available"
				}
				if e.Selected {
					marker = "*"
				}
				cmd.Printf("%s %s\t%s\t%s\n", marker, e.Name, e.Format, state)
			}
			return nil
		},
	}

	return command
}
