package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/nelvko/proxyctl/internal/app"
	"github.com/nelvko/proxyctl/internal/kernel"
	"github.com/nelvko/proxyctl/internal/redact"
	"github.com/spf13/cobra"
)

func newStatusCmd() *cobra.Command {
	var jsonOutput bool
	command := &cobra.Command{
		Use:   "status",
		Short: "Show the selected kernel, shared service, subscription and shell state",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			a, err := loadApp(cmd)
			if err != nil {
				return err
			}
			if jsonOutput {
				return json.NewEncoder(cmd.OutOrStdout()).Encode(readStatusJSON(a))
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
	command.Flags().BoolVar(&jsonOutput, "json", false, "Output status as JSON")
	return command
}

type statusJSON struct {
	Kernel struct {
		State string  `json:"state"`
		Name  *string `json:"name"`
	} `json:"kernel"`
	Service statusServiceJSON `json:"service"`
	Proxy   statusProxyJSON   `json:"proxy"`
	Shell   struct {
		ProxyVariables []string `json:"proxy_variables"`
	} `json:"shell"`
	Profile struct {
		State string  `json:"state"`
		Name  *string `json:"name"`
		Total *int    `json:"total"`
		Error *string `json:"error"`
	} `json:"profile"`
}

type statusServiceJSON struct {
	State string  `json:"state"`
	Scope string  `json:"scope"`
	Error *string `json:"error"`
}

type statusProxyJSON struct {
	State string  `json:"state"`
	HTTP  *string `json:"http"`
	SOCKS *string `json:"socks"`
	Error *string `json:"error"`
}

func readStatusJSON(a *app.App) statusJSON {
	var result statusJSON
	result.Kernel.State = "none"
	result.Service = statusServiceJSON{State: "unavailable", Scope: "user"}
	result.Proxy.State = "unavailable"
	if cfg := a.Config.SelectedKernel(); cfg != nil {
		result.Kernel.State = "selected"
		result.Kernel.Name = statusText(cfg.Name)
		k, err := a.Kernel()
		result.Service, result.Proxy = readStatusRuntime(k, err)
	}
	result.Shell.ProxyVariables = []string{}
	for _, key := range []string{"HTTP_PROXY", "http_proxy", "HTTPS_PROXY", "https_proxy", "ALL_PROXY", "all_proxy", "NO_PROXY", "no_proxy"} {
		if os.Getenv(key) != "" {
			result.Shell.ProxyVariables = append(result.Shell.ProxyVariables, key)
		}
	}
	if err := a.LoadSubscriptions(); err != nil {
		result.Profile.State = "unknown"
		result.Profile.Error = statusText(err.Error())
	} else {
		result.Profile.State = "none"
		result.Profile.Name = statusText(a.Subscriptions.Use)
		if result.Profile.Name != nil {
			result.Profile.State = "selected"
		}
		total := len(a.Subscriptions.Profiles)
		result.Profile.Total = &total
	}
	return result
}

// statusRuntime only permits the read-only queries needed by status.
type statusRuntime interface {
	IsActive() (bool, error)
	ProxyEndpoints() (kernel.Endpoints, error)
}

func readStatusRuntime(k statusRuntime, kernelErr error) (statusServiceJSON, statusProxyJSON) {
	service := statusServiceJSON{State: "unknown", Scope: "user"}
	proxy := statusProxyJSON{State: "unknown"}
	if kernelErr != nil {
		service.Error = statusText(kernelErr.Error())
		proxy.Error = statusText(kernelErr.Error())
		return service, proxy
	}
	running, err := k.IsActive()
	switch {
	case err != nil:
		service.Error = statusText(err.Error())
	case running:
		service.State = "running"
	default:
		service.State = "stopped"
	}
	endpoints, err := k.ProxyEndpoints()
	if err != nil {
		proxy.Error = statusText(err.Error())
	} else {
		proxy.State = "none"
		if endpoints.HTTP != "" {
			proxy.HTTP = statusText(redact.URL(endpoints.HTTP))
			proxy.State = "available"
		}
		if endpoints.SOCKS != "" {
			proxy.SOCKS = statusText(redact.URL(endpoints.SOCKS))
			proxy.State = "available"
		}
	}
	return service, proxy
}

func statusText(raw string) *string {
	if raw == "" {
		return nil
	}
	text := redact.Text(raw)
	return &text
}
