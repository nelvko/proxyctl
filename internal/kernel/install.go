package kernel

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"hash"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/nelvko/proxyctl/internal/download"
	"github.com/nelvko/proxyctl/internal/redact"
	"github.com/nelvko/unisvc"
)

// downloadTimeout bounds the whole fetch generously: the stall watchdog in
// download.Downloader.Fetch, not wall-clock time, detects stalled transfers.
const downloadTimeout = 30 * time.Minute

// Download fetches a built-in kernel release and extracts its binary to bin,
// replacing it atomically. Mirrors must reproduce the release digest when
// one is available. It returns ErrUpToDate for an installed latest release.
func Download(ctx context.Context, k Kernel, bin string, downloads *download.Downloader) error {
	if runtime.GOOS == "windows" {
		return errors.New("windows service management is not supported yet")
	}

	downloads.Message(false, "resolving latest release…")
	art, err := k.LatestArtifact(ctx, downloads)
	if err != nil {
		return err
	}
	// An API answer without a digest is unexpected: refuse mirror bytes we
	// cannot verify (the version.txt fallback is the only legitimate
	// digest-less path, and it is TLS-anchored direct github.com anyway).
	if art.FromAPI && art.SHA256 == "" {
		if len(downloads.Mirrors()) > 0 && os.Getenv("PROXYCTL_NO_VERIFY") == "" {
			return fmt.Errorf("release %s carries no digest; refusing mirror downloads that cannot be verified (set PROXYCTL_NO_VERIFY=1 to override)", art.Version)
		}
		downloads.Message(true, "proxyctl: release %s carries no digest; continuing with TLS-only verification\n", art.Version)
	}
	if cur, err := k.InstalledVersion(ctx); err == nil && cur == art.Version {
		return ErrUpToDate
	}
	sources, err := downloads.MirrorCandidates(art.URL)
	if err != nil {
		return err
	}
	if art.SHA256 == "" && os.Getenv("PROXYCTL_NO_VERIFY") == "" && len(downloads.Mirrors()) > 0 {
		// The version.txt fallback has no digest, so mirror bytes cannot
		// be checked against the release. Use the direct source only.
		sources = []string{art.URL}
		downloads.Message(true, "proxyctl: skipping mirrors for unverified release\n")
	}
	if err := os.MkdirAll(filepath.Dir(bin), 0o755); err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(ctx, downloadTimeout)
	defer cancel()

	// Stage next to the destination: /tmp is often a small tmpfs, and the
	// repo's own pattern (subscription profiles) already stages temp files
	// beside their destination.
	f, err := os.CreateTemp(filepath.Dir(bin), ".kernel-archive-*")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tmpName := f.Name()
	defer func() {
		f.Close()
		os.Remove(tmpName)
	}()

	var errs []error
	for _, src := range sources {
		if err := fetchArtifact(ctx, src, art, f, downloads); err != nil {
			errs = append(errs, err)
			continue
		}
		if err := extractExtraFiles(f, bin, art); err != nil {
			return fmt.Errorf("extract kernel companions: %w", err)
		}
		if err := extractArtifact(f, bin, art); err != nil {
			return fmt.Errorf("extract kernel: %w", err)
		}
		return os.Chmod(bin, 0o755)
	}
	display := make([]string, len(sources))
	for i, src := range sources {
		display[i] = redact.URL(src)
	}
	err = fmt.Errorf("download failed, tried %s: %w", strings.Join(display, " "), errors.Join(errs...))
	if len(sources) == 1 {
		// Direct github.com alone was tried and failed — the standard remedy
		// on a blocked route is a mirror.
		return fmt.Errorf("%w\ntry a GitHub mirror: --mirror https://ghfast.top or GH_PROXY=https://ghfast.top", err)
	}
	return err
}

// fetchArtifact downloads src into f and, when the artifact carries a
// digest, verifies the bytes before they are trusted.
func fetchArtifact(ctx context.Context, src string, art *Artifact, f *os.File, downloads *download.Downloader) error {
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return err
	}
	if err := f.Truncate(0); err != nil {
		return err
	}
	downloads.Message(false, "downloading %s", redact.URL(src))

	w := &resettableWriter{f: f, h: sha256.New()}
	progress := downloads.Progress()
	n, err := downloads.Fetch(ctx, src, w, progress, download.RequestOptions{})

	if err != nil {
		return err
	}
	if art.SHA256 == "" {
		return nil // unverifiable source (version.txt fallback)
	}
	if art.Size > 0 && n != art.Size {
		return fmt.Errorf("size mismatch for %s: got %d bytes, want %d", redact.URL(src), n, art.Size)
	}
	if got := hex.EncodeToString(w.h.Sum(nil)); got != art.SHA256 {
		return fmt.Errorf("integrity check failed for %s: sha256 %s, want %s", redact.URL(src), got, art.SHA256)
	}
	return nil
}

// resettableWriter tees the archive into a digest accumulator and can be
// rewound for a clean retry of a single source.
type resettableWriter struct {
	f *os.File
	h hash.Hash
}

func (w *resettableWriter) Write(p []byte) (int, error) {
	n, err := w.f.Write(p)
	if n > 0 {
		_, _ = w.h.Write(p[:n])
	}
	return n, err
}

// Reset rewinds the file and the digest for a clean retry.
func (w *resettableWriter) Reset() error {
	w.h.Reset()
	if _, err := w.f.Seek(0, io.SeekStart); err != nil {
		return err
	}
	return w.f.Truncate(0)
}

// InstallService installs the service, replacing a previously installed
// unit so re-running install converges to the new spec instead of failing.
// A service that was running before is restarted afterwards.
func InstallService(k Kernel, spec *unisvc.Spec) error {
	wasRunning := false
	on, err := k.IsActive()
	if err != nil {
		return err
	}
	wasRunning = on

	err = k.Install(spec)
	if errors.Is(err, unisvc.ErrAlreadyInstalled) {
		if err := k.Uninstall(); err != nil {
			return fmt.Errorf("reinstall service: %w", err)
		}
		err = k.Install(spec)
	}
	if err != nil {
		return err
	}
	if wasRunning {
		return k.Start()
	}
	return nil
}
