package cmd

import (
	"fmt"

	"github.com/nelvko/proxyctl/internal/app"
	pkernel "github.com/nelvko/proxyctl/internal/kernel"
	"github.com/spf13/cobra"
)

// listCmd lists kernels with install and active status.
var listCmd = &cobra.Command{
	Use:         "list",
	Aliases:     []string{"ls"},
	Short:       "List kernels",
	Long:        `List known kernels with install and active status.`,
	Args:        cobra.NoArgs,
	GroupID:     KernelGroup.ID,
	Annotations: map[string]string{"skipRuntime": "true"},
	RunE: func(cmd *cobra.Command, args []string) error {
		a, err := app.Load()
		if err != nil {
			return err
		}
		for _, e := range pkernel.List(a.Config) {
			marker := " "
			state := "not implemented"
			switch {
			case e.Installed && e.Active:
				state = "installed, active"
			case e.Installed:
				state = "installed"
			case e.Implemented:
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
	RootCmd.AddCommand(listCmd)
}
