package kernel

import (
	"errors"
	"fmt"

	"charm.land/bubbles/v2/key"
	"charm.land/huh/v2"
	pkernel "github.com/nelvko/proxyctl/internal/kernel"
)

func pickKernel(title string) (string, error) {
	if !isInteractive() {
		return "", fmt.Errorf("no terminal available, run `proxyctl install <name>` instead")
	}
	implemented := pkernel.ImplementedNames()
	if len(implemented) == 0 {
		return "", errors.New("no installable kernel")
	}
	options := make([]huh.Option[string], 0, len(implemented))
	for _, name := range implemented {
		options = append(options, huh.NewOption(string(pkernel.FormatOf(name))+" config · "+name, name))
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
	).WithKeyMap(km)
	if err := form.Run(); err != nil {
		return "", err
	}
	return name, nil
}
