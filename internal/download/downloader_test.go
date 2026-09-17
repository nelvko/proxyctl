package download

import (
	"net/http"
	"testing"
)

func TestDownloaderIsolation(t *testing.T) {
	t.Setenv("GH_PROXY", "")
	for _, key := range []string{"HTTP_PROXY", "http_proxy", "HTTPS_PROXY", "https_proxy"} {
		t.Setenv(key, "")
	}
	mirrors := []string{"https://one.example"}
	one := New(mirrors, AutoClient("127.0.0.1:7890"), nil)
	mirrors[0] = "https://changed.example"
	two := New([]string{"https://two.example"}, AutoClient(""), nil)
	if two.Client() != http.DefaultClient || one.Client() == two.Client() {
		t.Fatal("transport leaked across operations")
	}
	snapshot := one.Mirrors()
	snapshot[0] = "https://changed.example"
	if one.Mirrors()[0] != "https://one.example" || two.Mirrors()[0] != "https://two.example" {
		t.Fatal("mirror state leaked")
	}
	t.Setenv("HTTP_PROXY", "http://127.0.0.1:8888")
	if AutoClient("127.0.0.1:7890") != http.DefaultClient {
		t.Fatal("stale kernel transport overrides environment")
	}
}

func TestMirrorWarningsReachEachOperation(t *testing.T) {
	t.Setenv("GH_PROXY", "")
	for range 2 {
		warnings := 0
		d := New([]string{"http://mirror.example"}, nil, func(e Event) {
			if e.Warning {
				warnings++
			}
		})
		for range 2 {
			if _, err := d.MirrorCandidates("https://github.com/example/asset"); err != nil {
				t.Fatal(err)
			}
		}
		if warnings != 1 {
			t.Fatalf("want one warning per operation, got %d", warnings)
		}
	}
}
