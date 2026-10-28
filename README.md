# proxyctl

Manage proxy kernels as user services, subscription profiles, and proxy
settings in individual shells. Service management currently requires Linux
with systemd. Mihomo and sing-box have built-in installation presets; other
executables can be installed from local files.

Run `proxyctl install` without a name to choose a built-in kernel. The picker
shows configuration formats, installation status and the selected kernel.
Press Enter to install, or q, Esc or Ctrl+C to cancel. To install a local
executable, provide a name and `--bin`, with `--type` for a compatible fork.

## Quick start

```sh
proxyctl install mihomo

# Add this to ~/.bashrc or ~/.zshrc, using the matching shell name:
eval "$(proxyctl init zsh)"

# The initial configuration uses direct connections; no subscription is required.
proxyctl on
proxyctl status
proxyctl off

# Select a subscription when you want to use its configuration:
proxyctl sub add 'https://example.com/subscription' --name work --use
proxyctl on
```

## Terminal dashboard

Run `proxyctl` without arguments in a terminal to open the dashboard. Narrow or
short terminals show page tabs across the top; at 110 columns and 19 rows or
larger the pages move to a left sidebar. Click a tab/sidebar item or press
`1`–`7` to switch pages. `Tab` and `Shift+Tab` cycle pages. Navigation starts
focused: use `↑`/`↓` to switch sidebar pages (or `←`/`→` on top tabs), then `Enter` to
focus the page content. Within a page, `↑`/`↓` select rows, `Enter` applies a
subscription or chooses a node, and `Esc` returns to navigation. `r` refreshes
and `q` exits. The footer shows the current focus and relevant keys.
Without a terminal, the bare command prints help.

The overview, subscription and read-only settings pages use proxyctl's own
state. The nodes, connections, rules and live logs pages need a running kernel
with its Clash-compatible control API enabled. The dashboard reads the control
address and secret from the **active kernel configuration**; it does not assume
port 9090 or change existing configurations. For Mihomo, configure
`external-controller` and `secret`; for sing-box, configure
`experimental.clash_api.external_controller`
and `experimental.clash_api.secret`. Bind the API to `127.0.0.1` and set a
private secret. If you use subscriptions, put these settings in a global
override and reapply the selected profile so they survive profile updates.
Without the API, the dashboard explains why those pages are unavailable.
The rules page loads on entry and refreshes every 10 seconds or with `r`.
Settings show the active configuration and the CLI commands used to change it.
The wide sidebar shows live upload/download rates from the
control API. Unavailable rates appear as `—`.

The dashboard distinguishes shared kernel service state from the calling
shell's proxy variables. To change the parent shell environment, run
`proxyctl on` or `proxyctl off` in that shell.

Mihomo and sing-box installations initialize a minimal valid configuration with
a mixed HTTP/SOCKS listener at `127.0.0.1:7890` and direct outbound routing.
Subscriptions are optional: `start` needs a valid kernel configuration, while
`on` also needs a usable proxy inbound. Neither command requires a selected
subscription. `on` labels the built-in default as
`(direct; default configuration)`; `status` shows `config: default (direct)`.
This label is based on an exact match with the built-in configuration, not
merely the absence of a selected subscription.

Existing nonempty configuration files are preserved, including manual
configurations. `start`, `restart`, `on` and kernel switching also initialize
missing managed configuration files and repair older zero-byte placeholders
with the minimal default. Invalid nonempty files report validation errors
instead of being replaced. Adding a subscription without `--use` only validates
and caches it; it leaves the active default or manual configuration unchanged.

To use sing-box, start with `proxyctl install sing-box`. Add a sing-box JSON
subscription when needed. Its active file is `config.json`; Mihomo uses Clash
YAML.

For another kernel, download and extract its executable first, then install it
from a local file:

```sh
proxyctl install mycore --bin ./mycore
# For a compatible Mihomo or sing-box fork:
proxyctl install myfork --bin ./myfork --type mihomo
```

