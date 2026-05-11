package httpx

import (
	"context"
	"fmt"
	"io"
	"net/http"
	URL "net/url"
	"os"
	"path"
)

// GhProxy returns the URL of the GitHub proxy if the GITHUB_PROXY environment variable is set,
// otherwise it returns the original URL.
func GhProxy(raw string) string {
	ghProxy := os.Getenv("GITHUB_PROXY")
	if ghProxy == "" {
		return raw
	}
	ghProxyURL, err := URL.Parse(ghProxy)
	if err != nil {
		return raw
	}
	rawURL, err := URL.Parse(raw)
	if err != nil {
		return raw
	}
	ghProxyURL.Path = path.Join(ghProxyURL.Path, rawURL.String())
	return ghProxyURL.String()
}

func Download(ctx context.Context, url string, dst io.Writer) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("unexpected status: %s", resp.Status)
	}

	_, err = io.Copy(dst, resp.Body)
	return err
}
