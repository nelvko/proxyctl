package kernel

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"runtime"
	"runtime/debug"
	"strings"
	"time"

	"charm.land/huh/v2/spinner"
	"golang.org/x/sys/cpu"

	"github.com/nelvko/proxyctl/config"
	"github.com/nelvko/proxyctl/httpx"
	"github.com/nelvko/unisvc"
)

type Mihomo struct {
	unisvc.Service
}

func (m Mihomo) TestConfig(configFile string) error {
	cfg := config.Get()
	cmd := exec.Command(
		cfg.Kernel.BinPath,
		"-t",
		"-f", configFile,
		"-d", cfg.Kernel.ConfigDir,
	)
	b, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("failed to test config: %w\n%s", err, b)
	}
	return nil
}

func (m Mihomo) LatestVersion() (string, error) {
	return "", nil
}

func (m Mihomo) Upgrade() error {
	return nil
}

const (
	mihomoDownloadBaseURL = "https://github.com/MetaCubeX/mihomo/releases/latest/download/"
	mihomoVersionURL      = mihomoDownloadBaseURL + "version.txt"
)

func MihomoLatestVersion() (string, error) {
	versionURL := httpx.GhProxy(mihomoVersionURL)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var content string
	action := func() {
		resp, _ := httpx.Request(ctx, versionURL, http.MethodGet, nil, nil)
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		content = strings.TrimSpace(string(body))
	}

	err := spinner.New().
		Type(spinner.Dots).
		Title("Fetching mihomo latest version").
		Context(ctx).
		Action(action).
		Run()

	if err != nil {
		return "", fmt.Errorf("failed to fetch version: %w", err)
	}

	return content, nil
}

func MihomoDownloadURL() (string, error) {
	latestVersion, err := MihomoLatestVersion()
	if err != nil {
		return "", err
	}
	filename := mihomoBaseName() + "-" + latestVersion
	if runtime.GOOS == "windows" {
		filename += ".zip"
	} else {
		filename += ".gz"
	}
	return httpx.GhProxy(mihomoDownloadBaseURL + filename), nil
}

func mihomoBaseName() string {
	var (
		GOARM   string
		GOMIPS  string
		GOAMD64 string
	)
	if info, ok := debug.ReadBuildInfo(); ok {
		for _, bs := range info.Settings {
			switch bs.Key {
			case "GOARM":
				GOARM = bs.Value
			case "GOMIPS":
				GOMIPS = bs.Value
			case "GOAMD64":
				GOAMD64 = detectAMD64Level()
			}
		}
	}

	switch runtime.GOARCH {
	case "arm":
		// mihomo-linux-armv5
		return fmt.Sprintf("mihomo-%s-%sv%s", runtime.GOOS, runtime.GOARCH, GOARM)
	case "arm64":
		if runtime.GOOS == "android" {
			// mihomo-android-arm64-v8
			return fmt.Sprintf("mihomo-%s-%s-v8", runtime.GOOS, runtime.GOARCH)
		} else {
			// mihomo-linux-arm64
			return fmt.Sprintf("mihomo-%s-%s", runtime.GOOS, runtime.GOARCH)
		}
	case "mips", "mipsle":
		// mihomo-linux-mips-hardfloat
		return fmt.Sprintf("mihomo-%s-%s-%s", runtime.GOOS, runtime.GOARCH, GOMIPS)
	case "amd64":
		// mihomo-linux-amd64-v1
		return fmt.Sprintf("mihomo-%s-%s-%s", runtime.GOOS, runtime.GOARCH, GOAMD64)
	default:
		// mihomo-linux-386
		// mihomo-linux-mips64
		// mihomo-linux-riscv64
		// mihomo-linux-s390x
		return fmt.Sprintf("mihomo-%s-%s", runtime.GOOS, runtime.GOARCH)
	}
}

func detectAMD64Level() string {
	// v3 必须同时满足
	if cpu.X86.HasAVX &&
		cpu.X86.HasAVX2 &&
		cpu.X86.HasBMI1 &&
		cpu.X86.HasBMI2 &&
		cpu.X86.HasFMA {
		return "v3"
	}

	// v2 必须同时满足
	if cpu.X86.HasSSE3 &&
		cpu.X86.HasSSSE3 &&
		cpu.X86.HasSSE41 &&
		cpu.X86.HasSSE42 &&
		cpu.X86.HasPOPCNT {
		return "v2"
	}

	return "v1"
}
