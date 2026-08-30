package kernel

import (
	"fmt"

	rootcmd "github.com/nelvko/proxyctl/cmd"
	"github.com/nelvko/proxyctl/internal/app"
	pkernel "github.com/nelvko/proxyctl/internal/kernel"
	"github.com/nelvko/proxyctl/internal/ui"
	"github.com/spf13/cobra"
)

// installCmd represents the install command
var installCmd = &cobra.Command{
	Use:   "install [name]",
	Short: "Download and install a kernel",
	Long: `Download and install a kernel with its user service.

Run without an argument to pick interactively. The first
installed kernel becomes active.`,
	Args:      cobra.MaximumNArgs(1),
	ValidArgs: pkernel.ImplementedNames(),
	RunE: func(cmd *cobra.Command, args []string) error {
		a, err := app.Load()
		if err != nil {
			return err
		}
		name := ""
		if len(args) == 1 {
			name = args[0]
		} else {
			name, err = rootcmd.PickKernel()
			if err != nil {
				return err
			}
		}
		if err := a.InstallKernel(name); err != nil {
			return err
		}
		ui.Ok(fmt.Sprintf("kernel %q installed", name))
		return nil
	},
}

func init() {
	kernelCmd.AddCommand(installCmd)
}
