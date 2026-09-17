package ui

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"slices"
	"strings"

	"github.com/nelvko/proxyctl/internal/subscription"
)

// Edit runs the external editor on a staged profile, then validates and commits.
// The caller owns the transaction and terminal suspension (if any).
func Edit(ctx context.Context, svc *subscription.Service, name, editor string, stdin io.Reader, stdout, stderr io.Writer) error {
	session, err := svc.BeginEdit(name)
	if err != nil {
		return err
	}
	defer session.Close()
	cmd, err := editorCommand(ctx, session.Path(), editor)
	if err != nil {
		return err
	}
	cmd.Stdin, cmd.Stdout, cmd.Stderr = stdin, stdout, stderr
	if err := cmd.Run(); err != nil {
		return err
	}
	return session.Commit()
}

func editorCommand(ctx context.Context, file, editor string) (*exec.Cmd, error) {
	editorList := []string{os.Getenv("EDITOR"), "nvim", "vim", "vi", "nano", "code", "notepad"}
	if editor != "" {
		editorList = slices.Insert(editorList, 0, editor)
	}

	for _, edt := range editorList {
		if edt == "" {
			continue
		}
		parts := strings.Fields(edt)
		if len(parts) == 0 {
			if edt == editor {
				return nil, errors.New("editor command cannot be blank")
			}
			continue
		}
		cmdName := parts[0]
		args := append(parts[1:], file)
		cmdPath, err := exec.LookPath(cmdName)
		if err == nil {
			return exec.CommandContext(ctx, cmdPath, args...), nil
		}
		if edt == editor {
			return nil, fmt.Errorf("specified editor not found: %w", err)
		}
	}

	return nil, errors.New("no supported editor found. please specify a valid editor via --editor flag or set the EDITOR environment variable")
}
