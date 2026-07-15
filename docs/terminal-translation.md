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

## CSS gradient → per-cell colouring, and three ways of giving up

`.barfill` in the mock is a `linear-gradient(90deg, pink, purple)`. A terminal
cell holds one foreground colour, so a gradient can only be approximated by
colouring each cell of the bar separately.

That reads as a gradient only with 24-bit colour. On a 256-colour terminal the
interpolation quantises into visible bands, which looks like a rendering fault
rather than a design. So the bar degrades in three steps, chosen from the Lipgloss
colour profile at startup:

| Terminal | Bar |
|---|---|
| truecolor | per-cell interpolation, pink → purple, blended in Luv |
| 256 / 16 colours | solid accent fill |
| `--no-color`, `NO_COLOR`, dumb | plain characters, no colour |
| `--no-unicode` | `#` and `-` instead of `█` and `░` |

Blending happens in Luv rather than RGB: a straight RGB interpolation between
pink and purple passes through a muddy grey midpoint.

The consequence for the design is a rule, not just an implementation detail: **the
bar never carries information the row does not also carry as text.** It decorates
the percentage column rather than replacing it, because on two of the four paths
above the gradient does not exist.

The ramp is precomputed at 64 steps. Building a Lipgloss style per cell per row
per frame cost 4.1 ms/frame against 0.28 ms for the precomputed ramp, and the eye
cannot separate neighbouring steps at that resolution.

## Scan percentage → counts

The mock's scan line counts down a percentage: `walking directories · 42%`. In the
mock that number is `Math.random()`, and in a real scan it cannot be anything else.
The analyzer does not know how large the tree is until it has walked it, so any
percentage would be invented.

The scan line reports what is actually known — items seen, bytes so far, and the
path currently being walked. The blinking cursor from the mock is kept, driven off
the same 100 ms tick that samples progress, so the two animations cannot beat
against each other.

## The delete modal → what gdu's `d` used to mean

Not a terminal limitation, but a deliberate break with upstream, recorded here
because someone coming from gdu will notice it.

gdu's `d` deletes permanently — `os.RemoveAll`, and it is gone. The brief asks for
recoverable deletes, and the two cannot both be true of one key. So:

- `d` moves the item to the OS trash, and `u` undoes it.
- `D` deletes permanently.
- `e` empties a file.

The modal states what will actually be true afterwards rather than merely asking
whether you are sure. In particular it says, every time, that **the trash does not
free disk space** — the item stays on the same volume. That is the fact most likely
to catch out the person who opened cdu precisely because a disk was full, and it is
the reason the permanent delete keeps a key of its own rather than being buried
behind a flag.

Two things the mock's modal does not have, and that the terminal has no trouble
with:

- **The destructive button never holds the focus.** A reflexive Enter cancels.
- **Protected paths need the word typed out.** The filesystem root, `$HOME`, the
  directories directly inside `$HOME`, and any mount point cannot be deleted by any
  sequence of single keypresses — the focus cannot even reach the button until
  `DELETE` is complete.

## Filtering → fuzzy, but size still decides the order

The mock has no filter; this is a translation of gdu's `/` search into something a
little more forgiving. Typing `/` narrows the current directory by a fuzzy
subsequence match — `nmd` finds `node_modules` — and the matched runes are lit up
in the name so the reason a row survived is visible.

The one real decision was ordering, because it fights the tool's reason to exist.
Classic fuzzy find ranks by match score, which in a disk usage tool would let a
well-spelled small file float above the large one it matches less tidily. So the
fuzzy match decides only *what* is shown; the size sort still decides the *order*.
Biggest first, always.

The match is its own ~25-line subsequence matcher rather than a library: the
candidate set is one directory, never the whole tree, so there is nothing to
optimise and nothing to justify a dependency.

## Viewing a file → a capped, sniffed pager

`v` opens the selected file in a read-only pager. Two things a naive "read it and
show it" gets wrong, and this does not:

- **A multi-gigabyte file is not pulled into memory.** The read is capped at 1 MiB;
  past that the footer says the view is truncated. A pager is for looking.
- **A binary is not dumped as mojibake.** A NUL byte in the head — which text does
  not have and a binary almost always does — refuses the file with a message.

It reuses the list's own windowed scrolling rather than `bubbles/viewport`, for the
same reason the list does: to keep the exact-height guarantee that stops the frame
scrolling on its own.

## Scan cancellation → process exit

Not a terminal limitation, but an engine one, and it shows up in the UI.

gdu's analyzer exposes no cancellation: no `context.Context`, no `Stop()`. Pressing
`q` during a scan therefore tears down the Bubble Tea program and exits, letting
the walk goroutine die with the process — which is effectively what gdu does too.
Fixing this properly means adding a context to `pkg/analyze`, which is
upstream-owned; the right venue is a PR to gdu, not a local edit that would put the
engine on our merge conflict surface.
