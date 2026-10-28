package cmd

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/nelvko/proxyctl/internal/config"
	"github.com/nelvko/proxyctl/internal/i18n"
	"github.com/nelvko/proxyctl/internal/ui"
	"github.com/spf13/cobra"
)

func newThemeCmd() *cobra.Command {
	command := &cobra.Command{
		Use: "theme", Short: "Import or export TUI appearance preferences", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error { return cmd.Help() },
	}
	var force bool
	export := &cobra.Command{
		Use: "export [path|-]", Short: "Export theme YAML to a file or stdout", Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			theme, err := config.LoadTheme()
			if err != nil {
				return err
			}
			if err := ui.ValidateThemeConfig(theme); err != nil {
				return fmt.Errorf("invalid theme: %w", err)
			}
			if len(args) == 0 || args[0] == "-" {
				return config.WriteTheme(cmd.OutOrStdout(), theme)
			}
			var data bytes.Buffer
			if err := config.WriteTheme(&data, theme); err != nil {
				return err
			}
			return writeThemeExport(args[0], data.Bytes(), force)
		},
	}
	export.Flags().BoolVar(&force, "force", false, "Replace an existing export file")
	importCommand := &cobra.Command{
		Use: "import <path|->", Short: "Validate and save theme YAML from a file or stdin", Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			r := cmd.InOrStdin()
			if args[0] != "-" {
				file, err := os.Open(args[0])
				if err != nil {
					return err
				}
				defer file.Close()
				r = file
			}
			theme, err := config.ReadTheme(r)
			if err != nil {
				return err
			}
			if err := ui.ValidateThemeConfig(theme); err != nil {
				return fmt.Errorf("invalid theme: %w", err)
			}
			if err := config.SaveTheme(cmd.Context(), theme); err != nil {
				return err
			}
			language, _ := config.LoadLanguage()
			locale, _ := i18n.Resolve(language)
			_, err = fmt.Fprintln(cmd.ErrOrStderr(), i18n.Text(locale, "theme.imported"))
			return err
		},
	}
	command.AddCommand(export, importCommand)
	return command
}

func writeThemeExport(path string, data []byte, force bool) error {
	if !force {
		file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
		if err != nil {
			if errors.Is(err, os.ErrExist) {
				return fmt.Errorf("export file already exists; use --force to replace it: %s", path)
			}
			return err
		}
		_, writeErr := file.Write(data)
		closeErr := file.Close()
		if err := errors.Join(writeErr, closeErr); err != nil {
			_ = os.Remove(path)
			return err
		}
		return nil
	}
	// A forced export replaces the directory entry, so existing broad file
	// permissions and symlink targets cannot turn into leaked or truncated data.
	file, err := os.CreateTemp(filepath.Dir(path), ".proxyctl-theme-*")
	if err != nil {
		return err
	}
	temp := file.Name()
	defer os.Remove(temp)
	_, writeErr := file.Write(data)
	closeErr := file.Close()
	if err := errors.Join(writeErr, closeErr); err != nil {
		return err
	}
	return os.Rename(temp, path)
}
