package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

// initCmd emits the shell integration: a proxyctl() wrapper so that
// shellEval commands (on/off) apply to the current shell.
var initCmd = &cobra.Command{
	Use:   "init [shell]",
	Short: "Print the shell integration script",
	Long: `Print the shell integration script.

Add the integration to your shell:

    eval "$(proxyctl init bash)"     # bash, in .bashrc
    eval "$(proxyctl init zsh)"      # zsh, in .zshrc
    proxyctl init fish | source      # fish, in config.fish

Without an argument the shell is detected from the environment.`,
	Args:      cobra.MaximumNArgs(1),
	ValidArgs: []string{"bash", "zsh", "fish"},
	RunE: func(cmd *cobra.Command, args []string) error {
		shell := ""
		if len(args) == 1 {
			shell = args[0]
		} else {
			shell = defaultShell()
		}

		var script string
		switch shell {
		case "bash", "zsh":
			script = posixScript(shell)
		case "fish":
			script = fishScript()
		default:
			return fmt.Errorf("unsupported shell %q, available: bash, zsh, fish", shell)
		}

		fmt.Print(script)
		return nil
	},
}

func init() {
	RootCmd.AddCommand(initCmd)
}

func defaultShell() string {
	base := filepath.Base(strings.TrimSpace(os.Getenv("SHELL")))
	switch base {
	case "fish":
		return "fish"
	case "zsh":
		return "zsh"
	default:
		return "bash"
	}
}

// evalCommands collects the commands whose stdout must be eval'd, so new
// shellEval commands join the wrapper without users reinstalling it.
func evalCommands() []string {
	var names []string
	for _, c := range RootCmd.Commands() {
		if c.Annotations["shellEval"] == "true" {
			names = append(names, c.Name())
		}
	}
	return names
}

func posixScript(shell string) string {
	return fmt.Sprintf(`# proxyctl shell integration (%s) — eval "$(proxyctl init %s)"
export PROXYCTL_WRAPPED=1
export PROXYCTL_SHELL=%s
proxyctl() {
	# Exactly one argument: a bare on/off, whose stdout is shell code.
	# Anything else (--help, extra args) runs directly — its stdout is
	# not shell code and must never be eval'd.
	case "$1" in
		%s)
			if [ "$#" -ne 1 ]; then
				command proxyctl "$@"
				return
			fi
			local __proxyctl_out
			__proxyctl_out="$(command proxyctl "$@")" || return $?
			eval "$__proxyctl_out"
			;;
		*)
			command proxyctl "$@"
			;;
	esac
}
`, shell, shell, shell, strings.Join(evalCommands(), "|"))
}

func fishScript() string {
	return fmt.Sprintf(`# proxyctl shell integration (fish) — proxyctl init fish | source
set -gx PROXYCTL_WRAPPED 1
set -gx PROXYCTL_SHELL fish
function proxyctl
	switch $argv[1]
		case %s
			if test (count $argv) -ne 1
				command proxyctl $argv
				return
			end
			command proxyctl $argv | source
			return $pipestatus[1]
		case '*'
			command proxyctl $argv
	end
end
`, strings.Join(evalCommands(), " "))
}
