package kernel

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/nelvko/proxyctl/internal/config"
	pkernel "github.com/nelvko/proxyctl/internal/kernel"
	"github.com/spf13/cobra"
)

func newInstallCmd() *cobra.Command {
	command := &cobra.Command{
		Use: "install [name]", Short: "Install a kernel binary and user service",
		Long: `Install Mihomo or sing-box from their latest release, or copy a local binary with --bin.

Run without a name to select a built-in kernel, showing its configuration
format and installation status. For a local binary, provide a name and --bin.
--type reuses Mihomo or sing-box configuration behavior when the local binary
is compatible.

The local executable must already be extracted and is copied into proxyctl's
managed directory. Mihomo and sing-box start with a local mixed proxy on
127.0.0.1:7890 and direct outbound connections; subscriptions are optional.
Existing nonempty configurations are preserved.`,
		Args:      cobra.MaximumNArgs(1),
		ValidArgs: pkernel.ImplementedNames(),
		RunE: mutation(func(cmd *cobra.Command, args []string) error {
			binPath, _ := cmd.Flags().GetString("bin")
			kind, _ := cmd.Flags().GetString("type")
			if cmd.Flags().Changed("bin") && binPath == "" {
				return fmt.Errorf("--bin requires a path")
			}
			if cmd.Flags().Changed("type") && binPath == "" {
				return fmt.Errorf("--type requires --bin")
			}
			if cmd.Flags().Changed("mirror") && binPath != "" {
				return fmt.Errorf("--mirror applies only to built-in downloads")
			}
			name := ""
			if len(args) == 1 {
				name = args[0]
			} else if binPath != "" {
				return fmt.Errorf("a kernel name is required with --bin")
			}
			a, err := loadApp(cmd)
			if err != nil {
				return err
			}
			if len(args) == 0 {
				name, err = promptInstall(cmd.Context(), a.Config)
				if err != nil {
					if cancelled(err) {
						cmd.PrintErrln("cancelled")
						return nil
					}
					return err
				}
			}
			var options []config.KernelConfig
			if binPath != "" {
				kcfg, err := localKernelConfig(name, binPath, kind)
				if err != nil {
					return err
				}
				if cmd.Flags().Changed("type") && kcfg.Type == "custom" {
					return fmt.Errorf("--type must be mihomo or sing-box")
				}
				options = append(options, kcfg)
			} else if len(args) == 0 {
				// Selecting a built-in download replaces any saved local source.
				kcfg, err := config.DefaultKernelConfig(name)
				if err != nil {
					return err
				}
				if existing := a.Config.KernelByName(name); existing != nil {
					kcfg = *existing
				}
				kcfg.Type, kcfg.Source = "", ""
				options = append(options, kcfg)
			}
			if binPath == "" {
				if mirrors, _ := cmd.Flags().GetStringSlice("mirror"); len(mirrors) > 0 {
					a.Config.Mirror = strings.Join(mirrors, ",")
				}
			}
			if err := a.InstallKernel(cmd.Context(), name, options...); err != nil {
				return err
			}
			cmd.Printf("kernel %q installed\n", name)
			if k, err := pkernel.New(a.Config.KernelByName(name)); err == nil {
				if initial, err := pkernel.IsDefaultConfig(k); err == nil && initial {
					cmd.Println("configuration: default (direct); subscription optional")
				}
			}
			return nil
		}),
	}
	command.Flags().StringSlice("mirror", nil, "GitHub mirror prefix(es) for built-in downloads")
	command.Flags().String("bin", "", "Install from a local kernel executable")
	command.Flags().String("type", "", "Use mihomo or sing-box behavior for a compatible local binary")
	return command
}

func localBinaryPath(value string) (string, error) {
	if value == "" {
		return "", fmt.Errorf("--bin requires a path")
	}
	path, err := filepath.Abs(value)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(path)
	if err != nil {
		return "", fmt.Errorf("local binary: %w", err)
	}
	if !info.Mode().IsRegular() {
		return "", fmt.Errorf("local binary is not a regular file: %s", path)
	}
	return path, nil
}

func localKernelConfig(name, binPath, kind string) (config.KernelConfig, error) {
	kcfg, err := config.DefaultKernelConfig(name)
	if err != nil {
		return config.KernelConfig{}, err
	}
	path, err := localBinaryPath(binPath)
	if err != nil {
		return config.KernelConfig{}, err
	}
	if kind == "" {
		kind = "custom"
		if name == "mihomo" || name == "sing-box" {
			kind = name
		}
	}
	if kind != "custom" && kind != "mihomo" && kind != "sing-box" {
		return config.KernelConfig{}, fmt.Errorf("--type must be mihomo or sing-box")
	}
	kcfg.Type, kcfg.Source = kind, path
	fileName := "config.yaml"
	if kind == "sing-box" {
		fileName = "config.json"
	}
	kcfg.ConfigFile = filepath.Join(kcfg.ConfigDir, fileName)
	return kcfg, nil
}
