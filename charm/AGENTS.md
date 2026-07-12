# charm

## Purpose

cdu's default interactive interface, built on Bubble Tea, Lipgloss and Bubbles.
It sits alongside gdu's original tview interface in `tui/`, which stays reachable
via `--classic`.

## Ownership

cdu-owned. This is a new directory, so it never conflicts on an upstream merge.

- `ui.go` — `charm.UI`, the `app.UI` implementation and the entry point.
- `model.go` — the Bubble Tea model, messages, scan commands, cursor/window math.
- `view.go` — `View()`, layout, breakpoints, row rendering.
- `style.go` — the palette and the resolved Lipgloss styles.
- `util.go` — truncation, padding, formatting.

## Local Contracts

- **Embed `*common.UI`, like `tui.UI` does.** That is what hands us the analyzer
  field and gdu's ignore-pattern engine, and satisfies most of `app.UI` for free.
  Never reimplement scanning, sorting or deletion here — call the engine.
- **The list is windowed, and `bubbles/viewport` is deliberately not used for it.**
  `viewport.SetContent` takes the *entire* content as one string, so a directory
  with tens of thousands of entries would be fully rendered on every frame —
  exactly the cost virtualization exists to avoid. `viewList` renders only
  `rows[offset:offset+visibleRows]`. Do not "fix" this by adopting viewport.
- **Rows are materialised once per directory.** `fs.Item.GetFiles` returns an
  iterator; walking and sorting it every frame would put the engine on the render
  hot path. `enterDir` fills `m.rows`; invalidate it when the directory changes.
- **Every size derives from `tea.WindowSizeMsg`.** No hardcoded width, column
  position, or row count in `View()`. `visibleRows()` is always ≥ 1, so degenerate
  terminals clamp instead of producing negative sizes.
- **No colour-only meaning.** Selection also carries a `▌` marker; read errors and
  hard links carry glyphs. State must survive `--no-color`, `NO_COLOR` and the
  `mono` theme.
- **Colours come from the palette struct**, never from a literal in the render
  path. `style.go` is the seed of the theme system.
- **The analyzer cannot be cancelled** — it has no context and no `Stop()`.
  Quitting mid-scan ends the program and lets the walk die with the process. Do
  not add cancellation by editing `pkg/analyze`; that file is upstream-owned.

## Work Guidance

Not yet implemented, and each pointing at `--classic` with an explicit error
rather than failing silently: `ListDevices` (`-d`), `ReadFromStorage`
(`--read-from-storage`). Deletion, sorting keys, the gradient bars, mouse and the
help/disks/top-files screens land in later slices.

## Verification

    go test ./charm/...

The suite covers navigation, the never-panic rule across seven terminal sizes
(including 0×0 and 1×1), windowing under a 5000-row listing, and a full Bubble Tea
program run driven headlessly with injected input and output.

## Child DOX Index

None.
