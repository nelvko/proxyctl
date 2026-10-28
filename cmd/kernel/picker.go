package kernel

import (
	"errors"

	"charm.land/bubbles/v2/key"
	"charm.land/huh/v2"
	"github.com/nelvko/proxyctl/internal/config"
	"github.com/nelvko/proxyctl/internal/ui"
)

// pickKernel prompts for an installed kernel. Callers report a cancelled pick
// with cancelled(err).
func pickKernel(title string, options []huh.Option[string]) (string, error) {
	if !isInteractive() {
		return "", errors.New("no terminal available; pass the kernel name as an argument instead")
	}
	if len(options) == 0 {
		return "", errors.New("no kernel to choose from")
	}

	var name string
	km := huh.NewDefaultKeyMap()
	// q quits a pure select (no inputs to steal the key from), matching the
	// list TUI; huh's default is ctrl+c only.
	km.Quit = key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "quit"))
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title(title).
				// huh's footer only shows field bindings, never the
				// form-level Quit — the key must be advertised here.
				Description("q to cancel").
				Options(options...).
				Value(&name),
		),
	).WithTheme(ui.HuhTheme()).WithKeyMap(km)
	if err := form.Run(); err != nil {
		return "", err
	}
	return name, nil
}

// cancelled reports whether the caller aborted an interactive prompt.
func cancelled(err error) bool { return errors.Is(err, huh.ErrUserAborted) }

// installedOptions lists the installed kernels, marking the selected one and
// returning it as the pre-selected value so enter keeps the current choice.
func installedOptions(kernels []config.KernelConfig, selected string) ([]huh.Option[string], string) {
	options := make([]huh.Option[string], 0, len(kernels))
	for _, k := range kernels {
		label := k.Name
		if k.Name == selected {
			label += " (selected)"
		}
		options = append(options, huh.NewOption(label, k.Name))
	}
	return options, selected
}
