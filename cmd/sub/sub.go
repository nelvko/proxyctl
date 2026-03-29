package sub

import (
	"fmt"

	tea "charm.land/bubbletea/v2"
	"github.com/nelvko/proxyctl/cmd"
	"github.com/nelvko/proxyctl/kernel"
	"github.com/spf13/cobra"
)

// subCmd represents the sub command
var subCmd = &cobra.Command{
	Use:   "sub",
	Short: "Manage subscription profiles",
	Long: `Manage subscription profiles.

Run without a subcommand to open the TUI.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return tui()
	},
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		parent := cmd.Parent()
		var err error
		if err = parent.PersistentPreRunE(parent, args); err != nil {
			return err
		}

		if err = loadSubConfig(); err != nil {
			return err
		}

		if k, err = kernel.New(); err != nil {
			return err
		}
		return nil
	},
}

var (
	k kernel.Kernel
)

func tui() error {
	p := tea.NewProgram(initialModel())
	if _, err := p.Run(); err != nil {
		return fmt.Errorf("Error running program: %w", err)
	}
	return nil
}

var interactive bool

func init() {
	subCmd.PersistentFlags().BoolVarP(&interactive, "interactive", "i", false, "enable interactive TUI mode")
	cmd.RootCmd.AddCommand(subCmd)
}
