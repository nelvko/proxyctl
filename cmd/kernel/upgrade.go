package kernel

import (
	"errors"
	"fmt"
	"strings"

	pkernel "github.com/nelvko/proxyctl/internal/kernel"
	"github.com/spf13/cobra"
)

// upgradeCmd refreshes a kernel binary from its configured source.
func newUpgradeCmd() *cobra.Command {
	command := &cobra.Command{
		Use:   "upgrade [name]",
		Short: "Refresh a kernel binary",
		Long: `Download the latest built-in release or copy the saved local binary again.

Run without an argument to refresh the selected kernel.
The service is restarted if it was running.

Built-in downloads try any configured mirror (--mirror flag, the mirror
config key, or the GH_PROXY env variable) before github.com
itself, and verify each source against the release digest from
the GitHub API.`,
		Args: cobra.MaximumNArgs(1),

		RunE: mutation(func(cmd *cobra.Command, args []string) error {
			a, err := loadApp(cmd)
			if err != nil {
				return err
			}
			name := ""
			if len(args) == 1 {
				name = args[0]
			}
			target := name
			if target == "" {
				target = a.Config.Use
			}
			if entry := a.Config.KernelByName(target); entry != nil && entry.Source != "" && cmd.Flags().Changed("mirror") {
				return fmt.Errorf("--mirror applies only to built-in downloads")
			}
			// Not persisted: only `install` writes the mirror choice.
			if ms, _ := cmd.Flags().GetStringSlice("mirror"); len(ms) > 0 {
				a.Config.Mirror = strings.Join(ms, ",")
			}
			if err := a.UpgradeKernel(cmd.Context(), name); err != nil {
				if errors.Is(err, pkernel.ErrUpToDate) {
					if name == "" {
						name = "active kernel"
					}
					cmd.Println(fmt.Sprintf("%s already up to date", name))
					return nil
				}
				return err
			}
			if name == "" {
				name = "active kernel"
			}
			cmd.Println(fmt.Sprintf("%s upgraded", name))
			return nil
		}),
	}
	command.Flags().StringSlice("mirror", nil, "GitHub mirror prefix(es) for built-in downloads")
	return command
}
