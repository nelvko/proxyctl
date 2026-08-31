package kernel

import (
	rootcmd "github.com/nelvko/proxyctl/cmd"
	"github.com/nelvko/proxyctl/internal/app"
	"github.com/nelvko/proxyctl/internal/httpx"
	"github.com/spf13/cobra"
)

// kernelCmd represents the kernel command
var kernelCmd = &cobra.Command{
	Use:   "kernel",
	Short: "Manage proxy kernels",
	Long: `Manage proxy kernels (mihomo, clash, sing-box).

Run without a subcommand to list kernels.

Downloads try any configured mirror (--mirror flag, the mirror config
key, or the GH_PROXY env variable) before github.com itself, and verify
each source against the release digest from the GitHub API.`,
	GroupID:     rootcmd.ManageGroupID(),
	Annotations: map[string]string{"skipRuntime": "true"},
	RunE: func(cmd *cobra.Command, args []string) error {
		return listCmd.RunE(cmd, args)
	},
}

// resolveMirror applies an explicit --mirror for this invocation; the
// config field is already applied by app.Load, and GH_PROXY overrides both
// inside httpx. It reports whether the flag was given (only install
// persists it).
func resolveMirror(cmd *cobra.Command, _ *app.App) bool {
	ms, _ := cmd.Flags().GetStringSlice("mirror")
	if len(ms) == 0 {
		return false
	}
	httpx.SetMirrors(ms...)
	return true
}

func init() {
	kernelCmd.PersistentFlags().StringSlice("mirror", nil, "GitHub mirror prefix(es) for downloads, e.g. https://ghfast.top (GH_PROXY env overrides; saved to the config on install)")
	rootcmd.RootCmd.AddCommand(kernelCmd)
}
