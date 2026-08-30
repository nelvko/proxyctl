package kernel

import (
	rootcmd "github.com/nelvko/proxyctl/cmd"
	"github.com/spf13/cobra"
)

// kernelCmd represents the kernel command
var kernelCmd = &cobra.Command{
	Use:   "kernel",
	Short: "Manage proxy kernels",
	Long: `Manage proxy kernels (mihomo, clash, sing-box).

Run without a subcommand to list kernels.`,
	GroupID:     rootcmd.ManageGroupID(),
	Annotations: map[string]string{"skipRuntime": "true"},
	RunE: func(cmd *cobra.Command, args []string) error {
		return listCmd.RunE(cmd, args)
	},
}

func init() {
	rootcmd.RootCmd.AddCommand(kernelCmd)
}
