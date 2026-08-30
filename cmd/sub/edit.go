package sub

import (
	"github.com/nelvko/proxyctl/internal/profile"
	"github.com/spf13/cobra"
)

// editCmd represents the edit command
var editCmd = &cobra.Command{
	Use:   "edit <name>",
	Short: "Edit a subscription profile",
	Long: `Edit a subscription profile via editor.

	With --editor or $EDITOR to specify the editor command. The edited
profile will be test after the editor exits.`,
	Args: validArgWithInteractive,
	RunE: func(cmd *cobra.Command, args []string) error {
		profiles := profiles()
		if interactive {
			return profile.TUI(profiles)
		}
		profileName := args[0]
		return profiles.Edit(profileName, editor)

	},
}

var (
	editor string
)

func init() {
	subCmd.AddCommand(editCmd)
	editCmd.Flags().StringVarP(&editor, "editor", "e", editor, "Editor command")
}
