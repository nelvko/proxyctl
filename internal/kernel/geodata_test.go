package kernel

import (
	"bytes"
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

	// Keep the digest API out of the test: an unreachable local address
	// makes geodataReleaseMetadata degrade to direct-only, no digest.
	origAPI := geodataReleasesAPI
	geodataReleasesAPI = "http://127.0.0.1:1/nope"
	t.Cleanup(func() { geodataReleasesAPI = origAPI })

	// Above geodataMinSize: an error-page-sized payload must be rejected.
	payload := bytes.Repeat([]byte("g"), geodataMinSize+64)

	dir := t.TempDir()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(payload)
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
	if len(got) != len(payload) {
		t.Fatalf("GeoIP.dat size = %d, want %d", len(got), len(payload))
	}
}

// TestEnsureGeodataRejectsErrorPage pins the sticky-pollution guard: a 200
// response that is really an error page must never occupy the target path,
// because the Stat check in ensureGeodata would treat it as seeded forever.
func TestEnsureGeodataRejectsErrorPage(t *testing.T) {
	t.Setenv("GH_PROXY", "")
	origAPI := geodataReleasesAPI
	geodataReleasesAPI = "http://127.0.0.1:1/nope"
	t.Cleanup(func() { geodataReleasesAPI = origAPI })

	dir := t.TempDir()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("<html>mirror error page</html>"))
	}))
	defer srv.Close()
	orig := geodataFiles
	geodataFiles = map[string]string{"GeoSite.dat": srv.URL + "/geosite.dat"}
	t.Cleanup(func() { geodataFiles = orig })

	cfgFile := filepath.Join(dir, "profile.yaml")
	if err := os.WriteFile(cfgFile, []byte("rules:\n  - GEOSITE,cn,DIRECT\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	m := Mihomo{cfg: config.KernelConfig{ConfigDir: dir}}
	m.ensureGeodata(cfgFile)

	if _, err := os.Stat(filepath.Join(dir, "GeoSite.dat")); !os.IsNotExist(err) {
		t.Fatalf("an error-page payload was written to GeoSite.dat (stat err = %v)", err)
	}
}
