package kernel

import (
	"fmt"
	"strings"

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
		// An explicit --mirror is remembered so later upgrades reuse it;
		// InstallKernel's save persists it with the kernel entry.
		if resolveMirror(cmd, a) {
			ms, _ := cmd.Flags().GetStringSlice("mirror")
			a.Cfg.Mirror = strings.Join(ms, ",")
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
		if err := a.InstallKernel(cmd.Context(), name); err != nil {
			return err
		}
		ui.Ok(fmt.Sprintf("kernel %q installed", name))
		return nil
	},
}

func init() {
	kernelCmd.AddCommand(installCmd)
}
