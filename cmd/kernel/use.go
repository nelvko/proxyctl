package kernel

import (
	"fmt"

	"github.com/nelvko/proxyctl/internal/bootstrap"
	pkernel "github.com/nelvko/proxyctl/internal/kernel"
	"github.com/nelvko/proxyctl/internal/log"
	"github.com/spf13/cobra"
)

// useCmd represents the use command
var useCmd = &cobra.Command{
	Use:   "use <name>",
	Short: "Switch the active kernel",
	Long: `Switch the active kernel.

The previously active service is stopped and the current
subscription is re-applied on the new kernel.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		if err := pkernel.SetActive(name); err != nil {
			return err
		}

		// Re-apply the current subscription on the new kernel.
		rt, err := bootstrap.LoadRuntime()
		if err != nil {
			return err
		}
		p, perr := rt.Profiles.Using()
		if perr != nil {
			log.Ok(fmt.Sprintf("kernel %q used (no active subscription, add one with `proxyctl sub add`)", name))
			return nil
		}
		if err := rt.Profiles.Use(p.Name); err != nil {
			return fmt.Errorf("kernel switched but subscription re-apply failed: %w", err)
		}
		log.Ok(fmt.Sprintf("kernel %q used successfully", name))
		return nil
	},
}

func init() {
	kernelCmd.AddCommand(useCmd)
}
