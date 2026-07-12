# Claude Code task — `cdu`: a Charm-themed fork of `gdu`

## Context

I'm building **`cdu`** ("charm disk usage") — a fork of [`dundee/gdu`](https://github.com/dundee/gdu)
(MIT licensed, written in Go) that gives its interactive TUI a completely new look built on the
**Charm** stack (Bubble Tea + Lipgloss + Bubbles). The name follows the `du` family convention:
ncdu = *ncurses* du, gdu = *go* du, **cdu = *charm* du**. The existing disk-scanning engine is good
and fast — I want to **keep the analyzer core untouched** and only replace the presentation layer.

A second goal is that `cdu` should be able to **track upstream `gdu` releases** over time, ideally
with automation, so engine improvements upstream can flow into `cdu` without a full manual rewrite.
See the "Staying in sync with upstream gdu" section — investigate its feasibility early, because the
answer shapes the whole architecture.

The current interactive UI uses `rivo/tview` (on top of `gdamore/tcell`). The analysis logic
lives in a separate package from the UI, so this should be a UI-layer swap, not a rewrite.

## Ground rules

1. **Explore before touching anything.** Map the repo first. Identify the boundary between the
   analyzer/core packages (the disk walker, the file tree model, sorting, deletion) and the
   `tview` UI package. Report that boundary back to me before writing UI code.
2. **Charm UI is the default; keep the classic mode as a fallback.** In `cdu`, launching with no
   flag opens the new Charm interface. Keep gdu's original `tview` interactive UI available behind
   `--classic` (and keep non-interactive and export modes byte-for-byte identical to upstream).
   Retaining the classic path matters for side-by-side comparison **and** for upstream syncing —
   see the sync section.
3. **Preserve the MIT license.** Keep `LICENSE.md` and Daniel Milde's copyright notice intact.
   Add a `NOTICE`/README note stating this is a fork of `dundee/gdu` and that the new UI uses
   the Charm libraries (also MIT). Do not imply it's the official gdu.
4. **Reuse, don't reimplement.** The new UI should call the existing analyzer, tree structure,
   sorting, and delete/empty operations. No duplicated scanning logic.
5. **Feature parity is the target**, in this order: navigation (up/down, enter to descend, back
   to parent) → size bars + percentages → delete/empty with confirmation → sorting modes →
   ignore patterns / apparent-size flags → color/config options → help modal.
6. **The design below is the default theme, not a hardcoded one.** Everything about the look —
   colors, the gradient endpoints, icons, border style — must be a *theme* that ships as the
   default but is fully overridable per user via config. See the "Theming / config" section.
7. **Commits must not credit the tool.** Author every commit as me (the repo owner). Do **not** add
   `Co-Authored-By: Claude`, `Generated with Claude Code`, or any similar trailer, footer, or
   signature to commit messages, PR descriptions, or generated files. Keep commit messages plain and
   about the change itself.
8. **Match gdu's platform support exactly.** `cdu` must build and run on the same OS/architecture
   targets as upstream gdu — see the "Platforms & build targets" section.

## Non-goals

To keep scope tight, `cdu` is **not**:
- a rewrite of the analyzer/engine — it reuses gdu's;
- a GUI or web app — it's a terminal UI (any web/GUI experiments stay in `poc/`);
- a new feature superset of gdu — the aim is parity + the Charm UI + theming + install/update polish,
  not inventing disk-analysis features gdu doesn't have;
- a general TUI framework or reusable widget library;
- a place for cloud sync, accounts, or telemetry of any kind.
If a change doesn't serve the reskin, parity, or the release/versioning machinery, raise it with me
before building it rather than expanding scope.

## Stack

- `github.com/charmbracelet/bubbletea` — the Elm-architecture runtime (Model / Update / View).
- `github.com/charmbracelet/lipgloss` — styling: rounded borders, colors, layout.
- `github.com/charmbracelet/bubbles` — use `viewport` for the scrollable list (**important for
  performance**, see below), `help` for key hints, `spinner` for the scan indicator.

### Two technical translations you must get right

- **The list must be virtualized.** Bubble Tea re-renders the whole `View()` string every update.
  A directory can hold thousands of entries, and gdu's selling point is speed — so render only the
  visible rows via `bubbles/viewport`, never the whole slice. This is the main performance risk;
  treat it as a first-class requirement, not an afterthought.
