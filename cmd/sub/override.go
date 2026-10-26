package sub

import (
	"errors"
	"fmt"

	"github.com/nelvko/proxyctl/internal/app"
	"github.com/nelvko/proxyctl/internal/subscription"
	"github.com/nelvko/proxyctl/internal/ui"
	"github.com/spf13/cobra"
)

const overrideMergeHelp = `Overrides are configuration objects merged in this order:
source configuration, global override for its format, then profile override.
Objects merge recursively; arrays replace the whole array; scalars and null
replace the previous value. Null is a literal value, not a deletion command.`

func newOverrideCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "override",
		Short: "Manage global and per-profile configuration overrides",
		Long: `Manage a profile override by name or a format-specific global override
with --global. Every operation requires an explicit target.

Saving or clearing overrides validates compatible profiles and leaves the
active configuration unchanged. A global patch without compatible profiles
receives syntax validation until a profile is applied. Run sub use to apply
the result.

` + overrideMergeHelp,
		Args: cobra.NoArgs,
	}
	cmd.AddCommand(newOverrideEditCmd(), newOverrideShowCmd(), newOverrideClearCmd())
	return cmd
}

func overrideArgs(cmd *cobra.Command, args []string) error {
	if err := cobra.MaximumNArgs(1)(cmd, args); err != nil {
		return err
	}
	global, _ := cmd.Flags().GetBool("global")
	if global && len(args) != 0 {
		return errors.New("a profile name and --global are mutually exclusive")
	}
	if len(args) == 1 && args[0] == "" {
		return errors.New("profile name cannot be empty")
	}
	if !global && len(args) == 0 {
		return errors.New("specify a profile name or --global")
	}
	return nil
}

func completeOverrideProfiles(cmd *cobra.Command, args []string, prefix string) ([]string, cobra.ShellCompDirective) {
	if global, _ := cmd.Flags().GetBool("global"); global {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	return completeProfiles(cmd, args, prefix)
}

func newOverrideEditCmd() *cobra.Command {
	var global bool
	var editor string
	cmd := &cobra.Command{
		Use:   "edit [name]",
		Short: "Edit and validate an override without applying it",
		Long: `Edit an override using --editor or $EDITOR. Supply a profile name or
--global for the selected kernel's configuration format.

Edits are staged separately; an editor or validation failure preserves the
saved override. With no compatible profiles, a global override receives syntax
validation now and kernel validation when a profile is applied.

` + overrideMergeHelp,
		Args:              overrideArgs,
		ValidArgsFunction: completeOverrideProfiles,
		RunE: mutation(func(cmd *cobra.Command, args []string) error {
			svc, err := app.LoadRuntimeProfiles(cmd.Context(), ui.Reporter(cmd.ErrOrStderr()))
			if err != nil {
				return err
			}
			name := ""
			if len(args) == 1 {
				name = args[0]
			}
			if err := ui.EditOverride(cmd.Context(), svc, name, global, editor, cmd.InOrStdin(), cmd.OutOrStdout(), cmd.ErrOrStderr()); err != nil {
				return err
			}
			if global && len(svc.Compatible()) == 0 {
				cmd.Println("no compatible profiles; override syntax checked, final configuration will be validated when a profile is applied")
			}
			printOverrideChange(cmd, name, global, "saved")
			return nil
		}),
	}
	cmd.Flags().BoolVar(&global, "global", false, "Edit the global override for the selected kernel's configuration format")
	cmd.Flags().StringVarP(&editor, "editor", "e", "", "Editor command")
	return cmd
}

func newOverrideShowCmd() *cobra.Command {
	var global bool
	cmd := &cobra.Command{
		Use:   "show [name]",
		Short: "Print a saved override, including its raw configuration values",
		Long: `Print the saved override for a profile name or --global for the selected
kernel's configuration format. Override content is printed as saved and may
include credentials. No service is started or configuration applied.`,
		Args:              overrideArgs,
		ValidArgsFunction: completeOverrideProfiles,
		RunE: func(cmd *cobra.Command, args []string) error {
			var svc *subscription.Service
			var err error
			if global {
				svc, err = app.LoadRuntimeProfiles(cmd.Context(), ui.Reporter(cmd.ErrOrStderr()))
			} else {
				svc, err = app.LoadProfileMetadata(cmd.Context())
			}
			if err != nil {
				return err
			}
			name := ""
			if len(args) == 1 {
				name = args[0]
			}
			data, err := svc.ReadOverride(name, global)
			if err != nil {
				return err
			}
			if data == nil {
				if global {
					cmd.Println("no global override for the selected kernel's configuration format")
				} else {
					cmd.Printf("no override for profile %q\n", name)
				}
				return nil
			}
			_, err = cmd.OutOrStdout().Write(data)
			return err
		},
	}
	cmd.Flags().BoolVar(&global, "global", false, "Show the global override for the selected kernel's configuration format")
	return cmd
}

func newOverrideClearCmd() *cobra.Command {
	var global bool
	cmd := &cobra.Command{
		Use:   "clear [name]",
		Short: "Clear an override without changing the active configuration",
		Long: `Clear the saved override for a profile name or --global for the selected
kernel's configuration format. Compatible profiles are validated before
removing the override. The active file and running service remain unchanged;
run sub use to apply the result.`,
		Args:              overrideArgs,
		ValidArgsFunction: completeOverrideProfiles,
		RunE: mutation(func(cmd *cobra.Command, args []string) error {
			svc, err := app.LoadRuntimeProfiles(cmd.Context(), ui.Reporter(cmd.ErrOrStderr()))
			if err != nil {
				return err
			}
			name := ""
			if len(args) == 1 {
				name = args[0]
			}
			if err := svc.ClearOverride(name, global); err != nil {
				return err
			}
			printOverrideChange(cmd, name, global, "cleared")
			return nil
		}),
	}
	cmd.Flags().BoolVar(&global, "global", false, "Clear the global override for the selected kernel's configuration format")
	return cmd
}

func printOverrideChange(cmd *cobra.Command, name string, global bool, action string) {
	target := fmt.Sprintf("override for profile %q", name)
	applyName := profileNameArg(name)
	if global {
		target = "global override for the selected configuration format"
		applyName = "<name>"
	}
	cmd.Printf("%s %s; active configuration unchanged; run `proxyctl sub use %s` to apply\n", target, action, applyName)
}
