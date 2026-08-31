package kernel

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"runtime/debug"
	"strconv"
	"strings"
	"time"

	"go.yaml.in/yaml/v3"
	"golang.org/x/sys/cpu"

	"github.com/nelvko/proxyctl/internal/config"
	"github.com/nelvko/proxyctl/internal/httpx"
	"github.com/nelvko/unisvc"
)

type Mihomo struct {
	unisvc.Service

	cfg config.KernelConfig
}

func (m Mihomo) ConfigFile() string         { return m.cfg.ConfigFile }
func (m Mihomo) ConfigFormat() ConfigFormat { return FormatClash }

// TestConfig tests the configuration file for Mihomo by executing the Mihomo binary.
// It captures the output and returns an error if the test fails.
func (m Mihomo) TestConfig(configFile string) error {
	cmd := exec.Command(
		m.cfg.Bin,
		"-t",
		"-f", configFile,
		"-d", m.cfg.ConfigDir,
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("test config: \n%s", out)
	}
	return nil
}

const (
	mihomoDownloadBaseURL = "https://github.com/MetaCubeX/mihomo/releases/latest/download/"
	mihomoVersionURL      = mihomoDownloadBaseURL + "version.txt"
	mihomoReleasesAPI     = "https://api.github.com/repos/MetaCubeX/mihomo/releases/latest"
)

// LatestArtifact describes the latest mihomo release. The GitHub API is the
// primary source: it carries the asset digest that pins what a mirror must
// serve. When it is unreachable and a mirror is configured, resolution fails
// closed unless PROXYCTL_NO_VERIFY is set; direct downloads fall back to
// version.txt without a digest, since github.com TLS still authenticates the
// origin.
func (m Mihomo) LatestArtifact() (*Artifact, error) {
	art, apiErr := latestArtifactFromAPI()
	if apiErr == nil {
		return art, nil
	}
	if os.Getenv("PROXYCTL_NO_VERIFY") != "" {
		fmt.Fprintf(os.Stderr, "proxyctl: PROXYCTL_NO_VERIFY set, skipping integrity check (%v)\n", apiErr)
		return artifactFromVersionTXT()
	}
	if len(httpx.Mirrors()) > 0 {
		return nil, fmt.Errorf("%w\nmirror downloads verify integrity against api.github.com; unset the mirror or set PROXYCTL_NO_VERIFY=1 to skip verification", apiErr)
	}
	fmt.Fprintf(os.Stderr, "proxyctl: release metadata unavailable (%v); continuing via version.txt without integrity verification\n", apiErr)
	return artifactFromVersionTXT()
}

