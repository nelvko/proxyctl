package kernel

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// InstallLocal copies an already extracted executable into proxyctl's managed
// directory. The source remains untouched. sing-box's optional companion
// library is copied when it is beside the downloaded executable.
func InstallLocal(ctx context.Context, source, bin string, singBox bool) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	info, err := os.Stat(source)
	if err != nil {
		return fmt.Errorf("local binary: %w", err)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("local binary is not a regular file: %s", source)
	}
	if err := os.MkdirAll(filepath.Dir(bin), 0o755); err != nil {
		return err
	}
	if singBox {
		companion := filepath.Join(filepath.Dir(source), "libcronet.so")
		if info, err := os.Stat(companion); err == nil {
			if !info.Mode().IsRegular() {
				return fmt.Errorf("companion is not a regular file: %s", companion)
			}
			if err := copyLocalFile(ctx, companion, filepath.Join(filepath.Dir(bin), "libcronet.so"), 0o644); err != nil {
				return err
			}
		} else if !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	return copyLocalFile(ctx, source, bin, 0o755)
}

func copyLocalFile(ctx context.Context, source, destination string, mode os.FileMode) error {
	src, err := os.Open(source)
	if err != nil {
		return err
	}
	defer src.Close()
	return writeStagedFile(destination, &cancelReader{ctx: ctx, r: src}, mode)
}

type cancelReader struct {
	ctx context.Context
	r   io.Reader
}

func (r *cancelReader) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.r.Read(p)
}
