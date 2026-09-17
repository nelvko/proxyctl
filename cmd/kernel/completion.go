package kernel

import (
	"strings"

	"github.com/nelvko/proxyctl/internal/config"
	"github.com/spf13/cobra"
)

func completeInstalled(cmd *cobra.Command, args []string, prefix string) ([]string, cobra.ShellCompDirective) {
	if len(args) > 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	cfg, err := config.Load()
	if err != nil {
		return nil, cobra.ShellCompDirectiveError
	}
	var names []string
	for _, k := range cfg.Kernels {
		if strings.HasPrefix(k.Name, prefix) {
			names = append(names, k.Name)
		}
	}
	return names, cobra.ShellCompDirectiveNoFileComp
}
