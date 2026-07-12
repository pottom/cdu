# What the terminal can't do, and what we did instead

The design mocks in `docs/design/` are HTML. Some of what they express has no
terminal equivalent. This records each gap and the translation chosen, so the
decisions are reviewable rather than buried in the renderer.

Entries are added as each slice lands.

## Selection "glow" → marker + fill + bold

The mock gives the selected row a pink box-shadow. Terminals have no shadow, no
blur, and no alpha compositing.

Translated to: a filled background, a bold white name, and a bright `▌` marker in
the left gutter. The marker matters as much as the colour — it is what makes the
selection legible under `--no-color`, `NO_COLOR`, the `mono` theme, and to a
colourblind reader. The same rule applies to every other state: read-error and
hard-linked rows carry a glyph, not just a hue.

## `bubbles/viewport` → our own windowed list

The brief asked for the list to be virtualized "via `bubbles/viewport`". Those two
things turn out to be in tension.

`viewport.SetContent` takes the entire content as a single string and only windows
the *display*. For a directory with tens of thousands of entries, that means
building — and, with per-cell gradient colouring, ANSI-escaping — every row on
every frame. That is precisely the O(n)-per-frame cost that virtualization exists
to remove, and gdu's whole identity is speed.

So `charm/view.go` renders `rows[offset:offset+visibleRows]` directly and does its
own cursor/offset math. Only what is on screen is ever turned into a string. The
intent of the requirement is met; the named component is not the way to meet it.

`bubbles` is still used where it earns its keep — `spinner` for the scan
indicator, and `help` for the key hints later.

## Scan cancellation → process exit

Not a terminal limitation, but an engine one, and it shows up in the UI.

gdu's analyzer exposes no cancellation: no `context.Context`, no `Stop()`. Pressing
`q` during a scan therefore tears down the Bubble Tea program and exits, letting
the walk goroutine die with the process — which is effectively what gdu does too.
Fixing this properly means adding a context to `pkg/analyze`, which is
upstream-owned; the right venue is a PR to gdu, not a local edit that would put the
engine on our merge conflict surface.
