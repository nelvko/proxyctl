package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

// initCmd emits the shell integration so on/off apply to the current shell.
func newInitCmd() *cobra.Command {
	command := &cobra.Command{
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
		GroupID:   "shell",
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

			cmd.Print(script)
			return nil
		},
	}
	return command
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

func posixScript(shell string) string {
	return fmt.Sprintf(`# proxyctl shell integration (%s) — eval "$(proxyctl init %s)"
export PROXYCTL_WRAPPED=1
export PROXYCTL_SHELL=%s
proxyctl() {
	# Evaluate only supported shell actions; help and other output pass through.
	local __proxyctl_apply_on_error=0
	case "${1-}" in
		on)
			if [ "$#" -ne 1 ]; then
				command proxyctl "$@"
				return
			fi
			;;
		off)
			local __proxyctl_arg
			for __proxyctl_arg in "${@:2}"; do
				case "$__proxyctl_arg" in
					--stop|--stop=1|--stop=t|--stop=T|--stop=true|--stop=TRUE|--stop=True) __proxyctl_apply_on_error=1 ;;
					--stop=0|--stop=f|--stop=F|--stop=false|--stop=FALSE|--stop=False) __proxyctl_apply_on_error=0 ;;
					--) ;;
					*) command proxyctl "$@"; return ;;
				esac
			done
			;;
		*)
			command proxyctl "$@"
			return
			;;
	esac
	local __proxyctl_out __proxyctl_status=0
	__proxyctl_out="$(command proxyctl "$@")" || __proxyctl_status=$?
	if [ "$__proxyctl_status" -ne 0 ] && [ "$__proxyctl_apply_on_error" -ne 1 ]; then
		return "$__proxyctl_status"
	fi
	eval "$__proxyctl_out" || return $?
	return "$__proxyctl_status"
}
`, shell, shell, shell)
}

func fishScript() string {
	return `# proxyctl shell integration (fish) — proxyctl init fish | source
set -gx PROXYCTL_WRAPPED 1
set -gx PROXYCTL_SHELL fish
function proxyctl
	set -l __proxyctl_apply_on_error 0
	switch "$argv[1]"
		case on
			if test (count $argv) -ne 1
				command proxyctl $argv
				return
			end
		case off
			for __proxyctl_arg in $argv[2..-1]
				switch $__proxyctl_arg
					case --stop --stop=1 --stop=t --stop=T --stop=true --stop=TRUE --stop=True
						set __proxyctl_apply_on_error 1
					case --stop=0 --stop=f --stop=F --stop=false --stop=FALSE --stop=False
						set __proxyctl_apply_on_error 0
					case --
					case '*'
						command proxyctl $argv
						return
				end
			end
		case '*'
			command proxyctl $argv
			return
	end
	set -l __proxyctl_out (command proxyctl $argv)
	set -l __proxyctl_status $status
	if test $__proxyctl_status -ne 0; and test $__proxyctl_apply_on_error -ne 1
		return $__proxyctl_status
	end
	printf '%s\n' $__proxyctl_out | source
	set -l __proxyctl_eval_status $status
	if test $__proxyctl_eval_status -ne 0
		return $__proxyctl_eval_status
	end
	return $__proxyctl_status
end
`
}
