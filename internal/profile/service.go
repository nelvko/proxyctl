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

	"github.com/nelvko/proxyctl/internal/config"
	"github.com/nelvko/proxyctl/internal/httpx"
	"github.com/nelvko/proxyctl/internal/kernel"
	"github.com/nelvko/proxyctl/internal/log"
)

const defaultDownloadTimeout = 10 * time.Second

type profile = config.Profile

type Service struct {
	appConfig *config.AppConfig
	subConfig *config.SubConfig
	kernel    kernel.Kernel
}

func NewService(appCfg *config.AppConfig, subCfg *config.SubConfig, k kernel.Kernel) *Service {
	return &Service{
		appConfig: appCfg,
		subConfig: subCfg,
		kernel:    k,
	}
}

func (s *Service) List() []profile {
	if s == nil || s.subConfig == nil {
		return nil
	}
	return append([]profile(nil), s.subConfig.Profiles...)
}

func (s *Service) CurrentName() string {
	if s == nil || s.subConfig == nil {
		return ""
	}
	return s.subConfig.Use
}

func (s *Service) Add(option *profile) error {
	if s == nil {
		return errors.New("profile service is nil")
	}
	if option == nil {
		return errors.New("profile is nil")
	}

	u, err := url.Parse(option.URL)
	if err != nil {
		return err
	}

	if option.Name == "" {
		option.Name = fmt.Sprintf("%d", time.Now().Unix())
	}
	if err := s.ValidateName(option.Name); err != nil {
		return err
	}

	tmpFile, err := os.CreateTemp("", "profile-*")
	if err != nil {
		return err
	}

	tmpName := tmpFile.Name()
	defer func() {
		tmpFile.Close()
		if tmpName != "" {
			_ = os.Remove(tmpName)
		}
	}()

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
		ctx, cancel := context.WithTimeout(context.Background(), s.downloadTimeout(option))
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

	if err := s.kernel.TestConfig(tmpName); err != nil {
		return err
	}

	option.File = filepath.Join(config.SubDir, option.Name+".yaml")
	if err := tmpFile.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpName, option.File); err != nil {
		return err
	}
	tmpName = ""

	s.subConfig.Profiles = append(s.subConfig.Profiles, *option)
	if err := s.save(); err != nil {
		return err
	}

	log.Ok(fmt.Sprintf("profile %q added successfully", option.Name))
	return nil
}

func (s *Service) ValidateName(name string) error {
	if s == nil || name == "" {
		return nil
	}
	ok := slices.ContainsFunc(s.subConfig.Profiles, func(p profile) bool {
		return p.Name == name
	})
	if ok {
		return fmt.Errorf("profile %q already exists", name)
	}
	return nil
}

func (s *Service) Delete(profileName string, force bool) error {
	if s == nil {
		return errors.New("profile service is nil")
	}

	tgt, err := s.Get(profileName)
	if err != nil {
		return err
	}

	if current, _ := s.Using(); current.Name == profileName {
		if !force {
			return fmt.Errorf("profile %q is currently in use", profileName)
		}
		s.subConfig.Use = ""
	}

	s.subConfig.Profiles = slices.DeleteFunc(s.subConfig.Profiles, func(p profile) bool {
		return p.Name == profileName
	})
	if err := s.save(); err != nil {
		return err
	}

	if err := os.Remove(tgt.File); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

func (s *Service) Get(name string) (profile, error) {
	if s == nil {
		return profile{}, errors.New("profile service is nil")
	}

	i := slices.IndexFunc(s.subConfig.Profiles, func(p profile) bool {
		return p.Name == name
	})
	if i == -1 {
		return profile{}, fmt.Errorf("profile %q not found\nUse `proxyctl sub list` to see available profiles", name)
	}
	return s.subConfig.Profiles[i], nil
}

func (s *Service) Using() (profile, error) {
	if s == nil {
		return profile{}, errors.New("profile service is nil")
	}
	if s.subConfig.Use == "" {
		return profile{}, errors.New("no active profile")
	}
	p, err := s.Get(s.subConfig.Use)
	if err != nil {
		return profile{}, fmt.Errorf("failed to get current profile: %w", err)
	}
	return p, nil
}

func (s *Service) Use(profileName string) error {
	if s == nil {
		return errors.New("profile service is nil")
	}

	i := slices.IndexFunc(s.subConfig.Profiles, func(p profile) bool {
		return p.Name == profileName
	})
	if i == -1 {
		return fmt.Errorf("can't find %s profile", profileName)
	}

	useFile := s.subConfig.Profiles[i].File
	kernelCfg := s.appConfig.Kernel.ConfigFile
	if err := s.kernel.TestConfig(useFile); err != nil {
		return err
	}

	bytes, err := os.ReadFile(useFile)
	if err != nil {
		return err
	}
	if err := os.WriteFile(kernelCfg, bytes, 0o666); err != nil {
		return err
	}

	if err := s.kernel.Restart(); err != nil {
		return err
	}
	active, err := s.kernel.IsActive()
	if err != nil {
		return err
	}
	if !active {
		return errors.New("kernel failed to become active after restart")
	}

	s.subConfig.Use = profileName
	return s.save()
}

func (s *Service) Edit(profileName, editor string) error {
	cmd, err := s.EditorCommand(profileName, editor)
	if err != nil {
		return err
	}
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return err
	}
	return s.ValidateProfile(profileName)
}

func (s *Service) EditorCommand(profileName, editor string) (*exec.Cmd, error) {
	p, err := s.Get(profileName)
	if err != nil {
		return nil, err
	}
	return editorCommand(p.File, editor)
}

func (s *Service) ValidateProfile(profileName string) error {
	p, err := s.Get(profileName)
	if err != nil {
		return err
	}
	return s.kernel.TestConfig(p.File)
}

func (s *Service) downloadTimeout(option *profile) time.Duration {
	if option.Update.Timeout > 0 {
		return option.Update.Timeout
	}
	return defaultDownloadTimeout
}

func (s *Service) save() error {
	if s == nil || s.subConfig == nil {
		return errors.New("sub config is nil")
	}
	return config.SaveSubConfig(s.subConfig)
}

func editorCommand(file, editor string) (*exec.Cmd, error) {
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
