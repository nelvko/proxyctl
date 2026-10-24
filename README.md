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
proxyctl sub add 'https://example.com/subscription' --name work --use

# Add this to ~/.bashrc or ~/.zshrc, using the matching shell name:
eval "$(proxyctl init zsh)"

proxyctl on
proxyctl status
proxyctl off
```

To use sing-box, start with `proxyctl install sing-box` and add a sing-box JSON
subscription. Its active file is `config.json`; Mihomo uses Clash YAML.

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
proxy variables. To stop the shared service for all shells, use `proxyctl stop`.
Other shells retain their environment variables until they run `proxyctl off`.

## Commands

The kernel is the default management object, so its operations are top-level
verbs. Subscriptions have their own `sub` namespace.

| Command | Behavior |
| --- | --- |
| `install [name]` | Download and install Mihomo or sing-box, or install a local executable with `--bin`. With no name, choose a built-in kernel with format and installation status shown. The first installed kernel is selected. |
| `uninstall <name>` | Stop and remove the kernel's service, binary, managed config directory and registration. |
| `upgrade [name]` | Download the latest built-in release or recopy the saved local file. Restart a running service. |
| `list` | List known kernels, installation status and the selected kernel. |
| `use [name]` | Select an installed kernel (choose interactively when omitted) and apply its selected compatible subscription. If none is selected for that format, leave the target service stopped. |
| `start` | Start the selected shared kernel service without changing shell variables. |
| `stop` | Stop the selected shared service without changing shell variables. |
| `restart` | Restart the selected shared service, starting it if stopped. |
| `status` | Show the selected kernel, shared service, proxy endpoints, shell variables and subscription. |
| `on` | Enable the current shell's proxy variables, starting the service if needed. |
| `off` | Clear the current shell's proxy variables; the service keeps running. |
| `env [--unset]` | Print shell statements without starting or stopping a service. `--unset` works without a kernel or valid configuration. |
| `init [shell]` | Print integration code for bash, zsh or fish. Does not edit shell configuration files. |

Selecting a kernel or subscription does not modify any shell's environment.
If a switch changes the proxy address or port, run `proxyctl on` again in the
shells that use it.

## Subscriptions

`subscription` is an alias for `sub`; `ls` aliases `list`, and
`rm`, `delete`, and `del` alias `remove`. For example, `proxyctl subscription ls` lists profiles.

| Command | Behavior |
| --- | --- |
| `sub` | Open the interactive subscription manager. Requires a terminal. |
| `sub list` | List saved profiles without requiring an installed kernel. |
| `sub add [url]` | Download, validate and save a profile. With no URL, prompt for its source and name. Does not select it or start a service. If no profile is selected, show the next steps: `sub use <name>` and `on`. |
| `sub add <url> --use` | Also select and apply the added profile, preserving service running state. |
| `sub use [name]` | Select and apply a profile. Restart a running kernel; stage the configuration when stopped. |
| `sub update [name...]` | Fetch named profiles from their sources, or the selected profile when no names are supplied. Use `--all` to update every profile compatible with the selected kernel; names and `--all` are mutually exclusive. Apply changes to the selected profile without starting a stopped service. |
| `sub edit [name]` | Edit a temporary copy using `$EDITOR` or `--editor`. Validate before replacing the local profile. Does not apply the edit. |
| `sub edit [name] --apply` | Also select and apply the edited profile, preserving service running state. |
| `sub remove [name]` | Remove an unselected profile and its cached file, preserving the remote or file URL source. |
| `sub remove [name] --force` | Permit removing the selected profile; stop the selected kernel and remove its applied configuration too. |
| `sub modify <name>` | Change source, name or download settings without downloading or applying. Without flags, opens a form in a terminal. |
| `sub show [name]` | Inspect metadata; defaults to the selected profile. Source credentials are hidden unless `--show-url` is supplied. |

For `use`, `edit` and `remove`, omitting the name prompts for the target of
that operation. Scripts must supply a name. The general manager is opened
only by `proxyctl sub`; there is no global `--interactive` flag.

Profiles record their configuration format. `sub list` shows profiles of all
formats; `sub use`, `edit`, and `update` require a compatible selected kernel.
Switching between Mihomo and sing-box remembers one selected profile per
format and never applies Clash YAML to sing-box or sing-box JSON to Mihomo.

Sources can use HTTP, HTTPS or `file://` URLs. An update replaces local edits
with source content. After a local edit, use `sub use <name>` or `edit --apply`
to apply it. Failed applications are retried by `sub update` even when the
source content is unchanged.

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
and each update reads the original path again, replacing local edits. It is
not an independent import; HTTP settings do not apply to file reads.

`sub add` and `sub update` accept `--keep-failed`. Complete downloads rejected
by the kernel are saved under the application's `diagnostics/` directory
(directory mode 0700, file mode 0600). The error prints the actual path; these
files are not registered or applied. Remove them after inspection. Without
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
| `internal/subscription` | Subscription source settings, validation, cached content and application. |
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

Optional OrbStack/systemd integration targets live in `make/orbstack.mk` and
remain available through `make e2e`, `make e2e-shell` and `make shell`. They use
`VM` and `VM_BIN` overrides. `make e2e-reset` removes proxyctl state in the test VM.
The default build and `verify` require neither OrbStack nor Docker.
