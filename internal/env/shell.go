package env

import (
	"fmt"
	"os"
	"strings"
)

// Shell reports the calling shell family. The shell integration exports
// PROXYCTL_SHELL, which wins over marker detection (FISH_VERSION is not
// exported by fish, so detection alone misses it).
func Shell() string {
	switch s := os.Getenv("PROXYCTL_SHELL"); s {
	case "bash", "zsh", "fish":
		return s
	}
	switch {
	case os.Getenv("FISH_VERSION") != "":
		return "fish"
	case os.Getenv("ZSH_VERSION") != "":
		return "zsh"
	default:
		return "bash"
	}
}

// Export renders statements that apply the environment in the given shell
// (bash and zsh share POSIX syntax).
func (e ProxyEnv) Export(shell string) []string {
	var lines []string
	add := func(key, value string) {
		if value == "" {
			if shell == "fish" {
				lines = append(lines, fmt.Sprintf("set -e %s", key), fmt.Sprintf("set -e %s", lower(key)))
			} else {
				lines = append(lines, fmt.Sprintf("unset %s", key), fmt.Sprintf("unset %s", lower(key)))
			}
			return
		}
		// Single quotes: no parameter or command expansion in any of the
		// three shells, so values containing $ or ` survive eval verbatim.
		if shell == "fish" {
			lines = append(lines, fmt.Sprintf("set -gx %s %s", key, shQuote(value)))
			lines = append(lines, fmt.Sprintf("set -gx %s %s", lower(key), shQuote(value)))
			return
		}
		lines = append(lines, fmt.Sprintf("export %s=%s", key, shQuote(value)))
		lines = append(lines, fmt.Sprintf("export %s=%s", lower(key), shQuote(value)))
	}
	add("HTTP_PROXY", e.HTTP)
	add("HTTPS_PROXY", e.HTTPS)
	add("ALL_PROXY", e.All)
	add("NO_PROXY", e.NO)
	return lines
}

// Unset renders statements that remove the environment.
func (e ProxyEnv) Unset(shell string) []string {
	var lines []string
	for _, key := range []string{"HTTP_PROXY", "HTTPS_PROXY", "ALL_PROXY", "NO_PROXY"} {
		if shell == "fish" {
			lines = append(lines, fmt.Sprintf("set -e %s", key), fmt.Sprintf("set -e %s", lower(key)))
			continue
		}
		lines = append(lines, fmt.Sprintf("unset %s", key), fmt.Sprintf("unset %s", lower(key)))
	}
	if shell == "fish" {
		// Erasing an absent variable returns nonzero in fish; cleanup is idempotent.
		lines = append(lines, "true")
	}
	return lines
}

func lower(s string) string {
	return strings.ToLower(s)
}

// shQuote wraps value in single quotes, escaping embedded quotes with the
// '\' trick that bash, zsh and fish all understand.
func shQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", `'\''`) + "'"
}
