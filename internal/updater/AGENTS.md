# internal/updater

## Purpose

The startup check for a newer cdu release. It asks GitHub for the latest tag and
compares it to the running build; the header shows a mark when one is behind.

## Ownership

cdu-owned. A new directory, so it never conflicts on an upstream merge.

- `updater.go` — `LatestTag` (the releases API call) and `IsNewer` (the compare).

`internal/selfupdate` reuses both; the mark's rendering lives in `charm/`.

## Local Contracts

- **Read-only, and it says nothing about the machine.** One GET to the public
  releases API, no identifiers, no telemetry. It must stay skippable —
  `CDU_NO_UPDATE_CHECK` (honoured in `charm/`) keeps a locked-down install offline.
- **A failed check is swallowed, never shown.** No network, a rate limit, a 404
  before the first release — none is worth a word to someone analyzing a disk. The
  caller drops the error.
- **The timeout is short.** A startup check that hangs the interface waiting on a
  slow network is worse than one that never runs.
- **Versions compare numerically, component by component**, so `v1.10.0` beats
  `v1.9.0` — a string compare gets that backwards. A build with no numeric version
  (`development`) is below everything, so a dev binary is never told it is current.
  Build metadata (`+gdu…`) and pre-release suffixes are cut before comparing.

## Verification

    go test ./internal/updater/...

`IsNewer` is table-tested: patch/minor ordering, the 10-vs-9 numeric case, the
with/without-`v` forms, `development` as below-everything, and an unparseable tag as
never-newer.

## Child DOX Index

None.
