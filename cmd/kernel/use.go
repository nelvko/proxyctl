package kernel

import (
	"errors"
	"fmt"
	"strings"

	"charm.land/huh/v2"
	"github.com/spf13/cobra"
)

func newUseCmd() *cobra.Command {
	return &cobra.Command{
		Use: "use [name]", Short: "Select an installed kernel",
		Long: `Select an installed kernel; without a name, choose interactively.
Apply the selected subscription. If the previous kernel was running, move the
running session to the new one; otherwise keep the service stopped.
Shell environment variables are unchanged.`,
		Args: func(cmd *cobra.Command, args []string) error {
			if err := cobra.MaximumNArgs(1)(cmd, args); err != nil {
				return err
			}
			if len(args) == 0 && !isInteractive() {
				return errors.New("a kernel name is required; use proxyctl list, then proxyctl use <name>")
			}
			return nil
		},
		RunE: mutation(func(cmd *cobra.Command, args []string) error {
			a, err := loadApp(cmd)
			if err != nil {
				return err
			}
			if len(a.Config.Kernels) == 0 {
				return errors.New("no installed kernels; run proxyctl install first")
			}
			var name string
			if len(args) > 0 {
				name = args[0]
			} else {
				options := make([]huh.Option[string], 0, len(a.Config.Kernels))
				for _, k := range a.Config.Kernels {
					label := k.Name
					if k.Name == a.Config.Use {
						label += " (selected)"
					}
					options = append(options, huh.NewOption(label, k.Name))
				}
				name = a.Config.Use
				if err := huh.NewForm(huh.NewGroup(huh.NewSelect[string]().Title("Select an installed kernel").Options(options...).Value(&name))).Run(); err != nil {
					if errors.Is(err, huh.ErrUserAborted) {
						cmd.PrintErrln("cancelled")
						return nil
					}
					return err
				}
			}
			if a.Config.KernelByName(name) == nil {
				names := make([]string, 0, len(a.Config.Kernels))
				for _, k := range a.Config.Kernels {
					names = append(names, k.Name)
				}
				return fmt.Errorf("kernel %q is not installed; installed kernels: %s", name, strings.Join(names, ", "))
			}
			if err := a.UseKernel(cmd.Context(), name); err != nil {
				return err
			}
			if a.Subscriptions.Use == "" {
				cmd.Printf("kernel %q selected; no subscription selected (run proxyctl sub use <name> or sub add <url> --use)\n", name)
			} else {
				cmd.Printf("kernel %q selected\n", name)
			}
			return nil
		}),
	}
}
