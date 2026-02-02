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

	"github.com/charmbracelet/huh"
	"github.com/nelvko/proxyctl/config"
	"github.com/nelvko/proxyctl/httpx"
	"github.com/nelvko/proxyctl/kernel"
	"github.com/nelvko/proxyctl/log"
	"github.com/nelvko/unisvc/service"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// initCmd represents the init command
var initCmd = &cobra.Command{
	Use:     "init",
	Short:   "initialize proxyctl settings",
	Long:    ``,
	Aliases: []string{"i"},
	RunE: func(cmd *cobra.Command, args []string) error {
		if !force {
			if _, err := os.Stat(config.MyConfigFile); err == nil {
				return errors.New("Configuration already exists. Use --force to overwrite.")
			}
		}

		if err := InitSteps(); err != nil {
			return err
		}

		if err := os.MkdirAll(config.MyConfigDir, 0755); err != nil {
			log.Fail(fmt.Sprintf("Failed to create config directory: %v", err))
		}

		if err := viper.WriteConfigAs(config.MyConfigFile); err != nil {
			log.Fail(fmt.Sprintf("Failed to write configuration file: %v", err))
		}
		log.Ok("Successfully initialized: "+config.MyConfigFile, "✅")
		return nil
	},
}

var force bool

func init() {
	rootCmd.AddCommand(initCmd)
	initCmd.Flags().BoolVarP(&force, "force", "f", false, "Force initialization even if config file exists")
}

func selectExistService(svc service.Service, k kernel.Kernel) (bool, error) {
	var confirm bool
	err := huh.NewConfirm().
		Title(fmt.Sprintf("Detected %s with %s kernel. Proceed?", svc.InitSystem(), k)).
		Affirmative("Yes!").
		Negative("No.").
		Value(&confirm).
		Run()
	if err != nil {
		return confirm, err
	}
	return confirm, nil
}

func InitSteps() error {
	var svc service.Service
	for _, v := range kernel.AvailableKernel {
		svc = kernel.New(v)
		if IsInstalled, _ := svc.IsInstalled(); IsInstalled {
			ok, err := selectExistService(svc, v)
			if err != nil {
				return err
			}
			if ok {
				// todo
				return nil
			}
		}
	}
	var (
		useProxy bool
	)
	cfg := config.Init()

	// todo
	useProxy = true
	cfg.GitHubProxy = "https://gh-proxy.org"

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Select proxy kernel").
				Description("Choose the core to install and run.").
				Options(
					huh.NewOption(kernel.Mihomo, kernel.Mihomo),
					huh.NewOption(kernel.Clash, kernel.Clash),
					huh.NewOption(kernel.SingBox, kernel.SingBox),
				).
				Value(&cfg.Kernel.Name),

			// huh.NewInput().
			// 	Title("input the install path").
			// 	Description("Choose the core to install and run.").
			// 	Value(&cfg.Kernel.BinPath).
			// 	Validate(func(s string) error {
			// 		if os.Geteuid() != 0 && s == "/usr/local/bin" {
			// 			return fmt.Errorf("non-root user cannot install to %s, please change path or with sudo", s)
			// 		}
			// 		return nil
			// 	}),
		),
		huh.NewGroup(
			huh.NewConfirm().
				Title("Enable GitHub proxy").
				Description("Use a proxy to accelerate GitHub downloads in restricted networks.").
				Affirmative("Yes, use proxy").
				Negative("No, direct access").
				Value(&useProxy),
		),
		huh.NewGroup(
			huh.NewInput().
				Title("GitHub proxy prefix").
				Description("e.g. https://ghproxy.link").
				// Validate(func(s string) error {
				// 	if s == "" {
				// 		return errors.New("can't empty")
				// 	}
				// 	if s == "a" {
				// 		return errors.New("can't empty")
				// 	}
				// 	return nil
				// }).
				Value(&cfg.GitHubProxy),
		).WithHideFunc(func() bool {
			return !useProxy
		}),
	)

	if err := form.Run(); err != nil {
		return err
	}
	cfg.Kernel.ConfigDir = filepath.Join("/etc", cfg.Kernel.Name)
	cfg.Kernel.BinPath = filepath.Join("/usr/local/bin", cfg.Kernel.Name)
	if useProxy && cfg.GitHubProxy != "" {
		os.Setenv(httpx.GH_PROXY, cfg.GitHubProxy)
	}
	url, err := kernel.MihomoDownloadURL()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Second)
	defer cancel()

	file, err := os.CreateTemp("", filepath.Base(url))
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	defer file.Close()
	if err := httpx.Download(ctx, url, file); err != nil {
		return err
	}

	if err := httpx.Extract(file, cfg.Kernel.BinPath); err != nil {
		return err
	}
	if err := os.Chmod(cfg.Kernel.BinPath, 0755); err != nil {
		return err
	}

	config.Save(cfg)
	err = svc.Install(&service.Spec{
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
