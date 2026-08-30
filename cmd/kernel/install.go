package kernel

import (
	"fmt"

	rootcmd "github.com/nelvko/proxyctl/cmd"
	pkernel "github.com/nelvko/proxyctl/internal/kernel"
	"github.com/nelvko/proxyctl/internal/log"
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
	ValidArgs: pkernel.Names(),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := ""
		if len(args) == 1 {
			name = args[0]
		} else {
			var err error
			name, err = rootcmd.PickKernel()
			if err != nil {
				return err
			}
		}
		if err := pkernel.Install(name); err != nil {
			return err
		}
		log.Ok(fmt.Sprintf("kernel %q installed", name))
		return nil
	},
}

func init() {
	kernelCmd.AddCommand(installCmd)
}
