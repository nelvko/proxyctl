#!/usr/bin/env python3
"""Isolated Linux PTY smoke checks; optional real cores use simulated service status.

Run in an existing Linux environment, for example:
  orb -m proxyctl-e2e python3 scripts/tui-smoke.py dist/proxyctl-linux-arm64
  ... --mihomo /path/to/mihomo --sing-box /path/to/sing-box

Only child processes receive a temporary XDG_CONFIG_HOME. No service is
installed, started, stopped or restarted. Optional core processes run directly
against temporary configuration and loopback controller ports.
"""

import argparse
import fcntl
import hashlib
import json
import os
import pathlib
import pty
import re
import select
import shlex
import signal
import socket
import struct
import subprocess
import sys
import tempfile
import termios
import time
import urllib.parse
import urllib.request


class Terminal:
    def __init__(self, command, environment, width=120, height=24, background="dark"):
        self.master, slave = pty.openpty()
        fcntl.ioctl(slave, termios.TIOCSWINSZ, struct.pack("HHHH", height, width, 0, 0))
        self.process = subprocess.Popen(command, stdin=slave, stdout=slave, stderr=slave,
                                        env=environment, start_new_session=True)
        os.close(slave)
        self.output = bytearray()
        self.background = background
        self.answered_background = False
        self.drain(1.0)

    def drain(self, duration=0.15):
        deadline = time.monotonic() + duration
        while time.monotonic() < deadline:
            if not select.select([self.master], [], [], min(0.04, max(0, deadline-time.monotonic())))[0]:
                continue
            try:
                data = os.read(self.master, 65536)
            except OSError:
                break
            if not data:
                break
            self.output.extend(data)
            if not self.answered_background and b"\x1b]11;?" in self.output:
                if self.background is not None:
                    color = b"fafa/fafa/fafa" if self.background == "light" else b"1414/1616/2626"
                    os.write(self.master, b"\x1b]11;rgb:" + color + b"\x1b\\")
                self.answered_background = True

    def send(self, data, duration=0.15):
        os.write(self.master, data)
        self.drain(duration)

    def click(self, x, y):
        # Protocol coordinates are one-based; caller coordinates are cells.
        self.send(f"\x1b[<0;{x+1};{y+1}M".encode())
        self.send(f"\x1b[<0;{x+1};{y+1}m".encode())

    def wheel(self, x, y, down=True):
        self.send(f"\x1b[<{65 if down else 64};{x+1};{y+1}M".encode())

    def resize(self, width, height):
        fcntl.ioctl(self.master, termios.TIOCSWINSZ, struct.pack("HHHH", height, width, 0, 0))
        os.kill(self.process.pid, signal.SIGWINCH)
        self.drain(0.25)

    def close(self):
        try:
            if self.process.poll() is None:
                self.send(b"\x03", 0.2)
            self.process.wait(timeout=4)
            assert self.process.returncode == 0, f"TUI exited {self.process.returncode}"
            assert b"panic:" not in self.output, "TUI panic"
        finally:
            if self.process.poll() is None:
                self.process.terminate()
                try:
                    self.process.wait(timeout=3)
                except subprocess.TimeoutExpired:
                    self.process.kill()
                    self.process.wait(timeout=3)
            os.close(self.master)


def write_json(path, value):
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(json.dumps(value), encoding="utf-8")
    path.chmod(0o600)


def fixture(root):
    directory = root / "config" / "proxyctl"
    app_file = directory / "config.yaml"
    profiles_file = directory / "subscriptions.yaml"
    write_json(app_file, {"language": "en", "theme": {"mode": "dark", "name": "default"}})
    entries = []
    for index in range(12):
        source = root / f"source-{index}.yaml"
        source.write_text("proxies: []\nproxy-groups: []\nrules: ['MATCH,DIRECT']\n")
        entries.append({"name": f"Smoke {index:02}", "url": source.as_uri(),
                        "file": str(source), "format": "clash"})
    write_json(profiles_file, {"profiles": entries})
    environment = dict(os.environ, XDG_CONFIG_HOME=str(root / "config"),
                       TERM="xterm-256color", COLORTERM="truecolor")
    environment.pop("NO_COLOR", None)
    return environment, app_file, profiles_file


def hashes(paths):
    return [hashlib.sha256(path.read_bytes()).hexdigest() for path in paths]


