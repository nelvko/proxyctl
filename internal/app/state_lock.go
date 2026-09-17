package app

import (
	"context"

	"github.com/nelvko/proxyctl/internal/config"
)

// WithStateLock serializes state mutations across processes. Load state inside run.
// It does not roll back changes; each operation owns its recovery. Never nest locks.
func WithStateLock(ctx context.Context, run func() error) error {
	unlock, err := config.Lock(ctx)
	if err != nil {
		return err
	}
	defer unlock()
	return run()
}
