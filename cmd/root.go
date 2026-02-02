/*
Copyright © 2025 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"errors"
	"fmt"
	"os"
	"slices"

	"github.com/charmbracelet/huh"
	"github.com/nelvko/proxyctl/config"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:              "proxyctl",
	Short:            "Go proxy elegantly",
	Long:             ``,
	SilenceUsage:     true,
	SilenceErrors:    false,
	PersistentPreRun: CheckIsInitialized,
}

var manageGroup = &cobra.Group{ID: "manage", Title: "Management Commands"}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		// fmt.Fprintln(os.Stderr, err)
	}
}

func init() {
	rootCmd.AddGroup(manageGroup)
	config.Load()
	// DetectInitSystem()
	// _, _ := LoadConfig()
	// Here you will define your flags and configuration settings.
	// Cobra supports persistent flags, which, if defined here,
	// will be global for your application.

	// rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.clashgo.yaml)")

	// Cobra also supports local flags, which will only run
	// when this action is called directly.
}

func CheckIsInitialized(cmd *cobra.Command, args []string) {
	whiteList := []string{"init", "completion", "help"}
	if slices.Contains(whiteList, cmd.Name()) {
		return
	}

	if errors.Is(viper.ReadInConfig(), os.ErrNotExist) {
		var confirm bool
		form := huh.NewForm(
			huh.NewGroup(
				huh.NewConfirm().
					Title("🔧 Proxyctl is not initialized").
					Description("🤔 Would you like to run the setup wizard now?").
					Affirmative("🚀 Yes, let's go!").
					Negative("🚫 No, maybe later").
					Value(&confirm),
			),
		)
		err := form.Run()
		if err != nil {
			fmt.Println("\nAborted.")
			os.Exit(130)
		}
		if !confirm {
			// fmt.Println("No problem! You can initialize whenever you're ready by running `proxyctl init`.")
			os.Exit(0)
		}
		initCmd.Run(initCmd, []string{})

	}
}
