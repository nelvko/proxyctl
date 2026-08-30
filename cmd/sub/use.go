package sub

import (
	"fmt"

	"github.com/nelvko/proxyctl/internal/subscription"
	"github.com/nelvko/proxyctl/internal/ui"
	"github.com/spf13/cobra"
)

// useCmd represents the use command
var useCmd = &cobra.Command{
	Use:   "use <name>",
	Short: "Use a subscription profile",
	Long:  `Use the specified subscription profile for the proxy kernel.`,
	Args:  validArgWithInteractive,
	RunE: func(cmd *cobra.Command, args []string) error {
		profiles, err := profiles()
		if err != nil {
			return err
		}
		if interactive {
			return subscription.TUI(profiles)
		}
		profileName := args[0]
		if err := profiles.Use(profileName); err != nil {
			return err
		}
		ui.Ok(fmt.Sprintf("profile %q used successfully", profileName))
		return nil

	},
}

func init() {
	subCmd.AddCommand(useCmd)
}
