package kernel

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
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

// configTestTimeout bounds a config test: a validating mihomo may fetch
// missing geodata, which on a blocked route would otherwise hang sub
// add/use/update forever.
const configTestTimeout = 60 * time.Second

// geodataBudget bounds the whole pre-seeding pass: this runs on interactive
// paths (sub add/use/update), so a blocked route must degrade to letting
// mihomo fetch, not stall the command for minutes per file.
const geodataBudget = 90 * time.Second

// geodataMinSize rejects obviously wrong payloads: every rules-dat asset is
// well above 1 MiB, while a mirror error page or captive portal is not.
// Without this a 200 error page would occupy the target path forever
// (ensureGeodata skips files that already exist).
const geodataMinSize = 1 << 20

// geodataBaseURL points at the MetaCubeX rules-dat release; every asset is
// a plain github.com download, so the mirror candidates apply.
const geodataBaseURL = "https://github.com/MetaCubeX/meta-rules-dat/releases/latest/download/"

// geodataReleasesAPI is the trust anchor for geodata digests. A var so
// tests can point at a local server.
var geodataReleasesAPI = "https://api.github.com/repos/MetaCubeX/meta-rules-dat/releases/latest"

// geodataFiles maps the on-disk names mihomo looks for to their download
// URLs. A var so tests can point at a local server.
var geodataFiles = map[string]string{
	"GeoSite.dat":  geodataBaseURL + "geosite.dat",
	"GeoIP.dat":    geodataBaseURL + "geoip.dat",
	"geoip.metadb": geodataBaseURL + "geoip.metadb",
}

// geodataMeta pins one geodata asset.
type geodataMeta struct {
	sha256 string
	size   int64
}

// TestConfig tests the configuration file for Mihomo by executing the Mihomo binary.
// It captures the output and returns an error if the test fails.
func (m Mihomo) TestConfig(configFile string) error {
	// A config referencing geodata makes `mihomo -t` fetch any missing file
	// itself — on a blocked route that hangs until the timeout below. Seed
	// first so the test runs on local files.
	m.ensureGeodata(configFile)

	ctx, cancel := context.WithTimeout(context.Background(), configTestTimeout)
	defer cancel()
	cmd := exec.CommandContext(
		ctx,
		m.cfg.Bin,
		"-t",
		"-f", configFile,
		"-d", m.cfg.ConfigDir,
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		if ctx.Err() != nil {
			return fmt.Errorf("test config timed out after %s — mihomo may be downloading geodata (GeoSite.dat/GeoIP.dat) on a blocked route; pre-seed ~/.config/proxyctl/%s/ via a mirror and retry:\n%s", configTestTimeout, m.cfg.Name, out)
		}
		return fmt.Errorf("test config: \n%s", out)
	}
	return nil
}

// ensureGeodata pre-downloads the geodata files a config references into
// the kernel's config dir, verifying each source against the release digest
// from the GitHub API — a mirror supplies bytes, never identity, same as
// kernel downloads. Without a digest anchor the mirror candidates are
// dropped in favor of TLS-anchored github.com. Files already present are
// left alone; failures degrade to letting mihomo fetch (bounded by the
// config-test timeout).
func (m Mihomo) ensureGeodata(configFile string) {
	if m.cfg.ConfigDir == "" {
		return
	}
	body, err := os.ReadFile(configFile)
	if err != nil {
		return
	}
	needed := geodataNeeded(body)
	if len(needed) == 0 {
		return
	}

	metas := geodataReleaseMetadata()
	if metas == nil && len(httpx.Mirrors()) > 0 && os.Getenv("PROXYCTL_NO_VERIFY") == "" {
		fmt.Fprintf(os.Stderr, "proxyctl: geodata digests unavailable; using github.com directly (set PROXYCTL_NO_VERIFY=1 to allow mirrors)\n")
	}

	ctx, cancel := context.WithTimeout(context.Background(), geodataBudget)
	defer cancel()
	for _, name := range needed {
		if _, err := os.Stat(filepath.Join(m.cfg.ConfigDir, name)); err == nil {
			continue
		}
		if err := m.fetchGeodata(ctx, name, metas[name]); err != nil {
			fmt.Fprintf(os.Stderr, "proxyctl: could not pre-seed %s (%v); letting mihomo fetch it\n", name, err)
		}
	}
}

