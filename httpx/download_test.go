package httpx

import (
	"os"
	"testing"
)

func TestDownload(t *testing.T) {
	f, _ := os.Create("./mihomo.gz")
	Download(nil, "https://gh-proxy.com/https://github.com/MetaCubeX/mihomo/releases/download/v1.19.19/mihomo-darwin-amd64-compatible-v1.19.19.gz", f)
}
