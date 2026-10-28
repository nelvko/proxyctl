# TUI themes

The dashboard keeps its purple dark appearance by default, with stronger text
contrast, quieter separators and low-saturation selection backgrounds. Theme
settings live in proxyctl's existing `config.yaml`; there is no separate theme
file or new dependency. Kernel configuration files are unrelated.

## Configuration

`config.yaml` is in proxyctl's user configuration directory: normally
`~/.config/proxyctl` on Linux, `~/Library/Application Support/proxyctl` on macOS,
or `%AppData%\proxyctl` on Windows. Linux also respects `XDG_CONFIG_HOME` through
Go's `os.UserConfigDir`.

```yaml
theme:
  name: tokyo-night
  mode: auto
```

Stable built-in names are:

| Configuration name | Display name |
| --- | --- |
| `default` | Default |
| `catppuccin-mocha` | Catppuccin Mocha |
| `tokyo-night` | Tokyo Night |
| `dracula` | Dracula |
| `nord` | Nord |

`mode` accepts `auto`, `dark` and `light`:

- With no explicit `name`, `auto` follows a terminal background response. It
  starts with the safe dark default when no response is available.
- An explicitly named palette, including `default` and `custom`, stays dark in
  `auto` mode. This preserves an intentional color choice on light terminals.
- `dark` and `light` explicitly select the palette's background variant,
  regardless of terminal detection. The named dark schemes have light
  adaptations with matching accent families; these are proxyctl adaptations.

Omitting `theme` keeps automatic defaults and remains compatible with existing
configuration files. Restoring default appearance removes the appearance
preferences and allows background detection again. Invalid names or modes use
the default palette or automatic mode and produce a TUI warning.

## Appearance controls

Open Settings with `7`, then `Enter` to focus its appearance controls. Use
arrows to choose mode, palette or restore defaults. `Enter` or `Space` opens a
select; arrows and mouse hover preview the focused option immediately.
`Enter` or clicking an option saves it. `Esc`, clicking outside, changing pages
or quitting cancels an unconfirmed preview. A failed save restores the last
saved palette and displays an error. The reset action saves automatic defaults;
custom color overrides are cleared when resetting.

The same page includes an independent language select with live preview,
confirmation and cancellation. Its saved `language` field shares `config.yaml`
with `theme`; resetting appearance preserves it. See [language support and
migration](i18n.md) for locale precedence and the translation boundary.

On short terminals the focused control stays visible; `PageUp`/`PageDown` and
the mouse wheel scroll the complete settings document, including runtime
errors, control API hints and CLI modification commands. Resizing clamps the
scroll position to the actual content. Arrow keys return to the appearance
controls. A terminal must have enough cells to display a control and its
options: when no option row or label can be shown, selection is paused until
the window grows; cancellation and search remain available. Keyboard operation
remains available when mouse motion or background detection is unsupported.

Node and subscription pages expose a clickable `[搜索 /]` action as well as
the `/` shortcut. The searchable selector temporarily owns the content pane;
`Esc` restores the parent page and its focus. Clicking or confirming a result
uses the existing node-selection or subscription-application operation.
Results are resolved by name against the latest snapshot before applying.
Wrong-shaped appearance fields also inherit defaults and produce a warning;
they do not block CLI kernel operations. Invalid YAML syntax or malformed
non-appearance settings still report the existing configuration errors.

## Custom colors

Use `name: custom` to enable color overrides. `base` can name any built-in
palette and defaults to `default`. Missing or invalid colors inherit the base
palette, so a custom theme can override just one token:

```yaml
theme:
  name: custom
  mode: dark
  base: default
  colors:
    background: "#141626"
    surface: "#1C1E32"
    foreground: "#C7CBE0"
    muted: "#999AB5"
    primary: "#A78BFA"
    border: "#34304F"
    success: "#4ADE80"
    warning: "#FBBF24"
    error: "#FB7185"
    selected: "#302843"
    hover: "#25243B"
    disabled: "#75758F"
```

Colors must be quoted six-digit hexadecimal RGB values (`#RRGGBB`). Unknown
tokens are ignored with a warning. Invalid values fall back independently;
one invalid color does not discard the other valid overrides. Color overrides
are only active for `custom`, so choosing a built-in palette cannot accidentally
retain a previous custom color. The TUI does not edit individual color tokens.

`foreground` is normal text, `muted` is secondary text and `disabled` is an
unavailable control. `primary` marks headings and focused controls. `border`
colors separators. `selected` and `hover` are backgrounds, with foreground text
and textual selection markers retained for terminals with limited colors.
`success`, `warning` and `error` are status roles. Custom colors are format
validated; the user is responsible for their contrast.

## Import and export

Export the saved preferences to stdout or a standalone YAML file:

```sh
proxyctl theme export
proxyctl theme export appearance.yaml
proxyctl theme export appearance.yaml --force
```

An omitted path or `-` writes YAML only to stdout, making shell redirection and
pipes usable. File exports use private permissions (`0600` on Unix). Existing
files are preserved unless `--force` is supplied. A forced export replaces the
file through a temporary file in the same directory.

Import the same `theme:` document shape from a file or stdin:

