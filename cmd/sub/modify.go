package sub

import (
	"errors"

	"github.com/nelvko/proxyctl/internal/app"
	"github.com/nelvko/proxyctl/internal/ui"
	"github.com/spf13/cobra"
)

func newModifyCmd() *cobra.Command {
	var name, source string
	var download downloadFlags
	cmd := &cobra.Command{
		Use: "modify <name>", Short: "Change subscription metadata without downloading or applying",
		Long: `Change the name, source URL or download settings of a saved profile.
Cached content and service state are unchanged. Use update to fetch new content.
Use --user-agent='' to clear a custom agent, --timeout=0 for the default timeout,
and --download-via=env to follow environment proxy settings.
Without flags, opens a metadata form in a terminal.`,
		Args: cobra.ExactArgs(1),
		RunE: mutation(func(cmd *cobra.Command, args []string) error {
			changed := cmd.Flags().Changed("name") || cmd.Flags().Changed("url") || download.changed(cmd)
			if !changed && !isInteractive() {
				return errors.New("specify at least one metadata option; see proxyctl sub modify --help")
			}
			options, err := download.options(cmd)
			if err != nil {
				return err
			}
			svc, err := app.LoadProfileMetadata(cmd.Context())
			if err != nil {
				return err
			}
			p, err := svc.Get(args[0])
			if err != nil {
				return err
			}
			if cmd.Flags().Changed("name") {
				p.Name = name
			}
			if cmd.Flags().Changed("url") {
				p.URL = source
			}
			p.Download, err = options.Apply(p.Download)
			if err != nil {
				return err
			}
			if !changed {
				if err := ui.PromptModify(svc, &p); err != nil {
					return cancelled(cmd, err)
				}
			}
			if err := svc.Set(args[0], p); err != nil {
				return err
			}
			cmd.Printf("profile %q metadata saved\n", p.Name)
			if !changed || cmd.Flags().Changed("url") || download.changed(cmd) {
				cmd.Printf("Next: proxyctl sub update %s\n", profileNameArg(p.Name))
			}
			return nil
		}),
	}
	cmd.Flags().StringVarP(&name, "name", "n", "", "New profile name")
	cmd.Flags().StringVar(&source, "url", "", "New HTTP, HTTPS or file source URL")
	download.bind(cmd, true)
	return cmd
}
