package sub

import (
	"errors"
	"strings"

	"github.com/nelvko/proxyctl/internal/app"
	"github.com/nelvko/proxyctl/internal/config"
	"github.com/nelvko/proxyctl/internal/redact"
	"github.com/nelvko/proxyctl/internal/ui"
	"github.com/spf13/cobra"
)

func newAddCmd() *cobra.Command {
	var use, keepFailed bool
	var draft config.Profile
	var download downloadFlags
	cmd := &cobra.Command{
		Use:   "add [url]",
		Short: "Download, validate and save a subscription profile",
		Long: `Add a profile from an HTTP, HTTPS or file URL. Without a URL, prompt
for the source, name and optional download settings. Adding does not select a profile or start a service.
Use --use to also select and apply it, preserving the service's running state.
Download settings are saved for future updates. --download-via selects the route.
A file:// URL links to a local source: updates read that source again and replace
local edits; this is not an independent file import.`,
		Args: cobra.MaximumNArgs(1),
		RunE: mutation(func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && !isInteractive() {
				return errors.New("a source URL is required; run `proxyctl sub add <url>`")
			}
			options, err := download.options(cmd)
			if err != nil {
				return err
			}
			draft.Download, err = options.Apply(draft.Download)
			if err != nil {
				return err
			}
			svc, err := app.LoadRuntimeProfiles(cmd.Context(), ui.Reporter(cmd.ErrOrStderr()))
			if err != nil {
				return err
			}
			svc.KeepFailed = keepFailed
			profile := draft
			for {
				if len(args) == 0 {
					if err := ui.PromptAdd(svc, &profile); err != nil {
						return cancelled(cmd, err)
					}
				} else {
					profile.URL = args[0]
				}
				submitted := profile
				if err := svc.Add(&submitted); err != nil {
					if len(args) != 0 || cmd.Context().Err() != nil {
						return err
					}
					cmd.PrintErrln(redact.Error(err))
					continue
				}
				profile = submitted
				break
			}
			cmd.Printf("profile %q added\n", profile.Name)
			if use {
				result, err := svc.Use(profile.Name)
				if err != nil {
					return err
				}
				cmd.Printf("profile %q selected; %s\n", profile.Name, ui.Application(result))
			} else if svc.SelectedName() == "" {
				cmd.Println("No profile is selected.")
				cmd.Printf("Next: proxyctl sub use %s\n", profileNameArg(profile.Name))
				cmd.Println("Then: proxyctl on")
			}
			return nil
		}),
	}
	cmd.Flags().StringVarP(&draft.Name, "name", "n", "", "Profile name; defaults to the response filename or source name")
	cmd.Flags().BoolVar(&use, "use", false, "Select and apply the added profile without starting a stopped service")
	cmd.Flags().BoolVar(&keepFailed, "keep-failed", false, "Keep complete downloads that fail validation in diagnostics/; remove them manually after inspection")
	download.bind(cmd, false)
	return cmd
}

// Profile names cannot contain backslashes or newlines. Single quoting other
// special characters works in bash, zsh and fish; -- protects leading dashes.
func profileNameArg(name string) string {
	arg := name
	if strings.ContainsFunc(name, func(r rune) bool {
		return !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') ||
			(r >= '0' && r <= '9') || strings.ContainsRune("_-.", r))
	}) {
		arg = "'" + strings.ReplaceAll(name, "'", `'\''`) + "'"
	}
	if strings.HasPrefix(name, "-") {
		arg = "-- " + arg
	}
	return arg
}
