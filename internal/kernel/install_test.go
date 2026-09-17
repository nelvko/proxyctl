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

	"github.com/nelvko/proxyctl/internal/download"
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

	if err := fetchArtifact(context.Background(), srv.URL, art, f, download.New(nil, nil, nil)); err != nil {
		t.Fatalf("fetchArtifact() error = %v", err)
	}
	if got := readTmp(t, f); string(got) != string(payload) {
		t.Fatalf("content = %q, want the payload", got)
	}

	art.SHA256 = strings.Repeat("0", 64)
	err = fetchArtifact(context.Background(), srv.URL, art, f, download.New(nil, nil, nil))
	if err == nil || !strings.Contains(err.Error(), "integrity") {
		t.Fatalf("err = %v, want an integrity failure", err)
	}

	art.SHA256 = hex.EncodeToString(sum[:])
	art.Size = int64(len(payload)) + 1
	err = fetchArtifact(context.Background(), srv.URL, art, f, download.New(nil, nil, nil))
	if err == nil || !strings.Contains(err.Error(), "size mismatch") {
		t.Fatalf("err = %v, want a size mismatch", err)
	}

	// No digest known: any bytes are accepted (version.txt fallback path).
	art.SHA256, art.Size = "", 0
	if err := fetchArtifact(context.Background(), srv.URL, art, f, download.New(nil, nil, nil)); err != nil {
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
