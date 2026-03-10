/*
Copyright © 2026 nelvko
*/
package sub

import (
	"fmt"
	"os"
	"slices"

	"github.com/nelvko/proxyctl/config"
	"github.com/nelvko/proxyctl/kernel"
	"github.com/nelvko/proxyctl/log"
	"github.com/spf13/cobra"
)

// useCmd represents the use command
var useCmd = &cobra.Command{
	Use:   "use [name]",
	Short: "Use the specified profile",
	Long:  ``,
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) != 1 {
			if err := tui(); err != nil {
				return err
			}
		} else {
			name = args[0]
		}
		if err := useFunc(name); err != nil {
			return err
		}
		log.Ok(fmt.Sprintf("使用订阅：%s", name))
		return nil

	},
}

func init() {
	SubCmd.AddCommand(useCmd)
}

func useFunc(profileName string) error {
	i := slices.IndexFunc(cfg.Items, func(p profile) bool {
		return p.Name == profileName
	})
	if i == -1 {
		return fmt.Errorf("can't find %s profile", profileName)
	}
	useFile := cfg.Items[i].File
	kernelCfg := config.Get().Kernel.ConfigFile
	// if _, err := os.Stat(kernelCfg); os.IsNotExist(err) {
	// 	if _, err := os.Create(kernelCfg); err != nil {
	// 		return err
	// 	}
	// }

	bytes, err := os.ReadFile(useFile)
	if err != nil {
		return err
	}
	if err := os.WriteFile(kernelCfg, bytes, 0666); err != nil {
		return err
	}
	k, err := kernel.New()
	if err != nil {
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
	cfg.Use = profileName
	return saveSubConfig()

}
