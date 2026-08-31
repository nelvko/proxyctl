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

	"golang.org/x/term"

	"github.com/nelvko/proxyctl/internal/config"
	"github.com/nelvko/proxyctl/internal/httpx"
	"github.com/nelvko/proxyctl/internal/ui"
	"github.com/nelvko/unisvc"
)

// downloadTimeout bounds the whole fetch generously: the stall watchdog in
// httpx.Download, not wall-clock time, is what catches dead connections.
const downloadTimeout = 30 * time.Minute

// DefaultConfig returns the default on-disk layout for a kernel:
// binary under ~/.local/share/proxyctl/kernels/<name>/,
// config under ~/.config/proxyctl/<name>/.
func DefaultConfig(name string) config.KernelConfig {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	return config.KernelConfig{
		Name:       name,
		Bin:        filepath.Join(home, ".local", "share", config.AppName, "kernels", name, name),
		ConfigDir:  filepath.Join(home, ".config", config.AppName, name),
		ConfigFile: filepath.Join(home, ".config", config.AppName, name, "config.yaml"),
	}
}

// Download fetches the latest kernel release and extracts the binary to bin,
// replacing it atomically. Sources are tried mirror-first and each one must
// reproduce the artifact's digest when it is known — a mirror supplies
// bytes, never identity. When the managed kernel is running, downloads ride
// its own inbound. It returns ErrUpToDate — without downloading — when the
// installed binary already is the latest release.
func Download(ctx context.Context, k Kernel, bin string) error {
	if runtime.GOOS == "windows" {
		// The asset is a .zip this path cannot extract and the service layer
		// is systemd-only; fail before the multi-MB fetch.
		return errors.New("windows is not supported yet (zip extraction and service management are missing)")
	}
	useLocalKernelIfRunning(k)

	ui.Err("resolving latest release…")
	art, err := k.LatestArtifact()
	if err != nil {
		return err
	}
	// Skip the multi-MB fetch when the binary on disk already is the latest.
	if cur, err := k.InstalledVersion(); err == nil && cur == art.Version {
		return ErrUpToDate
	}
	sources, err := httpx.MirrorCandidates(art.URL)
	if err != nil {
		return err
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
		if err := fetchArtifact(ctx, src, art, f); err != nil {
			errs = append(errs, err)
			continue
		}
		if err := httpx.Ungzip(f, bin); err != nil {
			return fmt.Errorf("extract kernel: %w", err)
		}
		return os.Chmod(bin, 0o755)
	}
	err = fmt.Errorf("download failed, tried %s: %w", strings.Join(sources, " "), errors.Join(errs...))
	if len(sources) == 1 {
		// Direct github.com alone was tried and failed — the standard remedy
		// on a blocked route is a mirror.
		return fmt.Errorf("%w\ntry a GitHub mirror: --mirror https://ghfast.top or GH_PROXY=https://ghfast.top", err)
	}
	return err
}

// useLocalKernelIfRunning routes downloads through the running kernel's own
// inbound — a fresh SSH session or cron has no proxy env even though the
// kernel it manages is up. Kernels exposing no inbound, a stopped kernel or
// an explicit shell proxy leave the transport untouched.
func useLocalKernelIfRunning(k Kernel) {
	inbound, ok := k.(interface{ InboundAddr() string })
	if !ok {
		return
	}
	on, err := k.IsActive()
	if err != nil || !on {
		return
	}
	httpx.UseLocalKernel(inbound.InboundAddr())
}

// fetchArtifact downloads src into f and, when the artifact carries a
// digest, verifies the bytes before they are trusted.
func fetchArtifact(ctx context.Context, src string, art *Artifact, f *os.File) error {
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return err
	}
	if err := f.Truncate(0); err != nil {
		return err
	}
	ui.Err(fmt.Sprintf("downloading %s", src))

	w := &resettableWriter{f: f, h: sha256.New()}
	progress := progressPrinter()
	n, err := httpx.Download(ctx, src, w, progress)
	if progress != nil {
		fmt.Fprintln(os.Stderr)
	}
	if err != nil {
		return err
	}
	if art.SHA256 == "" {
		return nil // unverifiable source (version.txt fallback)
	}
	if art.Size > 0 && n != art.Size {
		return fmt.Errorf("size mismatch for %s: got %d bytes, want %d", src, n, art.Size)
	}
	if got := hex.EncodeToString(w.h.Sum(nil)); got != art.SHA256 {
		return fmt.Errorf("integrity check failed for %s: sha256 %s, want %s", src, got, art.SHA256)
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

// progressPrinter returns an httpx progress callback that rewrites a single
// stderr line, throttled to displayed-value changes; nil when stderr is not
// a terminal, keeping pipes and captured output quiet.
func progressPrinter() func(total, n int64) {
	if !term.IsTerminal(int(os.Stderr.Fd())) {
		return nil
	}
	const mib = 1 << 20
	last := int64(-1)
	return func(total, n int64) {
		if n*10/mib == last {
			return
		}
		last = n * 10 / mib
		if total > 0 {
			fmt.Fprintf(os.Stderr, "\r%.1f / %.1f MiB", float64(n)/mib, float64(total)/mib)
		} else {
			fmt.Fprintf(os.Stderr, "\r%.1f MiB", float64(n)/mib)
		}
	}
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
