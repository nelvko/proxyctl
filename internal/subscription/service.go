package subscription

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
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
	"github.com/nelvko/proxyctl/internal/ui"
)

const defaultDownloadTimeout = 10 * time.Second

// ErrNoActiveProfile means no subscription has been chosen yet — a normal
// state, unlike a dangling Use pointing at a deleted profile.
var ErrNoActiveProfile = errors.New("no active profile")

type profile = config.Profile

type Service struct {
	subConfig *config.SubscriptionConfig
	kernel    kernel.Kernel
}

func NewService(subCfg *config.SubscriptionConfig, k kernel.Kernel) *Service {
	return &Service{
		subConfig: subCfg,
		kernel:    k,
	}
}

func (s *Service) List() []profile {
	return append([]profile(nil), s.subConfig.Profiles...)
}

func (s *Service) ActiveName() string {
	return s.subConfig.Use
}

func (s *Service) Add(draft *profile) error {
	if draft == nil {
		return errors.New("profile is nil")
	}

	u, err := url.Parse(draft.URL)
	if err != nil {
		return err
	}

	if draft.Name == "" {
		draft.Name = fmt.Sprintf("%d", time.Now().Unix())
	}
	if err := s.CheckNameAvailable(draft.Name); err != nil {
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

	if err := s.fetchSource(u, tmpFile, *draft); err != nil {
		return err
	}

	if err := s.kernel.TestConfig(tmpName); err != nil {
		return err
	}

	draft.File = filepath.Join(profilesDir, draft.Name+".yaml")
	if err := tmpFile.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpName, draft.File); err != nil {
		return err
	}
	tmpName = ""

	s.subConfig.Profiles = append(s.subConfig.Profiles, *draft)
	if err := s.save(); err != nil {
		return err
	}
	ui.Ok(fmt.Sprintf("profile %q added successfully", draft.Name))

	// The first profile becomes active automatically.
	if s.subConfig.Use == "" && len(s.subConfig.Profiles) == 1 {
		if err := s.Use(draft.Name); err != nil {
			return err
		}
		ui.Ok(fmt.Sprintf("profile %q activated (first profile)", draft.Name))
	}
	return nil
}

