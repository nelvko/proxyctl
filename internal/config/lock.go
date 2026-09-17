package config

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Lock serializes application transactions across CLI and TUI processes.
// Never unlink the lock file: replacing its inode would permit two owners.
func Lock(ctx context.Context) (func(), error) {
	d, err := dir()
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(d, 0o700); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(filepath.Join(d, "state.lock"), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, err
	}
	for {
		if err := ctx.Err(); err != nil {
			f.Close()
			return nil, err
		}
		acquired, err := tryLock(f)
		if err != nil {
			f.Close()
			return nil, fmt.Errorf("lock application state: %w", err)
		}
		if acquired {
			return func() { _ = f.Close() }, nil
		}
		select {
		case <-ctx.Done():
			f.Close()
			return nil, ctx.Err()
		case <-time.After(50 * time.Millisecond):
		}
	}
}
