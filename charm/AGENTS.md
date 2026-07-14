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
- `gradient.go` — the usage bar and its degradation across colour profiles.
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
- **The Charm UI owns the terminal exclusively.** Nothing else may attach a reader
  to it. `cmd/cdu/main.go` creates the tcell screen and tview application *only*
  for `--classic`; when both existed at once they raced Bubble Tea for stdin and
  each swallowed every other keystroke, so every key — including `q` — needed two
  presses.
- **Never measure or truncate a string that already carries styles.**
  `runewidth` counts escape bytes as visible columns, so cutting a styled row to
  the terminal width silently throws away most of its content and can leave a
  background escape unterminated. Compose rows as plain text at an exact width,
  then style. `lipgloss.Width` is escape-aware; `runewidth.StringWidth` is not.
  `width_test.go` guards this under a forced truecolor profile — without forcing
  it, Lipgloss falls back to plain ASCII in tests and the bug hides.
- **The analyzer cannot be cancelled** — it has no context and no `Stop()`.
  Quitting mid-scan ends the program and lets the walk die with the process. Do
  not add cancellation by editing `pkg/analyze`; that file is upstream-owned.
- **`View()` returns exactly `m.height` lines, with no trailing newline.** One
  line too many and the terminal scrolls on every frame. `padLines` both pads and
  clips for this reason: a list height is not always a whole number of entries,
  because an entry is two lines once the bar is drawn.
- **Entries and lines are different units.** An entry is two lines above
  `minWidthForBar` and one below it. `visibleRows()` counts entries and drives
  scrolling and paging; `visibleLines()` counts lines and drives rendering.
  Conflating them makes the list scroll by halves.
- **The bar never carries meaning the row does not also carry as text.** It is
  decoration for the percentage column, not a substitute for it — on a
  256-colour terminal it degrades to a solid fill, and without colour to plain
  characters, so anything encoded only in its gradient would be lost.
- **Anything that can block runs as a `tea.Cmd`, never inline in `Init`.** The
  walk and the mount-table read both can: `GetDevicesInfo` will hang on a stale
  network mount. The disk line simply does not appear if it cannot be resolved.
- **Benchmarks and rendering tests must force a colour profile.** The test process
  has no TTY, so Lipgloss emits no escapes and the gradient collapses to plain
  text — `BenchmarkView` measured 0.25 ms/frame while a real terminal would have
  spent 4.1 ms. See `benchTruecolor` and `withProfile`.

## Work Guidance

Not yet implemented, and each pointing at `--classic` with an explicit error
rather than failing silently: `ListDevices` (`-d`), `ReadFromStorage`
(`--read-from-storage`). Deletion, sorting keys, mouse and the help/disks/top-files
screens land in later slices. The footer advertises only bindings that exist —
do not list a key before it works.

## Verification

    go test ./charm/...
    go test ./charm/ -bench=. -benchtime=200x -run=XXX

The suite covers navigation, the never-panic rule across seven terminal sizes
(including 0×0 and 1×1), windowing under a 5000-row listing, exact frame height
on every screen across 72 size combinations, exact row and bar widths under a
forced truecolor profile, and a full Bubble Tea program run driven headlessly with
injected input and output.

`View()` costs ~0.28 ms and must stay flat as directory size grows — that number
being identical at 100 and 10,000 rows is the standing proof the list is windowed.

## Child DOX Index

None.
