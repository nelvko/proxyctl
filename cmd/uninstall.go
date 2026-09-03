package cmd

import (
	"errors"
	"fmt"

	"github.com/nelvko/proxyctl/internal/app"
	"github.com/nelvko/proxyctl/internal/httpx"
	pkernel "github.com/nelvko/proxyctl/internal/kernel"
	"github.com/nelvko/proxyctl/internal/ui"
	"github.com/spf13/cobra"
)

// uninstallCmd removes a kernel: its service, binary, config files and
// config entry.
var uninstallCmd = &cobra.Command{
	Use:     "uninstall <name>",
	Aliases: []string{"remove", "rm"},
	Short:   "Uninstall a kernel",
	Long: `Uninstall a kernel: stop its service, remove the unit,
the binary and config directories, and its config entry.`,
	Args:        cobra.ExactArgs(1),
	GroupID:     KernelGroup.ID,
	Annotations: map[string]string{"skipRuntime": "true"},
	RunE: func(cmd *cobra.Command, args []string) error {
		a, err := app.Load()
		if err != nil {
			return err
		}
		if err := a.UninstallKernel(args[0]); err != nil {
			return err
		}
		ui.Ok(fmt.Sprintf("kernel %q uninstalled", args[0]))
		return nil
	},
}

// upgradeCmd replaces a kernel binary with the latest release.
var upgradeCmd = &cobra.Command{
	Use:   "upgrade [name]",
	Short: "Upgrade a kernel to the latest release",
	Long: `Upgrade a kernel to the latest release.

Run without an argument to upgrade the active kernel.
The service is restarted if it was running.

Downloads try any configured mirror (--mirror flag, the mirror
config key, or the GH_PROXY env variable) before github.com
itself, and verify each source against the release digest from
the GitHub API.`,
	Args:        cobra.MaximumNArgs(1),
	GroupID:     KernelGroup.ID,
	Annotations: map[string]string{"skipRuntime": "true"},
	RunE: func(cmd *cobra.Command, args []string) error {
		a, err := app.Load()
		if err != nil {
			return err
		}
		// Not persisted: only `install` writes the mirror choice.
		if ms, _ := cmd.Flags().GetStringSlice("mirror"); len(ms) > 0 {
			httpx.SetMirrors(ms...)
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
				ui.Ok(fmt.Sprintf("%s already up to date", name))
				return nil
			}
			return err
		}
		if name == "" {
			name = "active kernel"
		}
		ui.Ok(fmt.Sprintf("%s upgraded", name))
		return nil
	},
}

func init() {
	RootCmd.AddCommand(uninstallCmd)
	RootCmd.AddCommand(upgradeCmd)
	upgradeCmd.Flags().StringSlice("mirror", nil, "GitHub mirror prefix(es) for downloads, e.g. https://ghfast.top (GH_PROXY env overrides)")
}
