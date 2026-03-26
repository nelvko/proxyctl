/*
Copyright © 2026 NAME HERE <EMAIL ADDRESS>
*/
package sub

import (
	"github.com/nelvko/proxyctl/kernel"
	"github.com/spf13/cobra"
)

// SubCmd represents the sub command
var SubCmd = &cobra.Command{
	Use:   "sub",
	Short: "Manage Subscriptions",
	Long:  ``,
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
