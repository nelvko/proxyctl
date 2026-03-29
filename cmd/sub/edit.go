package sub

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"slices"
	"strings"

	"github.com/spf13/cobra"
)

// editCmd represents the edit command
var editCmd = &cobra.Command{
	Use:   "edit <name>",
	Short: "Edit a subscription profile",
	Long: `Edit a subscription profile via editor.

With --editor or $EDITOR to specify the editor command. The edited
profile will be test after the editor exits.`,
	Args: validArgWithInteractive,
	RunE: func(cmd *cobra.Command, args []string) error {
		if interactive {
			return tui()
		}
		profileName := args[0]
		profile, err := getProfile(profileName)
		if err != nil {
			return err
		}
		editorCommand, err := getEditorCommand(profile.File)
		if err != nil {
			return err
		}
		editorCommand.Stdin = os.Stdin
		editorCommand.Stdout = os.Stdout
		editorCommand.Stderr = os.Stderr
		if err := editorCommand.Run(); err != nil {
			return err
		}
		return k.TestConfig(profile.File)
	},
}

func getEditorCommand(file string) (*exec.Cmd, error) {
	editorList := []string{os.Getenv("EDITOR"), "nvim", "vim", "vi", "nano", "code", "notepad"}
	if editor != "" {
		editorList = slices.Insert(editorList, 0, editor)
	}
	for _, edt := range editorList {
		if edt == "" {
			continue
		}
		parts := strings.Fields(edt)
		cmdName := parts[0]
		args := append(parts[1:], file)
		cmdPath, err := exec.LookPath(cmdName)
		if err == nil {
			return exec.Command(cmdPath, args...), nil
		}
		if edt == editor {
			return nil, fmt.Errorf("specified editor not found: %w", err)
		}
	}
	return nil, errors.New("no supported editor found. please specify a valid editor via --editor flag or set the EDITOR environment variable")
}

var (
	editor string
)

func init() {
	subCmd.AddCommand(editCmd)
	editCmd.Flags().StringVarP(&editor, "editor", "e", editor, "Editor command")
}
