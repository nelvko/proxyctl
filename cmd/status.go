package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/nelvko/proxyctl/internal/redact"
	"github.com/spf13/cobra"
)

func newStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show the selected kernel, shared service, subscription and shell state",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			a, err := loadApp(cmd)
			if err != nil {
				return err
			}
			if cfg := a.Config.SelectedKernel(); cfg == nil {
				cmd.Println("kernel:    none (run `proxyctl install` or `proxyctl use <name>`)")
				cmd.Println("service:   -")
				cmd.Println("proxy:     -")
			} else {
				cmd.Printf("kernel:    %s\n", cfg.Name)
				k, err := a.Kernel()
				if err != nil {
					cmd.Printf("service:   unknown (%v)\n", err)
				} else {
					running, err := k.IsActive()
					switch {
					case err != nil:
						cmd.Printf("service:   unknown (%v)\n", err)
					case running:
						cmd.Println("service:   running (user)")
					default:
						cmd.Println("service:   stopped")
					}
				}
				if k != nil {
					if e, err := k.ProxyEndpoints(); err == nil {
						var endpoints []string
						for _, v := range []string{e.HTTP, e.SOCKS} {
							if v != "" {
								endpoints = append(endpoints, redact.URL(v))
							}
						}
						cmd.Printf("proxy:     %s\n", strings.Join(endpoints, "  "))
					} else {
						cmd.Println("proxy:     -")
					}
				} else {
					cmd.Println("proxy:     -")
				}
			}
			var variables []string
			for _, key := range []string{"HTTP_PROXY", "http_proxy", "HTTPS_PROXY", "https_proxy", "ALL_PROXY", "all_proxy"} {
				if os.Getenv(key) != "" {
					variables = append(variables, key)
				}
			}
			if len(variables) == 0 {
				cmd.Println("shell:     no proxy variables set")
			} else {
				cmd.Printf("shell:     %s set\n", strings.Join(variables, ", "))
			}
			if err := a.LoadSubscriptions(); err != nil {
				cmd.Printf("profile:   unknown (%v)\n", err)
				return nil
			}
			profile := "none"
			if a.Subscriptions.Use != "" {
				profile = fmt.Sprintf("%s (%d total)", a.Subscriptions.Use, len(a.Subscriptions.Profiles))
			}
			cmd.Printf("profile:   %s\n", profile)
			return nil
		},
	}
}
