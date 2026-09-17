package download

import (
	"net/http"
	URL "net/url"
	"os"
	"strings"
)

// AutoClient chooses a transport afresh; an absent inbound cannot retain an
// earlier operation's proxy. Explicit environment proxies take precedence.
func AutoClient(addr string) *http.Client {
	for _, key := range []string{"HTTP_PROXY", "http_proxy", "HTTPS_PROXY", "https_proxy"} {
		if os.Getenv(key) != "" {
			return http.DefaultClient
		}
	}
	if client := ProxyClient(addr); client != nil {
		return client
	}
	return http.DefaultClient
}

// ProxyClient explicitly routes requests through an HTTP or SOCKS endpoint.
// Unlike automatic kernel discovery, explicit routing overrides shell proxies.
func ProxyClient(addr string) *http.Client {
	if addr == "" {
		return nil
	}
	if !strings.Contains(addr, "://") {
		addr = "http://" + addr
	}
	u, err := URL.Parse(addr)
	if err != nil || u.Host == "" {
		return nil
	}
	switch u.Scheme {
	case "http", "https", "socks5", "socks5h":
	default:
		return nil
	}
	t := http.DefaultTransport.(*http.Transport).Clone()
	t.Proxy = http.ProxyURL(u)
	return &http.Client{Transport: t}
}
