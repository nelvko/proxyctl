# TUI interaction validation

The automated terminal smoke checks live in `scripts/tui-smoke.py` and use
Python's standard library. They exercise the actual Bubble Tea program through
a PTY, including SGR mouse input, terminal background-query replies and
`SIGWINCH` resize events. These checks supplement the Go component and state
tests; they do not replace visual inspection in terminal applications.

## Running the isolated Linux checks

Build for the Linux environment's architecture and pass the executable's path:

```sh
env GOOS=linux GOARCH=arm64 go build -o dist/proxyctl-linux-arm64 .
python3 scripts/tui-smoke.py /absolute/path/to/proxyctl-linux-arm64
```

When using an existing OrbStack Linux VM with the project directory mounted:

```sh
orb -m proxyctl-e2e python3 /absolute/project/scripts/tui-smoke.py \
  /absolute/project/dist/proxyctl-linux-arm64
```

Every run creates temporary application and subscription settings through a
child-only `XDG_CONFIG_HOME`. It leaves the user's settings and services alone.
The script checks:

- Theme and language preview, cancellation, confirmation, persistence and reload.
- Appearance-document scrolling and wide/narrow terminal resizing.
- Subscription row clicks, filtered selections, details/help modals, modal
  scrolling/resizing, the removal dialog's default cancellation, and form
  navigation/cancellation using mouse buttons.
- Unchanged settings/metadata after cancelled interactions.
- TrueColor, `NO_COLOR`, indexed 256-color degradation and monochrome
  `TERM=dumb` output; keyboard navigation works without injected mouse input.
- A light OSC 11 background reply selects the light palette in Auto mode;
  an explicit scheme keeps precedence, and an absent reply keeps the dark
  fallback without saving a preference.

Optional real cores can exercise node selection against their loopback APIs:

```sh
python3 scripts/tui-smoke.py /absolute/path/to/proxyctl-linux-arm64 \
  --mihomo /absolute/path/to/mihomo \
  --sing-box /absolute/path/to/sing-box
```

The cores run directly with temporary configurations and ephemeral controller
ports. The script uses a child-only `PATH` shim to report an active service to
the dashboard; it rejects service-changing commands. Thus the controller and
node selection are real, while service status is simulated. This check does
not validate service installation or lifecycle management. Keep sing-box's
companion libraries beside its executable when its distribution requires them.

## SSH checks

From a machine with a previously trusted, noninteractive SSH connection:

```sh
python3 scripts/tui-smoke.py /absolute/remote/path/to/proxyctl-linux-arm64 \
  --ssh user@host
```

The binary path is on the remote host. The script requires Python 3 there,
creates temporary remote fixtures, and drives `ssh -tt` through a local PTY.
It checks dashboard theme/language preview cancellation, subscription
mouse/modal/filter operations, `NO_COLOR`, resizing and unchanged remote
configuration hashes. It does not install software, create a VM, change host
keys, or modify SSH configuration. Optional core checks run in Linux directly
rather than through this SSH mode.

## Verified scope

Checks performed on 2026-10-28:

| Environment | Result and scope |
| --- | --- |
| Debian bookworm arm64, OrbStack, Linux PTY | Dashboard and subscription interactions, persistence/cancellation, resizing, TrueColor/NO_COLOR/256 colors/TERM=dumb. |
| Actual `ssh -tt` connection to the same Linux VM | Dashboard and subscription keyboard/mouse/modal/filter checks, preview cancellation, resizing, TrueColor and NO_COLOR. |
| Mihomo v1.19.33 and sing-box v1.14.4, Linux arm64 | Keyboard and mouse node selection reached real local controller APIs; service status was simulated. |
| macOS native PTY | Read-only dashboard navigation, resizing, TrueColor and NO_COLOR. Application/subscription configuration hashes remained unchanged. |
| Windows Terminal | Windows amd64 cross-build passed; interactive testing was unavailable. Cross-compilation is not evidence of terminal behavior. |

The macOS checks used read-only navigation on the native binary. They did not
exercise isolated appearance persistence or mutation workflows: macOS
`os.UserConfigDir` uses the user's Application Support directory and ignores
`XDG_CONFIG_HOME`. The smoke script intentionally requires Linux for its
isolated local fixture mode. Native subscription-manager validation was
blocked by the existing local metadata's obsolete scheduling fields; the
loader rejected those fields as designed, and the file was left unchanged.
Subscription interactions were verified with isolated Linux and SSH fixtures.

PTY results verify event handling and output protocols. Real terminal font
rendering, Unicode glyph choices, mouse availability, remote multiplexers and
theme/background detection should still be inspected in the terminal
applications being supported. No interactive Windows Terminal result is
claimed.