The file is copied into proxyctl's managed directory; the original stays in
place. A custom kernel has a basic user service. Subscription validation and
`on` require a compatible built-in adapter selected with `--type mihomo` or
`--type sing-box`. `upgrade mycore` copies the saved local file again; run
`install mycore --bin /path/to/new/binary` to change its source.

For fish, add `proxyctl init fish | source` to `config.fish`.

`on` starts the selected kernel when needed, waits for its proxy endpoint,
and enables the proxy in the current shell. `off` only clears that shell's
proxy variables. Use `proxyctl off --stop` to clear the current shell and stop
the selected shared service, or `proxyctl stop` to stop it without clearing the
shell. Other shells retain their environment variables until they run
`proxyctl off`.

`off --stop` still clears the current shell when stopping fails, and returns
a nonzero status for the failure. An already stopped service or no selected
kernel needs no stop operation. Plain `off` and `off --stop=false` work even
when saved configuration is broken.

After updating proxyctl, reload `proxyctl init` in existing shells to enable
the new flag in the shell wrapper (`eval "$(proxyctl init zsh)"` for zsh).

## Commands

The kernel is the default management object, so its operations are top-level
verbs. Subscriptions have their own `sub` namespace.

| Command | Behavior |
| --- | --- |
| `install [name]` | Download and install Mihomo or sing-box with a minimal direct configuration, or install a local executable with `--bin`. Existing nonempty configuration is preserved. With no name, choose a built-in kernel with format and installation status shown. The first installed kernel is selected. |
| `uninstall <name>` | Stop and remove the kernel's service, binary, managed config directory and registration. |
| `upgrade [name]` | Download the latest built-in release or recopy the saved local file. Restart a running service. |
| `list` | List known kernels, installation status and the selected kernel. |
| `use [name]` | Select an installed kernel (choose interactively when omitted) and apply its selected compatible subscription, or use its existing/default configuration. Preserve whether the previous selected service was running, including when no subscription is selected. |
| `start` | Start the selected shared kernel service with its valid configuration, without requiring a subscription or changing shell variables. |
| `stop` | Stop the selected shared service without changing shell variables. |
| `restart` | Restart the selected shared service, starting it if stopped. |
| `status [--json]` | Show the selected kernel, shared service, proxy endpoints, shell variables and subscription. Use `--json` for structured output. |
| `on` | Enable the current shell's proxy variables after checking its inbound and starting the service if needed. A selected subscription is optional. |
| `off [--stop]` | Clear the current shell's proxy variables. With `--stop`, also stop the selected shared kernel service. |
| `env [--unset]` | Print shell statements without starting or stopping a service. `--unset` works without a kernel or valid configuration. |
| `init [shell]` | Print integration code for bash, zsh or fish. Does not edit shell configuration files. |

Selecting a kernel or subscription does not modify any shell's environment.
If a switch changes the proxy address or port, run `proxyctl on` again in the
shells that use it.

`status --json` emits one JSON object with `kernel`, `service`, `proxy`,
`shell` and `profile` sections. Service states distinguish `running`, `stopped`,
`unknown` (query failure) and `unavailable` (no selected kernel). Local query
failures have a redacted `error` field; unknown profile totals are `null`.
If the main configuration cannot be loaded, the command exits nonzero without
writing a JSON object.

Proxy endpoints are read from saved configuration without testing connectivity;
`proxy.state = "available"` means an endpoint is configured. Endpoints hide
credentials, and `shell.proxy_variables` contains only nonempty variable names,
including the upper/lowercase HTTP, HTTPS, ALL and NO_PROXY names. No variable
values or subscription source URLs are included.
`proxy.default_config` is `true` when the active file exactly matches the
built-in direct template, `false` for another readable configuration, and
`null` when the configuration cannot be read.

## Subscriptions

