package ui

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/nelvko/proxyctl/internal/config"
	"github.com/nelvko/proxyctl/internal/redact"
	"go.yaml.in/yaml/v3"
)

// controllerClient talks to the Clash-compatible API of the selected kernel.
// It is optional: manually managed configurations do not necessarily enable it.
type controllerClient struct {
	base   string
	secret string
	client *http.Client
}

var dashboardTransport = func() *http.Transport {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil // Controller requests must not use the shell's proxy.
	return transport
}()

var dashboardHTTPClient = &http.Client{Transport: dashboardTransport, Timeout: 4 * time.Second}

func controllerFromConfig(cfg *config.KernelConfig) (*controllerClient, error) {
	if cfg == nil {
		return nil, nil
	}
	raw, err := os.ReadFile(cfg.ConfigFile)
	if err != nil {
		return nil, fmt.Errorf("read active kernel config: %w", err)
	}
	var address, secret string
	kind := cfg.Type
	if kind == "" {
		kind = cfg.Name
	}
	switch kind {
	case "mihomo":
		var doc struct {
			Address string `yaml:"external-controller"`
			Secret  string `yaml:"secret"`
		}
		if err := yaml.Unmarshal(raw, &doc); err != nil {
			return nil, fmt.Errorf("read controller settings: %w", err)
		}
		address, secret = doc.Address, doc.Secret
	case "sing-box":
		var doc struct {
			Experimental struct {
				ClashAPI struct {
					Address string `json:"external_controller"`
					Secret  string `json:"secret"`
				} `json:"clash_api"`
			} `json:"experimental"`
		}
		if err := json.Unmarshal(raw, &doc); err != nil {
			return nil, fmt.Errorf("read controller settings: %w", err)
		}
		address, secret = doc.Experimental.ClashAPI.Address, doc.Experimental.ClashAPI.Secret
	default:
		return nil, nil
	}
	if address == "" {
		return nil, nil
	}
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return nil, fmt.Errorf("invalid controller address: %w", err)
	}
	if host == "" || host == "0.0.0.0" || host == "::" {
		host = "127.0.0.1"
	}
	return &controllerClient{
		base:   "http://" + net.JoinHostPort(host, port),
		secret: secret,
		client: dashboardHTTPClient,
	}, nil
}

func (c *controllerClient) request(ctx context.Context, method, path string, body io.Reader) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, method, c.base+path, body)
	if err != nil {
		return nil, err
	}
	if c.secret != "" {
		req.Header.Set("Authorization", "Bearer "+c.secret)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		resp.Body.Close()
		return nil, fmt.Errorf("controller returned HTTP %d", resp.StatusCode)
	}
	return resp, nil
}

