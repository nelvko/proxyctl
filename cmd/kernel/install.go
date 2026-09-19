package kernel

import (
	"fmt"
	"strings"

	pkernel "github.com/nelvko/proxyctl/internal/kernel"
	"github.com/spf13/cobra"
)

// installCmd installs a kernel — the default object of proxyctl.
func newInstallCmd() *cobra.Command {
	command := &cobra.Command{
		Use:   "install [name]",
		Short: "Download and install a kernel",
		Long: `Download and install a kernel with its user service.

Run without an argument to pick interactively. The first
installed kernel becomes selected.

Downloads try any configured mirror (--mirror flag, the mirror
config key, or the GH_PROXY env variable) before github.com
itself, and verify each source against the release digest from
the GitHub API.`,
		Args:      cobra.MaximumNArgs(1),
		ValidArgs: pkernel.ImplementedNames(),

		RunE: mutation(func(cmd *cobra.Command, args []string) error {
			a, err := loadApp(cmd)
			if err != nil {
				return err
			}
			// An explicit --mirror applies to this invocation and is remembered
			// so later upgrades reuse it; InstallKernel's save persists it.
			if ms, _ := cmd.Flags().GetStringSlice("mirror"); len(ms) > 0 {
				a.Config.Mirror = strings.Join(ms, ",")
			}
			name := ""
			if len(args) == 1 {
				name = args[0]
			} else {
				name, err = pickKernel("Select a kernel to install", installableOptions())
				if err != nil {
					if cancelled(err) {
						cmd.PrintErrln("cancelled")
						return nil
					}
					return err
				}
			}
			if err := a.InstallKernel(cmd.Context(), name); err != nil {
				return err
			}
			cmd.Println(fmt.Sprintf("kernel %q installed", name))
			return nil
		}),
	}
	command.Flags().StringSlice("mirror", nil, "GitHub mirror prefix(es) for downloads, e.g. https://ghfast.top (GH_PROXY env overrides; saved to the config)")
	return command
}
