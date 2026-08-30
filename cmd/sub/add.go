package sub

import (
	"fmt"
	"time"

	"github.com/nelvko/proxyctl/internal/config"
	"github.com/nelvko/proxyctl/internal/log"
	"github.com/nelvko/proxyctl/internal/subscription"
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
		profiles, err := profiles()
		if err != nil {
			return err
		}
		if interactive {
			if err := subscription.PromptAdd(profiles, draft); err != nil {
				return err
			}
		} else {
			draft.URL = args[0]
		}
		if err := profiles.Add(draft); err != nil {
			return err
		}
		// The first profile is activated by Add itself.
		if use && profiles.ActiveName() != draft.Name {
			if err := profiles.Use(draft.Name); err != nil {
				return err
			}
			log.Ok(fmt.Sprintf("profile %q used successfully", draft.Name))
		}
		return nil

	},
}

var (
	use   bool
	draft = &config.Profile{
		Update: config.UpdateConfig{
			Timeout: 10 * time.Second,
		},
	}
)

func init() {
	subCmd.AddCommand(addCmd)
	addCmd.Flags().StringVarP(&draft.Name, "name", "n", draft.Name, "Profile name; defaults to the current Unix timestamp")
	addCmd.Flags().BoolVarP(&use, "use", "u", use, "Use the new profile after adding it")
	addCmd.Flags().DurationVarP(&draft.Update.Timeout, "timeout", "t", draft.Update.Timeout, "HTTP(S) download timeout")
	// todo updateConfig
}