// geodataReleaseMetadata resolves digests and sizes for all three geodata
// assets in one API call; nil when the API is unreachable (callers degrade).
func geodataReleaseMetadata() map[string]geodataMeta {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, geodataReleasesAPI, nil)
	if err != nil {
		return nil
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := httpx.Client().Do(req)
	if err != nil || resp.StatusCode != http.StatusOK {
		if resp != nil {
			resp.Body.Close()
		}
		return nil
	}
	defer resp.Body.Close()

	var release struct {
		Assets []struct {
			Name   string `json:"name"`
			Size   int64  `json:"size"`
			Digest string `json:"digest"`
		} `json:"assets"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil
	}

	// Map API asset names ("geosite.dat") to our on-disk keys.
	byAsset := make(map[string]string, len(geodataFiles))
	for disk, u := range geodataFiles {
		byAsset[filepath.Base(u)] = disk
	}
	metas := make(map[string]geodataMeta, len(geodataFiles))
	for _, a := range release.Assets {
		if disk, ok := byAsset[a.Name]; ok {
			metas[disk] = geodataMeta{
				sha256: strings.TrimPrefix(a.Digest, "sha256:"),
				size:   a.Size,
			}
		}
	}
	return metas
}

// geodataNeeded lists the geodata file names a config references. GEOIP
// resolves against geoip.metadb by default and GeoIP.dat in geodata-mode,
// so both are seeded — guessing wrong is exactly the hang this prevents.
func geodataNeeded(config []byte) []string {
	var needed []string
	references := func(tokens ...string) bool {
		for _, t := range tokens {
			if bytes.Contains(config, []byte(t)) {
				return true
			}
		}
		return false
	}
	if references("GEOSITE", "geosite") {
		needed = append(needed, "GeoSite.dat")
	}
	if references("GEOIP", "geoip") {
		needed = append(needed, "geoip.metadb", "GeoIP.dat")
	}
	return needed
}

// fetchGeodata downloads one geodata file to its final path atomically,
// trying each allowed source and verifying size and digest when known.
func (m Mihomo) fetchGeodata(ctx context.Context, name string, meta geodataMeta) error {
	sources, err := httpx.MirrorCandidates(geodataFiles[name])
	if err != nil {
		return err
	}
	// No digest anchor: a mirror must not supply identity — keep only the
	// TLS-authenticated direct URL, unless verification is explicitly off.
	if meta.sha256 == "" && len(sources) > 1 && os.Getenv("PROXYCTL_NO_VERIFY") == "" {
		sources = sources[len(sources)-1:]
	}

	dst := filepath.Join(m.cfg.ConfigDir, name)
	f, err := os.CreateTemp(filepath.Dir(dst), "."+filepath.Base(dst)+".tmp*")
	if err != nil {
		return err
	}
	tmpName := f.Name()
	defer func() {
		f.Close()
		os.Remove(tmpName)
	}()

	statusf("downloading %s", name)
	var lastErr error
	for _, src := range sources {
		w := &resettableWriter{f: f, h: sha256.New()}
		progress := progressPrinter()
		n, err := httpx.Download(ctx, src, w, progress)
		if progress != nil {
			fmt.Fprintln(os.Stderr)
		}
		switch {
		case err != nil:
			lastErr = err
		case n < geodataMinSize:
			// An error page or truncated mirror payload must never occupy
			// the target path — the Stat check would treat it as done
			// forever.
			lastErr = fmt.Errorf("%s served %d bytes, below the %d minimum", src, n, geodataMinSize)
		case meta.size > 0 && n != meta.size:
			lastErr = fmt.Errorf("size mismatch for %s: got %d bytes, want %d", src, n, meta.size)
		case meta.sha256 != "" && hex.EncodeToString(w.h.Sum(nil)) != meta.sha256:
			lastErr = fmt.Errorf("integrity check failed for %s: sha256 %s, want %s", src, hex.EncodeToString(w.h.Sum(nil)), meta.sha256)
		default:
			if err := f.Close(); err != nil {
				return err
			}
			if err := os.Rename(tmpName, dst); err != nil {
				return err
			}
			return nil
		}
		if err := w.Reset(); err != nil {
			return err
		}
	}
	return lastErr
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
			FromAPI: true,
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
