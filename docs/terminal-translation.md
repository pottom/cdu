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

## The theme's background → the terminal's own

The mocks paint a dark aubergine field behind everything. cdu does not paint it:
there is no background token, and the terminal's own background shows through.

Painting it is possible but costs the thing people actually want from a theme.
A terminal background is the user's choice, often transparent or blurred, and the
sort of person who picks a theme is exactly the sort who set that up. Filling it
would also mean every style carrying an explicit background — an inner `\x1b[0m`
resets the outer one, so a background cannot simply be wrapped around a composed
frame.

The consequence is a rule: **a light theme needs a light terminal.** `Theme.Light`
records which are which and `cdu themes` says so. The bundled set is all dark
after `daylight` was dropped, which leaves `mono` — legible on any background,
because it uses no colour at all — as the only thing a light-terminal user has.
That is a real gap against the brief, and `TestNoBundledThemeIsLight` records it
rather than leaving it to be rediscovered.

## `mono` → the absence of colour, not a palette of greys

The brief asks for "high-contrast greyscale that reads without relying on hue".
The obvious reading — a theme whose tokens are all greys — cannot work here.

Because cdu does not paint the background (above), a fixed grey would have to be
legible on both a white and a black terminal. No grey is. The `--no-color` path
already solves this by conveying state through bold, reverse and underline
instead of hue, it is legible on any background, and `charm/nocolor_test.go`
already audits it.

So `mono` is that path with a name on it: `plain: true`, no tokens. `--theme mono`
and `--no-color` render identically, which is one code path and one audit rather
than two.

## Colour that means something → checked, not eyeballed

Not a terminal limitation — a consequence of having more than one palette.

A token pairing that works in one theme can be unreadable in another, and it will
not be noticed, because the failure is a legible-looking screen in a theme nobody
on the team runs. `Selected` and `Ink` are both "the bright one" and are the same
white in charm; on a palette whose danger colour is *light*, that white landed on
it at 1.3:1 and the delete button vanished — in a modal, on one theme.

`internal/theme/contrast_test.go` checks every foreground/background pair that
`newStyles` actually composes, on every theme, at WCAG AA for bold text (3:1). It
found four real problems the first time it ran. Adding a theme cannot skip it, and
composing a new pairing in `style.go` means adding it there.

## Icons → a glyph and a space, and only if you ask

The mock has no icons; `--icons` is a translation of what exa made people expect
from a terminal file listing. Two things stand between the idea and the terminal.

**The font.** The glyphs are Nerd Font private-use codepoints. A terminal without
a patched font draws a box, and cdu cannot find out which it is — there is no way
to ask a terminal what font it has. So the flag is opt-in: the person with the
font asks for it, and everyone else never sees a column of tofu. The same reason
`--no-unicode` beats `--icons` rather than the other way round.

**The width, which is worse, because it is invisible until it isn't.** A Nerd Font
ships in variants: the *Mono* ones squeeze each icon into one cell, the plain ones
draw it across two. `runewidth` says one either way — the codepoint's width is
ambiguous, and the answer lives in the font file, which cdu never sees. Getting
this wrong shifts every column right of the icon by one, on some people's
terminals and not others.

The fix is not to measure better; it is to not need to. **The icon cell is always
a glyph followed by a space**, so it is two columns wide whichever variant is
installed: a single-width glyph leaves the space blank, a double-width one draws
over it, and the size column starts in the same place regardless. exa arrived at
the same answer. It is why `iconWidth` is 2 and why nothing may ever sit
immediately after the glyph.

## Scan cancellation → the door that was already there

Not a terminal limitation. An engine one, and for a long time it stood.

gdu's analyzer exposes no cancellation: no `context.Context`, no `Stop()`. So
`q` during a scan tore down the program and let the walk goroutine die with the
process — which is effectively what gdu does too. The obvious fix is a context in
`pkg/analyze`, and that is exactly what the fork strategy forbids: the engine is
upstream's, and putting it on the merge conflict surface is the cost this whole
architecture exists to avoid paying, on every gdu release, forever.

The way out was already in the interface. The analyzer asks *us* whether to
ignore each directory, through a hook we supply, and it asks **before it
descends**:

```go
if a.ignoreDir(name, entryPath) { continue }
```

So `ignoreFunc` answers "ignore it" from the moment cancel is set. The walk skips
every directory it has not yet opened, and the goroutines unwind on their own —
promptly, because the work left is bounded by the directories already open.
`fileTypeFilter` does the same one level down, before the `stat`. Nothing is
abandoned to run in the background; the walk really stops. No upstream file
changed.

The tree it returns is thrown away. The directories the walk never opened are not
marked or empty in it — they are *absent*, so every parent above them reports
less than it holds. A disk usage tool showing sizes that are quietly too small is
worse than one showing nothing at all, and the key next to `esc` is `d`.

`esc` cancels and `q` still quits, because `esc` means "out of this" on every
other screen and a `q` that meant something else here would be a binding you
could not trust. Cancelling lands wherever the scan interrupted: the device list,
the tree a rescan was refreshing, or out — when the scan was the only thing cdu
had been asked to do, there is nothing behind it to show.
