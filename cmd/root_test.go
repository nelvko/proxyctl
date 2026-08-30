package cmd

import (
	"testing"

	"github.com/spf13/cobra"
)

func TestNeedsRuntime(t *testing.T) {
	root := &cobra.Command{Use: "proxyctl"}

	kernelGroup := &cobra.Command{Use: "kernel"}
	kernelGroup.Annotations = map[string]string{"skipRuntime": "true"}
	kernelInstall := &cobra.Command{Use: "install"}
	kernelGroup.AddCommand(kernelInstall)

	onCmd := &cobra.Command{Use: "on"}
	subCmd := &cobra.Command{Use: "sub"}
	subUse := &cobra.Command{Use: "use"}
	subCmd.AddCommand(subUse)

	completion := &cobra.Command{Use: "completion"}
	completionBash := &cobra.Command{Use: "bash"}
	completion.AddCommand(completionBash)

	man := &cobra.Command{Use: "man"} // injected by fang at runtime
	help := &cobra.Command{Use: "help"}
	complete := &cobra.Command{Use: "__complete"}

	root.AddCommand(kernelGroup, onCmd, subCmd, completion, man, help, complete)

	cases := []struct {
		name string
		cmd  *cobra.Command
		want bool
	}{
		{"bare root", root, false},
		{"kernel group", kernelGroup, false},
		{"kernel install (annotated ancestor)", kernelInstall, false},
		{"on", onCmd, true},
		{"sub group", subCmd, true},
		{"sub use", subUse, true},
		{"completion", completion, false},
		{"completion bash", completionBash, false},
		{"man", man, false},
		{"help", help, false},
		{"__complete", complete, false},
	}
	for _, tc := range cases {
		if got := needsRuntime(root, tc.cmd); got != tc.want {
			t.Errorf("needsRuntime(%s) = %v, want %v", tc.name, got, tc.want)
		}
	}
}
