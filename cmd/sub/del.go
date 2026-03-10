/*
Copyright © 2026 NAME HERE <EMAIL ADDRESS>
*/
package sub

import (
	"fmt"
	"os"
	"slices"

	"github.com/nelvko/proxyctl/log"
	"github.com/spf13/cobra"
)

// delCmd represents the del command
var delCmd = &cobra.Command{
	Use:     "del <name>",
	Aliases: []string{"delete"},
	Short:   "Delete Subscription profile",
	Long:    ``,
	Args:    cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name = args[0]
		if err := delProfile(name); err != nil {
			return fmt.Errorf("failed to delete profile: %w", err)
		}
		log.Ok("删除成功")
		return nil
	},
}
var (
	force bool
)

func init() {
	delCmd.Flags().BoolVarP(&force, "force", "f", force, "force delete even if the profile is in use")
	SubCmd.AddCommand(delCmd)
}
func delProfile(name string) error {
	i := slices.IndexFunc(cfg.Items, func(p profile) bool {
		return p.Name == name
	})
	if i == -1 {
		return fmt.Errorf("can't find %s profile", name)
	}
	tgt := cfg.Items[i]
	if cfg.Use == tgt.Name && !force {
		return fmt.Errorf("%s is currently in use", cfg.Use)
	}

	if err := os.Remove(tgt.File); err != nil {
		return err
	}
	v.Set("items", slices.Delete(cfg.Items, i, i+1))
	return v.WriteConfig()
}
