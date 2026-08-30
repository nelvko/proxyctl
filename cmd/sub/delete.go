package sub

import (
	"fmt"

	"github.com/nelvko/proxyctl/internal/log"
	"github.com/nelvko/proxyctl/internal/subscription"
	"github.com/spf13/cobra"
)

// delCmd represents the del command
var delCmd = &cobra.Command{
	Use:     "delete <name>",
	Aliases: []string{"del"},
	Short:   "Delete a subscription profile",
	Long: `Delete a subscription profile.
`,
	Args: validArgWithInteractive,
	RunE: func(cmd *cobra.Command, args []string) error {
		profiles, err := profiles()
		if err != nil {
			return err
		}
		if interactive {
			return subscription.TUI(profiles)
		}
		profileName := args[0]

		if err := profiles.Delete(profileName, force); err != nil {
			return err
		}
		log.Ok(fmt.Sprintf("profile %q deleted successfully", profileName))
		return nil
	},
}
var (
	force bool
)

func init() {
	delCmd.Flags().BoolVarP(&force, "force", "f", force, "force delete even if the profile is in use")
	subCmd.AddCommand(delCmd)
}
