package config

import "testing"

func TestKernelSetRemoveActive(t *testing.T) {
	cfg := &Config{}

	if cfg.SelectedKernel() != nil {
		t.Fatal("SelectedKernel() != nil on empty config")
	}

	mihomo := KernelConfig{Name: "mihomo"}
	clash := KernelConfig{Name: "clash"}

	cfg.SetKernel(mihomo)
	cfg.SetKernel(clash)
	if len(cfg.Kernels) != 2 {
		t.Fatalf("Kernels = %d entries, want 2", len(cfg.Kernels))
	}

	// SetKernel replaces, does not duplicate.
	cfg.SetKernel(KernelConfig{Name: "mihomo", Bin: "/new/bin"})
	if len(cfg.Kernels) != 2 {
		t.Fatalf("SetKernel duplicated entry: %d", len(cfg.Kernels))
	}
	for _, k := range cfg.Kernels {
		if k.Name == "mihomo" && k.Bin != "/new/bin" {
			t.Fatalf("SetKernel did not replace: %+v", k)
		}
	}

	// SelectedKernel resolves by Use.
	cfg.Use = "clash"
	if got := cfg.SelectedKernel(); got == nil || got.Name != "clash" {
		t.Fatalf("SelectedKernel() = %+v, want clash", got)
	}

	// Dangling Use (kernel removed) resolves to nil.
	cfg.RemoveKernel("clash")
	if cfg.SelectedKernel() != nil {
		t.Fatalf("SelectedKernel() = %+v after removal, want nil", cfg.SelectedKernel())
	}
	if len(cfg.Kernels) != 1 {
		t.Fatalf("RemoveKernel left %d entries", len(cfg.Kernels))
	}

	// Removing an unknown kernel is a no-op.
	cfg.RemoveKernel("sing-box")
	if len(cfg.Kernels) != 1 {
		t.Fatalf("RemoveKernel of unknown kernel changed the set")
	}
}