`subscription` is an alias for `sub`; `ls` aliases `list`, and
`rm`, `delete`, and `del` alias `remove`. For example, `proxyctl subscription ls` lists profiles.

| Command | Behavior |
| --- | --- |
| `sub` | Open the interactive subscription manager. Requires a terminal. |
| `sub list` | List saved profiles without requiring an installed kernel. |
| `sub add [url]` | Download, validate and save a profile. With no URL, prompt for its source and name. Does not select it, change the active configuration or start a service. If no profile is selected, explain how to apply it with `sub use <name>`. |
| `sub add <url> --use` | Also select and apply the added profile, preserving service running state. |
| `sub use [name]` | Select and apply a profile. Restart a running kernel; stage the configuration when stopped. |
| `sub update [name...]` | Fetch named profiles from their sources, or the selected profile when no names are supplied. Use `--all` to update every profile compatible with the selected kernel; names and `--all` are mutually exclusive. Apply changes to the selected profile without starting a stopped service. |
| `sub edit [name]` | Edit a temporary copy of the source cache using `$EDITOR` or `--editor`. Validate the effective configuration before saving. Does not apply the edit; a later update replaces these source edits. |
| `sub edit [name] --apply` | Also select and apply the edited profile, preserving service running state. |
| `sub remove [name]` | Remove an unselected profile, its cached file and its override, preserving the remote or file URL source. |
| `sub remove [name] --force` | Permit removing the selected profile; stop the selected kernel and remove its applied configuration too. |
| `sub modify <name>` | Change source, name or download settings without downloading or applying. Without flags, opens a form in a terminal. |
| `sub show [name]` | Inspect metadata; defaults to the selected profile. Source credentials are hidden unless `--show-url` is supplied. |
| `sub override edit <name>` / `sub override edit --global` | Edit and validate a profile override or the selected format's global override. Save without applying. Supports `--editor` / `-e`. |
| `sub override show <name>` / `sub override show --global` | Print the saved override, including raw configuration values. |
| `sub override clear <name>` / `sub override clear --global` | Validate the remaining configuration, then remove the override without applying. |

For `use`, `edit` and `remove`, omitting the name prompts for the target of
that operation. Scripts must supply a name. The general manager is opened
only by `proxyctl sub`; there is no global `--interactive` flag.

Profiles record their configuration format. `sub list` shows profiles of all
formats; `sub use`, `edit`, and `update` require a compatible selected kernel.
Switching between Mihomo and sing-box remembers one selected profile per
format and never applies Clash YAML to sing-box or sing-box JSON to Mihomo.
When the target format has no selected profile, switching uses that kernel's
existing or minimal direct configuration and preserves the running/stopped
state. Adding a profile alone does not replace that configuration; use
`sub use <name>` or `sub add ... --use` to select and apply it.

Sources can use HTTP, HTTPS or `file://` URLs. An update replaces edits made by
`sub edit` with source content, but preserves separately saved overrides. After
a source edit, use `sub use <name>` or `edit --apply` to apply it. Failed
applications are retried by `sub update` even when the source content is
unchanged.

### Configuration overrides

Use overrides for changes that should survive subscription updates. The
effective configuration is built in this order:

```text
source cache → global override for its format → profile override → kernel validation
```

Clash profiles use YAML; sing-box profiles use strict JSON. Objects merge
recursively, arrays replace the entire previous array, and scalar values replace
previous values. `null` is a literal replacement, not a deletion instruction.
An empty object `{}` makes no changes. When merging, the source and each override
must be a single object; duplicate keys and extra documents are rejected. JSON
comments and trailing commas are not supported. There are no script or field
deletion operators.

Every override command requires either a profile name or `--global`, exclusively.
There is no implicit current-profile or global target. `--global` applies only
to the selected kernel's configuration format, so Mihomo and sing-box keep
separate global overrides. Editing or clearing requires a compatible selected
kernel. Showing a named profile's override needs no kernel; showing the global
override needs a selected adapter to determine its format.

