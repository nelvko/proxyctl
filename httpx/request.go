package httpx

import (
	"context"
	"io"
	"net/http"
	URL "net/url"
	"runtime"
	"strings"
	"time"
)

const (
	GH_PROXY = "GH_PROXY"
)

func Request(ctx context.Context, rawURL, method string, header map[string][]string, body io.Reader) (*http.Response, error) {
	method = strings.ToUpper(method)
	urlRes, err := URL.Parse(rawURL)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest(method, urlRes.String(), body)
	if err != nil {
		return nil, err
	}

	for k, v := range header {
		for _, v := range v {
			req.Header.Add(k, v)
		}
	}

	if user := urlRes.User; user != nil {
		password, _ := user.Password()
		req.SetBasicAuth(user.Username(), password)
	}

	req = req.WithContext(ctx)

	transport := &http.Transport{
		DisableKeepAlives:     runtime.GOOS == "android",
		MaxIdleConns:          100,
		IdleConnTimeout:       30 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}

	client := http.Client{Transport: transport}
	return client.Do(req)
}
