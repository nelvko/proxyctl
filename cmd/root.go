/*
Copyright © 2025 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"

	"github.com/nelvko/proxyctl/cmd/sub"
	"github.com/nelvko/proxyctl/config"
	"github.com/nelvko/proxyctl/kernel"
	"github.com/spf13/cobra"
)

var k kernel.Kernel

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:           "proxyctl",
	Short:         "Go proxy elegantly",
	Long:          ``,
	SilenceUsage:  true,
	SilenceErrors: true,
	Annotations:   skipInit,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		whiteList := []string{"init", "completion", "help"}
		var err error
		if slices.Contains(whiteList, cmd.Name()) {
			return nil
		}
		if err := config.LoadAppConfig(); errors.Is(err, os.ErrNotExist) {
			if err := setupWizard(); err != nil {
				return err
			}
		}
		if k, err = kernel.New(); err != nil {
			return err
		}
		return nil

	},
}

var manageGroup = &cobra.Group{ID: "manage", Title: "Management Commands"}
var skipInit map[string]string

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		msg := strings.TrimRight(err.Error(), "\n")
		fmt.Fprintln(os.Stderr, "Error: "+msg)
	}
}

func init() {
	rootCmd.AddCommand(sub.SubCmd)

	rootCmd.AddGroup(manageGroup)
}