- **Gradient bars are per-cell coloring, not CSS gradients.** A terminal has no gradient fill. To
  get the pink→purple bar in the design, interpolate between the two hex endpoints across the bar's
  width and color each cell (each `█`/block rune) with its own `lipgloss.Color`. Build a small
  helper that takes `(fraction float64, width int)` and returns the styled string. Use 24-bit
  truecolor; let Lipgloss degrade gracefully on lesser terminals.

Async scanning maps cleanly onto Bubble Tea: run the analyzer in a `tea.Cmd`, feed progress and
completion back as `tea.Msg`. Use that pattern rather than blocking.

### Responsive layout & resize — build this in from day one

The UI must handle terminal resizing gracefully **from the first commit**, not as a later polish pass.
Retrofitting responsiveness means redoing every layout calculation, so bake it into the architecture:

- **Single source of truth for size.** Bubble Tea delivers a `tea.WindowSizeMsg` on startup and on every
  resize. Store the current `width`/`height` in the model and **recompute all layout from it in `View()`**.
  Never hardcode a width, a column position, or a bar length — derive every one of them from the current
  terminal size.
- **Reflow, don't smear.** On resize: recompute column widths, the per-row gradient bar length, the
  header/footer width, and the `viewport` height (so the virtualized list shows the right number of rows).
  Long names and breadcrumb paths truncate with an ellipsis (middle-truncate paths so both ends stay
  readable), never wrap into broken rows.
- **Graceful degradation at narrow widths.** Define breakpoints: below a threshold, drop the least
  essential elements first (e.g. hide the per-row bar, then the percentage column, then shorten the
  header) so the core name+size stays usable on a narrow pane. Lipgloss should measure with proper
  wide-rune width, not byte length.
- **Never panic on tiny terminals.** Handle degenerate sizes (height smaller than header+footer, width
  of a few columns) by clamping to a minimal safe layout instead of crashing or producing negative sizes.
- **Test it explicitly.** Verify at several sizes including very small, during a live scan (the spinner/
  progress line must reflow too), inside `tmux` splits, and on rapid continuous resize.

---

## DESIGN SPEC

This is the exact look to reproduce. It's a dark, purple-leaning "Charm" identity — hot pink is
the signature accent, used sparingly on the selection and the wordmark.

