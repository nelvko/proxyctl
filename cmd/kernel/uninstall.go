package kernel

import (
	"fmt"

	"github.com/nelvko/proxyctl/internal/app"
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
		a, err := app.Load()
		if err != nil {
			return err
		}
		if err := a.UninstallKernel(args[0]); err != nil {
			return err
		}
		log.Ok(fmt.Sprintf("kernel %q uninstalled", args[0]))
		return nil
	},
}

// upgradeCmd represents the upgrade command
var upgradeCmd = &cobra.Command{
	Use:   "upgrade [name]",
	Short: "Upgrade a kernel to the latest release",
	Long: `Upgrade a kernel to the latest release.

Run without an argument to upgrade the active kernel.
The service is restarted if it was running.`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		a, err := app.Load()
		if err != nil {
			return err
		}
		name := ""
		if len(args) == 1 {
			name = args[0]
		}
		if err := a.UpgradeKernel(name); err != nil {
			return err
		}
		if name == "" {
			name = "active kernel"
		}
		log.Ok(fmt.Sprintf("%s upgraded", name))
		return nil
	},
}

func init() {
	kernelCmd.AddCommand(uninstallCmd)
	kernelCmd.AddCommand(upgradeCmd)
}
