package kernel

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nelvko/proxyctl/internal/config"
)

func TestSingBoxEndpointsAndServiceSpec(t *testing.T) {
	file := filepath.Join(t.TempDir(), "config.json")
	data := `{"inbounds":[{"type":"mixed","listen":"::","listen_port":7890,"users":[{"username":"user","password":"p@ss"}]}]}`
	if err := os.WriteFile(file, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
	s := SingBox{cfg: config.KernelConfig{Bin: "/managed/sing-box", ConfigDir: filepath.Dir(file), ConfigFile: file}}
	e, err := s.ProxyEndpoints()
	if err != nil {
		t.Fatal(err)
	}
	if e.HTTP != "http://user:p%40ss@[::1]:7890" || e.SOCKS != "socks5h://user:p%40ss@[::1]:7890" {
		t.Fatalf("endpoints = %+v", e)
	}
	spec := s.ServiceSpec()
	if spec.Command != s.cfg.Bin || !strings.Contains(strings.Join(spec.Args, " "), file) {
		t.Fatalf("service spec = %+v", spec)
	}
	if err := os.WriteFile(file, []byte(`{"inbounds":[{"type":"mixed","listen_port":0}]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := s.ProxyEndpoints(); err == nil {
		t.Fatal("accepted port zero")
	}
}

func TestSingBoxReleaseAsset(t *testing.T) {
	arch := singBoxArch()
	name := "sing-box-1.14.2-linux-" + arch + ".tar.gz"
	art, err := singBoxArtifact(&githubRelease{Tag: "v1.14.2", Assets: []githubAsset{{Name: name, URL: "https://github.com/example/release", SHA256: strings.Repeat("a", 64), Size: 123}}})
	if err != nil {
		t.Fatal(err)
	}
	if art.Version != "1.14.2" || art.Archive != "tar.gz" || art.SHA256 == "" || !strings.HasSuffix(art.Entry, "/sing-box") || len(art.Extra) != 1 {
		t.Fatalf("artifact = %+v", art)
	}
}
