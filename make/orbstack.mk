# Optional systemd integration tests on the developer's OrbStack VM.
VM ?= proxyctl-test
VM_BIN ?= $(HOME)/pc-e2e/proxyctl
FIXTURE := $(CURDIR)/test/e2e-minimal.yaml

.PHONY: vm vm-build e2e e2e-reset e2e-shell shell

e2e-shell: vm-build vm ## shell-integration E2E in the VM (bash)
	orb -m $(VM) bash -ic '\
	  eval "$$(proxyctl init bash)" && \
	  type proxyctl | head -1 && \
	  proxyctl on && \
	  test $$(env | grep -c "127.0.0.1:7890") -ge 4 && \
	  curl -sx http://127.0.0.1:7890 -o /dev/null -w "proxy: %{http_code}\n" -m 15 http://www.baidu.com && \
	  proxyctl off && \
	  test $$(env | grep -c "HTTP_PROXY") -eq 0 && \
	  echo SHELL_E2E_GREEN'

shell: vm-build vm ## interactive shell in the OrbStack VM (proxyctl on PATH)
	@echo ">> try: proxyctl list | proxyctl sub (TUI) | proxyctl on"
	orb -m $(VM)

vm: ## ensure the OrbStack test machine exists
	@orb list | grep -q "$(VM)" || orb create debian $(VM)

e2e: vm-build vm ## read-only run on the OrbStack VM
	orb -m $(VM) bash -c '$(VM_BIN) status; $(VM_BIN) list; $(VM_BIN) sub list'

e2e-reset: vm-build vm ## wipe VM state, fresh install + subscription + proxy check
	orb -m $(VM) bash -c '\
	  $(VM_BIN) uninstall mihomo 2>/dev/null; \
	  rm -rf ~/.config/proxyctl; \
	  $(VM_BIN) install mihomo && \
	  $(VM_BIN) sub add file://$(FIXTURE) -n e2e --use && \
	  $(VM_BIN) start && \
	  systemctl --user is-active mihomo.service && \
	  sleep 2 && \
	  curl -sx http://127.0.0.1:7890 -o /dev/null -w "proxy check: %{http_code}\n" -m 15 http://www.baidu.com'
	@echo "note: HTTPS through the minimal DIRECT-only fixture is flaky mihomo behavior; real subscriptions carry full dns/proxies config."

vm-build: build-linux
	mkdir -p $(dir $(VM_BIN))
	cp $(LINUX_BIN) $(VM_BIN)
