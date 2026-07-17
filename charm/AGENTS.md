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
- `disks.go`, `diskgroup.go` — `cdu -d`: the device table, grouped by disk.
- `topfiles.go` — `T`: the largest files anywhere in the scan.
- `help.go` — `?`: every key on one screen.
- `cancel.go` — `esc` during a scan.
- `duplicates.go` — `F`: the duplicate search, its screen, and the browser mark.
- `icons.go` — the icon cell: markers by default, Nerd Font glyphs behind `--icons`.
- `icons_table.go` — **generated** from exa's `icons.rs`; do not hand-edit.
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
- **Colours come from the active `theme.Theme`**, never from a literal in the
  render path — `lg()` in `style.go` is the only door they come through. Tokens
  name a role, not a hue: `Accent`, not `pink`. charm's accent happens to be
  pink, midnight's is cyan and phosphor's is green — a renderer reaching for
  `pink` would be telling the truth in exactly one theme.
- **`Selected` and `Ink` look alike and are not.** `Selected` sits on `Panel`, a
  surface; `Ink` sits on `Danger`/`Dim`, which are colours and can be light even
  in a dark theme. They are the same white in charm, which is why fusing them
  went unnoticed until a theme with a light danger colour made the delete button
  unreadable. `internal/theme/contrast_test.go` checks every pairing that
  `newStyles` actually composes — add to it when you compose a new one.
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
  it, Lipgloss falls back to plain ASCII in tests and the bug hides. `plainKeys`
  and `renderKeys` are the pattern: build both versions rune-for-rune, measure
  the plain one, print the styled one. The viewer's footer broke this rule for
  three slices and only a *second theme* exposed it — mono emits fewer escapes,
  so the same cut kept a different amount of text.
- **No line may be wider than the terminal**, on any screen, at any size:
  `TestNoLineIsWiderThanTheTerminal` checks every width from 0 to 100. An
  overflowing line is soft-wrapped by the terminal, which makes the frame taller
  than it claims and walks it down the screen — the horizontal twin of the bug
  `padLines` exists for. Every component floors its own columns (size at 10, name
  at 4), and those floors add up to more than a narrow terminal has, so each one
  gives up in turn: the margin, then the padding, then the chrome, then all but
  the one thing worth saying. Below one column `View` draws nothing at all.
  `clipTo` is the tool: it fits plain text to *exactly* a width, because
  truncation alone can come back a column short rather than split a wide rune.
- **A scan is cancelled through the analyzer's own ignore hook, and only there.**
  The analyzer takes no context and has no `Stop()`, and `pkg/analyze` is
  upstream's — editing it would put the scanning engine on the merge conflict
  surface, which is the one thing the fork strategy exists to prevent. But the
  analyzer asks *us* whether to descend into each directory, and asks before
  descending: `ui.ignoreFunc` answers "ignore it" once `ui.cancel` is set, so the
  walk skips every directory it has not opened and unwinds on its own, bounded by
  what is already open. `fileTypeFilter` does the same one level down, before the
  `stat`. `cancel_test.go` proves it stops rather than pretending to, by walking
  the same tree twice and comparing the work done.
- **A cancelled scan's tree is thrown away, never shown.** The directories the
  walk never opened are *absent* from it — not marked, not empty, absent — so
  every parent above them reports less than it holds. A disk usage tool quietly
  showing sizes that are too small is worse than one showing nothing, and the
  next key along is `d`.
- **`esc` cancels, `q` quits.** `esc` means "out of this" on every other screen,
  and a scan is a state you can want out of; `q` meaning something else on one
  screen is how a binding stops being trustworthy. Cancelling goes back to
  whatever the scan interrupted — the device list, the tree a rescan was
  refreshing, or out, when it was the only thing cdu was asked to do.
- **`rescan` keeps the old tree until a new one arrives.** It is still true, and
  it is where `esc` goes back to.