func (s *Service) CheckNameAvailable(name string) error {
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

	if current, _ := s.Active(); current.Name == profileName {
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

func (s *Service) Active() (profile, error) {
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
		if rerr := s.kernel.Restart(); rerr != nil {
			return fmt.Errorf("switch failed: %v (kernel config restored, restore restart failed: %v, run `proxyctl on`)", switchErr, rerr)
		}
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

func (s *Service) downloadTimeout(draft *profile) time.Duration {
	if draft.Update.Timeout > 0 {
		return draft.Update.Timeout
	}
	return defaultDownloadTimeout
}

// fetchSource streams the profile source (a local copy for file URLs, a
// download for http(s)) into dst, honoring the profile's timeout and
// UseProxy settings.
func (s *Service) fetchSource(u *url.URL, dst io.Writer, p profile) error {
	switch u.Scheme {
	case "file":
		src, err := os.Open(u.Path)
		if err != nil {
			return err
		}
		defer src.Close()
		_, err = io.Copy(dst, src)
		return err
	case "http", "https":
		cl := httpx.Client()
		if p.Update.UseProxy {
			if kc := s.kernelClient(); kc != nil {
				cl = kc
			}
		}
		ctx, cancel := context.WithTimeout(context.Background(), s.downloadTimeout(&p))
		defer cancel()
		if _, err := httpx.DownloadVia(cl, ctx, u.String(), dst, nil); err != nil {
			if errors.Is(err, context.DeadlineExceeded) {
				return errors.New("download timed out, please try again later or specify a longer timeout")
			}
			return err
		}
		return nil
	default:
		return fmt.Errorf("unsupported scheme: %s", u.Scheme)
	}
}

// kernelClient returns a client routed through the running kernel's inbound
// for profiles with UseProxy, or nil when that is unavailable. Per-request:
// it must not leak into other profiles' downloads.
func (s *Service) kernelClient() *http.Client {
	inbound, ok := s.kernel.(interface{ InboundAddr() string })
	if !ok {
		return nil
	}
	if on, err := s.kernel.IsActive(); err != nil || !on {
		return nil
	}
	return httpx.KernelClient(inbound.InboundAddr())
}

// Update re-fetches the named profiles (all of them when none are named).
// A profile whose content did not change is left untouched; the active one
// is re-applied to the kernel only when it actually changed. A failing
// profile does not stop the others; the errors are joined.
func (s *Service) Update(names ...string) error {
	targets, err := s.resolveTargets(names)
	if err != nil {
		return err
	}

	var errs []error
	activeUpdated := false
	for _, p := range targets {
		changed, err := s.updateProfile(p)
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", p.Name, err))
			continue
		}
		if !changed {
			ui.Ok(fmt.Sprintf("profile %q unchanged", p.Name))
			continue
		}
		ui.Ok(fmt.Sprintf("profile %q updated", p.Name))
		if s.ActiveName() == p.Name {
			activeUpdated = true
		}
	}
	// Re-apply even when other profiles failed: the active profile's file
	// on disk already changed, and skipping it would strand the kernel on
	// the old subscription forever — the next update would see identical
	// bytes and report "unchanged".
	if activeUpdated {
		if err := s.applyActive(); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// applyActive puts the active profile's (already updated) bytes into the
// kernel config. A running kernel restarts to pick them up; a stopped one
// only gets the staged config — `proxyctl on` activates it, and an update
// must never resurrect a kernel the user deliberately stopped.
func (s *Service) applyActive() error {
	running, err := s.kernel.IsActive()
	if err != nil {
		return err
	}
	if running {
		return s.Use(s.ActiveName())
	}
	p, err := s.Active()
	if err != nil {
		return err
	}
	b, err := os.ReadFile(p.File)
	if err != nil {
		return err
	}
	if err := os.WriteFile(s.kernel.ConfigFile(), b, 0o644); err != nil {
		return err
	}
	ui.Err(fmt.Sprintf("kernel is stopped; updated config staged, `proxyctl on` activates it"))
	return nil
}

func (s *Service) resolveTargets(names []string) ([]profile, error) {
	if len(names) == 0 {
		return s.List(), nil
	}
	targets := make([]profile, 0, len(names))
	for _, name := range names {
		p, err := s.Get(name)
		if err != nil {
			return nil, err
		}
		targets = append(targets, p)
	}
	return targets, nil
}

// updateProfile re-fetches one profile and swaps its file in atomically,
// after the kernel accepts the new config. It reports whether the content
// actually changed; identical content leaves the file — and the kernel —
// untouched.
func (s *Service) updateProfile(p profile) (bool, error) {
	u, err := url.Parse(p.URL)
	if err != nil {
		return false, err
	}
	profilesDir, err := config.ProfilesDir()
	if err != nil {
		return false, err
	}
	if err := os.MkdirAll(profilesDir, 0o755); err != nil {
		return false, err
	}
	tmpFile, err := os.CreateTemp(profilesDir, ".profile-*")
	if err != nil {
		return false, err
	}
	tmpName := tmpFile.Name()
	defer func() {
		tmpFile.Close()
		if tmpName != "" {
			_ = os.Remove(tmpName)
		}
	}()

	if err := s.fetchSource(u, tmpFile, p); err != nil {
		return false, err
	}
	if err := s.kernel.TestConfig(tmpName); err != nil {
		return false, err
	}
	if !contentChanged(tmpName, p.File) {
		return false, nil
	}
	if err := tmpFile.Close(); err != nil {
		return false, err
	}
	if err := os.Rename(tmpName, p.File); err != nil {
		return false, err
	}
	tmpName = ""
	return true, nil
}

// contentChanged reports whether tmp differs from the profile's current
// file; a missing or unreadable current file counts as changed.
func contentChanged(tmp, cur string) bool {
	fresh, err := os.ReadFile(tmp)
	if err != nil {
		return true
	}
	current, err := os.ReadFile(cur)
	if err != nil {
		return true
	}
	return !bytes.Equal(fresh, current)
}

func (s *Service) save() error {
	return config.SaveSubscriptionConfig(s.subConfig)
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