def exercise_dashboard(terminal):
    try:
        terminal.send(b"7")
        terminal.send(b"\r")
        # Theme and language previews must be reversible before persistence.
        terminal.send(b"\r")
        terminal.send(b"\x1b[B")
        terminal.send(b"\x1b")
        terminal.send(b"\x1b[B")
        terminal.send(b"\r")
        terminal.send(b"\x1b[B")
        terminal.send(b"\x1b")
        terminal.send(b"\x1b[B")
        terminal.send(b"\x1b[B")
        terminal.send(b"\r")
        start = len(terminal.output)
        terminal.send(b"\x1b[A")
        assert "外观".encode() in terminal.output[start:], "language preview did not render"
        start = len(terminal.output)
        terminal.send(b"\x1b")
        assert b"Appearance" in terminal.output[start:], "language cancel did not restore English"
        for _ in range(10):
            terminal.send(b"\x1b[6~", 0.08)
        assert b"proxyctl sub override edit --global" in terminal.output, "settings tail not reachable"
        terminal.resize(120, 24)
        for _ in range(3):  # Return from Language to Mode, passing Reset without activation.
            terminal.send(b"\x1b[A")
        terminal.send(b"\r")
        terminal.send(b"\x1b[B")
        terminal.resize(42, 10)
        terminal.send(b"\x1b")  # Cancel; do not confirm or save fixture preferences.
        terminal.send(b"q")
    finally:
        terminal.close()


def dashboard_check(binary, root, monochrome):
    environment, app_file, profiles_file = fixture(root)
    if monochrome:
        environment["NO_COLOR"] = "1"
    before = hashes([app_file, profiles_file])
    terminal = Terminal([str(binary)], environment, 80, 12)
    exercise_dashboard(terminal)
    assert hashes([app_file, profiles_file]) == before, "preview changed persisted state"
    assert b"\x1b[?1049h" in terminal.output, "alternate screen not enabled"
    assert b"\x1b[?1003h" in terminal.output, "mouse motion not enabled"
    if monochrome:
        assert b"[38;" not in terminal.output and b"[48;" not in terminal.output, "NO_COLOR ignored"
    return {"mode": "NO_COLOR" if monochrome else "TrueColor", "settings": True,
            "theme_language_preview_cancel": True, "resize": True, "config_unchanged": True}


def persistence_check(binary, root):
    environment, app_file, profiles_file = fixture(root)
    profile_hash = hashes([profiles_file])
    terminal = Terminal([str(binary)], environment, 120, 24)
    try:
        terminal.send(b"7")
        terminal.click(40, 4)  # Scheme trigger in the wide appearance document.
        terminal.click(40, 6)  # Mocha follows the selected Default in the visible viewport.
        assert "name: catppuccin-mocha" in app_file.read_text(), "mouse theme confirmation not saved"
        terminal.send(b"\x1b[B")
        terminal.send(b"\x1b[B")
        terminal.send(b"\r")
        terminal.send(b"\x1b[A")
        terminal.send(b"\r")
        assert "language: zh-CN" in app_file.read_text(), "language confirmation not saved"
        terminal.send(b"q")
    finally:
        terminal.close()
    terminal = Terminal([str(binary)], environment, 120, 24)
    try:
        terminal.send(b"7")
        assert "外观".encode() in terminal.output, "saved language not restored at startup"
        terminal.send(b"q")
    finally:
        terminal.close()
    assert hashes([profiles_file]) == profile_hash, "appearance persistence changed subscription metadata"
    return {"theme_mouse_confirm_save": True, "language_keyboard_confirm_save": True,
            "reload": True, "subscriptions_unchanged": True}


def capability_check(binary, root, term):
    environment, app_file, profiles_file = fixture(root)
    environment["TERM"] = term
    environment.pop("COLORTERM", None)
    before = hashes([app_file, profiles_file])
    terminal = Terminal([str(binary)], environment)
    try:
        terminal.send(b"7")
        terminal.send(b"\r")
        terminal.send(b"\r")
        terminal.send(b"\x1b[B")
        terminal.send(b"\x1b")
        terminal.send(b"q")
    finally:
        terminal.close()
    output = bytes(terminal.output)
    assert b"Appearance" in output, "keyboard-only navigation failed"
    if term == "dumb":
        assert b"[38;" not in output and b"[48;" not in output, "dumb terminal retained colors"
    else:
        assert b";5;" in output, "256-color profile not used"
        assert not re.search(rb"\x1b\[[0-9;:]*(?:38|48)[;:]2[;:]", output), "RGB did not degrade"
    assert hashes([app_file, profiles_file]) == before, "keyboard-only preview changed configuration"
    return {"terminal": term, "color_degradation": True, "keyboard_only": True,
            "config_unchanged": True}


