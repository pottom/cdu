# internal/selfupdate

## Purpose

`cdu update`: replace the running binary with the latest release. The actionable
other half of `internal/updater`'s startup check.

## Ownership

cdu-owned. A new directory, so it never conflicts on an upstream merge.

- `selfupdate.go` — `Update`, and the pure helpers it is built from (asset naming,
  download, checksum, archive extraction). The cobra `update` command is in
  `cmd/cdu/main.go`.

## Local Contracts

- **Reuse `internal/updater`, do not re-implement the check.** `Update` calls
  `LatestTag`/`IsNewer` and returns `ErrUpToDate` when there is nothing to do.
- **Verify before swapping — always.** The archive is checked against the release's
  `sha256sums.txt` before its binary is extracted. A self-updater that swapped in an
  unverified download would be a supply-chain hole with a friendly name.
- **The swap is atomic and rolls back.** `minio/selfupdate.Apply` does the
  platform-specific replace (including the Windows rename dance) so a half-written
  binary can never be left behind. Do not hand-roll the replace.
- **Only public release assets are read; nothing is sent.** Same restraint as the
  startup check.
- **Asset names track GoReleaser's `name_template` exactly.** `cdu_<version>_<os>_
  <arch>.<ext>`, version without its leading `v`, `.zip` on Windows. If the release
  archive naming changes, change it here too — the two must not drift.
- **32-bit arm is declined (`ErrUnsupported`).** Its GOARM (v5/v6/v7, which the asset
  name encodes) is not exposed at runtime, and guessing wrong would fetch the wrong
  build. `install.sh` reads it from `uname` and covers that case; the command says so.
- **Package-manager installs should not self-update.** A Homebrew/apt copy and cdu's
  would disagree; the command points such users back to their package manager. (The
  detection is a known follow-up; keep the escape hatch in mind when adding it.)

## Verification

    go test ./internal/selfupdate/...

The pure helpers are unit-tested with in-memory archives: asset naming (incl. the arm
refusal and the Windows zip), checksum match/mismatch/missing against GoReleaser's
two-space format, and extraction from a `.tar.gz` and a `.zip` including a binary
under a directory prefix. The download and swap run only against a real release.

## Child DOX Index

None.
