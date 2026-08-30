package httpx

import "testing"

func TestGhProxy(t *testing.T) {
	const target = "https://github.com/MetaCubeX/mihomo/releases/latest/download/mihomo-darwin-arm64-v1.20.0.gz"

	t.Run("no proxy env returns original", func(t *testing.T) {
		t.Setenv("GITHUB_PROXY", "")
		if got := GhProxy(target); got != target {
			t.Fatalf("GhProxy() = %q, want %q", got, target)
		}
	})

	t.Run("proxy env prefixes the target", func(t *testing.T) {
		t.Setenv("GITHUB_PROXY", "https://mirror.example.com/gh")
		got := GhProxy(target)
		want := "https://mirror.example.com/gh/" + target
		if got != want {
			t.Fatalf("GhProxy() = %q, want %q", got, want)
		}
	})

	t.Run("invalid proxy env returns original", func(t *testing.T) {
		t.Setenv("GITHUB_PROXY", "://not-a-url")
		if got := GhProxy(target); got != target {
			t.Fatalf("GhProxy() = %q, want %q", got, target)
		}
	})
}
