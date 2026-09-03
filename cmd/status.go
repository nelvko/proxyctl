package cmd

import (
	"fmt"
	"strings"

	"github.com/nelvko/proxyctl/internal/app"
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
		a, err := app.Load()
		if err != nil {
			return err
		}

		kcfg := a.Config.ActiveKernel()
		if kcfg == nil {
			fmt.Println("kernel:    none (run `proxyctl install`)")
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
		if a.Subscriptions.Use != "" {
			profile = fmt.Sprintf("%s (%d total)", a.Subscriptions.Use, len(a.Subscriptions.Profiles))
		}
		fmt.Printf("profile:   %s\n", profile)
		return nil
	},
}

func init() {
	RootCmd.AddCommand(statusCmd)
}