// latestArtifactFromAPI resolves version, asset URL, digest and size in one
// request. It is fetched directly (HTTP(S)_PROXY env still applies, so
// `proxyctl on` routes it through the local kernel): ghproxy-style mirrors
// only front github.com paths, and the API is the trust anchor for the
// digest — a mirror must supply bytes, never identity.
func latestArtifactFromAPI() (*Artifact, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, mihomoReleasesAPI, nil)
	if err != nil {
		return nil, fmt.Errorf("fetch release metadata: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := httpx.Client().Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch release metadata: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		if resp.StatusCode == http.StatusForbidden {
			return nil, fmt.Errorf("fetch release metadata: %s (rate-limited?)", resp.Status)
		}
		return nil, fmt.Errorf("fetch release metadata: %s", resp.Status)
	}

	var release struct {
		TagName string `json:"tag_name"`
		Assets  []struct {
			Name               string `json:"name"`
			BrowserDownloadURL string `json:"browser_download_url"`
			Size               int64  `json:"size"`
			Digest             string `json:"digest"`
		} `json:"assets"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, fmt.Errorf("decode release metadata: %w", err)
	}

	name := mihomoBaseName() + "-" + release.TagName + archiveSuffix()
	for _, a := range release.Assets {
		if a.Name != name {
			continue
		}
		return &Artifact{
			URL:     a.BrowserDownloadURL,
			Version: release.TagName,
			SHA256:  strings.TrimPrefix(a.Digest, "sha256:"),
			Size:    a.Size,
		}, nil
	}
	return nil, fmt.Errorf("release %s has no %q asset", release.TagName, name)
}

// artifactFromVersionTXT builds the artifact from version.txt (mirror
// candidates included), without a digest.
func artifactFromVersionTXT() (*Artifact, error) {
	version, err := latestVersion()
	if err != nil {
		return nil, err
	}
	return &Artifact{
		URL:     mihomoDownloadBaseURL + mihomoBaseName() + "-" + version + archiveSuffix(),
		Version: version,
	}, nil
}

// latestVersion fetches version.txt, trying each mirror candidate before
// github.com itself.
func latestVersion() (string, error) {
	urls, err := httpx.MirrorCandidates(mihomoVersionURL)
	if err != nil {
		return "", err
	}
	var lastErr error
	for _, u := range urls {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		var body bytes.Buffer
		_, err := httpx.Download(ctx, u, &body, nil)
		cancel()
		if err == nil {
			if version := strings.TrimSpace(body.String()); version != "" {
				return version, nil
			}
			lastErr = errors.New("empty response")
			continue
		}
		lastErr = err
	}
	return "", fmt.Errorf("fetch version: %w", lastErr)
}

func archiveSuffix() string {
	if runtime.GOOS == "windows" {
		return ".zip"
	}
	return ".gz"
}

// InboundAddr returns the kernel's local HTTP inbound (mixed-port, or the
// plain http port) as host:port, or "" when the active config exposes none.
func (m Mihomo) InboundAddr() string {
	b, err := os.ReadFile(m.cfg.ConfigFile)
	if err != nil {
		return ""
	}
	var c struct {
		MixedPort int `yaml:"mixed-port"`
		Port      int `yaml:"port"`
	}
	if err := yaml.Unmarshal(b, &c); err != nil {
		return ""
	}
	for _, p := range []int{c.MixedPort, c.Port} {
		if p > 0 && p < 65536 {
			return net.JoinHostPort("127.0.0.1", strconv.Itoa(p))
		}
	}
	return ""
}

// InstalledVersion reports the on-disk binary's release version by running
// it with -v; it errors when the binary is missing or its output cannot be
// parsed.
func (m Mihomo) InstalledVersion() (string, error) {
	out, err := exec.Command(m.cfg.Bin, "-v").CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("query version: %w", err)
	}
	version := parseVersion(string(out))
	if version == "" {
		return "", fmt.Errorf("query version: unrecognized output: %q", strings.TrimSpace(string(out)))
	}
	return version, nil
}

// parseVersion extracts the release token ("v1.19.30") from `mihomo -v`
// output, which embeds it among build metadata:
// "Mihomo Meta v1.19.30 linux amd64 with go1.26.6 …".
func parseVersion(out string) string {
	for _, field := range strings.Fields(out) {
		if len(field) > 1 && field[0] == 'v' && strings.Contains(field, ".") {
			return field
		}
	}
	return ""
}

func mihomoBaseName() string {
	var (
		GOARM   string
		GOMIPS  string
		GOAMD64 string
	)
	if info, ok := debug.ReadBuildInfo(); ok {
		for _, bs := range info.Settings {
			switch bs.Key {
			case "GOARM":
				GOARM = bs.Value
			case "GOMIPS":
				GOMIPS = bs.Value
			case "GOAMD64":
				GOAMD64 = detectAMD64Level()
			}
		}
	}

	switch runtime.GOARCH {
	case "arm":
		// mihomo-linux-armv5
		return fmt.Sprintf("mihomo-%s-%sv%s", runtime.GOOS, runtime.GOARCH, GOARM)
	case "arm64":
		if runtime.GOOS == "android" {
			// mihomo-android-arm64-v8
			return fmt.Sprintf("mihomo-%s-%s-v8", runtime.GOOS, runtime.GOARCH)
		} else {
			// mihomo-linux-arm64
			return fmt.Sprintf("mihomo-%s-%s", runtime.GOOS, runtime.GOARCH)
		}
	case "mips", "mipsle":
		// mihomo-linux-mips-hardfloat
		return fmt.Sprintf("mihomo-%s-%s-%s", runtime.GOOS, runtime.GOARCH, GOMIPS)
	case "amd64":
		if runtime.GOOS == "android" {
			// mihomo-android-amd64 — android asset names carry no level.
			return fmt.Sprintf("mihomo-%s-%s", runtime.GOOS, runtime.GOARCH)
		}
		// mihomo-linux-amd64-v1
		return fmt.Sprintf("mihomo-%s-%s-%s", runtime.GOOS, runtime.GOARCH, GOAMD64)
	default:
		// mihomo-linux-386
		// mihomo-linux-mips64
		// mihomo-linux-riscv64
		// mihomo-linux-s390x
		return fmt.Sprintf("mihomo-%s-%s", runtime.GOOS, runtime.GOARCH)
	}
}

func detectAMD64Level() string {
	// v3 必须同时满足
	if cpu.X86.HasAVX &&
		cpu.X86.HasAVX2 &&
		cpu.X86.HasBMI1 &&
		cpu.X86.HasBMI2 &&
		cpu.X86.HasFMA {
		return "v3"
	}

	// v2 必须同时满足
	if cpu.X86.HasSSE3 &&
		cpu.X86.HasSSSE3 &&
		cpu.X86.HasSSE41 &&
		cpu.X86.HasSSE42 &&
		cpu.X86.HasPOPCNT {
		return "v2"
	}

	return "v1"
}
