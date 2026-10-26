package config

import "testing"

func TestKernelLayoutRejectsSharedNamesAndRelativeHome(t *testing.T) {
	for _, name := range []string{"profiles", "diagnostics", "kernels", "overrides", "config.yaml", "subscriptions.yaml", "state.lock", "..", "../mihomo", "mihomo/other"} {
		if _, err := DefaultKernelConfig(name); err == nil {
			t.Fatalf("accepted unsafe name %q", name)
		}
	}
	t.Setenv("HOME", "")
	if _, err := DataDir(); err == nil {
		t.Fatal("missing home fell back to relative path")
	}
	t.Setenv("HOME", "relative")
	if _, err := DataDir(); err == nil {
		t.Fatal("relative home accepted")
	}
}