```sh
proxyctl sub override edit work --editor vim
proxyctl sub override show work
proxyctl sub use work

# Edit the global override for the currently selected format:
proxyctl sub override edit --global -e vim
proxyctl sub use work

# Remove this profile's override, then apply the remaining layers:
proxyctl sub override clear work
proxyctl sub use work
```

For a Clash profile, an override can contain:

```yaml
mixed-port: 7891
log-level: warning
```

For a sing-box profile, an override can contain:

```json
{"log": {"level": "warn"}}
```

The editor defaults to `$EDITOR` when `--editor` is omitted. Saving or clearing
an override validates the resulting configuration but leaves the active file
and service unchanged. Use `sub use <name>` to apply it. A global change is
checked against every cached profile of that format; any failure preserves the
previous override. When saving a global override with no matching profiles,
only syntax can be checked, and the command reports this limitation. Clearing
a required override can fail if the remaining configuration would be invalid.

Adding, editing, updating, applying and switching to a profile all validate its
effective configuration. Downloads remain in the original source cache. An
update saves changed source content but does not restart a service when the
effective content is unchanged. Updating a selected profile also applies changed
overrides even when the source download is identical. A stopped service remains
stopped. If the applied proxy address changes, run `proxyctl on` again.

Global overrides live at `overrides/clash.yaml` and `overrides/sing-box.json`
under the proxyctl configuration directory. Profile overrides are stored beside
their stable cache file as `<cache-file>.override.yaml` or
`<cache-file>.override.json`. Renaming a profile preserves its override; deleting
the profile removes it. Saved overrides use file mode 0600. The shared
`overrides` directory is reserved and protected from kernel uninstallation.
`sub override show` prints the saved patch, not the merged
configuration, and explicitly exposes its raw values, which may include
credentials. Ordinary `sub show` metadata output continues to hide source
credentials.

### Names, downloads and diagnostics

An explicit `--name` takes priority. Otherwise, the HTTP response's
`Content-Disposition` filename (`filename*` preferred) supplies the name,
falling back to the local filename or source hostname. Generated names are
sanitized and receive a suffix on collision. Updates never rename profiles.
Cache filenames are stable random identifiers, independent of display names.

`--use` selects and applies an added profile. Download routing is selected with
`--download-via env|direct|kernel`: `env` follows HTTP(S)_PROXY and NO_PROXY;
`direct` disables explicit HTTP proxies (OS TUN/VPN routing still applies);
`kernel` requires a running managed kernel and uses its authenticated HTTP or
SOCKS endpoint, without fallback or automatic startup.

`add` and `modify` save download settings. On `update`, explicitly provided
settings apply only to that invocation, including every target of `--all`.
Omitted settings inherit each profile's saved values. `--timeout 0` restores
the default 10s total download budget, including retries but excluding kernel
validation. The default User-Agent remains `Go-http-client/1.1`.

```sh
proxyctl sub add https://example.com/sub --name work --download-via kernel
proxyctl sub modify work --name office
proxyctl sub modify office --user-agent 'Client/1.0' --timeout 30s
proxyctl sub modify office --download-via env
proxyctl sub update office --download-via direct --timeout 1m
proxyctl sub modify office --header 'X-Client: proxyctl'
proxyctl sub modify office --remove-header X-Client --insecure=false
proxyctl sub show office
```

`--header 'Name: value'` can be repeated for distinct names; header replacement
and `--remove-header` are case insensitive. `--user-agent` and a User-Agent
header must not conflict. `--insecure` disables certificate verification only
for HTTPS subscription downloads; verification is enabled by default and can
be restored with `--insecure=false`. Request settings and TLS transports are
isolated; custom headers are stripped on redirects to a different origin.
Normal inspection displays header names, not their values.

`file://` is a linked local source: its contents are copied into the cache,
and each update reads the original path again, replacing source-cache edits
while keeping overrides. It is not an independent import; HTTP settings do not
apply to file reads.

