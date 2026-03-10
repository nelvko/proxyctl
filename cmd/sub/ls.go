/*
Copyright © 2026 NAME HERE <EMAIL ADDRESS>
*/
package sub

import (
	"fmt"

	tea "charm.land/bubbletea/v2"
	"github.com/spf13/cobra"
)

// lsCmd represents the ls command
var lsCmd = &cobra.Command{
	Use:     "ls",
	Aliases: []string{"list"},
	Short:   "List Subscription profiles",
	Long:    ``,
	RunE: func(cmd *cobra.Command, args []string) error {
		return tui()
	},
}

func init() {
	SubCmd.AddCommand(lsCmd)
}

func tui() error {
	p := tea.NewProgram(initialModel())
	if _, err := p.Run(); err != nil {
		return fmt.Errorf("Error running program: %w", err)
	}
	return nil
}
