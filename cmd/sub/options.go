package sub

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/nelvko/proxyctl/internal/subscription"
	"github.com/spf13/cobra"
)

type downloadFlags struct {
	route, userAgent       string
	timeout                time.Duration
	insecure               bool
	headers, removeHeaders []string
}

func (f *downloadFlags) bind(cmd *cobra.Command, remove bool) {
	cmd.Flags().StringVar(&f.route, "download-via", "env", "Download route: env (HTTP(S)_PROXY/NO_PROXY), direct, or kernel (required)")
	cmd.Flags().StringVar(&f.userAgent, "user-agent", "", "HTTP User-Agent; empty restores the default")
	cmd.Flags().DurationVarP(&f.timeout, "timeout", "t", 0, fmt.Sprintf("Total HTTP(S) download budget including retries, excluding validation; 0 uses %s", subscription.DefaultDownloadTimeout))
	cmd.Flags().BoolVar(&f.insecure, "insecure", false, "Skip HTTPS subscription certificate verification; false verifies certificates")
	cmd.Flags().StringArrayVar(&f.headers, "header", nil, "HTTP request header as 'Name: value'; repeat for different names")
	if remove {
		cmd.Flags().StringArrayVar(&f.removeHeaders, "remove-header", nil, "Remove a saved request header by name (case insensitive)")
	}
	_ = cmd.RegisterFlagCompletionFunc("download-via", func(_ *cobra.Command, _ []string, prefix string) ([]string, cobra.ShellCompDirective) {
		var routes []string
		for _, route := range []string{"env", "direct", "kernel"} {
			if strings.HasPrefix(route, prefix) {
				routes = append(routes, route)
			}
		}
		return routes, cobra.ShellCompDirectiveNoFileComp
	})
}

func (f *downloadFlags) changed(cmd *cobra.Command) bool {
	for _, name := range []string{"download-via", "user-agent", "timeout", "insecure", "header", "remove-header"} {
		if cmd.Flags().Changed(name) {
			return true
		}
	}
	return false
}

func (f *downloadFlags) options(cmd *cobra.Command) (subscription.DownloadOptions, error) {
	o := subscription.DownloadOptions{RemoveHeaders: f.removeHeaders}
	var err error
	if o.Headers, err = subscription.ParseHeaders(f.headers); err != nil {
		return o, err
	}
	if cmd.Flags().Changed("user-agent") {
		if _, ok := o.Headers["User-Agent"]; ok {
			return o, errors.New("--user-agent conflicts with --header User-Agent")
		}
		o.UserAgent = &f.userAgent
	}
	if cmd.Flags().Changed("download-via") {
		if f.route == "" {
			return o, errors.New("--download-via must be env, direct or kernel")
		}
		o.Route = &f.route
	}
	if cmd.Flags().Changed("timeout") {
		o.Timeout = &f.timeout
	}
	if cmd.Flags().Changed("insecure") {
		o.Insecure = &f.insecure
	}
	for _, name := range f.removeHeaders {
		for set := range o.Headers {
			if strings.EqualFold(name, set) {
				return o, errors.New("cannot set and remove the same header")
			}
		}
	}
	return o, nil
}
