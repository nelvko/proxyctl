/*
Copyright © 2025 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"charm.land/huh/v2"
	"charm.land/lipgloss/v2"
	"github.com/nelvko/proxyctl/config"
	"github.com/nelvko/proxyctl/httpx"
	"github.com/nelvko/proxyctl/kernel"
	"github.com/nelvko/proxyctl/log"
	"github.com/nelvko/unisvc"
	"github.com/spf13/cobra"
)

// initCmd represents the init command
var initCmd = &cobra.Command{
	Use:     "init",
	Short:   "initialize proxyctl settings",
	Long:    ``,
	Aliases: []string{"i"},
	RunE: func(cmd *cobra.Command, args []string) error {
		if !force {
			if _, err := os.Stat(config.AppConfigFile); err == nil {
				return errors.New("Configuration already exists. Use --force to overwrite.")
			}
		}

		if err := InitSteps(); err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(config.AppConfigFile), 0755); err != nil {
			return err
		}

		if _, err := os.Stat(config.AppConfigFile); os.IsNotExist(err) {
			os.MkdirAll(filepath.Dir(config.AppConfigFile), os.ModePerm)
			f, err := os.Create(config.AppConfigFile)
			if err != nil {
				return err
			}
			defer f.Close()
		}
		if err := config.Save(); err != nil {
			return fmt.Errorf("Failed to write configuration file: %w", err)
		}
		log.Ok("Successfully initialized", "✅")
		return nil
	},
}

var force bool

func init() {
	rootCmd.AddCommand(initCmd)
	initCmd.Flags().BoolVarP(&force, "force", "f", false, "Force initialization even if config file exists")
}

// func selectExistService(svc service.Service, k kernel.Kernel) (bool, error) {
// 	var confirm bool
// 	err := huh.NewConfirm().
// 		Title(fmt.Sprintf("Detected %s service: %s. Do you want to proceed with using this service?", svc.InitSystem(), k)).
// 		Affirmative("Yes!").
// 		Negative("No.").
// 		Value(&confirm).
// 		Run()
// 	if err != nil {
// 		return confirm, err
// 	}
// 	return confirm, nil
// }

func CanWriteTo(path string) error {
	if len(path) == 0 {
		return fmt.Errorf("can't empty")

	}
	current := filepath.Clean(path)
	for {
		parent := filepath.Dir(current)
		if parent == current {
			break
		}
		_, err := os.Stat(parent)
		if os.IsNotExist(err) {
			current = parent
			continue
		}
		f, err := os.CreateTemp(parent, "test*")
		if os.IsPermission(err) {
			return fmt.Errorf("Permission denied: unable to write to %s.  Try running with sudo or change the path.", path)
		}
		if err == nil {
			f.Close()
			os.Remove(f.Name())
			return nil
		}
	}
	return nil
}
func checkExists() string {
	_, err := os.Stat(cfg.Kernel.BinPath)
	if err == nil {
		s := lipgloss.NewStyle().Foreground(lipgloss.Color("9"))
		return s.Render("already exists files: %s, it will overwrite old")
	}
	return "proxy kernel's bin path  "

}

func defaultBinPath() string {
	// if os.Geteuid() == 0 {
	// 	return filepath.Join("/usr/local/bin", cfg.Kernel.Name)
	// } else {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".local", "bin", cfg.Kernel.Name)
	// }
}

func defaultConfigDir() string {
	// if os.Geteuid() == 0 {
	// return filepath.Join("/etc", cfg.Kernel.Name)
	// } else {
	cfgDir, _ := os.UserConfigDir()
	return filepath.Join(cfgDir, cfg.Kernel.Name)
	// }
}

var cfg = config.Get()

const (
	mihomo  = "mihomo"
	clash   = "clash"
	singbox = "sing-box"
)

func InitSteps() error {
	var svc unisvc.Service
	// var installedKernel = []kernel.Kernel{}
	// for _, v := range kernel.AvailableKernel {
	// 	svc, _ = kernel.New(v)
	// 	if IsInstalled, _ := svc.IsInstalled(); IsInstalled {
	// 		installedKernel = append(installedKernel, v)
	// 	}
	// }
	// if len(installedKernel) > 0 {
	// 	selectExistService(installedKernel)
	// }

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Select proxy kernel").
				DescriptionFunc(func() string {
					return fmt.Sprintf("download and install the %s kernel", cfg.Kernel.Name)
				}, &cfg.Kernel.Name).
				Options(
					huh.NewOption(mihomo, mihomo),
					huh.NewOption(clash, clash),
					huh.NewOption(singbox, singbox),
				).
				Value(&cfg.Kernel.Name),

			huh.NewInput().
				Title("proxy kernel's bin path").
				PlaceholderFunc(defaultBinPath, &cfg.Kernel.Name).
				SuggestionsFunc(func() []string {
					return []string{defaultBinPath()}
				}, &cfg.Kernel.Name).
				Value(&cfg.Kernel.BinPath).
				Validate(CanWriteTo),

			huh.NewInput().
				Title("proxy kernel's config dir").
				PlaceholderFunc(defaultConfigDir, &cfg.Kernel.Name).
				SuggestionsFunc(func() []string {
					return []string{defaultConfigDir()}
				}, &cfg.Kernel.ConfigDir).
				Value(&cfg.Kernel.ConfigDir).
				Validate(CanWriteTo),
		),
	)

	if err := form.Run(); err != nil {
		return err
	}
	cfg.Kernel.ConfigFile = filepath.Join(cfg.Kernel.ConfigDir, "config.yaml")
	if err := os.MkdirAll(cfg.Kernel.ConfigDir, 0755); err != nil {
		return err
	}
	if _, err := os.Create(cfg.Kernel.ConfigFile); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(cfg.Kernel.BinPath), 0755); err != nil {
		return err
	}
	url, err := kernel.MihomoDownloadURL()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Second)
	defer cancel()

	f, err := os.CreateTemp("", filepath.Base(url))
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}

	defer func() {
		f.Close()
		os.Remove(f.Name())
	}()

	if err := httpx.Download(ctx, url, f); err != nil {
		return err
	}

	// if err := f.Sync(); err != nil {
	// 	return fmt.Errorf("failed to sync file: %w", err)
	// }
	// if _, err := f.Seek(0, io.SeekStart); err != nil {
	// 	return fmt.Errorf("failed to seek file: %w", err)
	// }

	if err := httpx.Ungzip(f, cfg.Kernel.BinPath); err != nil {
		return fmt.Errorf("failed to extract file: %w", err)
	}
	if err := os.Chmod(cfg.Kernel.BinPath, 0755); err != nil {
		return err
	}

	svc, _ = kernel.New(cfg.Kernel.Name)

	err = svc.Install(&unisvc.Spec{
		Description: "proxy daemon",
		Command:     cfg.Kernel.BinPath,
		Args: []string{
			"-d", cfg.Kernel.ConfigDir,
		},
	})
	if err != nil {
		return err
	}
	return nil
}
