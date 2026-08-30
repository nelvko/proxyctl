VM      := proxyctl-test
BIN     := $(HOME)/pc-e2e/proxyctl
FIXTURE := $(CURDIR)/test/e2e-minimal.yaml

.PHONY: vm build-linux vet test smoke verify e2e e2e-reset e2e-shell shell

e2e-shell: build-linux vm ## shell-integration E2E in the VM (bash)
	orb -m $(VM) bash -ic '\
	  eval "$$(proxyctl init bash)" && \
	  type proxyctl | head -1 && \
	  proxyctl on && \
	  test $$(env | grep -c "127.0.0.1:7890") -ge 4 && \
	  curl -sx http://127.0.0.1:7890 -o /dev/null -w "proxy: %{http_code}\n" -m 15 http://www.baidu.com && \
	  proxyctl off && \
	  test $$(env | grep -c "HTTP_PROXY") -eq 0 && \
	  echo SHELL_E2E_GREEN'

shell: build-linux vm ## interactive shell in the OrbStack VM (proxyctl on PATH)
	@echo ">> try: proxyctl kernel list | proxyctl sub (TUI) | proxyctl on"
	orb -m $(VM)

vm: ## ensure the OrbStack test machine exists
	@orb list | grep -q "$(VM)" || orb create debian $(VM)

build-linux: ## cross-compile the linux binary and verify it is ELF
	GOOS=linux go build -o $(BIN) .
	@head -c 4 $(BIN) | od -An -tx1 | tr -d ' \n' | grep -q "^7f454c46" || { echo "NOT ELF (GOOS=linux missing?)"; exit 1; }
	@echo "linux binary: $(BIN)"

vet: ## vet both platforms
	go vet ./... && GOOS=linux go vet ./...

test: ## unit tests
	go test ./...

smoke: build-linux ## command-surface smoke in an alpine container
	docker run --rm -v $(BIN):/p:ro alpine /p kernel list

verify: vet test smoke ## everything that needs no systemd

e2e: build-linux vm ## read-only run on the OrbStack VM
	orb -m $(VM) bash -c '$(BIN) status; $(BIN) kernel list; $(BIN) sub list'

e2e-reset: build-linux vm ## wipe VM state, fresh install + subscription + proxy check
	orb -m $(VM) bash -c '\
	  $(BIN) kernel uninstall mihomo 2>/dev/null; \
	  rm -rf ~/.config/proxyctl; \
	  $(BIN) kernel install mihomo && \
	  $(BIN) sub add file://$(FIXTURE) -n e2e && \
	  systemctl --user is-active mihomo.service && \
	  sleep 2 && \
	  curl -sx http://127.0.0.1:7890 -o /dev/null -w "proxy check: %{http_code}\n" -m 15 http://www.baidu.com'
	@echo "note: HTTPS through the minimal DIRECT-only fixture is flaky mihomo behavior; real subscriptions carry full dns/proxies config."
