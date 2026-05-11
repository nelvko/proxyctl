package sub

import (
	"fmt"
	"time"

	rootcmd "github.com/nelvko/proxyctl/cmd"
	"github.com/nelvko/proxyctl/internal/config"
	"github.com/nelvko/proxyctl/internal/log"
	"github.com/nelvko/proxyctl/internal/profile"
	"github.com/spf13/cobra"
)

func validArgWithInteractive(cmd *cobra.Command, args []string) error {
	if interactive {
		return nil
	}
	return cobra.ExactArgs(1)(cmd, args)
}

// addCmd represents the add command
var addCmd = &cobra.Command{
	Use:   "add <url>",
	Short: "Add a subscription profile from a source URL",
	Long: `Add a subscription profile from an HTTP, HTTPS, or file URL.

The profile is validated before it is saved. If this is the first
profile, it becomes active automatically.`,
	Args: validArgWithInteractive,
	RunE: func(cmd *cobra.Command, args []string) error {
		profiles := rootcmd.Runtime().Profiles
		if interactive {
			if err := profile.PromptAdd(profiles, option); err != nil {
				return err
			}
		} else {
			option.URL = args[0]
		}
		if err := profiles.Add(option); err != nil {
			return err
		}
		if use || (profiles.CurrentName() == "" && len(profiles.List()) == 1) {
			if err := profiles.Use(option.Name); err != nil {
				return err
			}
			log.Ok(fmt.Sprintf("profile %q used successfully", option.Name))
		}
		return nil

	},
}

var (
	use    bool
	option = &config.Profile{
		Update: config.UpdateConfig{
			Timeout: 10 * time.Second,
		},
	}
)

func init() {
	subCmd.AddCommand(addCmd)
	addCmd.Flags().StringVarP(&option.Name, "name", "n", option.Name, "Profile name; defaults to the current Unix timestamp")
	addCmd.Flags().BoolVarP(&use, "use", "u", use, "Use the new profile after adding it")
	addCmd.Flags().DurationVarP(&option.Update.Timeout, "timeout", "t", option.Update.Timeout, "HTTP(S) download timeout")
	// todo updateConfig
}
