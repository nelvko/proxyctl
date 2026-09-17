package kernel

import (
	"errors"
	"fmt"
	"strings"

	pkernel "github.com/nelvko/proxyctl/internal/kernel"
	"github.com/spf13/cobra"
)

// upgradeCmd replaces a kernel binary with the latest release.
func newUpgradeCmd() *cobra.Command {
	command := &cobra.Command{
		Use:   "upgrade [name]",
		Short: "Upgrade a kernel to the latest release",
		Long: `Upgrade a kernel to the latest release.

Run without an argument to upgrade the active kernel.
The service is restarted if it was running.

Downloads try any configured mirror (--mirror flag, the mirror
config key, or the GH_PROXY env variable) before github.com
itself, and verify each source against the release digest from
the GitHub API.`,
		Args: cobra.MaximumNArgs(1),

		RunE: mutation(func(cmd *cobra.Command, args []string) error {
			a, err := loadApp(cmd)
			if err != nil {
				return err
			}
			// Not persisted: only `install` writes the mirror choice.
			if ms, _ := cmd.Flags().GetStringSlice("mirror"); len(ms) > 0 {
				a.Config.Mirror = strings.Join(ms, ",")
			}
			name := ""
			if len(args) == 1 {
				name = args[0]
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
	command.Flags().StringSlice("mirror", nil, "GitHub mirror prefix(es) for downloads, e.g. https://ghfast.top (GH_PROXY env overrides)")
	return command
}
