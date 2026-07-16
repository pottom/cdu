# internal/config

## Purpose

Where cdu's configuration lives, and how it inherits gdu's.

## Ownership

cdu-owned. A new directory, so it never conflicts on an upstream merge.

This package exists **because** `cmd/cdu/main.go` does not: main.go is upstream's
file and our merge conflict surface, so every line added there is re-merged on
every gdu release. main.go calls `Resolve` and `WriteFile`, and that is all.

## Local Contracts

- **`~/.config/cdu` on every platform**, or `$XDG_CONFIG_HOME/cdu`. Deliberately
  not `os.UserConfigDir`, which would put a terminal tool in
  `~/Library/Application Support` on macOS and `%AppData%` on Windows. nvim, gh,
  bat, ripgrep, starship and gdu itself all decline to live there. It is also
  where someone will look for `themes/`, and it keeps cdu's config next door to
  the gdu config it inherits.
- **The gdu paths are hardcoded exactly as gdu hardcodes them.** gdu does not
  consult `XDG_CONFIG_HOME`, so neither may `gduPath` — it would look for gdu's
  config somewhere gdu would never have written it, and the fallback would
  silently never fire.
- **The gdu fallback is read-only, and announced.** A fork that ignored the config
  of the tool it forked would drop someone's ignore patterns on first run, and it
  would read as cdu being broken rather than as cdu looking elsewhere.
- **Read path and write path are not the same thing.** `--write-config` writes to
  cdu's own path even when the config was read from gdu's — writing back over
  gdu's config is what the obvious implementation does and is the exact opposite
  of what the fallback notice promises. An explicit `--config-file` wins, because
  then the file was named by hand. `writeConfigPath` in main.go is the split.
- **`WriteFile` creates the directory.** gdu wrote to a dotfile in `$HOME`, which
  always exists; cdu's path is a directory that may not, and a `--write-config`
  failing with "no such file or directory" is a poor first impression.
- **`isFile`, not `os.Stat`.** A *directory* named `cdu.yaml` is not a config.

## Work Guidance

Notices from here reach the user on the Charm interface's status line
(`Flags.ConfigNotice` → `charm.WithNotice`), never on stderr: cdu opens the
alternate screen immediately and would wipe anything printed before it.

`ThemeDir` is consumed by `internal/theme`'s `LoadUserThemes`.

## Verification

    go test ./internal/config/...

The suite points `HOME` and `XDG_CONFIG_HOME` at a temporary tree, so no test can
read or write a real config. It covers the directory choice, XDG precedence, both
gdu fallback locations in gdu's own order, the directory-is-not-a-file case, and
that `WriteFile` creates its directory at 0600.

## Child DOX Index

None.
