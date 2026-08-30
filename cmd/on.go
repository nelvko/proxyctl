package cmd

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"time"

	"github.com/nelvko/proxyctl/internal/env"
	"github.com/spf13/cobra"
)

// onCmd represents the on command
var onCmd = &cobra.Command{
	Use:   "on",
	Short: "Enable the proxy environment in the current shell",
	Long: `Enable the proxy environment in the current shell.

Starts the kernel service if needed, then prints shell statements
that apply the proxy environment to stdout. With the shell
integration (eval "$(proxyctl init bash)"), they apply to the
current shell; the kernel config decides the actual address.`,
	GroupID:     manageGroup.ID,
	Annotations: map[string]string{"shellEval": "true"},
	RunE: func(cmd *cobra.Command, args []string) error {
		k, err := App().Kernel()
		if err != nil {
			return err
		}

		if active, err := k.IsActive(); err != nil {
			return err
		} else if !active {
			if err := k.Start(); err != nil {
				return err
			}
		}

		e, err := env.Resolve(k.ConfigFile())
		if err != nil {
			return err
		}
		// The unit is Type=simple: systemd reports active as soon as the
		// process is forked, so probe the actual endpoint instead.
		if err := waitReachable(e); err != nil {
			return err
		}

		printShell(e.Export(env.Shell()))
		fmt.Fprintf(os.Stderr, "😼 proxy on: %s\n", firstNonEmpty(e.HTTP, e.All))
		return nil
	},
}

func init() {
	RootCmd.AddCommand(onCmd)
}

// waitReachable retries until at least one resolved proxy endpoint accepts
// TCP connections.
func waitReachable(e env.ProxyEnv) error {
	for _, addr := range []string{e.HTTP, e.All} {
		if addr == "" {
			continue
		}
		for range 10 {
			u, err := url.Parse(addr)
			if err != nil {
				return err
			}
			conn, err := net.DialTimeout("tcp", u.Host, 500*time.Millisecond)
			if err == nil {
				conn.Close()
				return nil
			}
			time.Sleep(300 * time.Millisecond)
		}
	}
	return fmt.Errorf("proxy endpoint not reachable: %s", firstNonEmpty(e.HTTP, e.All))
}

// printShell writes shell statements to stdout — the only thing allowed
// there for shellEval commands.
func printShell(lines []string) {
	for _, line := range lines {
		fmt.Println(line)
	}
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}
