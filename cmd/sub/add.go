/*
Copyright © 2026 NAME HERE <EMAIL ADDRESS>
*/
package sub

import (
	"context"
	"errors"
	"fmt"
	"io"
	URL "net/url"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"charm.land/huh/v2"
	"charm.land/lipgloss/v2"
	"github.com/nelvko/proxyctl/httpx"
	"github.com/nelvko/proxyctl/log"
	"github.com/spf13/cobra"
)

// addCmd represents the add command
var addCmd = &cobra.Command{
	Use:   "add [url]",
	Short: "Add a Subscription profile",
	Long: `Import a configuration profile from a remote URL or a local file path.
Supported schemes:
  - Remote: http://, https://
  - Local:  file://`,
	SuggestFor: []string{"install", "import", "put"},
	Args:       cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) != 1 {
			if err := tuiAdd(); err != nil {
				return err
			}
		} else {
			url = args[0]
		}
		if name == "" {
			name = strconv.FormatInt(time.Now().Unix(), 10)
		}
		if err := checkUniqueName(name); err != nil {
			return err
		}
		profilePath := filepath.Join(profilesDir, name+".yaml")
		dst, err := os.Create(profilePath)
		if err != nil {
			return err
		}
		defer dst.Close()

		u, err := URL.Parse(url)
		if err != nil {
			return err
		}
		switch strings.ToLower(u.Scheme) {
		case "file":
			src, err := os.Open(u.Path)
			if err != nil {
				return fmt.Errorf("failed to open file: %w", err)
			}
			defer src.Close()
			io.Copy(dst, src)
		default:
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			if err := httpx.Download(ctx, url, dst); err != nil {
				return err
			}
		}
		p := profile{
			Name: name,
			Url:  url,
			File: profilePath,
		}
		if err := addProfile(p); err != nil {
			return err
		}
		log.Ok("订阅添加成功")
		return nil
	},
}

func addProfile(p profile) error {
	var err error
	if cfg.Use == "" && len(cfg.Items) == 0 {
		defer func() {
			if err == nil {
				useFunc(p.Name)
				log.Ok("use profile ok")
			}
		}()
	}
	cfg.Items = append(cfg.Items, p)
	err = saveSubConfig()
	return err
}

func checkUniqueName(name string) error {
	if name == "" {
		return errors.New("can't empty")
	}
	ok := slices.ContainsFunc(cfg.Items, func(p profile) bool {
		return p.Name == name
	})
	if ok {
		return fmt.Errorf("profile '%s' already exists", name)
	}
	return nil
}

var (
	// flags
	name string

	// args
	url string
)

func init() {
	SubCmd.AddCommand(addCmd)
	addCmd.Flags().StringVarP(&name, "name", "n", name, "Specified profile's unique name")
}
var descStyle=lipgloss.NewStyle().Foreground(lipgloss.Color("#767676"))
func initialForm() *huh.Form {
	return huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Subscription URL").
				Description(descStyle.Render("support http, file scheme")).
				Value(&url).
				Validate(func(s string) error {
					if url == "" {
						return errors.New("can't empty")
					}
					return nil
				}),
			huh.NewInput().
				Title("Subscription Name").
				Description(descStyle.Render("should be unique")).
				Validate(checkUniqueName).
				Value(&name),
		),
	)
}

func tuiAdd() error {
	if err := initialForm().Run(); err != nil {
		return err
	}
	return nil
}
