package sub

import (
	"fmt"

	"github.com/nelvko/proxyctl/internal/log"
	"github.com/nelvko/proxyctl/internal/profile"
	"github.com/spf13/cobra"
)

// useCmd represents the use command
var useCmd = &cobra.Command{
	Use:   "use <name>",
	Short: "Use a subscription profile",
	Long:  `Use the specified subscription profile for the proxy kernel.`,
	Args:  validArgWithInteractive,
	RunE: func(cmd *cobra.Command, args []string) error {
		if interactive {
			return profile.TUI()
		}
		profileName := args[0]
		if err := profile.Use(profileName); err != nil {
			return err
		}
		log.Ok(fmt.Sprintf("profile %q used successfully", profileName))
		return nil

	},
}

func init() {
	subCmd.AddCommand(useCmd)
}
