package cmd

import (
	"fmt"

	"github.com/nelvko/proxyctl/internal/app"
	"github.com/nelvko/proxyctl/internal/ui"
	"github.com/spf13/cobra"
)

// useCmd switches the active kernel.
var useCmd = &cobra.Command{
	Use:   "use <name>",
	Short: "Switch the active kernel",
	Long: `Switch the active kernel.

The previously active service is stopped and the current
subscription is re-applied on the new kernel.`,
	Args:        cobra.ExactArgs(1),
	Annotations: map[string]string{"skipRuntime": "true"},
	RunE: func(cmd *cobra.Command, args []string) error {
		a, err := app.Load()
		if err != nil {
			return err
		}
		if err := a.UseKernel(args[0]); err != nil {
			return err
		}
		if a.Subscription.Use == "" {
			ui.Ok(fmt.Sprintf("kernel %q used (no active subscription, add one with `proxyctl sub add`)", args[0]))
			return nil
		}
		ui.Ok(fmt.Sprintf("kernel %q used successfully", args[0]))
		return nil
	},
}

func init() {
	RootCmd.AddCommand(useCmd)
}
