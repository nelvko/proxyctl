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

// ErrNoActiveProfile means no subscription has been chosen yet — a normal
// state, unlike a dangling Use pointing at a deleted profile.
var ErrNoActiveProfile = errors.New("no active profile")

type profile = config.Profile

type Service struct {
	subConfig *config.SubConfig
	kernel    kernel.Kernel
}

func NewService(subCfg *config.SubConfig, k kernel.Kernel) *Service {
	return &Service{
		subConfig: subCfg,
		kernel:    k,
	}
}

func (s *Service) List() []profile {
	return append([]profile(nil), s.subConfig.Profiles...)
}

func (s *Service) CurrentName() string {
	return s.subConfig.Use
}

func (s *Service) Add(option *profile) error {
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

	// Create the temp file next to its destination: renaming across
	// filesystems (/tmp is often tmpfs) fails with EXDEV.
	profilesDir, err := config.ProfilesDir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(profilesDir, 0o755); err != nil {
		return err
	}
	tmpFile, err := os.CreateTemp(profilesDir, ".profile-*")
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

	option.File = filepath.Join(profilesDir, option.Name+".yaml")
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

	// The first profile becomes active automatically.
	if s.subConfig.Use == "" && len(s.subConfig.Profiles) == 1 {
		return s.Use(option.Name)
	}
	return nil
}

func (s *Service) ValidateName(name string) error {
	if name == "" {
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
	i := slices.IndexFunc(s.subConfig.Profiles, func(p profile) bool {
		return p.Name == name
	})
	if i == -1 {
		return profile{}, fmt.Errorf("profile %q not found\nUse `proxyctl sub list` to see available profiles", name)
	}
	return s.subConfig.Profiles[i], nil
}

func (s *Service) Using() (profile, error) {
	if s.subConfig.Use == "" {
		return profile{}, ErrNoActiveProfile
	}
	p, err := s.Get(s.subConfig.Use)
	if err != nil {
		return profile{}, fmt.Errorf("failed to get current profile: %w", err)
	}
	return p, nil
}

func (s *Service) Use(profileName string) error {
	i := slices.IndexFunc(s.subConfig.Profiles, func(p profile) bool {
		return p.Name == profileName
	})
	if i == -1 {
		return fmt.Errorf("can't find %s profile", profileName)
	}

	useFile := s.subConfig.Profiles[i].File
	kernelCfg := s.kernel.ConfigFile()
	if err := s.kernel.TestConfig(useFile); err != nil {
		return err
	}

	newBytes, err := os.ReadFile(useFile)
	if err != nil {
		return err
	}
	// Keep the old bytes so a failed restart can restore them — otherwise
	// the kernel runs the new subscription while profiles.yaml still
	// points at the old one.
	old, oldErr := os.ReadFile(kernelCfg)
	if err := os.WriteFile(kernelCfg, newBytes, 0o644); err != nil {
		return err
	}

	if err := s.commitUse(kernelCfg, old, oldErr); err != nil {
		return err
	}

	s.subConfig.Use = profileName
	if err := s.save(); err != nil {
		return fmt.Errorf("kernel switched but failed to persist subscription state: %w", err)
	}
	return nil
}

// commitUse restarts the kernel and verifies it came back up, rolling the
// kernel config back on failure.
func (s *Service) commitUse(kernelCfg string, old []byte, oldErr error) error {
	var switchErr error
	if err := s.kernel.Restart(); err != nil {
		switchErr = err
	} else if active, err := s.kernel.IsActive(); err != nil {
		switchErr = err
	} else if !active {
		switchErr = errors.New("kernel failed to become active after restart")
	}
	if switchErr == nil {
		return nil
	}

	if oldErr != nil {
		return fmt.Errorf("switch failed: %v (kernel config left switched; previous config unreadable)", switchErr)
	}
	if err := os.WriteFile(kernelCfg, old, 0o644); err == nil {
		_ = s.kernel.Restart()
		return fmt.Errorf("switch failed: %v (kernel config restored)", switchErr)
	}
	return fmt.Errorf("switch failed: %v (kernel config left switched; restore %s manually)", switchErr, kernelCfg)
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
