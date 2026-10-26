#!/usr/bin/env bash
# Exercise real user services using isolated state and uniquely named kernels.
# Supply already extracted Linux binaries; the test itself does not download.
set -euo pipefail

if [[ $# != 3 ]]; then
    echo "usage: $0 <proxyctl> <mihomo executable> <sing-box executable>" >&2
    exit 2
fi
for dependency in systemctl python3 curl ss readlink; do
    command -v "$dependency" >/dev/null || { echo "missing dependency: $dependency" >&2; exit 2; }
done
[[ $(uname -s) == Linux ]] || { echo "Linux with user systemd is required" >&2; exit 2; }
for binary in "$@"; do
    [[ -x $binary ]] || { echo "executable not found: $binary" >&2; exit 2; }
done
systemctl --user show-environment >/dev/null
if [[ -n $(ss -H -ltn 'sport = :7890') ]]; then
    echo "port 7890 is already in use; existing services were left untouched" >&2
    exit 2
fi

cli=$(readlink -f "$1")
mihomo_binary=$(readlink -f "$2")
singbox_binary=$(readlink -f "$3")
fixtures=$(cd "$(dirname "$0")/../test" && pwd)
scratch=$(mktemp -d "${TMPDIR:-/tmp}/proxyctl-systemd.XXXXXX")
token=$(printf '%s' "${scratch##*.}" | tr '[:upper:]' '[:lower:]')
mihomo="pce2e-mihomo-$token"
singbox="pce2e-singbox-$token"
runtime_units="${XDG_RUNTIME_DIR:?XDG_RUNTIME_DIR is required}/systemd/user"
server_pid=

cleanup() {
    local result=$?
    trap - EXIT
    set +e
    if [[ $result != 0 ]]; then
        systemctl --user status "$mihomo.service" "$singbox.service" --no-pager >&2
        journalctl --user -u "$mihomo.service" -u "$singbox.service" -n 30 --no-pager >&2
    fi
    systemctl --user stop "$mihomo.service" "$singbox.service" >/dev/null 2>&1
    for name in "$mihomo" "$singbox"; do
        local link="$runtime_units/$name.service"
        if [[ -L $link && $(readlink "$link") == "$scratch/config/systemd/user/$name.service" ]]; then
            rm -- "$link"
        fi
    done
    rm -f -- "$runtime_units/$singbox.service.d/proxyctl-e2e.conf"
    rmdir -- "$runtime_units/$singbox.service.d" >/dev/null 2>&1
    systemctl --user daemon-reload >/dev/null 2>&1
    systemctl --user reset-failed "$mihomo.service" "$singbox.service" >/dev/null 2>&1
    if [[ -n $server_pid ]]; then
        kill "$server_pid" >/dev/null 2>&1
        wait "$server_pid" >/dev/null 2>&1
    fi
    rm -rf -- "$scratch"
    exit "$result"
}
trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM

# Scope the test home/config to CLI processes; keep the real user's runtime bus.
sandbox_home="$scratch/home"
sandbox_config="$scratch/config"
mkdir -p "$sandbox_home" "$sandbox_config" "$scratch/bin"
printf '#!/usr/bin/env bash\nexec env HOME=%q XDG_CONFIG_HOME=%q %q "$@"\n' \
    "$sandbox_home" "$sandbox_config" "$cli" > "$scratch/bin/proxyctl"
chmod +x "$scratch/bin/proxyctl"
export PATH="$scratch/bin:$PATH"
unset HTTP_PROXY http_proxy HTTPS_PROXY https_proxy ALL_PROXY all_proxy NO_PROXY no_proxy

python3 - "$scratch/http-port" <<'PY' &
import http.server
import pathlib
import sys

class Handler(http.server.BaseHTTPRequestHandler):
    def do_GET(self):
        body = b"proxyctl local fixture\n"
        self.send_response(200)
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)

    def log_message(self, *args):
        pass

server = http.server.HTTPServer(("127.0.0.1", 0), Handler)
pathlib.Path(sys.argv[1]).write_text(str(server.server_port))
server.serve_forever()
PY
server_pid=$!
for ((attempt = 0; attempt < 50; attempt++)); do
    [[ -s $scratch/http-port ]] && break
    sleep 0.1
done
[[ -s $scratch/http-port ]] || { echo "local HTTP fixture did not start" >&2; exit 1; }
origin="http://127.0.0.1:$(cat "$scratch/http-port")/probe"

pid_of() {
    systemctl --user show "$1.service" --property=MainPID --value
}

assert_active() {
    systemctl --user is-active --quiet "$1.service"
    [[ $(pid_of "$1") -gt 0 ]]
}

assert_stopped() {
    [[ $(systemctl --user show "$1.service" --property=ActiveState --value) == inactive ]]
}

assert_shell_clear() {
    [[ -z ${HTTP_PROXY+x}${http_proxy+x}${HTTPS_PROXY+x}${https_proxy+x}${ALL_PROXY+x}${all_proxy+x}${NO_PROXY+x}${no_proxy+x} ]]
}

assert_proxy() {
    local body
    for ((attempt = 0; attempt < 30; attempt++)); do
        if body=$(curl --silent --show-error --fail --max-time 1 --noproxy '' --proxy http://127.0.0.1:7890 "$origin" 2>/dev/null) && [[ $body == 'proxyctl local fixture' ]]; then
            return
        fi
        sleep 0.1
    done
    echo "local HTTP fixture was not reachable through the proxy" >&2
    return 1
}

assert_status() {
    command proxyctl status --json > "$scratch/status.json"
    python3 - "$scratch/status.json" "$@" <<'PY'
import json
import os
import sys

path, kernel, service, profile, total, default_config = sys.argv[1:]
with open(path) as source:
    status = json.load(source)
assert set(status) == {"kernel", "service", "proxy", "shell", "profile"}, status
assert status["kernel"] == {"state": "selected", "name": kernel}, status
assert status["service"] == {"state": service, "scope": "user", "error": None}, status
assert status["proxy"]["state"] == "available", status
assert status["proxy"]["http"] == "http://127.0.0.1:7890", status
assert status["proxy"]["error"] is None, status
assert status["proxy"]["default_config"] is (default_config == "true"), status
assert status["profile"] == {
    "state": "none" if profile == "-" else "selected",
    "name": None if profile == "-" else profile,
    "total": int(total),
    "error": None,
}, status
variables = ["HTTP_PROXY", "http_proxy", "HTTPS_PROXY", "https_proxy", "ALL_PROXY", "all_proxy", "NO_PROXY", "no_proxy"]
assert status["shell"]["proxy_variables"] == [key for key in variables if os.environ.get(key)], status
PY
}

echo "Install isolated kernels ($mihomo, $singbox)"
command proxyctl install "$mihomo" --bin "$mihomo_binary" --type mihomo
command proxyctl install "$singbox" --bin "$singbox_binary" --type sing-box
# The user manager keeps its normal search path, so expose only these units.
systemctl --user link --runtime "$sandbox_config/systemd/user/$mihomo.service" "$sandbox_config/systemd/user/$singbox.service"
systemctl --user daemon-reload
assert_status "$mihomo" stopped - 0 true

eval "$(command proxyctl init bash)"
proxyctl start
assert_active "$mihomo"
first_pid=$(pid_of "$mihomo")
assert_proxy
proxyctl on
[[ $HTTP_PROXY == http://127.0.0.1:7890 && $http_proxy == "$HTTP_PROXY" ]]
assert_status "$mihomo" running - 0 true
proxyctl off
assert_shell_clear
assert_active "$mihomo"
[[ $(pid_of "$mihomo") == "$first_pid" ]]
proxyctl on
proxyctl off --stop
assert_shell_clear
assert_stopped "$mihomo"
assert_status "$mihomo" stopped - 0 true
echo "PASS no-subscription start/on, off keeps service, off --stop stops service"

# Older releases created zero-byte placeholders. Repair them on first use.
mihomo_config="$sandbox_config/proxyctl/$mihomo/config.yaml"
singbox_config="$sandbox_config/proxyctl/$singbox/config.json"
: > "$mihomo_config"
proxyctl on
assert_status "$mihomo" running - 0 true
before_restart=$(pid_of "$mihomo")
proxyctl restart
assert_active "$mihomo"
after_restart=$(pid_of "$mihomo")
[[ $before_restart != "$after_restart" ]]
assert_proxy
echo "PASS restart changed MainPID ($before_restart -> $after_restart)"

cp -- "$mihomo_config" "$scratch/default.yaml"
proxyctl sub add "file://$fixtures/e2e-minimal.yaml" --name clash-profile
cmp -- "$mihomo_config" "$scratch/default.yaml"
[[ $(pid_of "$mihomo") == "$after_restart" ]]
assert_status "$mihomo" running - 1 true
echo "PASS adding a subscription keeps the default configuration and running service"

# Invalid nonempty user files must be preserved and rejected before stopping.
cp -- "$singbox_config" "$scratch/default.json"
printf '{invalid\n' > "$singbox_config"
if proxyctl use "$singbox" > "$scratch/invalid-config.log" 2>&1; then
    echo "an invalid target configuration was accepted" >&2
    exit 1
fi
[[ $(cat "$singbox_config") == '{invalid' ]]
assert_active "$mihomo"
[[ $(pid_of "$mihomo") == "$after_restart" ]]
assert_status "$mihomo" running - 1 true
cp -- "$scratch/default.json" "$singbox_config"

# Let validation pass, but make systemd skip the target to exercise rollback.
mkdir -p "$runtime_units/$singbox.service.d"
printf '[Unit]\nConditionPathExists=%s\n' "$scratch/allow-target" \
    > "$runtime_units/$singbox.service.d/proxyctl-e2e.conf"
systemctl --user daemon-reload
if proxyctl use "$singbox" > "$scratch/failed-start.log" 2>&1; then
    echo "a skipped target service was accepted as active" >&2
    exit 1
fi
assert_active "$mihomo"
assert_stopped "$singbox"
assert_status "$mihomo" running - 1 true
assert_proxy
rm -- "$runtime_units/$singbox.service.d/proxyctl-e2e.conf"
rmdir -- "$runtime_units/$singbox.service.d"
systemctl --user daemon-reload
echo "PASS a failed target start restores the previous service and selection"

: > "$singbox_config"
proxyctl use "$singbox"
assert_stopped "$mihomo"
assert_active "$singbox"
proxyctl on
assert_proxy
assert_status "$singbox" running - 1 true
proxyctl off --stop
assert_shell_clear
assert_stopped "$singbox"

# A valid config can run without a shell proxy inbound. on must not replace it.
printf '{}\n' > "$singbox_config"
proxyctl start
assert_active "$singbox"
if command proxyctl on > "$scratch/no-inbound.stdout" 2> "$scratch/no-inbound.stderr"; then
    echo "on accepted a configuration without proxy inbounds" >&2
    exit 1
fi
[[ ! -s $scratch/no-inbound.stdout && $(cat "$singbox_config") == '{}' ]]
assert_shell_clear
proxyctl stop
cp -- "$scratch/default.json" "$singbox_config"
proxyctl on
assert_active "$singbox"
assert_proxy
echo "PASS empty-config repair, invalid-config protection and manual config without inbounds"

proxyctl sub add "file://$fixtures/e2e-minimal.json" --name sing-profile --use
assert_status "$singbox" running sing-profile 2 false
assert_proxy
proxyctl use "$mihomo"
assert_stopped "$singbox"
assert_active "$mihomo"
assert_status "$mihomo" running - 2 true
assert_proxy
proxyctl sub use clash-profile
assert_status "$mihomo" running clash-profile 2 false
assert_proxy
before_wrong_format=$(pid_of "$mihomo")
if proxyctl sub use sing-profile > "$scratch/wrong-format.log" 2>&1; then
    echo "a sing-box profile was accepted by Mihomo" >&2
    exit 1
fi
[[ $(pid_of "$mihomo") == "$before_wrong_format" ]]
assert_status "$mihomo" running clash-profile 2 false
proxyctl use "$singbox"
assert_stopped "$mihomo"
assert_active "$singbox"
assert_status "$singbox" running sing-profile 2 false
assert_proxy
proxyctl use "$mihomo"
assert_stopped "$singbox"
assert_active "$mihomo"
assert_status "$mihomo" running clash-profile 2 false
assert_proxy
echo "PASS format isolation and per-format subscription selection restored"

proxyctl off --stop
assert_shell_clear
assert_stopped "$mihomo"
assert_stopped "$singbox"
assert_status "$mihomo" stopped clash-profile 2 false

# Local upgrades and reinstalls preserve user config and restart running units.
for name in "$mihomo" "$singbox"; do
    proxyctl use "$name"
    proxyctl on
    before_upgrade=$(pid_of "$name")
    config_file="$mihomo_config"
    binary="$mihomo_binary"
    kind=mihomo
    profile=clash-profile
    if [[ $name == "$singbox" ]]; then
        config_file="$singbox_config"
        binary="$singbox_binary"
        kind=sing-box
        profile=sing-profile
    fi
    cp -- "$config_file" "$scratch/preserved-config"
    proxyctl upgrade "$name"
    assert_active "$name"
    [[ $(pid_of "$name") != "$before_upgrade" ]]
    cmp -- "$config_file" "$scratch/preserved-config"
    assert_proxy
    proxyctl install "$name" --bin "$binary" --type "$kind"
    assert_active "$name"
    cmp -- "$config_file" "$scratch/preserved-config"
    assert_status "$name" running "$profile" 2 false
    assert_proxy
    proxyctl off --stop
    assert_shell_clear
done
echo "PASS local upgrade and reinstall preserve configurations and running services"

proxyctl uninstall "$mihomo"
proxyctl uninstall "$singbox"
[[ ! -e $sandbox_config/proxyctl/$mihomo && ! -e $sandbox_config/proxyctl/$singbox ]]
[[ ! -e $sandbox_config/systemd/user/$mihomo.service && ! -e $sandbox_config/systemd/user/$singbox.service ]]
command proxyctl status --json > "$scratch/status.json"
python3 - "$scratch/status.json" <<'PY'
import json
import sys
with open(sys.argv[1]) as source:
    status = json.load(source)
assert status["kernel"]["state"] == "none", status
assert status["service"]["state"] == "unavailable", status
PY
echo "PASS uninstall removes both test services and managed configurations"
echo "SYSTEMD_E2E_GREEN (isolated services, local HTTP fixture, JSON status)"