```sh
proxyctl theme import appearance.yaml
proxyctl theme import - < appearance.yaml
proxyctl theme export - | proxyctl theme import -
```

Imports validate the complete document before saving. Unknown fields, unknown
palette or base names, invalid modes, invalid color tokens and colors outside
`#RRGGBB` are rejected without changing the current configuration. Inactive
custom overrides are validated too. A reset can be imported as `theme: {}`.
Unlike normal startup's forgiving fallback, importing a typo is an error.

Exports contain the saved preferences and partial overrides, preserving all
theme fields on round-trip. They do not contain kernel or subscription state,
or expand the palette into colors that depend on the current terminal.
Imports replace only appearance preferences using the shared state lock and
preserve the latest kernel and mirror settings. Reopen an already running TUI
after an external import; changes made in its appearance controls preview
immediately within that TUI.

## Implementation and persistence

`internal/ui/theme.go` owns the semantic `Theme` structure, built-in registry
and resolution rules. Components receive a resolved palette, keeping colors
out of their business logic. `internal/config/theme.go` owns the persisted
`ThemeConfig` and theme save operation.

Previewing changes only the active TUI state. Saving appearance acquires the
existing shared state lock, reloads the latest configuration and atomically
writes just the updated theme field. This preserves kernel selection,
installation metadata and mirror changes made while the TUI was open.
Cancelling a preview restores the prior appearance without writing the file.

The application relies on Bubble Tea v2's terminal color-profile conversion
for 16/256-color terminals. A non-empty `NO_COLOR`, `TERM=dumb`, or a detected
ASCII/no-color profile disables ANSI styling. Selection and status retain
text markers. Backgrounds are rendered as cells, without changing terminal
default colors through OSC commands. Background detection is optional: keyboard operation
and default rendering must remain usable when a terminal does not answer
color queries or provide mouse events.

The implementation keeps the existing Bubble Tea dashboard model, adaptive
sidebar/tabs, Huh forms and Bubbles subscription list. `Dropdown` uses an inline
layout that pushes subsequent rows down, with Bubbles Viewport handling option
scrolling, clipping and resizing. `SelectModal` reuses List's built-in
`FilterInput`, synchronous fuzzy filtering, cursor navigation and pagination
inside the content pane. Search matches both labels and values. The wrappers
only add theme styling, preview/confirmation/cancellation, disabled-option
handling and mouse hit mapping. Both share `Rect` hit regions and semantic
tokens. This avoids fragile character-grid overlays and new dependencies.
The settings document also uses Viewport for scrolling and clipping; the same
document construction supplies its mouse regions, so content height and hit
coordinates do not rely on separate estimates.
The dashboard requests Bubble Tea v2's `MouseModeAllMotion` for hover; terminals
that only send clicks can still navigate, and every primary action has a
keyboard path. The generic confirmation/details modal uses Lip Gloss's native
cell compositor and Bubbles Viewport; its buttons share the theme and hit-region
implementation. A visual color editor, complex animations and arbitrary
component overlays remain outside this implementation.

Tests cover palette precedence, partial overrides, invalid-value fallbacks,
stable names, built-in text contrast, backwards-compatible configuration and
appearance saves that preserve other application settings. Cross compilation
does not establish rendering or input compatibility with an actual terminal.
See the [terminal validation record](tui-validation.md) for the completed Linux,
SSH and macOS checks and the remaining Windows Terminal verification.


## Dashboard presentation

The overview uses four themed cards: current subscription, selected proxy group,
traffic statistics, and kernel/environment details. Wide screens use two
columns; narrow screens stack the cards. Bubbles Viewport keeps the full
document scrollable instead of omitting fields on short screens. `Enter`
focuses the content, arrows and `PageUp`/`PageDown` scroll it, and `Esc` returns
to navigation. The mouse wheel scrolls the document; visible card buttons use
the same bounds for rendering and clicks.

`p` opens subscriptions and `n` opens nodes. `g` searches for a proxy group to
display without changing its active node. `/` opens node selection for the
displayed Selector group and confirms through the existing controller operation.
Other group types remain read-only. Targets are resolved by stable name against
the latest snapshot before applying; cancellation preserves the parent focus
and scroll position.

The traffic card shows active connections, core-reported cumulative upload and
download, and optional core-reported Go memory. Live upload/download rates stay
in the sidebar and are not repeated in the overview. Missing or unsupported
metrics show `—`, while known zero values remain zero. Cumulative traffic is
neither subscription quota nor a cross-restart ledger; reported memory is not
OS process RSS. Subscription sources show only a hostname or local-file label.
The version is the installed binary's version, cached until its identity/file
metadata changes or the user explicitly refreshes.

Header and footer use low-contrast slash fill on the main background. Titles
and hints reserve their own space; the slash runs expand or shrink with the
terminal width. The final rendered cell grid fills unspecified foreground and
background colors after nested styles, so ANSI resets cannot expose the
terminal's unrelated background color.

A failed subscription metadata read shows an unavailable value instead of a
zero count. The overview displays a short warning; `d` or the visible Details
button opens the existing scrollable modal with the complete redacted error.
Closing the modal restores the previous focus. Configuration repair remains
separate from this presentation change.
