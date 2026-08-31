package kernel

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/nelvko/proxyctl/internal/config"
)

func TestGeodataNeeded(t *testing.T) {
	cases := []struct {
		name, config string
		want         []string
	}{
		{"geosite rule", "rules:\n  - GEOSITE,cn,DIRECT\n", []string{"GeoSite.dat"}},
		{"geoip rule", "rules:\n  - GEOIP,CN,DIRECT\n", []string{"geoip.metadb", "GeoIP.dat"}},
		{"both", "rules:\n  - GEOSITE,cn,DIRECT\n  - GEOIP,CN,DIRECT\n", []string{"GeoSite.dat", "geoip.metadb", "GeoIP.dat"}},
		{"none", "rules:\n  - MATCH,DIRECT\n", nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := geodataNeeded([]byte(tc.config))
			if len(got) != len(tc.want) {
				t.Fatalf("geodataNeeded() = %v, want %v", got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Fatalf("geodataNeeded()[%d] = %q, want %q", i, got[i], tc.want[i])
				}
			}
		})
	}
}

func TestEnsureGeodataSeedsMissingFiles(t *testing.T) {
	t.Setenv("GH_PROXY", "")

	dir := t.TempDir()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("dat-bytes-for:" + filepath.Base(r.URL.Path)))
	}))
	defer srv.Close()
	orig := geodataFiles
	geodataFiles = map[string]string{
		"GeoSite.dat":  srv.URL + "/geosite.dat",
		"GeoIP.dat":    srv.URL + "/geoip.dat",
		"geoip.metadb": srv.URL + "/geoip.metadb",
	}
	t.Cleanup(func() { geodataFiles = orig })

	cfgFile := filepath.Join(dir, "profile.yaml")
	if err := os.WriteFile(cfgFile, []byte("rules:\n  - GEOIP,CN,DIRECT\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// An existing file must not be re-fetched.
	existing := filepath.Join(dir, "geoip.metadb")
	if err := os.WriteFile(existing, []byte("already-here"), 0o644); err != nil {
		t.Fatal(err)
	}

	m := Mihomo{cfg: config.KernelConfig{ConfigDir: dir}}
	m.ensureGeodata(cfgFile)

	if got, _ := os.ReadFile(existing); string(got) != "already-here" {
		t.Fatalf("existing geoip.metadb was overwritten: %q", got)
	}
	got, err := os.ReadFile(filepath.Join(dir, "GeoIP.dat"))
	if err != nil {
		t.Fatal("GeoIP.dat was not seeded:", err)
	}
	if string(got) != "dat-bytes-for:geoip.dat" {
		t.Fatalf("GeoIP.dat content = %q", got)
	}
}