`sub add` and `sub update` accept `--keep-failed`. Complete downloads whose
effective configuration fails validation are saved under the application's
`diagnostics/` directory
(directory mode 0700, file mode 0600). The error prints the actual path; these
files contain the original downloaded content, not the merged configuration,
and are not registered or applied. Remove them after inspection. Without
the flag, failed downloads are cleaned up and validation errors explain how
to retain them. Empty, interrupted and cancelled downloads are not retained.

Saved HTTP request settings live under each profile's `download:` mapping
(`config.DownloadConfig`), shared by add and update. The former `update:` mapping
and scheduling fields (`enable`, `interval`, `cron`) are not supported. Unknown
subscription configuration fields fail loading instead of being ignored.

### Interactive behavior and concurrent operations

The subscription manager can open without an installed kernel. Inspecting or
modifying metadata and removing an unselected profile need no kernel; adding,
updating, applying and validating edits require one.

The subscription manager searches names first, then source hostnames. Use
Enter to select/apply, `u` to update the highlighted profile, `a` to add, `s`
to show details, `m` to modify metadata, `e` to edit configuration content,
`dd` to remove, `?` to expand help, and `q` to quit. `v` has no
selection or quit meaning. During work, Ctrl+C requests cancellation and the
manager waits for the operation to finish safely before accepting new actions.

Add and modify forms share download settings and validation with the CLI.
Advanced HTTP options are optional and hidden for file sources. Failed or
cancelled downloads retain the draft for correction and retry; success or
explicit form cancellation clears it. If another process changes a profile
while its metadata form is open, submission asks you to reopen the form
instead of overwriting the newer settings. The details view scrolls with
arrow keys and returns to the list with Esc or q.

State-changing commands serialize through a local file lock, held from state
loading through commit or rollback, including downloads and editors. Waiting
for the lock is cancellable. Queries and the idle manager do not hold it; each
manager operation reloads current state after acquiring it. This deliberately
serializes long mutations too, avoiding stale writes without an optimistic
transaction protocol. Service calls already in progress and rollback finish
before the lock is released.

Update output reports each profile as updated, unchanged or failed, and
reports application/restart or staging when relevant. Service commands report
already-running/stopped states and print in-progress feedback on terminals.
Human-readable URLs hide credentials and source paths; executable `env`/`on`
output preserves real credentials so shell integration remains functional.
Dynamic completion reads saved names without starting services or downloads.

## Code organization

| Package | Responsibility |
| --- | --- |
| `cmd` | Root command assembly, shell integration, status and private execution helpers. |
| `cmd/kernel` | Kernel management and service commands, completion and interactive selection; registered at the root through `kernel.NewCommands()`. |
| `cmd/sub` | The subscription command group, its request flags, completion and interactive target selection; registered through `sub.NewCommand()`. |
| `internal/app` | Runtime assembly, mutation locking and workflows spanning kernels and subscriptions. |
| `internal/config` | Persisted models, shared paths, atomic writes and state locking. |
| `internal/subscription` | Subscription source settings, source caches, configuration overrides, effective validation and application. |
| `internal/kernel` | Kernel adapters, release metadata, binary installation/extraction and service integration. |
| `internal/download` | Remote content retrieval shared by add, update, install and upgrade: transport, retries, timeouts, mirrors and progress. |
| `internal/redact` | Sensitive URL/error redaction and control-character filtering, independent of presentation. |
| `internal/ui` | Terminal output, prompts, progress, external editor execution and the full-screen subscription manager. |
| `internal/env` | Shell environment representation and statement generation from proxy endpoints. |

`app.WithStateLock` serializes mutations; it provides no automatic rollback.
Each workflow owns its commit and recovery. Small private `mutation` adapters in
command packages acquire the lock before loading state. TUI operations choose
`app.MutateProfileMetadata` or `app.MutateRuntimeProfiles` explicitly, loading
fresh state inside the same lock. Downloads and external editors remain inside
this boundary; domain methods do not acquire nested locks.

