package kernel

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/nelvko/proxyctl/internal/config"
)

func TestFetchArtifactVerifiesDigest(t *testing.T) {
	payload := []byte("pretend kernel archive, long enough to exercise the copy")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(payload)
	}))
	defer srv.Close()

	f, err := os.CreateTemp(t.TempDir(), "artifact-*")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	sum := sha256.Sum256(payload)
	art := &Artifact{SHA256: hex.EncodeToString(sum[:]), Size: int64(len(payload))}

	if err := fetchArtifact(context.Background(), srv.URL, art, f); err != nil {
		t.Fatalf("fetchArtifact() error = %v", err)
	}
	if got := readTmp(t, f); string(got) != string(payload) {
		t.Fatalf("content = %q, want the payload", got)
	}

	art.SHA256 = strings.Repeat("0", 64)
	err = fetchArtifact(context.Background(), srv.URL, art, f)
	if err == nil || !strings.Contains(err.Error(), "integrity") {
		t.Fatalf("err = %v, want an integrity failure", err)
	}

	art.SHA256 = hex.EncodeToString(sum[:])
	art.Size = int64(len(payload)) + 1
	err = fetchArtifact(context.Background(), srv.URL, art, f)
	if err == nil || !strings.Contains(err.Error(), "size mismatch") {
		t.Fatalf("err = %v, want a size mismatch", err)
	}

	// No digest known: any bytes are accepted (version.txt fallback path).
	art.SHA256, art.Size = "", 0
	if err := fetchArtifact(context.Background(), srv.URL, art, f); err != nil {
		t.Fatalf("fetchArtifact() without digest error = %v", err)
	}
}

func readTmp(t *testing.T, f *os.File) []byte {
	t.Helper()
	if _, err := f.Seek(0, 0); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(f.Name())
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestInboundAddr(t *testing.T) {
	write := func(content string) Mihomo {
		t.Helper()
		f, err := os.CreateTemp(t.TempDir(), "config-*.yaml")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := f.WriteString(content); err != nil {
			t.Fatal(err)
		}
		f.Close()
		return Mihomo{cfg: config.KernelConfig{ConfigFile: f.Name()}}
	}

	if got := write("mixed-port: 7890\n").InboundAddr(); got != "127.0.0.1:7890" {
		t.Fatalf("InboundAddr() = %q, want 127.0.0.1:7890", got)
	}
	if got := write("port: 1087\n").InboundAddr(); got != "127.0.0.1:1087" {
		t.Fatalf("InboundAddr() = %q, want 127.0.0.1:1087", got)
	}
	// mixed-port wins over port.
	if got := write("port: 1087\nmixed-port: 7890\n").InboundAddr(); got != "127.0.0.1:7890" {
		t.Fatalf("InboundAddr() = %q, want the mixed port", got)
	}
	if got := write("socks-port: 7891\n").InboundAddr(); got != "" {
		t.Fatalf("InboundAddr() = %q, want empty for a socks-only config", got)
	}
	if got := (Mihomo{cfg: config.KernelConfig{ConfigFile: "/nonexistent"}}).InboundAddr(); got != "" {
		t.Fatalf("InboundAddr() = %q, want empty for a missing config", got)
	}
}
