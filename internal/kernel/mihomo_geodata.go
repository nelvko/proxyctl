package kernel

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/nelvko/proxyctl/internal/download"
)

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

// ensureGeodata pre-downloads the geodata files a config references into
// the kernel's config dir, verifying each source against the release digest
// from the GitHub API — a mirror supplies bytes, never identity, same as
// kernel downloads. Without a digest anchor the mirror candidates are
// dropped in favor of TLS-anchored github.com. Files already present are
// left alone; failures degrade to letting mihomo fetch (bounded by the
// config-test timeout).
func (m Mihomo) ensureGeodata(ctx context.Context, configFile string) {
	downloads := m.downloader()
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

	metas := geodataReleaseMetadata(ctx, downloads)
	if metas == nil && len(downloads.Mirrors()) > 0 && os.Getenv("PROXYCTL_NO_VERIFY") == "" {
		downloads.Message(true, "proxyctl: geodata digests unavailable; using github.com directly (set PROXYCTL_NO_VERIFY=1 to allow mirrors)\n")
	}

	ctx, cancel := context.WithTimeout(ctx, geodataBudget)
	defer cancel()
	for _, name := range needed {
		if ctx.Err() != nil {
			return
		}
		if _, err := os.Stat(filepath.Join(m.cfg.ConfigDir, name)); err == nil {
			continue
		}
		if err := m.fetchGeodata(ctx, name, metas[name]); err != nil {
			downloads.Message(true, "proxyctl: could not pre-seed %s (%v); letting mihomo fetch it\n", name, err)
		}
	}
}

// geodataReleaseMetadata resolves digests and sizes for all three geodata
// assets in one API call; nil when the API is unreachable (callers degrade).
func geodataReleaseMetadata(ctx context.Context, downloads *download.Downloader) map[string]geodataMeta {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, geodataReleasesAPI, nil)
	if err != nil {
		return nil
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := downloads.Client().Do(req)
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
	downloads := m.downloader()
	sources, err := downloads.MirrorCandidates(geodataFiles[name])
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

	downloads.Message(false, "downloading %s", name)
	var lastErr error
	for _, src := range sources {
		w := &resettableWriter{f: f, h: sha256.New()}
		progress := downloads.Progress()
		n, err := downloads.Fetch(ctx, src, w, progress, download.RequestOptions{})

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
