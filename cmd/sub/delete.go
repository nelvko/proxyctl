package sub

import (
	"fmt"
	"os"
	"slices"

	"github.com/nelvko/proxyctl/log"
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
		if interactive {
			return tui()
		}
		profileName := args[0]
		if err := deleteProfile(profileName); err != nil {
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
func deleteProfile(name string) error {
	tgt, err := getProfile(name)
	if err != nil {
		return err
	}

	if subCfg.Use == tgt.Name && !force {
		return fmt.Errorf("profile %q is currently in use", subCfg.Use)
	}

	subCfg.Profiles = slices.DeleteFunc(subCfg.Profiles, func(p profile) bool {
		return p.Name == name
	})
	if err := saveSubConfig(); err != nil {
		return err
	}

	if err := os.Remove(tgt.File); err != nil {
		return err
	}

	return nil
}

func getProfile(name string) (profile, error) {
	i := slices.IndexFunc(subCfg.Profiles, func(p profile) bool {
		return p.Name == name
	})
	if i == -1 {
		return profile{}, fmt.Errorf("profile %q not found\nUse `proxyctl sub list` to see available profiles", name)
	}
	return subCfg.Profiles[i], nil
}