`app.LoadProfileMetadata` reads metadata without a main config or installed
kernel; `app.LoadRuntimeProfiles` explicitly binds a kernel. Operations requiring
validation/application reject a metadata-only service with `ErrNoRuntime`.
Profile reads and metadata writes copy header maps at the service boundary.

`app.Load()` only loads state. Each operation supplies its own downloader and
notification callback; no package-wide mirror/client/quiet flags are changed.
`app.StartProxy` starts the service and waits for a reachable proxy endpoint;
commands only format the shell output. `app.RemoveProfile` chooses whether a
removal requires runtime state. Kernel adapters own their service arguments
through `ServiceSpec`, as well as resolving HTTP/SOCKS endpoints;
subscription routing and shell output do not parse Mihomo YAML themselves.
The subscription package returns typed application/update states, while `ui`
renders their messages. Full-screen operations collect warnings for presentation
without direct stderr writes from download or kernel code.

Kernel layout is resolved in `config`. Uninstall validates ownership before
stopping a service, and recursively deletes only that kernel's default directories.
Shared roots, other resources and symlinked kernel directory components are
rejected. Custom config directories are retained; custom binaries are removed as
individual files. Missing or relative home/config roots cause explicit errors.

`download` names a reusable capability, not a CLI command. Subscription updates
and kernel upgrades use it to retrieve new content; version comparison, cache
replacement, restarting and rollback remain in their domain/application layers.
`Downloader.Fetch` is the single content-transfer entry point with explicit
request options. Archive extraction belongs to kernel installation.

Kernel and subscription commands live in `cmd/kernel` and `cmd/sub`, each owning
its command definitions, completion and interactive selection. Packages follow
code responsibilities; the root registers kernel commands directly, so users
still run `proxyctl install`, `proxyctl upgrade` and `proxyctl start`. Command
packages keep small Cobra adapters private while sharing state-lock semantics
through `app`, without an additional helper package.

See [the architecture change record](docs/architecture-review.md) for the reviewed
issues, implementation boundaries and regression coverage.

## Development

```sh
make                  # native binary in dist/proxyctl
GOWORK=off make verify # vet, tests, isolated CLI smoke and Linux build
GOWORK=off go test -race ./...
```

`go.mod` pins a remotely available `unisvc` revision; a sibling checkout is not
required. For local development of both modules, use an ignored workspace file:

```sh
go work init . ../unisvc
```

Keep `go.mod` free of machine-specific replacements. `GOWORK=off` verifies the
standalone dependency graph even when a local workspace exists.

Optional OrbStack/systemd integration targets live in `make/orbstack.mk`.
Supply extracted Linux executables visible inside the VM, with sing-box's
companion library beside its executable when provided by the release:

```sh
GOWORK=off make e2e-systemd \
  MIHOMO_BIN=/path/to/linux/mihomo \
  SING_BOX_BIN=/path/to/linux/sing-box
```

The suite uses temporary state, unique user service names, and a local HTTP
fixture. It covers subscription-free startup, empty-file migration, manual
configuration preservation, failed-switch rollback, shell cleanup, restart,
format-specific subscription selection, local upgrade/reinstall, uninstall and
JSON status. Both formats also exercise global/profile override precedence,
staged edits and clearing, rejected invalid patches, raw-content display and
cache preservation, and updates without restarting an unchanged effective
configuration. Port 7890 must be free. Test services and temporary files are
removed on exit; existing proxyctl state is preserved.

`make e2e`, `make e2e-shell` and `make e2e-reset` run this same isolated suite.
`make e2e-status` reads the VM's normal configuration; `make shell` opens an
interactive shell. Override `VM` and `VM_BIN` as needed. The default build and
`verify` require neither OrbStack nor Docker.
