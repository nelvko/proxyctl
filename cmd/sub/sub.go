package sub

import (
	"errors"
	"fmt"

	"charm.land/huh/v2"
	"github.com/nelvko/proxyctl/internal/app"
	"github.com/nelvko/proxyctl/internal/subscription"
	"github.com/nelvko/proxyctl/internal/ui"
	"github.com/spf13/cobra"
)

// NewCommand builds the subscription command group.
func NewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Aliases: []string{"subscription"},
		Use:     "sub",
		Short:   "Manage subscription profiles",
		Long:    "Manage subscription profiles. Run without a subcommand to open the interactive manager.",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if !isInteractive() {
				return errors.New("the subscription manager requires a terminal; use `proxyctl sub list` to list profiles")
			}
			svc, err := app.LoadProfileMetadata(cmd.Context())
			if err != nil {
				return err
			}
			return ui.RunSubscriptions(cmd.Context(), svc)
		},
	}
	cmd.AddCommand(newAddCmd(), newListCmd(), newUseCmd(), newEditCmd(), newRemoveCmd(), newUpdateCmd(), newModifyCmd(), newShowCmd())
	for _, child := range cmd.Commands() {
		switch child.Name() {
		case "use", "update", "edit", "remove", "modify", "show":
			child.ValidArgsFunction = completeProfiles
		}
	}
	return cmd
}

// An omitted name prompts only for the target of this specific operation.
func selectProfile(cmd *cobra.Command, args []string, svc *subscription.Service) (string, error) {
	if len(args) == 1 {
		return args[0], nil
	}
	if !isInteractive() {
		return "", fmt.Errorf("a profile name is required; run `%s <name>`", cmd.CommandPath())
	}
	profiles := svc.List()
	if cmd.Name() == "use" || cmd.Name() == "edit" {
		profiles = svc.Compatible()
	}
	options := make([]huh.Option[string], 0, len(profiles))
	for _, p := range profiles {
		options = append(options, huh.NewOption(p.Name, p.Name))
	}
	if len(options) == 0 {
		return "", errors.New("no profiles; run `proxyctl sub add <url>` first")
	}
	var name string
	err := huh.NewForm(huh.NewGroup(huh.NewSelect[string]().Title("Select a profile to " + cmd.Name()).Options(options...).Value(&name))).Run()
	return name, err
}

func cancelled(cmd *cobra.Command, err error) error {
	if errors.Is(err, huh.ErrUserAborted) {
		cmd.PrintErrln("cancelled")
		return nil
	}
	return err
}
