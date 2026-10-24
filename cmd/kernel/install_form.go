package kernel

import (
	"context"
	"errors"

	"charm.land/bubbles/v2/key"
	"charm.land/huh/v2"
	"github.com/nelvko/proxyctl/internal/config"
	pkernel "github.com/nelvko/proxyctl/internal/kernel"
)

func promptInstall(ctx context.Context, cfg *config.Config) (string, error) {
	if !isInteractive() {
		return "", errors.New("no terminal available; pass the kernel name as an argument instead")
	}
	var name string
	if err := newInstallForm(cfg, &name).RunWithContext(ctx); err != nil {
		return "", err
	}
	return name, nil
}

func newInstallForm(cfg *config.Config, name *string) *huh.Form {
	km := huh.NewDefaultKeyMap()
	km.Quit = key.NewBinding(key.WithKeys("q", "esc", "ctrl+c"))
	km.Select.Filter.SetEnabled(false)
	km.Select.Submit.SetHelp("enter", "install")
	selection := huh.NewSelect[string]().
		Title("Install kernel").
		Description("Download a kernel and set up its user service.\nq / esc to cancel").
		Options(installableOptions(cfg)...).
		Value(name)
	return huh.NewForm(huh.NewGroup(selection)).WithKeyMap(km)
}

func installableOptions(cfg *config.Config) []huh.Option[string] {
	var options []huh.Option[string]
	for _, name := range pkernel.ImplementedNames() {
		format := "Clash YAML"
		if pkernel.FormatOf(name) == pkernel.FormatSingBox {
			format = "sing-box JSON"
		}
		label := name + " · " + format
		if cfg.KernelByName(name) != nil {
			label += " · installed"
			if cfg.Use == name {
				label += ", selected"
			}
		} else {
			label += " · not installed"
		}
		options = append(options, huh.NewOption(label, name))
	}
	return options
}
