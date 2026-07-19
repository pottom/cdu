# cdu — charm disk usage

Fast disk usage analyzer with a [Charm](https://charm.sh) interface.

cdu is a fork of [gdu](https://github.com/dundee/gdu) by Daniel Milde. The
disk-analysis engine is gdu's, reused as-is — cdu keeps its speed and its
byte-for-byte export parity — and rebuilds the interactive interface on the Charm
stack (Bubble Tea, Lipgloss, Bubbles), adding themes, recoverable deletes,
multi-select, tree-wide search, and a self-updater. The name follows the family:
ncdu = *ncurses* du, gdu = *go* du, **cdu = *charm* du**.

> **Not the official gdu.** This is an independent fork, not affiliated with or
> endorsed by gdu. Report cdu's own bugs at <https://github.com/pottom/cdu/issues>.
> See [NOTICE](./NOTICE).

## Install

**Linux / macOS / BSD** — download, verify, and install the right build:

```sh
curl -fsSL https://raw.githubusercontent.com/pottom/cdu/main/install.sh | sh
```

**Windows** (PowerShell):

```powershell
irm https://raw.githubusercontent.com/pottom/cdu/main/install.ps1 | iex
```

The installers verify the release checksum — and the keyless
[cosign](https://www.sigstore.dev/) signature, when cosign is present — before
installing. They never use `sudo`; if the target directory is not writable they say
so and stop.

**Container:**

```sh
docker run --rm -it -v "$HOME:/data" ghcr.io/pottom/cdu /data
```

**From source** (Go 1.26+):

```sh
go install github.com/pottom/cdu/cmd/cdu@latest
```

**Update** an existing install to the latest release with `cdu update` (installed
through a package manager? update through that instead).

## Usage

```sh
cdu            # scan the current directory
cdu ~/         # scan a path
cdu -d         # pick from the mounted disks
```

Press **?** at any time for every key. The essentials:

| Key | Action |
| --- | --- |
| `↑ ↓` `k j`, `g` `G` | move, jump to top / bottom |
| `→ ↵ l`, `← h` | enter a directory, go to the parent |
| `/`, `f` | filter this directory, find files tree-wide |
| `s`, `t` | sort menu, column menu |
| `p`, `v`, `o` | theme picker, view a file, open in default app |
| `space`, `M`, `u` | mark a row, open the delete queue, unmark all |
| `d`, `D`, `e`, `U` | trash, delete for good, empty a file, undo the last trash |
| `r`, `T`, `F` | rescan, largest files, find duplicates |
| `esc`, `q` | back / cancel / clear marks, quit |

Deleting with `d` moves to the trash and is recoverable this session with `U`; `D`
deletes permanently and frees the space. A permission-denied delete offers to retry
with `sudo`.

The non-interactive and JSON export modes (`--non-interactive`, `-o`) are identical
to gdu's, and **`--classic`** opens gdu's original interface unchanged. See
`cdu --help` or `man cdu` for the full flag list.

## Themes

Five themes ship in the binary — `charm` (default), `midnight`, `ember`, `phosphor`,
and `mono` (no color, for any terminal). Pick one live with **`p`**, or set
`--theme`, or write your own:

```sh
cdu themes                                   # list them, in color
cdu themes dump charm > ~/.config/cdu/themes/mine.yaml
cdu --theme mine
```

## Configuration

cdu reads `~/.config/cdu/cdu.yaml` (or `$XDG_CONFIG_HOME/cdu/cdu.yaml`). On first
run it falls back, read-only, to an existing gdu config and tells you so; `cdu
--write-config` takes it over into cdu's own file.

## Building

```sh
make build      # ./cdu, stripped
make test
make lint
```

Releases are cut by [GoReleaser](https://goreleaser.com) on a `cdu-vX.Y.Z` tag: the
cross-compiled matrix, checksums, SBOMs, cosign signatures, and the
`ghcr.io/pottom/cdu` container image.

## License

MIT, the same as gdu. gdu's copyright and license are intact — see
[LICENSE.md](./LICENSE.md) and [NOTICE](./NOTICE) for the fork's attribution. cdu
carries its own version and records the embedded gdu release as build metadata:
`cdu vX.Y.Z+gduA.B.C`.
