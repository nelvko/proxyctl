package kernel

import (
	"github.com/spf13/cobra"
)

// NewCommands builds top-level operations on proxyctl's default object.
func NewCommands() []*cobra.Command {
	commands := []*cobra.Command{
		newInstallCmd(),
		newUninstallCmd(),
		newUpgradeCmd(),
		newListCmd(),
		newUseCmd()}
	for _, cmd := range commands {
		cmd.GroupID = "kernel"
		switch cmd.Name() {
		case "use", "uninstall", "upgrade":
			cmd.ValidArgsFunction = completeInstalled
		}
	}
	for _, action := range []string{"start", "stop", "restart"} {
		cmd := newServiceCmd(action)
		cmd.GroupID = "service"
		commands = append(commands, cmd)
	}
	return commands
}
