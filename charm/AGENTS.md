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
- `confirm.go` — the destructive actions: trash, delete, empty, undo, rescan.
- `modal.go` — the confirmation box, its buttons, and how it sheds lines to fit.
- `protected.go` — the paths that need the word typed out.
- `sort.go` — the two-key sort (`s` then a field) and the config default.
- `toggle.go` — the column toggles (`a`/`B`/`c`/`m`) and the `t` menu.
- `filter.go`, `fuzzy.go` — the `/` fuzzy filter and its match highlighting.
- `viewer.go` — the `v` file pager, with the binary sniff and the read cap.
- `mouse.go` — wheel-scroll and click-to-select, behind `--mouse`.
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

### Destructive actions

- **The filesystem half runs in a `tea.Cmd`; the tree half runs on the loop.**
  This is why `pkg/remove` is *not* called for deletes: it fuses the two, and they
  cannot share a goroutine. Removing a large tree takes seconds, so it must be off
  the render loop; the tree is read by `View`, so it must only be mutated on it.
  The tree half still goes through the engine's own `RemoveFile`, which is what
  carries the size and item count change up to the root.
- **`Dir.AddFile` does not restore ancestor sizes** — only `RemoveFile` walks up.
  Undo therefore calls `recomputeStats`, which is `UpdateStats` over the whole
  tree: no disk I/O, and it is the engine's own summation rather than ours.
- **`UpdateStats` is not idempotent against a used hard-link ledger.**
  `alreadyCounted` *records* every inode it is shown, so a second pass over the
  same `linkedItems` map counts every hard-linked file as zero bytes and the tree
  silently shrinks. `recomputeStats` always starts a fresh map, exactly as a scan
  does. `TestRecomputeIsStableAcrossHardLinks` guards it.
- **The modal takes every key, including `q`.** While a delete is being confirmed,
  `q` is a letter of the word being typed, not a way out of the program.
- **The destructive button never holds the focus on entry**, and on a protected
  path the focus cannot even reach it until `DELETE` is typed in full. No sequence
  of single keypresses may delete a protected path.
- **A disabled key says it is disabled.** `--no-delete` leaves the keys bound and
  reports why nothing happened; a key that silently does nothing reads as a broken
  interface. The same holds for `u` where the platform cannot restore.
- **The modal states the consequence, not just the question.** That trashing does
  not free disk space is the fact people most often do not know, so it is said
  every time.

### Modes, filter and columns

- **A mode swallows every key, including `q`.** The sort menu, the column menu and
  the `/` filter all take the next keystroke whole: while typing a filter, `q` is a
  letter, and mid-sort it is not a way out. An unknown key leaves the mode and says
  so rather than being eaten silently.
- **A mode nobody can see is a trap.** Whenever one is active the footer becomes its
  menu and the right-hand side names it (`sort by…`, `toggle column…`, `/query`).
- **Sorting is two keys** (`s` then a field), so the footer costs one hint, not
  four. The field is shown in its natural direction first — biggest, most, newest,
  names from A — because "sort by size, smallest first" is never what the keypress
  meant. Pressing the field again flips it.
- **`a` (apparent size) carries the sort.** Toggling the column re-points a
  size-sort at the shown figure, or the list would be ordered by a number no longer
  on screen. `B` (relative size) scales the bars to the largest row, measured once
  per directory in `enterDir` — finding it in `View` would walk every row per frame.
- **A column with no room says so.** On a narrow terminal `c`/`m` still flip, but
  `toggleLabel` reports that there is no width to draw them, rather than looking
  broken.

### Filter, viewer, mouse

- **The filter is a view, never a change to the tree.** `applyFilter` only rebuilds
  `m.filtered`; a delete under a filter still finds and removes the real item, from
  both lists. `m.items()` is the single accessor everything moves over. Navigating
  clears the filter — it is per-directory.
- **Match highlighting re-matches the laid-out cell**, not the original name, so the
  lit runes line up with exactly what is on screen after truncation, and the cell's
  width is unchanged. On the cursor row the highlight carries the selection
  background so the row stays one block; `width_test.go` checks both.
- **The viewer caps the read (1 MiB) and sniffs for binary (a NUL byte).** A pager
  is for looking, not for loading a database into RAM or dumping mojibake. It reuses
  the list's own windowed scrolling, so the exact-height rule holds; `q`/`esc`
  closes rather than quits.
- **The mouse is small and behind `--mouse`.** Wheel scrolls, left click selects,
  a click on the already-selected row opens it (there is no terminal double-click).
  No drag or hover — those cost the user their terminal's own text selection.

### Colour and unicode

- **`--no-color` drops colour, not attributes.** Bold, reverse and underline are the
  state cues that replace colour, so they are meant to survive it — the NO_COLOR
  spec forbids colour, not styling. `nocolor_test.go` allows them and forbids only
  colour escapes; under the Ascii profile it forbids every escape.
- **`--no-unicode` is scoped to the size bar, as in gdu** (its help says "for size
  bar"). The bar becomes `#`/`-`; the marker, the rule and the wordmark stay
  unicode, matching gdu rather than trying to be a full ASCII mode.

## Work Guidance

Not yet implemented, and each pointing at `--classic` with an explicit error
rather than failing silently: `ListDevices` (`-d`), `ReadFromStorage`
(`--read-from-storage`). The disks/top-files/help screens land in later slices. The
footer advertises only bindings that exist — do not list a key before it works, and
`u` appears only when there is something to undo.

Keys: `↑↓`/`jk` move, `→`/`enter` open, `←`/`h` back, `/` fuzzy filter, `s` sort
menu, `t` column menu (or direct `a`/`B`/`c`/`m`), `v` view file, `d` trash,
`D` delete permanently, `e` empty a file, `u` undo the last trash, `r` rescan.

## Verification

    go test ./charm/...
    go test ./charm/ -bench=. -benchtime=200x -run=XXX

The suite covers navigation, the never-panic rule across seven terminal sizes
(including 0×0 and 1×1), windowing under a 5000-row listing, exact frame height on
every screen across 96 size combinations, exact row and bar widths (including
filter highlighting) under a forced truecolor profile, the two-key sort and column
toggles, the fuzzy matcher, the viewer's binary sniff and read cap, mouse
scroll/click hit-testing, the colour/unicode audit across profiles, and a full
Bubble Tea program run driven headlessly with injected input and output.

`View()` costs ~0.28 ms and must stay flat as directory size grows — that number
being identical at 100 and 10,000 rows is the standing proof the list is windowed.

## Child DOX Index

None.