def background_check(binary, root, explicit, reply):
    environment, app_file, profiles_file = fixture(root)
    write_json(app_file, {"language": "en", "theme": {"mode": "auto", "name": "default" if explicit else ""}})
    before = hashes([app_file, profiles_file])
    terminal = Terminal([str(binary)], environment, background=reply)
    try:
        terminal.send(b"7")
        terminal.send(b"q")
    finally:
        terminal.close()
    if reply == "light" and not explicit:
        assert b"48;2;246;245;250" in terminal.output, "light OSC background did not select light palette"
    else:
        assert b"48;2;20;22;38" in terminal.output, "explicit/fallback dark palette missing"
        assert b"48;2;246;245;250" not in terminal.output, "explicit/fallback palette unexpectedly changed"
    assert hashes([app_file, profiles_file]) == before, "background detection saved a preference"
    return {"background_reply": reply, "explicit_scheme": explicit, "auto_policy": True,
            "config_unchanged": True}


def exercise_subscriptions(terminal):
    try:
        assert b"\x1b[?1003h" in terminal.output, "subscription mouse motion not enabled"
        terminal.click(8, 7)  # Native DefaultDelegate's second item (two lines + spacing).
        start = len(terminal.output)
        terminal.click(25, 2)  # Details toolbar button.
        assert b"Smoke 01" in terminal.output[start:], "mouse row/details mapping failed"
        terminal.send(b"\x1b")
        terminal.send(b"/")
        terminal.send(b"Smoke 10")
        terminal.send(b"\r")
        start = len(terminal.output)
        terminal.click(25, 2)
        assert b"Smoke 10" in terminal.output[start:], "filtered selection details failed"
        terminal.wheel(60, 12)
        terminal.resize(42, 10)
        terminal.resize(120, 24)
        terminal.send(b"\x1b")
        start = len(terminal.output)
        terminal.click(58, 2)  # Remove opens confirmation; Enter defaults to Cancel.
        assert b"Remove profile" in terminal.output[start:], "remove confirmation not shown"
        terminal.send(b"\r")
        terminal.click(67, 2)  # Help modal; outside click closes without changing selection.
        terminal.click(0, 0)
        terminal.send(b"\x1b")  # Clear the applied list filter.
        start = len(terminal.output)
        terminal.click(2, 2)  # Open Add and exercise native Huh validation via mouse Next.
        assert b"Source URL" in terminal.output[start:], "add form did not open"
        terminal.click(13, 2)
        terminal.click(21, 2)  # Cancel the draft, preserving all fixture metadata.
        terminal.resize(42, 10)
        terminal.send(b"\x1b[6~")
        terminal.resize(120, 24)
        terminal.wheel(8, 6)
        terminal.send(b"q")
    finally:
        terminal.close()


def subscription_check(binary, root, monochrome):
    environment, app_file, profiles_file = fixture(root)
    if monochrome:
        environment["NO_COLOR"] = "1"
    before = hashes([app_file, profiles_file])
    terminal = Terminal([str(binary), "sub"], environment)
    exercise_subscriptions(terminal)
    assert hashes([app_file, profiles_file]) == before, "cancelled subscription interaction changed metadata"
    if monochrome:
        assert b"[38;" not in terminal.output and b"[48;" not in terminal.output, "subscription NO_COLOR ignored"
    return {"subscription_mode": "NO_COLOR" if monochrome else "TrueColor", "mouse_rows": True,
            "filtered_details_modal": True, "modal_resize_scroll": True, "remove_default_cancel": True,
            "form_mouse_cancel": True, "config_unchanged": True}


