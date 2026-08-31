package sub

import (
	"errors"

	"github.com/nelvko/proxyctl/internal/subscription"
	"github.com/spf13/cobra"
)

// updateCmd represents the update command
var updateCmd = &cobra.Command{
	Use:   "update [name...]",
	Short: "Update subscription profiles",
	Long: `Update subscription profiles.

Every profile is re-fetched from its source URL (or only the named
ones) and validated before its file is replaced. A profile whose
content did not change is left untouched, so the kernel is restarted
only when the active profile really changed. A failing profile does
not stop the others.`,
	Args: cobra.ArbitraryArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		profiles, err := profiles()
		if err != nil {
			return err
		}
		if interactive && len(args) == 0 {
			return subscription.TUI(profiles)
		}
		if len(profiles.List()) == 0 {
			return errors.New("no profiles, run `proxyctl sub add <url>` first")
		}
		return profiles.Update(args...)
	},
}

func init() {
	subCmd.AddCommand(updateCmd)
}
