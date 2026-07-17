# internal/theme

## Purpose

Every colour cdu draws with: the `Theme` token set, the five bundled themes, the
themes a user writes, and the rules for putting them together.

## Ownership

cdu-owned. A new directory, so it never conflicts on an upstream merge. gdu's own
`style:` config block is untouched and still drives the classic interface.

- `theme.go` — the `Theme` struct, the `Color` type, validation, `Overlay`.
- `load.go` — the embedded bundled themes, the file parser, user themes.
- `themes/*.yaml` — the five bundled themes. **The palettes live here, not in Go.**
- `resolve.go` — `Config` (the `theme:` block) and the precedence rules.
- `list.go` — what `cdu themes` prints.

## Local Contracts

- **A token names a role, not a hue.** `Accent`, not `pink`. charm's accent is
  pink, midnight's is cyan, phosphor's is green — a name from one theme's palette
  is a lie in the other four.
- **`Selected` and `Ink` look alike and are not.** `Selected` is drawn on `Panel`,
  a surface, which is dark in a dark theme. `Ink` is drawn on `Danger` and `Dim`,
  which are *colours* and can be light even in a dark theme. They are the same
  white in charm, which is exactly why fusing them survived review — it only fails
  on a theme whose danger colour is light, in a modal, at 1.3:1.
- **`contrast_test.go` checks every pairing `charm/style.go` actually composes.**
  Compose a new one there, add it here. Colour pairings across a set of themes are
  not checkable by eye, and the failures hide in screens nobody opens by accident.
  The bar is WCAG AA for bold text, 3:1.
- **Colours are `#rrggbb` only.** Lipgloss would also take an ANSI index like
  `"5"`, but the usage bar blends its endpoints in Luv and an index has no value
  to blend — it would come out black, on one theme, in one place.
- **A theme's name is its filename.** A `name:` key would be a second source of
  truth and the two would eventually disagree.
- **There is no background token.** The terminal's own background shows through,
  which is what keeps transparency and blur working for the people most likely to
  care about themes. The price is that a light theme needs a light terminal —
  `Light` records which, and `cdu themes` says so. No bundled theme is light since
  daylight was dropped for phosphor; `TestNoBundledThemeIsLight` pins that.
- **`mono` is the absence of colour, not a set of greys.** cdu does not paint the
  background, so a fixed grey would have to be legible on both a white and a black
  terminal, and none is. `Plain` routes it through the same
  bold/reverse/underline path as `--no-color`, which is legible on both and is
  already audited by `charm/nocolor_test.go`. So mono is a tested path with a
  name.
- **Bundled themes panic on a parse error; user themes never do.** A broken
  embedded theme is a corrupt build — the same class as a bad regexp constant —
  and there is nothing to render around. `TestEveryBundledThemeParses` is what
  keeps that panic unreachable. A user's theme is input: report it, skip that one
  file, keep the rest, and never let it stop cdu opening.
- **Nothing here exits or fails to produce a theme.** An unknown name lists the
  real ones and falls back; a malformed token drops itself and inherits. Someone
  who typo'd a colour opened cdu because a disk was full.
- **Tokens are walked by reflection**, because validation, `Overlay` and the
  config writer all need to, and a hand-written list would let a token added later
  slip past all three in silence. `TestEveryColourFieldIsAToken` is the price.
- **`--no-color`/`NO_COLOR` are not handled here.** They already drive the plain
  render path, which is what mono is; forcing the theme too would be a second way
  of saying the same thing.
- **`userThemes` is written once at startup and read-only after**, like the rest
  of this program's configuration. A test that loads into it must reset it.

## Work Guidance

The five are the brief's five: `charm`, `midnight`, `daylight`→`phosphor`,
`ember`, `mono`. The set drifted to nine once (four Catppuccin flavours plus a
`glacier` that turned out to *be* midnight under another name) before being cut
back. The count is a promise, not decoration — check the brief before adding one.

`--write-config` writes only `theme.preset`, never the resolved tokens: writing
them would pin today's palette into every config ever written, and no later cdu
could improve one.

## Verification

    go test ./internal/theme/...
    go build -o /tmp/cdu ./cmd/cdu && /tmp/cdu themes

The suite covers the token reflection, hex validation, overlay and precedence,
every bundled theme parsing and being complete, the contrast of every pairing on
every theme, user themes (loading, `.yml`, shadowing a bundled name, a broken one
being skipped), and the listing under a forced ASCII profile.

`cdu themes` is the check the suite cannot do: it is the one command whose entire
output is colour.

## Child DOX Index

None.
