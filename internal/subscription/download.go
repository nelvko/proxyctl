package subscription

import (
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"unicode"

	"github.com/nelvko/proxyctl/internal/config"
	"github.com/nelvko/proxyctl/internal/download"
	"github.com/nelvko/proxyctl/internal/kernel"
	"github.com/nelvko/proxyctl/internal/redact"
)

func validateProfile(p profile) error {
	if err := ValidateSourceURL(p.URL); err != nil {
		return err
	}
	return ValidateDownloadSettings(p.Download)
}

func (s *Service) fetchSource(u *url.URL, dst io.Writer, p profile) (string, error) {
	if err := validateProfile(p); err != nil {
		return "", err
	}
	if err := s.ctx.Err(); err != nil {
		return "", err
	}
	switch u.Scheme {
	case "file":
		src, err := os.Open(u.Path)
		if err != nil {
			return "", err
		}
		defer src.Close()
		_, err = io.Copy(dst, &contextReader{ctx: s.ctx, reader: src})
		return cleanName(filepath.Base(u.Path)), err
	case "http", "https":
		transport := http.DefaultTransport.(*http.Transport).Clone()
		if DownloadRoute(p.Download) == "direct" {
			transport.Proxy = nil
		}
		cl := &http.Client{Transport: transport}
		if DownloadRoute(p.Download) == "kernel" {
			var err error
			cl, err = s.kernelClient()
			if err != nil {
				return "", err
			}
		}
		defer cl.CloseIdleConnections()
		name := cleanName(u.Hostname())
		headers := make(http.Header)
		for key, value := range p.Download.Headers {
			headers.Set(key, value)
		}
		ctx, cancel := context.WithTimeout(s.ctx, DownloadTimeout(p.Download))
		defer cancel()
		_, err := download.New(nil, cl, nil).Fetch(ctx, u.String(), dst, nil, download.RequestOptions{
			Headers: headers, UserAgent: DownloadUserAgent(p.Download), SkipCert: p.Download.SkipCert,
			OnResponse: func(headers http.Header) {
				name = cleanName(u.Hostname())
				_, params, err := mime.ParseMediaType(headers.Get("Content-Disposition"))
				if err == nil {
					if candidate := cleanName(params["filename"]); candidate != "" {
						name = candidate
					}
				}
			},
		})
		if errors.Is(err, context.DeadlineExceeded) {
			return "", fmt.Errorf("download timed out; increase the subscription timeout with sub modify <name> --timeout or retry with --timeout: %w", err)
		}
		return name, redact.Error(err)
	}
	return "", errors.New("unsupported source URL scheme")
}

type contextReader struct {
	ctx    context.Context
	reader io.Reader
}

func (r *contextReader) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.reader.Read(p)
}

func (s *Service) kernelClient() (*http.Client, error) {
	if s.kernel == nil {
		return nil, errors.New("--download-via=kernel requires a selected, running kernel")
	}
	running, err := s.kernel.IsActive()
	if err != nil {
		return nil, err
	}
	if !running {
		return nil, errors.New("--download-via=kernel requires a running kernel; run proxyctl start first")
	}
	endpoints, err := s.kernel.ProxyEndpoints()
	if err != nil {
		return nil, fmt.Errorf("resolve kernel download endpoint: %w", err)
	}
	endpoint := endpoints.HTTP
	if endpoint == "" {
		endpoint = endpoints.SOCKS
	}
	client := download.ProxyClient(endpoint)
	if client == nil {
		return nil, errors.New("kernel has no supported HTTP or SOCKS download endpoint")
	}
	return client, nil
}

func cleanName(value string) string {
	value = strings.ReplaceAll(value, "\\", "/")
	value = filepath.Base(value)
	value = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, value)
	value = strings.TrimSpace(value)
	for _, ext := range []string{".yaml", ".yml", ".json"} {
		if strings.HasSuffix(strings.ToLower(value), ext) {
			value = value[:len(value)-len(ext)]
			break
		}
	}
	runes := []rune(value)
	if len(runes) > 80 {
		value = string(runes[:80])
	}
	value = strings.TrimSpace(value)
	if value == "." || value == ".." || value == "/" {
		return ""
	}
	return value
}

func (s *Service) availableName(base string) string {
	if base == "" {
		base = "profile"
	}
	name := base
	for n := 2; s.CheckNameAvailable(name) != nil; n++ {
		name = fmt.Sprintf("%s-%d", base, n)
	}
	return name
}

func (s *Service) validateDownload(path string) error {
	if err := s.ctx.Err(); err != nil {
		return err
	}
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if info.Size() == 0 {
		return errors.New("downloaded configuration is empty; nothing was saved")
	}
	err = s.kernel.TestConfig(s.ctx, path)
	if err == nil {
		return nil
	}
	if s.ctx.Err() != nil {
		return s.ctx.Err()
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return err
	}
	var invalid *kernel.ConfigValidationError
	if !errors.As(err, &invalid) {
		return err
	}
	reason := redact.Text(strings.ReplaceAll(err.Error(), path, "downloaded configuration"))
	if !s.KeepFailed {
		return fmt.Errorf("%s\ndownloaded configuration was not saved; retry with --keep-failed to inspect it", reason)
	}
	dir, dirErr := config.DiagnosticsDir()
	if dirErr != nil {
		return errors.Join(errors.New(reason), dirErr)
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return errors.Join(errors.New(reason), err)
	}
	if err := os.Chmod(dir, 0o700); err != nil {
		return errors.Join(errors.New(reason), err)
	}
	data, readErr := os.ReadFile(path)
	if readErr != nil {
		return errors.Join(errors.New(reason), readErr)
	}
	f, saveErr := os.CreateTemp(dir, "invalid-*.yaml")
	if saveErr != nil {
		return errors.Join(errors.New(reason), saveErr)
	}
	_, saveErr = f.Write(data)
	saveErr = errors.Join(saveErr, f.Close())
	if saveErr != nil {
		_ = os.Remove(f.Name())
		return errors.Join(errors.New(reason), saveErr)
	}
	return fmt.Errorf("%s\nfailed configuration saved to %s (not applied; remove it when finished)", reason, f.Name())
}
