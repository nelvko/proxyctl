package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/nelvko/proxyctl/internal/config"
)

func runThemeCommand(input string, args ...string) (string, string, error) {
	root := NewRootCmd()
	var out, stderr bytes.Buffer
	root.SetIn(strings.NewReader(input))
	root.SetOut(&out)
	root.SetErr(&stderr)
	root.SetArgs(append([]string{"theme"}, args...))
	err := root.Execute()
	return out.String(), stderr.String(), err
}

func TestThemeImportFromStdinPreservesCoreAndExportStdout(t *testing.T) {
	isolatedConfig(t)
	core := &config.Config{Use: "mihomo", Mirror: "https://private-mirror.example", Language: "en", Kernels: []config.KernelConfig{{Name: "mihomo", Bin: "/private/kernel", ConfigFile: "/private/kernel.yaml"}}}
	if err := config.Save(core); err != nil {
		t.Fatal(err)
	}
	input := "theme:\n  name: custom\n  mode: dark\n  base: nord\n  colors:\n    primary: '#abcdef'\n"
	out, stderr, err := runThemeCommand(input, "import", "-")
	if err != nil || out != "" || !strings.Contains(stderr, "Theme imported") {
		t.Fatalf("stdin import: out=%q errout=%q err=%v", out, stderr, err)
	}
	want, err := config.ReadTheme(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	got, err := config.Load()
	if err != nil || !reflect.DeepEqual(got.Theme, want) || got.Use != core.Use || got.Mirror != core.Mirror || !reflect.DeepEqual(got.Kernels, core.Kernels) {
		t.Fatalf("import changed core settings: %+v, %v", got, err)
	}
	for _, args := range [][]string{{"export"}, {"export", "-"}} {
		out, stderr, err := runThemeCommand("", args...)
		if err != nil || stderr != "" || !strings.HasPrefix(out, "theme:\n") || strings.Contains(out, "private") || strings.Contains(out, "kernels") {
			t.Fatalf("stdout export leaked state or diagnostics: %q %q %v", out, stderr, err)
		}
		exported, err := config.ReadTheme(strings.NewReader(out))
		if err != nil || !reflect.DeepEqual(exported, want) {
			t.Fatalf("export mismatch: %+v, %v", exported, err)
		}
	}
}

func TestThemeImportRejectsInvalidWithoutWriting(t *testing.T) {
	for _, input := range []string{
		"theme: []\n", "theme:\n  name: typo\n", "theme:\n  mode: typo\n",
		"theme:\n  name: custom\n  base: typo\n", "theme:\n  name: custom\n  colors:\n    primary: '#123'\n",
		"theme:\n  name: custom\n  colors:\n    unknown: '#123456'\n", "theme:\n  name: nord\n  colors:\n    primary: invalid\n",
	} {
		t.Run(input, func(t *testing.T) {
			dir := isolatedConfig(t)
			if err := config.Save(&config.Config{Mirror: "unchanged", Theme: config.ThemeConfig{Name: "dracula"}}); err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(dir, "config.yaml")
			before, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			out, stderr, err := runThemeCommand(input, "import", "-")
			if err == nil || out != "" || stderr != "" {
				t.Fatalf("invalid import accepted or printed success: %q %q %v", out, stderr, err)
			}
			after, err := os.ReadFile(path)
			if err != nil || !bytes.Equal(before, after) {
				t.Fatalf("invalid import modified config: %s, %v", after, err)
			}
		})
	}
}

func TestThemeExportFileNoClobberForceAndFileImport(t *testing.T) {
	isolatedConfig(t)
	theme := config.ThemeConfig{Name: "nord", Mode: "light", Base: "default", Colors: map[string]string{"primary": "#123456"}}
	if err := config.SaveTheme(t.Context(), theme); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "appearance.yaml")
	if out, stderr, err := runThemeCommand("", "export", path); err != nil || out != "" || stderr != "" {
		t.Fatalf("file export: %q %q %v", out, stderr, err)
	}
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := runThemeCommand("", "export", path); err == nil || !strings.Contains(err.Error(), "--force") {
		t.Fatalf("existing export was overwritten: %v", err)
	}
	after, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("rejected export changed existing file")
	}
	if err := os.Chmod(path, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, err := runThemeCommand("", "export", path, "--force"); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0o600 {
		t.Fatalf("export permissions = %o", info.Mode().Perm())
	}
	if err := config.SaveTheme(t.Context(), config.ThemeConfig{}); err != nil {
		t.Fatal(err)
	}
	if _, _, err := runThemeCommand("", "import", path); err != nil {
		t.Fatal(err)
	}
	got, err := config.LoadTheme()
	if err != nil || !reflect.DeepEqual(got, theme) {
		t.Fatalf("file roundtrip lost inactive custom settings: %+v, %v", got, err)
	}
}

func TestThemeCommandRejectsArgumentsBeforeReadingState(t *testing.T) {
	dir := isolatedConfig(t)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "config.yaml"), []byte("broken: ["), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"import"}, {"import", "one", "two"}, {"export", "one", "two"}, {"unknown"}} {
		if _, _, err := runThemeCommand("", args...); err == nil || strings.Contains(err.Error(), "parsing") {
			t.Fatalf("%v did not validate arguments first: %v", args, err)
		}
	}
}

func TestThemeImportFeedbackUsesLanguageWithoutChangingYAML(t *testing.T) {
	for _, test := range []struct {
		language, expected string
	}{
		{"zh-CN", "主题已导入"},
		{"en", "Theme imported"},
	} {
		t.Run(test.language, func(t *testing.T) {
			isolatedConfig(t)
			t.Setenv("LC_ALL", "en_US.UTF-8")
			if err := config.Save(&config.Config{Language: test.language}); err != nil {
				t.Fatal(err)
			}
			out, stderr, err := runThemeCommand("theme: {name: nord}\n", "import", "-")
			if err != nil || out != "" || !strings.Contains(stderr, test.expected) {
				t.Fatalf("localized feedback: %q %q %v", out, stderr, err)
			}
			out, stderr, err = runThemeCommand("", "export", "-")
			if err != nil || stderr != "" || !strings.HasPrefix(out, "theme:\n") || strings.Contains(out, test.expected) || strings.Contains(out, "language:") {
				t.Fatalf("language changed YAML output: %q %q %v", out, stderr, err)
			}
		})
	}
}