**Visual reference:** the accompanying HTML mock files show this design rendered — `cdu-charm-mock.html`
(the main interactive browser: navigation, size bars, selection, delete modal, scan animation) and the
four feature screens `cdu-1-disks.html`, `cdu-2-largest-files.html`, `cdu-3-markers.html`,
`cdu-4-help.html`. They are the visual source of truth; the tokens and rules below are the spec to
implement in the terminal. (The mocks are HTML for previewing only — translate them to Bubble Tea /
Lipgloss, don't embed a browser.)

### Color tokens (24-bit hex)

| Role                     | Name    | Hex       |
|--------------------------|---------|-----------|
| Background (outermost)   | bg      | `#17131f` (deep aubergine, **not** pure black) |
| Terminal body            | term    | `#1d1729` |
| Inner rounded panels     | panel   | `#241c34` |
| Borders / dividers       | edge    | `#3a2f52` |
| **Signature accent**     | pink    | `#ff5fd1` |
| Danger / hot pink        | hot     | `#ff2fb3` |
| Secondary accent         | purple  | `#8b6dff` |
| Body text                | lav     | `#cfc6ef` |
| Muted text               | dim     | `#7d739e` |
| Size values              | mint    | `#4ff0c0` |
| Warning (optional)       | amber   | `#ffcc66` |

Bars and the wordmark use a **pink→purple** ramp (`#ff5fd1` → `#8b6dff`).

### Typography

It's a TUI, so it's monospace throughout — but treat the wordmark and the size column as the
typographic accents. Sizes are the mint color, right-aligned, tabular. Directory names slightly
brighter than file names.

### Layout (top to bottom)

```
┌───────────────────────────────────────────────────────────┐   ← window / term frame
│  gdu ✦   ( charm edition )              at ~/Developer/     │   ← header panel (rounded)
│  Macintosh HD  ▰▰▰▰▰▰▰▰▰▱▱▱▱▱▱  627 GB / 994 GB             │   ← disk usage bar (gradient)
├───────────────────────────────────────────────────────────┤
│  ▸   41 GB   node_modules/                           47%   │   ← rows: icon | size | name | pct
│      ▰▰▰▰▰▰▰▰▰▰▰▰▰▰▰▰▰▰▰▰▰▱▱▱▱▱▱▱▱▱▱▱▱▱▱▱▱▱▱▱▱▱▱▱▱          │     gradient bar under each row
│ ▸   26 GB   DerivedData/                             30%   │   ← SELECTED row (pink glow)
│      ▰▰▰▰▰▰▰▰▰▰▰▰▰▰▰▰▰▰▰▱▱▱▱▱▱▱▱▱▱▱▱▱▱▱▱▱▱▱▱▱▱▱▱▱▱          │
│  ▸   12 GB   go/                                     14%   │
│  ·    6 MB   .zsh_history                             0%   │   ← file: dim dot icon
├───────────────────────────────────────────────────────────┤
│  [↑↓] move  [→] open  [←] back  [d] delete    sorted by size│   ← footer key hints
└───────────────────────────────────────────────────────────┘
```

**Header panel** — rounded Lipgloss border (`RoundedBorder`), faint purple-tinted background.
Contains:
- Wordmark `gdu ✦` rendered with the pink→purple ramp, bold, letter-spaced.
- A pill `charm edition` — purple text inside a rounded purple border.
- Breadcrumb on the right: `at ~/current/path/` with the path in mint.
- A disk-usage line: label `Macintosh HD`, a full-width gradient bar, then `627 GB / 994 GB`
  with the used figure in pink.

**Scan indicator** (transient, shown only while walking) — a `spinner` cycling `◐ ◓ ◑ ◒`,
text `walking directories · NN%`, with a blinking pink block cursor. Hidden once the scan
completes. Respect a reduced-motion / `--no-progress` path that skips straight to the result.

**List rows** — a 4-column grid: `icon | size | name | pct`, with a thin gradient bar rendered
on the line beneath each row, spanning from the size column to the right edge. The bar width is
the entry's size relative to the **largest sibling** in the current directory (so the top item is
always full-width); the `pct` figure is the entry's share of the **parent directory** total.
- Directory icon `▸` in pink, name brighter.
- File icon `·` in dim, name in body color.
- Sizes: mint, right-aligned, human-readable (`GB`/`MB`).

**Selection (the signature element)** — the current row gets:
- background fill using a pink→purple low-alpha tint,
- a bright **pink** left/full border,
- bold white name.
Terminals have no box-shadow, so translate the mock's "glow" into a bright rounded border + bold
+ filled background. Spend the visual boldness here and keep everything else quiet.

**Footer** — key hints as little "keycap" pills: the key label in pink on a raised dark pill
(`[↑↓] move  [→] open  [←] back  [d] delete`), and on the right the current sort state
(`sorted by size · desc`). Use `bubbles/help` if convenient.

**Delete confirmation modal** — a centered rounded box with a **pink** border, title
"Delete this item?", body naming the item and its size (`node_modules/ (41 GB)` with the name in
pink), and two buttons: `Cancel` (quiet) and `Delete` (hot-pink danger fill). `Esc` cancels,
`Enter` confirms. Wire it to the existing delete/empty operation from the core.

### Interactions / keybindings

| Keys                          | Action                          |
|-------------------------------|---------------------------------|
| `↑` / `k`, `↓` / `j`          | move selection                  |
| `→` / `l` / `Enter`           | descend into highlighted dir    |
| `←` / `h` / `Backspace`       | go to parent                    |
| `d`                           | delete selected (confirm modal) |
| `e`                           | empty selected directory        |
| `s` / `n` / `c`               | sort by size / name / item count|
| `?`                           | help modal                      |
| `q` / `Ctrl-C`                | quit                            |

Default sort: size, descending.

---

## Deletion safety

A prettier, faster-to-navigate UI makes accidental deletion *easier* — the quick `d` on a glowing
selected row is exactly where fat-finger mistakes happen. Treat destructive actions as a first-class
safety concern, not just a modal:

- **Honor the guards that exist.** Respect gdu's `--no-delete` and `--no-view-file` fully — when set,
  the keys are inert and the UI shows they're disabled.
- **Make the target unmistakable.** The confirm modal shows the full path and size, and for a directory,
  the item count it will remove. The destructive button is never the default focus for large/dangerous
  targets.
