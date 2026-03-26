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

// TestConfig tests the configuration file for Mihomo by executing the Mihomo binary.
// It captures the output and returns an error if the test fails.
func (m Mihomo) TestConfig(configFile string) error {
	cfg := config.Get()
	cmd := exec.Command(
		cfg.Kernel.Bin,
		"-t",
		"-f", configFile,
		"-d", cfg.Kernel.ConfigDir,
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("test config: \n%s", out)
	}
	return nil
}

// Upgrade core
func (m Mihomo) Upgrade() error {
	return nil
}

const (
	mihomoDownloadBaseURL = "https://github.com/MetaCubeX/mihomo/releases/latest/download/"
	mihomoVersionURL      = mihomoDownloadBaseURL + "version.txt"
)

// LatestVersion fetches the latest version of Mihomo from the Github.
func latestVersion() (string, error) {
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
		return "", fmt.Errorf("fetch version: %w", err)
	}

	return content, nil
}
// DownloadURL constructs the download URL for the Mihomo binary based on the latest version
// and the current system architecture. 
// It returns the download URL as a string or an error if it fails to fetch the latest version.
func (m Mihomo) DownloadURL() (string, error) {
	latestVersion, err := latestVersion()
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