func (c *controllerClient) get(ctx context.Context, path string, result any) error {
	resp, err := c.request(ctx, http.MethodGet, path, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return json.NewDecoder(io.LimitReader(resp.Body, 8<<20)).Decode(result)
}

type dashboardProxy struct {
	Name    string   `json:"name"`
	Type    string   `json:"type"`
	Now     string   `json:"now"`
	All     []string `json:"all"`
	History []struct {
		Delay int `json:"delay"`
	} `json:"history"`
}

func (c *controllerClient) proxies(ctx context.Context) ([]dashboardProxy, map[string]dashboardProxy, error) {
	var response struct {
		Proxies map[string]dashboardProxy `json:"proxies"`
	}
	if err := c.get(ctx, "/proxies", &response); err != nil {
		return nil, nil, err
	}
	groups := make([]dashboardProxy, 0)
	for _, proxy := range response.Proxies {
		if len(proxy.All) > 0 && proxy.Name != "GLOBAL" {
			groups = append(groups, proxy)
		}
	}
	sort.Slice(groups, func(i, j int) bool { return groups[i].Name < groups[j].Name })
	return groups, response.Proxies, nil
}

func (c *controllerClient) selectProxy(ctx context.Context, group, name string) error {
	payload, _ := json.Marshal(struct {
		Name string `json:"name"`
	}{name})
	resp, err := c.request(ctx, http.MethodPut, "/proxies/"+url.PathEscape(group), bytes.NewReader(payload))
	if err != nil {
		return err
	}
	return resp.Body.Close()
}

type dashboardConnection struct {
	ID       string   `json:"id"`
	Upload   int64    `json:"upload"`
	Download int64    `json:"download"`
	Rule     string   `json:"rule"`
	Chains   []string `json:"chains"`
	Metadata struct {
		Host            string `json:"host"`
		DestinationIP   string `json:"destinationIP"`
		DestinationPort string `json:"destinationPort"`
		Process         string `json:"process"`
	} `json:"metadata"`
}

type dashboardConnections struct {
	DownloadTotal int64                 `json:"downloadTotal"`
	UploadTotal   int64                 `json:"uploadTotal"`
	Connections   []dashboardConnection `json:"connections"`
	// Memory is the core's reported memory statistic, not process RSS.
	Memory *int64 `json:"memory"`

	// Optional metrics must distinguish a known zero from an unsupported or
	// omitted field. Both supported cores may encode an empty list as null.
	Available          bool `json:"-"`
	DownloadTotalKnown bool `json:"-"`
	UploadTotalKnown   bool `json:"-"`
}

func (result *dashboardConnections) UnmarshalJSON(data []byte) error {
	var raw struct {
		DownloadTotal *int64          `json:"downloadTotal"`
		UploadTotal   *int64          `json:"uploadTotal"`
		Memory        *int64          `json:"memory"`
		Connections   json.RawMessage `json:"connections"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	next := dashboardConnections{}
	if raw.Connections != nil {
		if err := json.Unmarshal(raw.Connections, &next.Connections); err != nil {
			return err
		}
		next.Available = true
	}
	if raw.DownloadTotal != nil && *raw.DownloadTotal >= 0 {
		next.DownloadTotal, next.DownloadTotalKnown = *raw.DownloadTotal, true
	}
	if raw.UploadTotal != nil && *raw.UploadTotal >= 0 {
		next.UploadTotal, next.UploadTotalKnown = *raw.UploadTotal, true
	}
	if raw.Memory != nil && *raw.Memory >= 0 {
		next.Memory = raw.Memory
	}
	*result = next
	return nil
}

func (c *controllerClient) connections(ctx context.Context) (dashboardConnections, error) {
	var result dashboardConnections
	err := c.get(ctx, "/connections", &result)
	return result, err
}

type dashboardRule struct {
	Type    string `json:"type"`
	Payload string `json:"payload"`
	Proxy   string `json:"proxy"`
}

func (c *controllerClient) rules(ctx context.Context) ([]dashboardRule, error) {
	var response struct {
		Rules []dashboardRule `json:"rules"`
	}
	if err := c.get(ctx, "/rules", &response); err != nil {
		return nil, err
	}
	return response.Rules, nil
}

type dashboardTraffic struct {
	Up, Down int64 // Bytes per second, as reported by the controller.
}

// streamTraffic reads one rate sample per line from the Clash-compatible API.
// Unlike regular API requests, this HTTP response stays open until canceled.
func (c *controllerClient) streamTraffic(ctx context.Context, out chan<- dashboardTraffic) error {
	streamClient := *c
	streamClient.client = &http.Client{Transport: c.client.Transport}
	resp, err := streamClient.request(ctx, http.MethodGet, "/traffic", nil)
	if err != nil {
		if ctx.Err() != nil {
			return nil
		}
		return err
	}
	defer resp.Body.Close()
	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 4096), 64<<10)
	for scanner.Scan() {
		var event struct {
			Up   *int64 `json:"up"`
			Down *int64 `json:"down"`
		}
		if json.Unmarshal(scanner.Bytes(), &event) != nil || event.Up == nil || event.Down == nil || *event.Up < 0 || *event.Down < 0 {
			continue
		}
		select {
		case out <- dashboardTraffic{Up: *event.Up, Down: *event.Down}:
		case <-ctx.Done():
			return nil
		}
	}
	if err := scanner.Err(); err != nil && !errors.Is(err, context.Canceled) && ctx.Err() == nil {
		return err
	}
	return nil
}

// streamLogs holds one HTTP stream while the log page is visible.
func (c *controllerClient) streamLogs(ctx context.Context, out chan<- string) error {
	streamClient := *c
	streamClient.client = &http.Client{Transport: c.client.Transport}
	resp, err := streamClient.request(ctx, http.MethodGet, "/logs", nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 4096), 1<<20)
	for scanner.Scan() {
		var entry struct {
			Type    string `json:"type"`
			Payload string `json:"payload"`
		}
		if json.Unmarshal(scanner.Bytes(), &entry) != nil {
			continue
		}
		line := time.Now().Format("15:04:05") + " " + strings.ToUpper(entry.Type) + " " + redact.Text(entry.Payload)
		select {
		case out <- line:
		case <-ctx.Done():
			return nil
		}
	}
	if err := scanner.Err(); err != nil && !errors.Is(err, context.Canceled) && ctx.Err() == nil {
		return err
	}
	return nil
}
