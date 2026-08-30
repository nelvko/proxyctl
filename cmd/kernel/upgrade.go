package kernel

import (
	"fmt"

	pkernel "github.com/nelvko/proxyctl/internal/kernel"
	"github.com/nelvko/proxyctl/internal/log"
	"github.com/spf13/cobra"
)

// upgradeCmd represents the upgrade command
var upgradeCmd = &cobra.Command{
	Use:   "upgrade [name]",
	Short: "Upgrade a kernel to the latest release",
	Long: `Upgrade a kernel to the latest release.

Run without an argument to upgrade the active kernel.
The service is restarted if it was running.`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := ""
		if len(args) == 1 {
			name = args[0]
		}
		if err := pkernel.Upgrade(name); err != nil {
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
	kernelCmd.AddCommand(upgradeCmd)
}