- **Protected-path guard.** Deleting a filesystem root, `$HOME`, or a volume mount point requires an
  extra, explicit confirmation (type-to-confirm or a second step), not a single keypress.
- **Prefer recoverable deletes.** Offer a "move to trash" mode (OS trash where available) as an option,
  and consider a short in-session undo for the last delete. Deletion still runs through the existing
  core operation — don't reimplement removal.
- **Report failures honestly.** Permission errors or partial deletes surface a clear message in the
  UI's own voice; never silently swallow them.

## Parity details easy to miss

Small things gdu users will expect — don't drop them in the reskin:

- **Mouse support.** gdu has `--mouse`; Bubble Tea supports mouse — wire clickable rows and wheel
  scroll, and keep `--no-mouse` working.
- **`NO_COLOR`.** Respect the `NO_COLOR` environment variable (standard) in addition to `--no-color`.
- **Theme discovery.** Add `cdu themes` to list/preview the 5 bundled themes, and `cdu --write-config`
  (gdu already has this) to dump the current effective config including the theme block.
- **Unicode fallback.** Keep gdu's `--no-unicode` path so size bars/icons degrade to ASCII.
- **Saved-scan browsing.** gdu can export a scan to JSON (`-o`) and re-open it (`-f`), so a scan taken
  on a remote/offline machine can be browsed locally. Make the Charm UI able to open a saved tree
  (`-f`), not just live scans — it's a genuine workflow, not just a background feature. Keep export
  (`-o`) working too.
- **Man page.** gdu ships a man page (`gdu.1.md` → `man gdu`). Maintain a `cdu.1.md` on the same model,
  but updated for `cdu`: the new default (Charm UI), the added flags/subcommands (`--classic`, `--theme`,
  `cdu update`, `cdu themes`, `cdu --write-config`), the 5 bundled themes and the `theme:` config block,
  and the interactive keymap. Generate the roff man page from it during build and include it in the
  release artifacts and packaging so `man cdu` works after install (this is also expected by Homebrew
  and distro packagers).

## Theming / config

The spec above defines the **default theme** (call it `charm`). It must not be hardcoded into the
render logic — factor every visual token into a `Theme` struct with sensible defaults, and let
users override any subset of it.

- **Extend gdu's existing config, don't invent a new one.** gdu already reads a YAML file at
  `$HOME/.config/gdu/gdu.yaml` (and `$HOME/.gdu.yaml`). Add a `theme:` block there rather than a
  separate file, so existing users keep one config.
- **Partial overrides + fallback.** A user setting only `theme.accent` must keep every other token
  at the default. Missing/invalid values fall back to the default theme (and to a safe basic-color
  variant on non-truecolor terminals) — never crash on a bad hex string; warn and continue.
- **Ship 5 bundled themes**, selectable with one line (`theme.preset: charm`), so people don't have
  to hand-pick colors. All five ship inside the binary and work immediately after install. A
  `--theme <name>` CLI flag overrides the config preset for a single run. The set:
  1. `charm` — **default**; the dark aubergine + pink→purple spec above.
  2. `midnight` — cool dark; deep blue/cyan accents, calmer than charm.
  3. `daylight` — light background theme for bright terminals.
  4. `ember` — warm dark; amber/orange/red accents.
  5. `mono` — high-contrast greyscale that reads without relying on hue (accessibility / colorblind-
     safe), and doubles as the safe fallback on non-truecolor terminals.
- **Precedence:** built-in default ← `theme.preset` ← individual `theme.*` overrides ← `--theme`
  flag / other CLI flags. Existing gdu flags like `--no-color` still win and force the mono path.

Suggested config shape (map the token table above onto these keys):

```yaml
theme:
  preset: charm            # charm | midnight | daylight | ember | mono  (or "custom")
  background: "#17131f"
  panel:      "#241c34"
  border:     "#3a2f52"
  accent:     "#ff5fd1"    # the signature / selection color
  accent-alt: "#8b6dff"    # second gradient endpoint
  text:       "#cfc6ef"
  muted:      "#7d739e"
  size:       "#4ff0c0"    # size column
  danger:     "#ff2fb3"
  bar:
    from: "#ff5fd1"        # gradient bar start
    to:   "#8b6dff"        # gradient bar end
    style: gradient        # gradient | solid
  border-style: rounded    # rounded | normal | thick | hidden
  icons:
    dir:  "▸"
    file: "·"
```

