package app

import (
	"context"
	"net/http"

	"github.com/nelvko/proxyctl/internal/download"
	"github.com/nelvko/proxyctl/internal/kernel"
)

// Transport policy belongs to the application. Kernel download helpers receive
// the chosen downloader, so release metadata and assets use the same client.
func (a *App) downloadKernel(ctx context.Context, k kernel.Kernel, bin string) error {
	endpoint := ""
	if running, err := k.IsActive(); err == nil && running {
		if endpoints, err := k.ProxyEndpoints(); err == nil {
			endpoint = endpoints.Preferred()
		}
	}
	client := download.AutoClient(endpoint)
	if client != http.DefaultClient {
		defer client.CloseIdleConnections()
	}
	return kernel.Download(ctx, k, bin, a.downloader().WithClient(client))
}