def ssh_check(binary, host):
    """Drive the remote terminal over ssh -tt, including resize propagation."""
    options = ["ssh", "-o", "BatchMode=yes", "-o", "ConnectTimeout=5", "-o",
               "StrictHostKeyChecking=yes", "-o", "UpdateHostKeys=no"]

    def remote_python(code, *arguments, data=None):
        remote = shlex.join(["python3", "-c", code, *map(str, arguments)])
        return subprocess.run([*options, host, remote], input=data, stdout=subprocess.PIPE,
                              stderr=subprocess.PIPE, check=True).stdout.decode().strip()

    source = pathlib.Path(__file__).read_bytes()
    setup = ("import pathlib,sys,tempfile; scope={'__name__':'proxyctl_fixture'}; "
             "exec(sys.stdin.read(),scope); root=pathlib.Path(tempfile.mkdtemp(prefix='proxyctl-tui-ssh-')); "
             "scope['fixture'](root); print(root)")
    root = remote_python(setup, data=source)
    state_files = [root + "/config/proxyctl/config.yaml", root + "/config/proxyctl/subscriptions.yaml"]
    hash_code = ("import hashlib,json,pathlib,sys; "
                 "print(json.dumps([hashlib.sha256(pathlib.Path(p).read_bytes()).hexdigest() for p in sys.argv[1:]]))")
    before = remote_python(hash_code, *state_files)
    checks = []
    try:
        for monochrome in (False, True):
            environment = dict(os.environ, TERM="xterm-256color", COLORTERM="truecolor")
            environment.pop("NO_COLOR", None)
            command = ["env", "-u", "NO_COLOR", "XDG_CONFIG_HOME=" + root + "/config",
                       "TERM=xterm-256color", "COLORTERM=truecolor"]
            if monochrome:
                command.append("NO_COLOR=1")
            command.append(str(binary))
            terminal = Terminal([*options, "-tt", host, shlex.join(command)], environment, 80, 12)
            exercise_dashboard(terminal)
            assert remote_python(hash_code, *state_files) == before, "SSH preview changed persisted state"
            if monochrome:
                assert b"[38;" not in terminal.output and b"[48;" not in terminal.output, "SSH NO_COLOR ignored"
            checks.append({"mode": "NO_COLOR" if monochrome else "TrueColor", "settings": True,
                           "theme_language_preview_cancel": True, "resize": True, "config_unchanged": True})
            terminal = Terminal([*options, "-tt", host, shlex.join([*command, "sub"])], environment)
            exercise_subscriptions(terminal)
            assert remote_python(hash_code, *state_files) == before, "SSH subscription cancellation changed metadata"
            if monochrome:
                assert b"[38;" not in terminal.output and b"[48;" not in terminal.output, "SSH subscription NO_COLOR ignored"
            checks.append({"subscription_mode": "NO_COLOR" if monochrome else "TrueColor", "mouse_modal_filter": True,
                           "resize": True, "config_unchanged": True})
    finally:
        remote_python("import shutil,sys; shutil.rmtree(sys.argv[1])", root)
    return checks


def free_port():
    with socket.socket() as listener:
        listener.bind(("127.0.0.1", 0))
        return listener.getsockname()[1]


def controller(address, path):
    request = urllib.request.Request(address + path, headers={"Authorization": "Bearer tui-local-token"})
    opener = urllib.request.build_opener(urllib.request.ProxyHandler({}))
    with opener.open(request, timeout=1) as response:
        return json.load(response)


