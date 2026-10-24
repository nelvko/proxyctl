package kernel

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"strconv"
	"strings"
	"time"

	"github.com/nelvko/proxyctl/internal/config"
	"github.com/nelvko/proxyctl/internal/download"
	"github.com/nelvko/unisvc"
)

type SingBox struct {
	unisvc.Service
	cfg config.KernelConfig
}

func (s SingBox) ServiceSpec() *unisvc.Spec {
	return &unisvc.Spec{Command: s.cfg.Bin, Args: []string{"-D", s.cfg.ConfigDir, "-c", s.cfg.ConfigFile, "run"}, Environment: []string{"LD_LIBRARY_PATH=" + filepath.Dir(s.cfg.Bin)}}
}
func (s SingBox) ConfigFile() string         { return s.cfg.ConfigFile }
func (s SingBox) ConfigFormat() ConfigFormat { return FormatSingBox }

func (s SingBox) TestConfig(ctx context.Context, file string) error {
	ctx, cancel := context.WithTimeout(ctx, configTestTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, s.cfg.Bin, "-D", s.cfg.ConfigDir, "-c", file, "check")
	cmd.Env = append(os.Environ(), "LD_LIBRARY_PATH="+filepath.Dir(s.cfg.Bin))
	out, err := cmd.CombinedOutput()
	if err == nil {
		return nil
	}
	if ctx.Err() != nil {
		return fmt.Errorf("validate configuration: %w", ctx.Err())
	}
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		return &ConfigValidationError{Output: strings.TrimSpace(string(out))}
	}
	return fmt.Errorf("run kernel validation: %w", err)
}

func (s SingBox) LatestArtifact(ctx context.Context, downloads *download.Downloader) (*Artifact, error) {
	if runtime.GOOS != "linux" {
		return nil, fmt.Errorf("sing-box service installation requires Linux, got %s", runtime.GOOS)
	}
	release, err := fetchGithubRelease(ctx, "https://api.github.com/repos/SagerNet/sing-box/releases/latest", downloads)
	if err != nil {
		return nil, err
	}
	return singBoxArtifact(release)
}

func singBoxArtifact(release *githubRelease) (*Artifact, error) {
	version := strings.TrimPrefix(release.Tag, "v")
	arch := singBoxArch()
	asset, err := release.Asset("sing-box-" + version + "-linux-" + arch + ".tar.gz")
	if err != nil {
		return nil, err
	}
	base := "sing-box-" + version + "-linux-" + arch + "/"
	return &Artifact{URL: asset.URL, Version: version, SHA256: asset.SHA256, Size: asset.Size, FromAPI: true, Archive: "tar.gz", Entry: base + "sing-box", Extra: []string{base + "libcronet.so"}}, nil
}

func singBoxArch() string {
	if runtime.GOARCH != "arm" {
		return runtime.GOARCH
	}
	if info, ok := debug.ReadBuildInfo(); ok {
		for _, setting := range info.Settings {
			if setting.Key == "GOARM" && (setting.Value == "5" || setting.Value == "6" || setting.Value == "7") {
				return "armv" + setting.Value
			}
		}
	}
	return "armv7"
}

func (s SingBox) InstalledVersion(ctx context.Context) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, s.cfg.Bin, "version")
	cmd.Env = append(os.Environ(), "LD_LIBRARY_PATH="+filepath.Dir(s.cfg.Bin))
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("query version: %w", err)
	}
	for _, line := range strings.Split(string(out), "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 3 && fields[0] == "sing-box" && fields[1] == "version" {
			return strings.TrimPrefix(fields[2], "v"), nil
		}
	}
	return "", fmt.Errorf("query version: unrecognized output: %q", strings.TrimSpace(string(out)))
}

func (s SingBox) ProxyEndpoints() (Endpoints, error) {
	raw, err := os.ReadFile(s.cfg.ConfigFile)
	if err != nil {
		return Endpoints{}, fmt.Errorf("read kernel config: %w", err)
	}
	var doc struct {
		Inbounds []struct {
			Type   string `json:"type"`
			Listen string `json:"listen"`
			Port   int    `json:"listen_port"`
			Users  []struct {
				Username string `json:"username"`
				Password string `json:"password"`
			} `json:"users"`
		} `json:"inbounds"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		return Endpoints{}, fmt.Errorf("parse kernel config: %w", err)
	}
	var endpoints Endpoints
	for _, in := range doc.Inbounds {
		if in.Type != "mixed" && in.Type != "http" && in.Type != "socks" {
			continue
		}
		if in.Port < 1 || in.Port > 65535 {
			return Endpoints{}, fmt.Errorf("invalid inbound port %d", in.Port)
		}
		host := in.Listen
		if host == "" || host == "0.0.0.0" || host == "*" {
			host = "127.0.0.1"
		}
		if host == "::" {
			host = "::1"
		}
		addr := net.JoinHostPort(host, strconv.Itoa(in.Port))
		auth := ""
		if len(in.Users) > 0 {
			auth = url.UserPassword(in.Users[0].Username, in.Users[0].Password).String() + "@"
		}
		if (in.Type == "mixed" || in.Type == "http") && endpoints.HTTP == "" {
			endpoints.HTTP = "http://" + auth + addr
		}
		if (in.Type == "mixed" || in.Type == "socks") && endpoints.SOCKS == "" {
			endpoints.SOCKS = "socks5h://" + auth + addr
		}
	}
	if endpoints.Preferred() == "" {
		return endpoints, fmt.Errorf("no usable HTTP, SOCKS or mixed inbound in %s", s.cfg.ConfigFile)
	}
	return endpoints, nil
}
