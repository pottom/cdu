# build

## Purpose

The variables a build stamps into the binary, and the one file that records which
gdu release the engine is at.

## Ownership

Mixed — the one directory that is, which is the whole reason for this doc.

- `build.go` — **upstream's, never edit.** It declares `Version`, `Time`, `User`,
  `RootPathPrefix`, replaced wholesale on every gdu sync. Adding a field here would
  conflict on the next merge.
- `cdu.go` — **cdu-owned.** It lives beside `build.go`, in the same package, *so
  that* `build.go` never has to be touched: anything cdu needs in the `build`
  package goes here instead.

## Local Contracts

- **`cdu.go`'s `GduVersion` is the single source of the synced gdu version.** It is
  compiled in as the default, surfaced by `cdu --version` as the `gdu base` line, and
  read by the Makefile — nothing else holds a copy. The upstream watcher bumps this
  one line in a sync PR. Never re-introduce a second copy (a `UPSTREAM_VERSION` file,
  a Makefile constant): that drift is exactly what consolidating it here removed.
- **`Version` is stamped, not defaulted.** Its upstream default is `development`;
  the Makefile and GoReleaser set it via `-ldflags -X` from cdu's own `cdu-v*` tag.
  A bare `go build` shows `development`, which is honest.

## Verification

`cdu --version` shows cdu's `Version` and the `gdu base` from `GduVersion`. There is
no Go test here — the file is two `var` declarations.

## Child DOX Index

None.