- **An analyzer is single-use until it is reset, and forgetting that is a panic.**
  `SignalGroup.Broadcast` *is* `close(ch)`, so a second `AnalyzeDir` on the same
  analyzer closes a closed channel and takes the program down. `ResetProgress`
  re-makes the channels; gdu calls it before every scan. **Every scan goes through
  `startScan`**, which is the only thing that calls it — a `scanCmd` reached any
  other way is this bug coming back. It shipped in the first slice and survived
  900 green tests, because every one of them stopped at the `tea.Cmd` instead of
  running it; `rescan_test.go` runs it.
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
- **The view is saved explicitly (`t` then `s`), never on exit.** Someone who
  turns the mtime column on to answer one question would otherwise find it on
  forever with no idea what did it. A view is a thing you try; a config is a thing
  you decide. `s` lives inside the `t` menu because that is where the settings it
  writes are — top-level `s` is already sort.
- **charm cannot write the config itself.** It cannot see `Flags` (`cmd/cdu/app`
  imports charm, not the reverse), and a writer that knew only the six fields it
  owns would silently drop the rest of the file. `WithConfigSaver` takes a
  callback; `app.saveView` folds the view into the whole struct and writes that.
  It writes cdu's own path even when a gdu config was read — the same split
  `--write-config` makes.
- **`CreateUI` reconciles a size sort with the apparent-size column** after the
  options are applied. `handleToggle` does this at runtime; without the same at
  startup, a config carrying both `show-apparent-size` and `sorting.by: size`
  opens ordered by a number the list is not showing.

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

### The other screens

- **The device list is grouped by physical disk, and the grouping is a spelled-out
  heuristic.** A flat table lied: six APFS volumes of one container each report
  the container's space, so it read as six half-terabyte disks. A header says "4
  volumes sharing one pool of space" — inferred from every device reporting the
  same total *and* free to the byte, because nothing in the mount table says it.
  `diskPatterns` lists each device-name form rather than inferring: "strip the
  trailing digits" turns `nvme0n1` into `nvme0n` and files `loop0` with `loop1`,
  and a wrong tree is invisible. `/dev/mapper` is left ungrouped — an LVM volume
  can span disks, which is the point of LVM.
- **Disk headers are not selectable.** A container has no mount point, so enter on
  one would do nothing, and a cursor you can park somewhere inert reads as broken.
- **The largest-files collect runs on the render loop**, which is the one
  deliberate exception to the tea.Cmd rule — and it is measured, not assumed: 3 ms
  at 12k items, 23 ms at 294k, 161 ms at 2.7M. The rule is about I/O that can hang
  without bound. Off-loop it would be a data race: `CollectTopFiles` reads the tree
  through `GetFiles`, which takes no lock (the engine ships `GetFilesLocked` for
  exactly this), and the render loop is the only thread allowed to mutate it.
- **The destructive keys ask `target()`, not the browser.** The largest-files list
  has no "current directory", so the item is asked where it lives. Any new screen
  with rows belongs in `target`.
- **The modal, the viewer and the help return to where they were opened from**, and
  those fields default to `screenBrowse` — the zero value of a `screen` is
  `screenScanning`, and closing onto the scan screen is not a place you can be.
- **`T` is not `--top`.** That flag forces gdu's non-interactive mode, whose output
  is byte-for-byte gdu's. The flag keeps its meaning; the screen has a key.
- **The help describes cdu's bindings, not the mock's.** `cdu-4-help.html` predates
  most of them and contradicts itself — it gives `d` as both "delete selected" and
  "list mounted disks", and `-d` is a flag. `TestHelpCoversEveryFooterKey` is the
  seam between the two places that describe keys: it cannot check the words are
  still true, but a key can never be silently undocumented. It caught one within a
  minute of being written.
