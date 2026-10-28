# Optional systemd integration tests on a developer OrbStack VM.
# The VM is disposable: only the project directory and ~/pc-e2e are mounted.
VM ?= proxyctl-e2e
VM_BIN ?= $(LINUX_BIN)
VM_BIN_PATH = $(abspath $(VM_BIN))
VM_PATH ?= $$HOME/.local/bin
# Keep the VM architecture aligned with the selected Linux build.
VM_ARCH ?= $(GOARCH)
# Already extracted Linux binaries; keep sing-box's companion library beside it.
# Relative Linux paths are resolved from the VM working directory ($(CURDIR)).
MIHOMO_BIN ?=
SING_BOX_BIN ?=
VM_CREATE = orb create --arch $(VM_ARCH) debian $(VM) && \
	orb -m $(VM) sudo -n apt-get install -y -qq python3 curl iproute2 >/dev/null

.PHONY: vm vm-ensure vm-build vm-rebuild e2e e2e-status e2e-systemd e2e-reset e2e-shell

e2e e2e-shell e2e-reset: e2e-systemd ## isolated lifecycle and shell E2E; existing state is preserved

e2e-systemd: vm-build vm-ensure ## real user services, two kernels and a local HTTP fixture
	@test -n "$(MIHOMO_BIN)" -a -n "$(SING_BOX_BIN)" || { echo 'Set MIHOMO_BIN and SING_BOX_BIN to extracted Linux executables' >&2; exit 2; }
	orb -m $(VM) bash "$(CURDIR)/scripts/e2e-systemd.sh" "$(VM_BIN_PATH)" "$(MIHOMO_BIN)" "$(SING_BOX_BIN)"

vm: vm-build vm-ensure ## interactive VM shell; "m" builds the checkout with the VM's Go
	@echo ">> in the VM: 'm' builds $(VM_BIN_PATH) (first run downloads Go)"
	orb -m $(VM) bash -c 'cd "$(CURDIR)" && export PATH="$(VM_PATH):$$PATH" && exec bash --rcfile <([[ ! -r "$$HOME/.bashrc" ]] || cat "$$HOME/.bashrc"; printf "\nm() { bash %q %q; }\n" "$(CURDIR)/scripts/vm-build.sh" "$(VM_BIN_PATH)") -i'

vm-ensure: ## ensure the OrbStack test machine exists
	@orb list | grep -q "^$(VM) " || { $(VM_CREATE); }
	@orb -m $(VM) bash -c 'mkdir -p "$(VM_PATH)" && ln -sf "$(VM_BIN_PATH)" "$(VM_PATH)/proxyctl"'

# Cross-compile on the host and use that binary in the VM (also refreshes an open make vm).
vm-build: build-linux vm-ensure
	orb -m $(VM) bash -c 'mkdir -p "$(dir $(VM_BIN_PATH))" && { cmp -s "$(LINUX_BIN)" "$(VM_BIN_PATH)" || { cp "$(LINUX_BIN)" "$(VM_BIN_PATH)"; echo ">> VM binary updated"; }; }'

vm-rebuild: vm-ensure ## build with the VM's Go
	orb -m $(VM) bash "$(CURDIR)/scripts/vm-build.sh" "$(VM_BIN_PATH)"

e2e-status: vm-build vm-ensure ## read-only overview using the VM's normal configuration
	orb -m $(VM) "$(VM_BIN_PATH)" status
	orb -m $(VM) "$(VM_BIN_PATH)" list
	orb -m $(VM) "$(VM_BIN_PATH)" sub list
