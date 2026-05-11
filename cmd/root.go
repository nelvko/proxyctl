package cmd

import (
	"fmt"
	"os"
	"slices"
	"strings"

	"github.com/nelvko/proxyctl/internal/app"
	"github.com/nelvko/proxyctl/internal/config"
	"github.com/spf13/cobra"
)

var AppCtx *app.Runtime

// RootCmd represents the base command when called without any subcommands
var RootCmd = &cobra.Command{
	Use:           config.AppName,
	Short:         "Go proxy elegantly",
	Long:          ``,
	SilenceUsage:  true,
	SilenceErrors: true,
	Annotations:   skipInit,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		var err error
		whiteList := []string{"init", "completion", "help"}
		if slices.Contains(whiteList, cmd.Name()) {
			return nil
		}
		if AppCtx, err = app.LoadRuntime(); err != nil {
			return err
		}
		return err
	},
}

var manageGroup = &cobra.Group{ID: "manage", Title: "Management Commands"}
var skipInit map[string]string

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the RootCmd.
func Execute() {
	if err := RootCmd.Execute(); err != nil {
		msg := strings.TrimRight(err.Error(), "\n")
		fmt.Fprintln(os.Stderr, "Error: "+msg)
	}
}

func init() {
	RootCmd.AddGroup(manageGroup)
}
