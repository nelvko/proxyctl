package config

import "testing"

func TestKernelSetRemoveActive(t *testing.T) {
	cfg := &AppConfig{}

	if cfg.ActiveKernel() != nil {
		t.Fatal("ActiveKernel() != nil on empty config")
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

	// ActiveKernel resolves by Use.
	cfg.Use = "clash"
	if got := cfg.ActiveKernel(); got == nil || got.Name != "clash" {
		t.Fatalf("ActiveKernel() = %+v, want clash", got)
	}

	// Dangling Use (kernel removed) resolves to nil.
	cfg.RemoveKernel("clash")
	if cfg.ActiveKernel() != nil {
		t.Fatalf("ActiveKernel() = %+v after removal, want nil", cfg.ActiveKernel())
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