Document the full key list and the five bundled themes in the README, with a short "how to make your
own theme" example.

## Config location & privacy

- **Decide the config path deliberately and document it.** Recommended: `cdu` uses its **own** config
  directory following platform conventions — `$XDG_CONFIG_HOME/cdu/` (falling back to `~/.config/cdu/`)
  on Linux, the equivalent on macOS, and `%AppData%\cdu\` on Windows — rather than silently writing into
  gdu's. For a smooth migration, optionally **read gdu's existing config** if no `cdu` config exists, and
  offer to import it. State the resolved path in `--help` and `cdu --write-config`.
- **No telemetry, and say so.** `cdu` collects and sends nothing. The only network access is the explicit
  update check (`cdu update`) and downloads it initiates. Put a plain "no telemetry / no phone-home"
  line in the README so users can trust it.

## Accessibility & localization

- **Screen-reader mode.** Provide a reduced, screen-reader-friendly output mode (dust has `-R` for this):
  linear, low-decoration, no reliance on bars/color to convey size. The `mono` theme is a step toward
  this; the SR mode goes further.
- **Don't encode meaning in color alone.** Every state shown with color (selection, danger, permission
  denied, hard-link) must also have a non-color cue (marker, label, position) so it survives `mono`,
  `NO_COLOR`, and colorblindness.
- **Units and localization.** Keep gdu's SI-vs-binary size options (`--si`, `--no-prefix`) and its
  locale-aware help where present; don't hardcode US-only formatting. Keep help strings centralized so
  translation is possible later.

## Install & self-update

`cdu` needs a frictionless install and a built-in updater, matching the convenience users expect from
tools like gdu/dust.

- **One-line install.** Provide an `install.sh` served from the repo so users can run a single
  `curl -fsSL https://<host>/install.sh | sh`. The script detects OS + arch, downloads the matching
  release artifact for the current platform, **verifies its checksum** against the published
  `sha256sums.txt`, and installs the binary into a sensible location on `PATH` (with a Windows
  PowerShell equivalent). Keep parity with the platform matrix above. Also keep the manual
  download-a-binary path documented.
- **Self-update built in.** Add a `cdu update` subcommand (and a `--version` that reports the current
  build). `cdu update` checks the latest GitHub release, and if newer, downloads the correct
  per-platform binary, verifies its checksum/signature, and atomically replaces the running executable
  in place. Use a maintained Go self-update library rather than hand-rolling the binary swap; cite
  which one and why.
- **Respect managed installs.** If `cdu` was installed by a package manager (Homebrew, Snap, conda,
  winget, distro repo), `cdu update` must detect that and **defer to the package manager** with a clear
  message instead of clobbering a managed binary. Provide a way to opt out of update checks entirely,
  and never phone home silently — only check when the user runs `update` (or explicitly opts into a
  background check).
- **Tie versioning to the engine.** The self-updater and `--version` should surface the embedded gdu
  engine version too (the `cdu vX.Y.Z +gduA.B.C` scheme from the sync section), so users know what
  changed on update.
- **Supply-chain security.** A `curl | sh` installer plus a self-updating binary is a real attack
  surface, so checksums alone aren't enough. **Sign releases** (cosign/sigstore keyless, or GPG) and
  publish a signature + an SBOM alongside each artifact. The installer and `cdu update` must **verify the
  signature**, not just the SHA-256, before replacing anything, and refuse on mismatch. Document how a
  user can verify a download manually.

## Platforms & build targets

`cdu` must support the **same OS/arch targets as upstream gdu** — don't narrow the matrix. gdu is pure
Go and ships broadly. Rather than hardcode a list that can drift, **read gdu's own release config**
(its `.goreleaser.yml` / GoReleaser setup and CI release workflow, plus its packaging: Homebrew, Snap,
conda-forge, COPR/RPM, winget) and mirror that target matrix. As of the current upstream release the
matrix includes, at minimum:

- **Linux:** amd64, arm64 (aarch64), arm, 386, ppc64le — including musl (static) builds.
- **macOS:** amd64 (Intel) and arm64 (Apple Silicon).
- **Windows:** amd64.

Requirements:
- The Charm UI (Bubble Tea / Lipgloss / Bubbles) is cross-platform Go, so it must **not drop any
  target** gdu supported. Verify the whole matrix still cross-compiles after the UI swap.
