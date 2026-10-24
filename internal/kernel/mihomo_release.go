package kernel

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"runtime/debug"
	"strings"
	"time"

	"github.com/nelvko/proxyctl/internal/download"
	"golang.org/x/sys/cpu"
)

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
func (m Mihomo) LatestArtifact(ctx context.Context, downloads *download.Downloader) (*Artifact, error) {
	art, apiErr := latestArtifactFromAPI(ctx, downloads)
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	if apiErr == nil {
		return art, nil
	}
	if os.Getenv("PROXYCTL_NO_VERIFY") != "" {
		downloads.Message(true, "proxyctl: PROXYCTL_NO_VERIFY set, skipping integrity check (%v)\n", apiErr)
		return artifactFromVersionTXT(ctx, downloads)
	}
	if len(downloads.Mirrors()) > 0 {
		return nil, fmt.Errorf("%w\nmirror downloads verify integrity against api.github.com; unset the mirror or set PROXYCTL_NO_VERIFY=1 to skip verification", apiErr)
	}
	downloads.Message(true, "proxyctl: release metadata unavailable (%v); continuing via version.txt without integrity verification\n", apiErr)
	return artifactFromVersionTXT(ctx, downloads)
}

// latestArtifactFromAPI resolves version, asset URL, digest and size in one
// request through the shared GitHub release helper.
func latestArtifactFromAPI(ctx context.Context, downloads *download.Downloader) (*Artifact, error) {
	release, err := fetchGithubRelease(ctx, mihomoReleasesAPI, downloads)
	if err != nil {
		return nil, err
	}
	name := mihomoBaseName() + "-" + release.Tag + archiveSuffix()
	asset, err := release.Asset(name)
	if err != nil {
		return nil, err
	}
	return &Artifact{
		URL:     asset.URL,
		Version: release.Tag,
		SHA256:  asset.SHA256,
		Size:    asset.Size,
		FromAPI: true,
		Archive: "gz",
	}, nil
}

// artifactFromVersionTXT builds the artifact from version.txt (mirror
// candidates included), without a digest.
func artifactFromVersionTXT(ctx context.Context, downloads *download.Downloader) (*Artifact, error) {
	version, err := latestVersion(ctx, downloads)
	if err != nil {
		return nil, err
	}
	return &Artifact{
		URL:     mihomoDownloadBaseURL + mihomoBaseName() + "-" + version + archiveSuffix(),
		Version: version,
		Archive: "gz",
	}, nil
}

// latestVersion fetches version.txt, trying each mirror candidate before
// github.com itself.
func latestVersion(ctx context.Context, downloads *download.Downloader) (string, error) {
	urls, err := downloads.MirrorCandidates(mihomoVersionURL)
	if err != nil {
		return "", err
	}
	var lastErr error
	for _, u := range urls {
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
		ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
		var body bytes.Buffer
		_, err := downloads.Fetch(ctx, u, &body, nil, download.RequestOptions{})
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

// InstalledVersion reports the on-disk binary's release version by running
// it with -v; it errors when the binary is missing or its output cannot be
// parsed.
func (m Mihomo) InstalledVersion(ctx context.Context) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, m.cfg.Bin, "-v").CombinedOutput()
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
