package cmd

import (
	"errors"
	"os"

	"github.com/nelvko/proxyctl/internal/config"
	"github.com/nelvko/proxyctl/internal/setup"
	"github.com/spf13/cobra"
)

// initCmd represents the init command
var initCmd = &cobra.Command{
	Use:     "init",
	Short:   "initialize proxyctl settings",
	Long:    ``,
	Aliases: []string{"i"},
	RunE: func(cmd *cobra.Command, args []string) error {
		if !force {
			if _, err := os.Stat(config.AppConfigFile); err == nil {
				return errors.New("configuration already exists. Use --force to overwrite")
			}
		}
		return setup.Wizard(yes)
	},
}

var (
	force bool
	yes   bool
)

func init() {
	RootCmd.AddCommand(initCmd)
	initCmd.Flags().BoolVarP(&force, "force", "f", false, "Force initialization even if config file exists")
	initCmd.Flags().BoolVarP(&yes, "yes", "y", false, "Use default values and skip interactive TUI")
}
