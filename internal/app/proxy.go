package app

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"time"

	"github.com/nelvko/proxyctl/internal/kernel"
	"github.com/nelvko/proxyctl/internal/redact"
)

// StartProxy starts the selected service and waits for a usable proxy endpoint.
// The caller owns the state lock; shell formatting belongs to the command.
func (a *App) StartProxy(ctx context.Context) (kernel.Endpoints, error) {
	if err := ctx.Err(); err != nil {
		return kernel.Endpoints{}, err
	}
	k, err := a.Kernel()
	if err != nil {
		return kernel.Endpoints{}, err
	}
	if err := kernel.EnsureDefaultConfig(k); err != nil {
		return kernel.Endpoints{}, err
	}
	endpoints, err := k.ProxyEndpoints()
	if err != nil {
		return kernel.Endpoints{}, err
	}
	if err := controlKernel(ctx, k, "start"); err != nil {
		return kernel.Endpoints{}, err
	}
	if err := waitReachable(ctx, endpoints); err != nil {
		return kernel.Endpoints{}, err
	}
	return endpoints, nil
}

// waitReachable retries until at least one resolved proxy endpoint accepts
// TCP connections.
func waitReachable(ctx context.Context, endpoints kernel.Endpoints) error {
	for _, addr := range []string{endpoints.HTTP, endpoints.SOCKS} {
		if addr == "" {
			continue
		}
		for range 10 {
			u, err := url.Parse(addr)
			if err != nil {
				return err
			}
			conn, err := (&net.Dialer{Timeout: 500 * time.Millisecond}).DialContext(ctx, "tcp", u.Host)
			if err == nil {
				conn.Close()
				return nil
			}
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(300 * time.Millisecond):
			}
		}
	}
	return fmt.Errorf("proxy endpoint not reachable: %s", redact.URL(endpoints.Preferred()))
}
