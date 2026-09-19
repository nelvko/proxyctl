package kernel

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/nelvko/proxyctl/internal/download"
)

// githubAsset is one downloadable file of a GitHub release, as the API
// describes it.
type githubAsset struct {
	Name   string
	URL    string
	Size   int64
	SHA256 string
}

// githubRelease is a release resolved through the GitHub API. The API is the
// trust anchor for digests: a mirror supplies bytes, never identity.
type githubRelease struct {
	Tag    string
	Assets []githubAsset
}

// Asset returns the named asset, or an error naming the release.
func (r *githubRelease) Asset(name string) (*githubAsset, error) {
	for i := range r.Assets {
		if r.Assets[i].Name == name {
			return &r.Assets[i], nil
		}
	}
	return nil, fmt.Errorf("release %s has no %q asset", r.Tag, name)
}

// AssetByBase returns the asset whose file name matches the base of url —
// the name a geodata download URL already carries.
func (r *githubRelease) AssetByBase(url string) *githubAsset {
	base := url[strings.LastIndex(url, "/")+1:]
	for i := range r.Assets {
		if r.Assets[i].Name == base {
			return &r.Assets[i]
		}
	}
	return nil
}

// releaseBudget bounds one metadata request; the endpoint is small.
const releaseBudget = 30 * time.Second

// fetchGithubRelease resolves a release through the GitHub API. The request
// is direct (HTTP(S)_PROXY still applies, so `proxyctl on` routes it through
// the local kernel): ghproxy-style mirrors only front github.com paths, and
// the API is the trust anchor for the digest.
func fetchGithubRelease(ctx context.Context, apiURL string, downloads *download.Downloader) (*githubRelease, error) {
	ctx, cancel := context.WithTimeout(ctx, releaseBudget)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("fetch release metadata: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := downloads.Client().Do(req)
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

	var body struct {
		TagName string `json:"tag_name"`
		Assets  []struct {
			Name               string `json:"name"`
			BrowserDownloadURL string `json:"browser_download_url"`
			Size               int64  `json:"size"`
			Digest             string `json:"digest"`
		} `json:"assets"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, fmt.Errorf("decode release metadata: %w", err)
	}

	release := &githubRelease{Tag: body.TagName, Assets: make([]githubAsset, 0, len(body.Assets))}
	for _, a := range body.Assets {
		release.Assets = append(release.Assets, githubAsset{
			Name: a.Name,
			URL:  a.BrowserDownloadURL,
			Size: a.Size,
			// The API reports digests as "sha256:<hex>"; an unrecognized
			// algorithm is not a digest we can verify against, so it is
			// dropped rather than compared as hex.
			SHA256: strings.TrimPrefix(a.Digest, "sha256:"),
		})
	}
	return release, nil
}
