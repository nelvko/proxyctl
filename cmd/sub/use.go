package sub

import (
	"fmt"
	"os"
	"slices"

	"github.com/nelvko/proxyctl/config"
	"github.com/nelvko/proxyctl/log"
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
			return tui()
		}
		profileName := args[0]
		if err := useFunc(profileName); err != nil {
			return err
		}
		log.Ok(fmt.Sprintf("profile %q used successfully", profileName))
		return nil

	},
}

func init() {
	subCmd.AddCommand(useCmd)
}

func useFunc(profileName string) error {
	i := slices.IndexFunc(subCfg.Profiles, func(p profile) bool {
		return p.Name == profileName
	})
	if i == -1 {
		return fmt.Errorf("can't find %s profile", profileName)
	}
	useFile := subCfg.Profiles[i].File
	kernelCfg := config.AppCfg.Kernel.ConfigFile
	if err := k.TestConfig(useFile); err != nil {
		return err
	}

	bytes, err := os.ReadFile(useFile)
	if err != nil {
		return err
	}
	if err := os.WriteFile(kernelCfg, bytes, 0666); err != nil {
		return err
	}
	if err := k.Restart(); err != nil {
		return err
	}
	active, err := k.IsActive()
	if err != nil {
		return err
	}
	if !active {
		return err
	}
	subCfg.Use = profileName
	return saveSubConfig()

}
