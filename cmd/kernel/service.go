package kernel

import (
	"os"

	"github.com/spf13/cobra"
	"golang.org/x/term"
)

func newServiceCmd(action string) *cobra.Command {
	return &cobra.Command{
		Use: action,
		Short: map[string]string{
			"start":   "Start the selected kernel service",
			"stop":    "Stop the selected kernel service for all shells",
			"restart": "Restart the selected kernel service",
		}[action],
		Long: "Control the selected kernel's shared user service. This does not change shell environment variables.",
		Args: cobra.NoArgs,
		RunE: mutation(func(cmd *cobra.Command, args []string) error {
			a, err := loadApp(cmd)
			if err != nil {
				return err
			}
			k, err := a.Kernel()
			if err != nil {
				return err
			}
			running, err := k.IsActive()
			if err != nil {
				return err
			}
			if (action == "start" && running) || (action == "stop" && !running) {
				state := "stopped"
				if running {
					state = "running"
				}
				cmd.Printf("kernel %q: already %s\n", a.Config.Use, state)
				return nil
			}
			if f, ok := cmd.ErrOrStderr().(*os.File); ok && term.IsTerminal(int(f.Fd())) {
				cmd.PrintErrf("kernel %q: %s in progress...\n", a.Config.Use, action)
			}
			if err := a.ControlKernel(cmd.Context(), action); err != nil {
				return err
			}
			cmd.Printf("kernel %q: %s completed\n", a.Config.Use, action)
			return nil
		}),
	}
}
