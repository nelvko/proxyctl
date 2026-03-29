package sub

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"time"

	"charm.land/huh/v2"
	"charm.land/lipgloss/v2"
	"github.com/nelvko/proxyctl/httpx"
	"github.com/nelvko/proxyctl/log"
	"github.com/spf13/cobra"
)

func validArgWithInteractive(cmd *cobra.Command, args []string) error {
	if interactive {
		return nil
	}
	return cobra.ExactArgs(1)(cmd, args)
}

// addCmd represents the add command
var addCmd = &cobra.Command{
	Use:   "add <url>",
	Short: "Add a subscription profile from a source URL",
	Long: `Add a subscription profile from an HTTP, HTTPS, or file URL.

The profile is validated before it is saved. If this is the first
profile, it becomes active automatically.`,
	Args: validArgWithInteractive,
	RunE: func(cmd *cobra.Command, args []string) error {
		if interactive {
			if err := tuiAdd(); err != nil {
				return err
			}
		} else {
			option.URL = args[0]
		}

		u, err := url.Parse(option.URL)
		if err != nil {
			return err
		}

		if option.Name == "" {
			option.Name = fmt.Sprintf("%d", time.Now().Unix())
		}
		if err := checkUniqueName(option.Name); err != nil {
			return err
		}
		tmpFile, err := os.CreateTemp("", "profile-*")
		if err != nil {
			return err
		}
		defer tmpFile.Close()

		switch u.Scheme {
		case "file":
			src, err := os.Open(u.Path)
			if err != nil {
				return err
			}
			defer src.Close()
			if _, err := io.Copy(tmpFile, src); err != nil {
				return err
			}
		case "http", "https":
			ctx, cancel := context.WithTimeout(context.Background(), option.Update.Timeout)
			defer cancel()
			if err := httpx.Download(ctx, u.String(), tmpFile); err != nil {
				if errors.Is(err, context.DeadlineExceeded) {
					return errors.New("download timed out, please try again later or specify a longer timeout")
				}
				return err
			}
		default:
			return fmt.Errorf("unsupported scheme: %s", u.Scheme)
		}
		if err := k.TestConfig(tmpFile.Name()); err != nil {
			return err
		}
		option.File = filepath.Join(subDir, option.Name+".yaml")
		if err := os.Rename(tmpFile.Name(), option.File); err != nil {
			return err
		}
		if err := addProfile(*option); err != nil {
			return err
		}
		log.Ok(fmt.Sprintf("profile %q added successfully", option.Name))
		if use || (subCfg.Use == "" && len(subCfg.Profiles) == 1) {
			if err := useFunc(option.Name); err != nil {
				return err
			}
			log.Ok(fmt.Sprintf("profile %q used successfully", option.Name))
		}
		return nil
	},
}

func addProfile(p profile) error {
	subCfg.Profiles = append(subCfg.Profiles, p)
	if err := saveSubConfig(); err != nil {
		return err
	}
	return nil
}

func checkUniqueName(name string) error {
	if name == "" {
		return nil
	}
	ok := slices.ContainsFunc(subCfg.Profiles, func(p profile) bool {
		return p.Name == name
	})
	if ok {
		return fmt.Errorf("profile %q already exists", name)
	}
	return nil
}

var (
	use    bool
	option = &profile{
		Update: updateConfig{
			Timeout: 10 * time.Second,
		},
	}
)

func init() {
	subCmd.AddCommand(addCmd)
	addCmd.Flags().StringVarP(&option.Name, "name", "n", option.Name, "Profile name; defaults to the current Unix timestamp")
	addCmd.Flags().BoolVarP(&use, "use", "u", use, "Switch to the new profile after adding it")
	addCmd.Flags().DurationVarP(&option.Update.Timeout, "timeout", "t", option.Update.Timeout, "HTTP(S) download timeout")
	// todo updateConfig
}

var descStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#767676"))

func initialForm() *huh.Form {
	return huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Source URL").
				Description(descStyle.Render("Supports http://, https://, and file:// URLs")).
				Value(&option.URL).
				Validate(validateSourceURL),
			huh.NewInput().
				Title("Profile Name").
				Description(descStyle.Render("Optional; defaults to the current Unix timestamp")).
				Validate(checkUniqueName).
				Value(&option.Name),
		),
	)
}

func validateSourceURL(raw string) error {
	if raw == "" {
		return errors.New("source URL cannot be empty")
	}
	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("invalid source URL: %w", err)
	}
	switch u.Scheme {
	case "http", "https", "file":
		return nil
	case "":
		return errors.New("source URL must include http://, https://, or file://")
	default:
		return fmt.Errorf("unsupported URL scheme %q", u.Scheme)
	}
}

func tuiAdd() error {
	if err := initialForm().Run(); err != nil {
		return err
	}
	return nil
}
