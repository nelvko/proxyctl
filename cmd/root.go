package cmd

import (
	"errors"
	"fmt"
	"slices"

	"github.com/nelvko/proxyctl/internal/bootstrap"
	"github.com/nelvko/proxyctl/internal/config"
	"github.com/spf13/cobra"
)

var runtimeCtx *bootstrap.Runtime

// RootCmd represents the base command when called without any subcommands
var RootCmd = &cobra.Command{
	Use:           config.AppName,
	Short:         "Go proxy elegantly",
	Long:          ``,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		var err error
		whiteList := []string{"init", "completion", "help"}
		if slices.Contains(whiteList, cmd.Name()) {
			return nil
		}
		if runtimeCtx, err = bootstrap.LoadRuntime(); err != nil {
			if errors.Is(err, config.ErrInitRequired) {
				return fmt.Errorf("proxyctl is not initialized. Run `proxyctl init` first")
			}
			return err
		}
		return err
	},
}

var manageGroup = &cobra.Group{ID: "manage", Title: "Management Commands"}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the RootCmd.
// func Execute() {
// 	if err := RootCmd.Execute(); err != nil {
// 		msg := strings.TrimRight(err.Error(), "\n")
// 		fmt.Fprintln(os.Stderr, "Error: "+msg)
// 	}
// }

func Runtime() *bootstrap.Runtime {
	return runtimeCtx
}

func init() {
	RootCmd.AddGroup(manageGroup)
}
