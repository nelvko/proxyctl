package sub

import (
	"slices"
	"strings"

	"github.com/nelvko/proxyctl/internal/config"
	"github.com/spf13/cobra"
)

func completeProfiles(cmd *cobra.Command, args []string, prefix string) ([]string, cobra.ShellCompDirective) {
	if cmd.Name() != "update" && len(args) > 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	if all, _ := cmd.Flags().GetBool("all"); all {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	cfg, err := config.LoadSubscriptionConfig()
	if err != nil {
		return nil, cobra.ShellCompDirectiveError
	}
	var names []string
	for _, p := range cfg.Profiles {
		if strings.HasPrefix(p.Name, prefix) && !slices.Contains(args, p.Name) {
			names = append(names, p.Name)
		}
	}
	return names, cobra.ShellCompDirectiveNoFileComp
}
