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
				return errors.New("configuration already exists. Use --force to overwrite")
			}
		}

		if err := InitSteps(); err != nil {
			return err
		}

		if _, err := os.Stat(config.AppConfigFile); os.IsNotExist(err) {
			os.MkdirAll(config.AppConfigPath, os.ModeDir)
			f, err := os.Create(config.AppConfigFile)
			if err != nil {
				return err
			}
			defer f.Close()
		}
		if err := config.SaveAppConfig(); err != nil {
			return fmt.Errorf("Failed to write configuration file: %w", err)
		}
		log.Ok("Successfully initialized", "✅")
		return nil
	},
}

var force bool

func init() {
	RootCmd.AddCommand(initCmd)
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
	_, err := os.Stat(config.AppCfg.Kernel.Bin)
	if err == nil {
		s := lipgloss.NewStyle().Foreground(lipgloss.Color("9"))
		return s.Render("already exists files: %s, it will overwrite old")
	}
	return "proxy kernel's bin path  "

}

const (
	mihomo  = "mihomo"
	clash   = "clash"
	singbox = "sing-box"
)

func InitSteps() error {
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
	var (
		appCfg    = config.AppCfg
		kernelCfg = &appCfg.Kernel
	)
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Select proxy kernel").
				// DescriptionFunc(func() string {
				// 	return fmt.Sprintf("download and install the %s kernel", kernelName)
				// }, &kernelName).
				Options(
					huh.NewOption(mihomo, mihomo),
					huh.NewOption(clash, clash),
					huh.NewOption(singbox, singbox),
				).
				Value(&kernelCfg.Name),

			huh.NewInput().
				Title("proxy kernel's binary").
				// PlaceholderFunc(defaultBinPath, &kernelName).
				// SuggestionsFunc(func() []string {
				// 	return []string{defaultBinPath()}
				// }, &config.AppCfg.Kernel.Name).
				Value(&kernelCfg.Bin).
				Validate(CanWriteTo),

			huh.NewInput().
				Title("configuration directory").
				// PlaceholderFunc(defaultConfigDir, &config.AppCfg.Kernel.Name).
				// SuggestionsFunc(func() []string {
				// 	return []string{defaultConfigDir()}
				// }, &config.AppCfg.Kernel.ConfigDir).
				Value(&kernelCfg.ConfigDir).
				Validate(CanWriteTo),
		),
	)

	if err := form.Run(); err != nil {
		return err
	}
	kernelCfg.ConfigFile = filepath.Join(kernelCfg.ConfigDir, "config.yaml")
	if err := os.MkdirAll(kernelCfg.ConfigDir, 0755); err != nil {
		return err
	}
	if _, err := os.Create(kernelCfg.ConfigFile); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(kernelCfg.Bin), 0755); err != nil {
		return err
	}
	k, _ = kernel.New(kernelCfg.Name)
	url, err := k.DownloadURL()
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

	if err := httpx.Ungzip(f, kernelCfg.Bin); err != nil {
		return fmt.Errorf("failed to extract file: %w", err)
	}
	if err := os.Chmod(kernelCfg.Bin, 0755); err != nil {
		return err
	}

	spec := unisvc.Spec{
		Command: kernelCfg.Bin,
		Args:    []string{"-d", kernelCfg.ConfigDir, "-f", kernelCfg.ConfigFile},
	}
	k, _ := kernel.New(kernelCfg.Name)
	if err := k.Install(&spec); err != nil {
		return err
	}
	return nil
}

func setupWizard() error {
	var confirm bool
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewConfirm().
				Title("🔧 Proxyctl is not initialized").
				Description("🤔 Would you like to run the setup wizard now?").
				Affirmative("🚀 Yes, let's go!").
				Negative("🚫 No, maybe later").
				Value(&confirm),
		),
	)

	if err := form.Run(); err != nil {
		return fmt.Errorf("failed to run program: %w", err)
	}
	if !confirm {
		fmt.Println("No problem! You can initialize whenever you're ready by running `proxyctl init`.")
		os.Exit(0)
	}
	return initCmd.RunE(initCmd, []string{})

}
