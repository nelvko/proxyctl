/*
Copyright © 2025 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"github.com/spf13/cobra"
)

// statusCmd represents the status command
var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show runtime status of proxy kernel",
	Long:  ``,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := k.Status(); err != nil {
			return err
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(statusCmd)
}
