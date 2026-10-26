package subscription

import (
	"errors"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/nelvko/proxyctl/internal/config"
)

func (s *Service) Add(draft *profile) error {
	if err := s.requireRuntime(); err != nil {
		return err
	}
	if draft == nil {
		return errors.New("profile is nil")
	}
	draft.Format = string(s.kernel.ConfigFormat())
	if draft.Format == "" {
		return errors.New("selected kernel has no subscription format; reinstall with --format")
	}

	if err := validateProfile(*draft); err != nil {
		return err
	}
	u, err := url.Parse(draft.URL)
	if err != nil {
		return err
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

	suggested, err := s.fetchSource(u, tmpFile, *draft)
	if err != nil {
		return err
	}
	if draft.Name == "" {
		draft.Name = s.availableName(suggested)
	}

	if err := s.validateDownload(*draft, tmpName); err != nil {
		return err
	}
	if err := s.ctx.Err(); err != nil {
		return err
	}

	ext := ".yaml"
	if draft.Format == "sing-box" {
		ext = ".json"
	}
	draft.File = filepath.Join(profilesDir, "profile-"+strings.TrimPrefix(filepath.Base(tmpName), ".profile-")+ext)
	if err := tmpFile.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpName, draft.File); err != nil {
		return err
	}
	tmpName = ""

	s.subConfig.Profiles = append(s.subConfig.Profiles, draft.Clone())
	if err := s.save(); err != nil {
		s.subConfig.Profiles = s.subConfig.Profiles[:len(s.subConfig.Profiles)-1]
		return errors.Join(err, os.Remove(draft.File))
	}
	return nil
}
