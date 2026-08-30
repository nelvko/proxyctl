package kernel

import (
	"fmt"

	pkernel "github.com/nelvko/proxyctl/internal/kernel"
	"github.com/nelvko/proxyctl/internal/log"
	"github.com/spf13/cobra"
)

// uninstallCmd represents the uninstall command
var uninstallCmd = &cobra.Command{
	Use:     "uninstall <name>",
	Aliases: []string{"remove", "rm"},
	Short:   "Uninstall a kernel",
	Long: `Uninstall a kernel: stop its service, remove the unit,
the binary and config directories, and its config entry.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		if err := pkernel.Uninstall(name); err != nil {
			return err
		}
		log.Ok(fmt.Sprintf("kernel %q uninstalled", name))
		return nil
	},
}

func init() {
	kernelCmd.AddCommand(uninstallCmd)
}