- **`F` finds duplicates, and it reads files — the only feature that does.** The
  search runs off the render loop (`internal/dup`), can be cancelled with `esc`
  like a scan, and shares the scan's cancel flag. A duplicated file is marked in
  the browser with `dupMark` (`▲`) in the accent, and the whole name takes the
  accent so it stands out of a long list rather than hiding a glyph at the end of
  a truncated name. `dropDuplicate` dissolves a group once one copy is left — a
  file is a duplicate of nothing. The mark is a glyph as well as a colour, so it
  survives mono and a colourblind eye, like the `H`/`!` flags.
- **`dupMark` is a geometric triangle, not `⚠`.** runewidth measures both as one
  cell, but `⚠` is in the emoji block and a colour terminal may draw it two cells
  wide, shifting the row. Never use an emoji-range glyph in a laid-out cell.

### Colour and unicode

- **`--no-color` drops colour, not attributes.** Bold, reverse and underline are the
  state cues that replace colour, so they are meant to survive it — the NO_COLOR
  spec forbids colour, not styling. `nocolor_test.go` allows them and forbids only
  colour escapes; under the Ascii profile it forbids every escape.
- **`--no-unicode` is scoped to the size bar, as in gdu** (its help says "for size
  bar"). The bar becomes `#`/`-`; the marker, the rule and the wordmark stay
  unicode, matching gdu rather than trying to be a full ASCII mode. The icon cell
  is the exception, because `--no-unicode` and `--icons` are a contradiction and
  the flag that says "you cannot" wins.

### Icons

- **`--icons` is opt-in and stays that way.** The glyphs are Nerd Font private-use
  codepoints; a terminal without a patched font draws a row of boxes, and cdu
  cannot ask what font is loaded. On by default would break the majority to please
  the minority.
- **The icon cell is glyph *plus a space*, always exactly `iconWidth`.** This is
  what makes the feature safe: `runewidth` measures a PUA codepoint as one cell,
  but a Nerd Font's *non-Mono* variants draw it across two, and cdu has no way to
  know which is installed. The trailing space is the room the wide draw spills
  into, so the size column stays put either way. exa does the same. Never put
  content immediately after the glyph, and never make the cell one column.
- **`icons_table.go` is generated, not written.** It comes from exa's
  `src/output/icons.rs` (MIT — see NOTICE), *not* from eza's: eza is EUPL-1.2, a
  copyleft licence cdu cannot take data from. A wrong codepoint is invisible — it
  renders as some other plausible icon — so it is transcribed mechanically.
- **Go's `\u` takes exactly four hex digits.** Nine of the glyphs are plane-15
  Material Design codepoints and need `\U` with eight; the generator got this
  wrong once and produced a four-digit icon followed by a literal digit.
  `TestEveryGlyphIsOneCellAndInThePrivateUseArea` catches it.
- **Lookup order is name, then directory, then extension.** A name beats an
  extension because `Dockerfile` and `.gitignore` have none worth reading; a
  directory beats an extension because `node_modules.bak` is a directory, not a
  backup file.

## Work Guidance

Still pointing at `--classic` with an explicit error rather than failing
silently: `ReadFromStorage` (`--read-from-storage`). The footer advertises only
bindings that exist — do not list a key before it works, and `u` appears only when
there is something to undo.

Keys: `↑↓`/`jk` move, `→`/`enter` open, `←`/`h` back, `/` fuzzy filter, `s` sort
menu, `t` column menu (or direct `a`/`B`/`c`/`m`; `t` then `s` saves the view),
`v` view file, `d` trash, `D` delete permanently, `e` empty a file, `u` undo the
last trash, `r` rescan, `T` largest files, `F` find duplicates, `?` help, `esc`
back / cancel a scan.
**`help.go` is the list that has to be right** — add a binding there, or the
drift test fails.

Flags this package reads: `--no-delete`, `--no-view-file`, `--mouse`, `--icons`,
`--no-unicode`, `--no-color`, `--theme`, and the `theme:`/`sorting:` config
blocks.

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