- Terminal capabilities vary — truecolor isn't guaranteed everywhere (notably older Windows consoles).
  The gradient/theme must **degrade gracefully** (Lipgloss color profile detection) rather than break;
  gdu's existing `--no-color` / no-unicode paths must keep working on every platform.
- Set up `cdu`'s release pipeline (GoReleaser or equivalent) to produce the same artifact set per
  target as gdu, so packaging parity is possible later.

## Staying in sync with upstream gdu

This is a real requirement, not a nice-to-have, and I want you to **investigate feasibility before
committing to an architecture** — report what you find and recommend an approach. The goal: when
`dundee/gdu` ships a new release with engine improvements, `cdu` can pick those up with minimal (ideally
automated) work.

The decisive question, answer it first:

> **Does gdu expose its analyzer as an importable, reasonably stable public Go API, or is the
> useful logic in `internal/` / tied to the `tview` UI?**

Look at the actual package layout (`pkg/`, `cmd/`, `internal/`, `stdout/`, `tui/`, etc.) and the
module path (`github.com/dundee/gdu/v5/...`). Then evaluate the two strategies below and tell me
which is viable, with evidence:

**Strategy A — depend on gdu as a Go module (preferred if possible).**
`cdu` is a *thin* project that imports gdu's analyzer/tree packages as a normal module dependency
and only adds the Charm UI + theming + `cmd`. Following upstream then means bumping one version in
`go.mod`. This is the automation-friendly path: `dependabot`/`renovate` can open a version-bump PR
automatically on each gdu release, CI runs the test suite, and if green you cut a `cdu` release.
Verify: are the packages you need actually exported (not `internal/`)? Is the public surface stable
across recent minor versions? Does deletion/scanning work through the exported API alone?

**Strategy B — source fork with periodic merges (fallback if A is blocked by `internal/`).**
`cdu` is a full fork of the tree; you track upstream via a git remote and periodically
`merge`/`rebase` new tags in. Because you *replaced* the default UI but *kept* `--classic`, most
merge conflicts will land in the classic UI layer and `cmd` wiring — manageable but not free.
Describe the branching model (e.g. keep a clean `upstream` tracking branch, a `charm` branch with
your UI, merge tags into `charm`), and where conflicts are likely to concentrate.

**Automation to propose (for whichever strategy wins):**
- A scheduled GitHub Actions workflow that watches upstream for new releases (releases API, or
  `dependabot`/`renovate` if Strategy A).
- On a new upstream version: open a PR (bump module version, or merge the tag), run build + full
  test suite + a smoke test of both `--classic` and the Charm UI.
- **Gate releases on green CI** — do not auto-publish on red. "Fully automatic" is only realistic for
  compatible updates; an upstream refactor of the analyzer API can break the build and will need a
  human. Make that failure mode explicit and safe (PR stays open, no release cut).
- A version scheme that records the upstream version it's built on, e.g. `cdu vX.Y.Z` with build
  metadata `+gdu5.36.1`, so it's always clear which gdu engine is inside.

**Deliverable for this section:** a short written recommendation (A vs B, with the evidence from the
package layout), plus a proposed `renovate`/`dependabot` config or a release-watcher workflow file
for the chosen strategy. Don't build the whole pipeline yet — validate the approach and scaffold it.

## Quality gates: tests, speed, demo

- **Snapshot tests for the UI.** Use Bubble Tea's `teatest` to golden-file the rendered `View()` at
  several fixed terminal sizes and for each of the 5 themes. This turns the responsive-layout and
  theming requirements into things CI actually verifies, not just promises. Include a tiny-terminal
  case so the "never panic" rule is enforced.
- **Speed guard.** gdu's whole identity is speed. Adapt gdu's existing benchmark suite and add a CI
  check that the Charm UI doesn't regress scan/analyze throughput beyond a small budget — the render
  layer must stay off the hot path (this is the companion to the list-virtualization requirement).
- **Reproducible demo.** Generate the README demo with Charm's `vhs` (a `.tape` script in the repo) so
  the GIF can be regenerated in CI and stays in sync with the real UI, rather than a hand-recorded clip.

## Repository setup & branching workflow

**Claude Code should help set up the whole GitHub repo to a healthy baseline**, not just write code.

