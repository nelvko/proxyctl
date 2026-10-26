# Optional systemd integration tests on the developer's OrbStack VM.
VM ?= proxyctl-test
VM_BIN ?= $(HOME)/pc-e2e/proxyctl
# Already extracted Linux binaries; keep sing-box's companion library beside it.
MIHOMO_BIN ?=
SING_BOX_BIN ?=

.PHONY: vm vm-build e2e e2e-status e2e-systemd e2e-reset e2e-shell shell

e2e e2e-shell e2e-reset: e2e-systemd ## isolated lifecycle and shell E2E; existing state is preserved

e2e-systemd: vm-build vm ## real user services, two kernels and a local HTTP fixture
	@test -n "$(MIHOMO_BIN)" -a -n "$(SING_BOX_BIN)" || { echo 'Set MIHOMO_BIN and SING_BOX_BIN to extracted Linux executables' >&2; exit 2; }
	orb -m $(VM) bash "$(CURDIR)/scripts/e2e-systemd.sh" "$(VM_BIN)" "$(MIHOMO_BIN)" "$(SING_BOX_BIN)"

shell: vm-build vm ## interactive shell in the OrbStack VM (proxyctl on PATH)
	@echo ">> try: proxyctl list | proxyctl sub (TUI) | proxyctl on"
	orb -m $(VM)

vm: ## ensure the OrbStack test machine exists
	@orb list | grep -q "$(VM)" || orb create debian $(VM)

e2e-status: vm-build vm ## read-only overview using the VM's normal configuration
	orb -m $(VM) bash -c '"$(VM_BIN)" status; "$(VM_BIN)" list; "$(VM_BIN)" sub list'

vm-build: build-linux
	mkdir -p "$(dir $(VM_BIN))"
	cp "$(LINUX_BIN)" "$(VM_BIN)"
