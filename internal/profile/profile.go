package profile

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/nelvko/proxyctl/internal/app"
	"github.com/nelvko/proxyctl/internal/config"
	"github.com/nelvko/proxyctl/internal/httpx"
	"github.com/nelvko/proxyctl/internal/log"
)

type profile = config.Profile

func Add(option *profile) error {
	u, err := url.Parse(option.URL)
	if err != nil {
		return err
	}

	if option.Name == "" {
		option.Name = fmt.Sprintf("%d", time.Now().Unix())
	}
	if err := checkUniqueName(option.Name); err != nil {
		return err
	}
	tmpFile, err := os.CreateTemp("", "profile-*")
	if err != nil {
		return err
	}
	defer tmpFile.Close()

	switch u.Scheme {
	case "file":
		src, err := os.Open(u.Path)
		if err != nil {
			return err
		}
		defer src.Close()
		if _, err := io.Copy(tmpFile, src); err != nil {
			return err
		}
	case "http", "https":
		ctx, cancel := context.WithTimeout(context.Background(), option.Update.Timeout)
		defer cancel()
		if err := httpx.Download(ctx, u.String(), tmpFile); err != nil {
			if errors.Is(err, context.DeadlineExceeded) {
				return errors.New("download timed out, please try again later or specify a longer timeout")
			}
			return err
		}
	default:
		return fmt.Errorf("unsupported scheme: %s", u.Scheme)
	}
	if err := app.AppCtx.Kernel.TestConfig(tmpFile.Name()); err != nil {
		return err
	}
	option.File = filepath.Join(config.SubDir, option.Name+".yaml")
	if err := os.Rename(tmpFile.Name(), option.File); err != nil {
		return err
	}
	app.AppCtx.SubConfig.Profiles = append(app.AppCtx.SubConfig.Profiles, *option)
	if err := app.AppCtx.SaveSubConfig(); err != nil {
		return err
	}
	log.Ok(fmt.Sprintf("profile %q added successfully", option.Name))
	return nil
}

func checkUniqueName(name string) error {
	if name == "" {
		return nil
	}
	ok := slices.ContainsFunc(app.AppCtx.SubConfig.Profiles, func(p profile) bool {
		return p.Name == name
	})
	if ok {
		return fmt.Errorf("profile %q already exists", name)
	}
	return nil
}

func Delete(profileName string, force bool) error {
	tgt, err := Get(profileName)
	if err != nil {
		return err
	}
	if p, _ := Using(); p.Name == profileName && !force {
		return fmt.Errorf("profile %q is currently in use", profileName)
	}
	app.AppCtx.SubConfig.Profiles = slices.DeleteFunc(app.AppCtx.SubConfig.Profiles, func(p profile) bool {
		return p.Name == profileName
	})
	if err := app.AppCtx.SaveSubConfig(); err != nil {
		return err
	}

	if err := os.Remove(tgt.File); err != nil {
		return err
	}

	return nil
}

func Get(name string) (profile, error) {
	i := slices.IndexFunc(app.AppCtx.SubConfig.Profiles, func(p profile) bool {
		return p.Name == name
	})
	if i == -1 {
		return profile{}, fmt.Errorf("profile %q not found\nUse `proxyctl sub list` to see available profiles", name)
	}
	return app.AppCtx.SubConfig.Profiles[i], nil
}

func Using() (profile, error) {
	p, err := Get(app.AppCtx.SubConfig.Use)
	if err != nil {
		return profile{}, fmt.Errorf("failed to get current profile: %w", err)
	}
	return p, nil
}

func Use(profileName string) error {
	i := slices.IndexFunc(app.AppCtx.SubConfig.Profiles, func(p profile) bool {
		return p.Name == profileName
	})
	if i == -1 {
		return fmt.Errorf("can't find %s profile", profileName)
	}
	useFile := app.AppCtx.SubConfig.Profiles[i].File
	kernelCfg := app.AppCtx.AppConfig.Kernel.ConfigFile
	if err := app.AppCtx.Kernel.TestConfig(useFile); err != nil {
		return err
	}

	bytes, err := os.ReadFile(useFile)
	if err != nil {
		return err
	}
	if err := os.WriteFile(kernelCfg, bytes, 0666); err != nil {
		return err
	}

	if err := app.AppCtx.Kernel.Restart(); err != nil {
		return err
	}
	active, err := app.AppCtx.Kernel.IsActive()
	if err != nil {
		return err
	}
	if !active {
		return err
	}
	app.AppCtx.SubConfig.Use = profileName
	return app.AppCtx.SaveSubConfig()

}

func Edit(profileName, editor string) error {
	profile, err := Get(profileName)
	if err != nil {
		return err
	}
	editorCommand, err := getEditorCommand(profile.File, editor)
	if err != nil {
		return err
	}
	editorCommand.Stdin = os.Stdin
	editorCommand.Stdout = os.Stdout
	editorCommand.Stderr = os.Stderr
	if err := editorCommand.Run(); err != nil {
		return err
	}
	return app.AppCtx.Kernel.TestConfig(profile.File)
}

func getEditorCommand(file, editor string) (*exec.Cmd, error) {
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