- **Protect `main` from the start.** As part of repo creation, enable branch protection on `main`
  immediately (via `gh api` branch-protection endpoints, once the initial commit is pushed), with:
  require a PR before merging, require status checks to pass and branches to be up to date, dismiss
  stale approvals, require linear history, and block force-pushes and deletions. Where a rule needs
  something not yet present (e.g. named required checks before CI exists), wire it as soon as the CI
  workflow lands. If any protection step can't be applied with the available token, put it in the
  admin checklist below.
- **Full repo scaffolding.** Initialize/verify: `.gitignore` (Go), the preserved `LICENSE.md` + `NOTICE`,
  `README`, `CONTRIBUTING`, issue/PR templates, `.editorconfig`, a `golangci-lint` config, CI workflows
  (build + test + lint + release via GoReleaser), the `renovate`/`dependabot` config from the sync
  section, a `CHANGELOG` (Conventional-Commits-driven), and repo labels. Settings that need admin rights
  (any remaining protection, required checks, secrets) — apply what's possible via the `gh` CLI and
  **document the rest as a short checklist** for me to click through.
- **Branch per change.** Every non-trivial change lands on its own branch, never committed straight to
  `main`, so anything is easy to revert and the history is traceable. Use conventional prefixes:
  `feat/`, `fix/`, `chore/`, `docs/`, `refactor/`, `test/`, `perf/`, `ci/`, `build/` + a short slug
  (e.g. `feat/gradient-bars`, `fix/resize-panic`). One reviewable PR per branch, kept small — this maps
  directly onto the vertical slices below.
- **Conventional Commits.** Message style `type: summary` (`feat:`, `fix:`, …) so the changelog can be
  generated and each PR reverts cleanly as a unit. Protect `main`: PR required, CI green before merge.

- **`docs/` for all project documents.** Every document produced for the project — design notes, the
  theming guide, architecture/ADR notes, the upstream-sync recommendation, etc. — goes in a top-level
  `docs/` folder. (Standard root files stay at root: `README`, `LICENSE.md`, `NOTICE`, and the man-page
  source.)
- **`poc/` for throwaway experiments, isolated from production.** Any proof-of-concept or spike goes in
  a top-level `poc/` directory and must **not mix with production code**: it's excluded from the
  production build (kept out of `cmd`/library packages, e.g. via build tags or a separate module),
  never imported by production packages, not shipped in release artifacts, and removable without
  affecting the build. Keep production clean of experimental code.

## Versioning

Follow a gdu-like convention: **SemVer with a `v` prefix** (gdu tags `v5.36.1`, `v5.34.0`, … and
carries its Go-module major in the path, `.../gdu/v5`). Concretely for `cdu`:

- **`cdu` versions its own changes**, independent of gdu's numbers. Start in `v0.x` during initial
  development, cut `v1.0.0` when the UI is stable. `MAJOR.MINOR.PATCH` reflects *cdu's* changes, and
  maps to Conventional Commits: `fix:` → patch, `feat:` → minor, a breaking change → major.
