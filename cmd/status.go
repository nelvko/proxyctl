package cmd

import (
	"fmt"
	"strings"

	"github.com/nelvko/proxyctl/internal/config"
	"github.com/nelvko/proxyctl/internal/env"
	pkernel "github.com/nelvko/proxyctl/internal/kernel"
	"github.com/spf13/cobra"
)

// statusCmd shows a cross-domain overview. It never prompts for a kernel
// install: no kernel is a valid, displayable state.
var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show proxy status",
	Long: `Show the active kernel, service state, proxy endpoints
and the current subscription.`,
	Annotations: map[string]string{"skipRuntime": "true"},
	RunE: func(cmd *cobra.Command, args []string) error {
		appCfg, err := config.LoadAppConfig()
		if err != nil {
			return err
		}

		kcfg := appCfg.ActiveKernel()
		if kcfg == nil {
			fmt.Println("kernel:    none (run `proxyctl kernel install`)")
			return nil
		}
		fmt.Printf("kernel:    %s\n", kcfg.Name)

		fmt.Print("service:   ")
		if k, err := pkernel.New(kcfg); err != nil {
			fmt.Printf("unknown (%v)\n", err)
		} else if on, err := k.IsActive(); err == nil && on {
			fmt.Println("running (user)")
		} else {
			fmt.Println("stopped")
		}

		fmt.Print("proxy:     ")
		if e, err := env.Resolve(kcfg.ConfigFile); err == nil {
			var endpoints []string
			for _, v := range []string{e.HTTP, e.All} {
				if v != "" {
					endpoints = append(endpoints, v)
				}
			}
			fmt.Println(strings.Join(endpoints, "  "))
		} else {
			fmt.Println("-")
		}

		profile := "none"
		if subCfg, err := config.LoadSubConfig(); err == nil && subCfg.Use != "" {
			profile = fmt.Sprintf("%s (%d total)", subCfg.Use, len(subCfg.Profiles))
		}
		fmt.Printf("profile:   %s\n", profile)
		return nil
	},
}

func init() {
	RootCmd.AddCommand(statusCmd)
}
