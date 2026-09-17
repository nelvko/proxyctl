package kernel

import (
	"fmt"

	"github.com/spf13/cobra"
)

// uninstallCmd removes a kernel: its service, binary, config files and
// config entry.
func newUninstallCmd() *cobra.Command {
	command := &cobra.Command{
		Use:     "uninstall <name>",
		Aliases: []string{"remove", "rm"},
		Short:   "Uninstall a kernel",
		Long: `Uninstall a kernel: stop its service, remove the unit,
the binary and config directories, and its config entry.`,
		Args: cobra.ExactArgs(1),

		RunE: mutation(func(cmd *cobra.Command, args []string) error {
			a, err := loadApp(cmd)
			if err != nil {
				return err
			}
			if err := a.UninstallKernel(args[0]); err != nil {
				return err
			}
			cmd.Println(fmt.Sprintf("kernel %q uninstalled", args[0]))
			return nil
		}),
	}

	return command
}