- **Record the embedded gdu engine as SemVer build metadata**, e.g. `cdu v0.3.1+gdu5.36.1` (the `+…`
  suffix per the SemVer spec doesn't affect version precedence). `cdu --version` and the self-updater
  surface both numbers so it's always clear which engine is inside.
- **Engine-only updates still cut a cdu release.** If the sync automation bumps just the gdu dependency
  and nothing else, that's a normal `cdu` patch/minor whose only change is the updated `+gdu…` metadata —
  the shipped binary changed, so it gets its own tag and changelog entry.
- **Go module path.** As a new module (`github.com/<you>/cdu`), `v0`/`v1` need no path suffix; only add a
  `/v2`+ suffix if `cdu` itself ever reaches major 2. Don't inherit gdu's `/v5`.
- **Tags drive releases.** Pushing a `vX.Y.Z` tag triggers the GoReleaser pipeline; the changelog is
  generated from Conventional Commits since the previous tag.

## Working process — ship in vertical slices

This brief is large; don't attempt it in one enormous commit. Work in **reviewable, vertical slices**,
each a small PR that builds and runs, and **stop at milestones for my review** rather than running to the
end. A sensible order:

1. Repo exploration + the two decisions (core/UI boundary; importable-API vs internal) — report back,
   no UI yet.
2. Minimal walking skeleton: scan → virtualized list → navigate (enter/back), size + selection, resize
   handling wired from the start.
3. Gradient bars + header/footer + scan spinner.
4. Deletion (with the safety section above) + empty.
5. Sorting, item-count/apparent-size toggles, mouse, `NO_COLOR`, unicode fallback.
6. Theme system + the 5 bundled themes + config loading.
7. Install script + self-update.
8. Snapshot tests, benchmarks, vhs demo, README.
9. Upstream-sync recommendation + scaffolded automation.

Pause after step 1, and after the skeleton in step 2, for feedback before going wide.

## Deliverables

1. A short written summary of the analyzer/UI boundary you found (before building).
2. The new Charm UI package, behind a flag, calling the existing core.
3. The gradient-bar helper and the viewport-virtualized list.
4. A `Theme` struct with the `charm` default plus the 4 other bundled themes (`midnight`, `daylight`,
   `ember`, `mono`), YAML config loading with partial override + fallback, and the `--theme` flag.
5. An `install.sh` one-line installer (+ Windows equivalent) with checksum verification, and a
   `cdu update` self-update subcommand that respects package-manager-managed installs.
6. Full repo scaffolding (CI, lint, GoReleaser, templates, changelog, renovate/dependabot) plus a
   short checklist of any admin-only settings for me to apply.
7. Updated README section: what the fork is, the flags, a screenshot placeholder, the theming
   guide (key list + 5 themes + custom example), install + self-update instructions, and the
   fork/attribution + license note.
8. A note on anything from the design that a terminal genuinely can't do, and how you translated it.

## Acceptance criteria

- `cdu` with no flag launches the new Charm interface, scans, navigates, and deletes correctly.
- `cdu --classic` reproduces gdu's original `tview` UI; non-interactive and export modes match upstream.
- Scrolling a directory with thousands of entries stays smooth (virtualized).
- The pink→purple gradient bars render per-cell on a truecolor terminal and degrade without crashing.
- Selection, header, footer, scan spinner, and delete modal match the design spec.
- A clear written recommendation exists for upstream syncing (Strategy A vs B) with a scaffolded
  automation config for the chosen path.
- `cdu` cross-compiles for the full gdu target matrix (Linux amd64/arm64/arm/386/ppc64le incl. musl,
  macOS Intel + Apple Silicon, Windows amd64), and the theme degrades gracefully where truecolor is absent.
- The layout reflows cleanly on terminal resize at any size (including very small and during a live
  scan), truncating rather than breaking, and never panics on tiny terminals.
- All 5 themes (`charm`, `midnight`, `daylight`, `ember`, `mono`) ship in the binary and switch via
  config or `--theme`.
- `curl -fsSL .../install.sh | sh` installs a working, checksum-verified binary; `cdu update` upgrades
  in place and defers to the package manager when the install is managed.
- Destructive actions are guarded: `--no-delete` is honored, the confirm modal shows full path/size,
  and protected paths (root, `$HOME`, mount points) need extra confirmation.
- `teatest` snapshot tests cover multiple sizes and all 5 themes; benchmarks show no meaningful scan
  regression vs upstream; the README demo is generated by `vhs`.
- Mouse, `NO_COLOR`, `--no-unicode`, `cdu themes`, and `cdu --write-config` all work.
- A `cdu.1.md` man page is maintained and `man cdu` works after install, covering the new flags and themes.
- Changes land via prefixed branches (`feat/`, `fix/`, …) and PRs, not direct commits to `main`; all
  project docs live under `docs/`; any PoC lives in `poc/` and is fully isolated from production code.
- `main` is branch-protected from repo creation (PR required, checks must pass, no force-push/delete).
- Versioning follows SemVer `vX.Y.Z` with the embedded gdu engine recorded as `+gduA.B.C` build metadata,
  surfaced by `cdu --version`.
- Releases are signed (cosign/GPG) with an SBOM; the installer and `cdu update` verify the signature, not
  just the checksum.
- The Charm UI can open a saved scan via `-f`, and `-o` export still works.
- Config lives in `cdu`'s own platform-appropriate path, no telemetry is sent, and a screen-reader mode
  plus non-color state cues are available.
- No commit, PR, or file credits Claude / Claude Code as an author or co-author.

Start by (1) exploring the repo and reporting the core/UI boundary, and (2) answering the "importable
public API vs internal/" question that decides the sync strategy. Report both back before building the
UI, and ask me before any change that would alter the classic (non-Charm) behavior.