def core_check(binary, core, kind, root):
    environment, app_file, profiles_file = fixture(root)
    work = root / "core"
    work.mkdir()
    port, group = free_port(), "TUI-Smoke"
    active = work / ("config.yaml" if kind == "mihomo" else "config.json")
    if kind == "mihomo":
        active.write_text(f"mode: rule\nlog-level: warning\nexternal-controller: 127.0.0.1:{port}\n"
                          "secret: tui-local-token\nproxies: []\nproxy-groups:\n"
                          "  - name: TUI-Smoke\n    type: select\n    proxies: [DIRECT, REJECT]\n"
                          "rules: ['MATCH,TUI-Smoke']\n")
        target = "REJECT"
        command = [str(core), "-d", str(work), "-f", str(active)]
    else:
        write_json(active, {"log": {"level": "warn"}, "experimental": {"clash_api": {
            "external_controller": f"127.0.0.1:{port}", "secret": "tui-local-token"}},
            "outbounds": [{"type": "direct", "tag": "DIRECT"}, {"type": "direct", "tag": "Backup"},
                          {"type": "selector", "tag": group, "outbounds": ["DIRECT", "Backup"], "default": "DIRECT"}],
            "route": {"final": group}})
        target = "Backup"
        command = [str(core), "-D", str(work), "-c", str(active), "run"]
    core_environment = dict(os.environ, LD_LIBRARY_PATH=str(core.parent))
    shim = root / "shim"
    shim.mkdir()
    systemctl = shim / "systemctl"
    systemctl.write_text("#!/bin/sh\nif [ \"$1\" = --user ] && [ \"$2\" = is-active ]; then\n"
                         "  printf 'active\\n'\n  exit 0\nfi\nprintf 'unexpected service operation\\n' >&2\nexit 9\n")
    systemctl.chmod(0o700)
    environment["PATH"] = str(shim) + os.pathsep + environment["PATH"]
    write_json(app_file, {"language": "en", "use": "tui-smoke", "kernels": [{
        "name": "tui-smoke", "type": kind, "bin": str(core), "configdir": str(work), "configfile": str(active)}],
        "theme": {"mode": "dark", "name": "default"}})
    before = hashes([app_file, profiles_file])
    address = f"http://127.0.0.1:{port}"
    group_path = "/proxies/" + urllib.parse.quote(group, safe="")
    with (work / "output.log").open("wb") as log:
        process = subprocess.Popen(command, stdout=log, stderr=log, env=core_environment)
        try:
            deadline = time.monotonic() + 10
            while True:
                try:
                    controller(address, "/proxies")
                    break
                except Exception:
                    if process.poll() is not None or time.monotonic() >= deadline:
                        raise AssertionError(f"{kind} controller did not start: {(work / 'output.log').read_text()[-2000:]}")
                    time.sleep(0.05)
            terminal = Terminal([str(binary)], environment)
            try:
                terminal.send(b"2", 0.6)
                terminal.send(b"\r")
                terminal.send(b"/")
                terminal.send(target.encode())
                terminal.send(b"\r", 0.6)
                assert controller(address, group_path)["now"] == target, "keyboard selection missed real controller"
                terminal.send(b"/")
                terminal.send(b"DIRECT")
                terminal.click(22, 3)  # First option row in the wide content-pane selector.
                terminal.drain(0.5)
                assert controller(address, group_path)["now"] == "DIRECT", "mouse selection missed real controller"
                terminal.send(b"q")
            finally:
                terminal.close()
        finally:
            process.terminate()
            try:
                process.wait(timeout=3)
            except subprocess.TimeoutExpired:
                process.kill()
                process.wait(timeout=3)
    assert hashes([app_file, profiles_file]) == before, "node selection changed application preferences"
    return {"core": kind, "controller": "real", "service_status": "simulated",
            "keyboard_selection": True, "mouse_selection": True, "config_unchanged": True}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("binary", type=pathlib.Path)
    parser.add_argument("--mihomo", type=pathlib.Path)
    parser.add_argument("--sing-box", type=pathlib.Path)
    parser.add_argument("--ssh", help="Drive an existing trusted SSH target with ssh -tt")
    arguments = parser.parse_args()
    if arguments.ssh:
        if arguments.mihomo or arguments.sing_box:
            parser.error("optional real core checks run in Linux directly, without --ssh")
        results = ssh_check(arguments.binary, arguments.ssh)
        print(json.dumps({"platform": "ssh-pty", "checks": results}, ensure_ascii=False))
        return
    if sys.platform != "linux":
        parser.error("run inside Linux; this script isolates settings through XDG_CONFIG_HOME")
    binary = arguments.binary.resolve()
    if not binary.is_file() or not os.access(binary, os.X_OK):
        parser.error(f"executable not found: {binary}")
    results = []
    with tempfile.TemporaryDirectory(prefix="proxyctl-tui-") as temporary:
        root = pathlib.Path(temporary)
        for monochrome in (False, True):
            results.append(dashboard_check(binary, root / ("mono" if monochrome else "color"), monochrome))
        results.append(persistence_check(binary, root / "persistence"))
        for term in ("xterm-256color", "dumb"):
            results.append(capability_check(binary, root / term, term))
        for name, explicit, reply in (("auto-light", False, "light"), ("explicit", True, "light"),
                                      ("undetected", False, None)):
            results.append(background_check(binary, root / name, explicit, reply))
        for monochrome in (False, True):
            results.append(subscription_check(binary, root / ("sub-mono" if monochrome else "sub-color"), monochrome))
        for kind, core in (("mihomo", arguments.mihomo), ("sing-box", arguments.sing_box)):
            if core:
                results.append(core_check(binary, core.resolve(), kind, root / kind))
    print(json.dumps({"platform": "linux-pty", "checks": results}, ensure_ascii=False))


if __name__ == "__main__":
    main()
