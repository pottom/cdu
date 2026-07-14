# trash

## Purpose

Moves files to the operating system's trash, and puts them back. This is what
makes cdu's default delete recoverable.

It exists because gdu has none: `pkg/remove` calls `os.RemoveAll`, which is final.
A disk usage tool exists to be pointed at things and told to delete them, which is
exactly the situation where a mistake is easiest to make and most expensive to
have made.

## Ownership

cdu-owned. A new directory, so it never conflicts on an upstream merge.

- `trash.go` — `Entry`, the errors, and `uniqueTarget`. No OS dependencies.
- `trash_unix.go` (`unix`) — device and mount-point resolution, the move, the
  restore.
- `trash_xdg.go` (`unix && !darwin`) — the freedesktop.org spec: Linux, the BSDs.
- `trash_darwin.go` — `~/.Trash`, and `<mount>/.Trashes/<uid>` for other volumes.
- `trash_windows.go` — `SHFileOperation` with `FOF_ALLOWUNDO`.
- `trash_unsupported.go` — everything else. Reports `Supported() == false`.

## Local Contracts

- **No cgo, on any target.** That is what rules out the existing cross-platform
  trash libraries and is why this is written by hand per platform. Every build
  runs `CGO_ENABLED=0`.
- **Never copy to trash across a volume boundary.** A rename cannot cross one, and
  copying gigabytes in response to a single keypress is a worse surprise than
  refusing. `MoveToTrash` returns `ErrCrossVolume`; the caller offers a permanent
  delete instead.
- **Never claim an undo the platform cannot honour.** Windows can recycle an item
  but cdu cannot take it back out without COM, so `RestoreSupported()` is false
  there and the interface must not offer the key. A "moved to trash" that had
  quietly deleted the file would be the worst failure this package could have.
- **Never overwrite on restore.** Something may have taken the name back since the
  delete. An undo that destroys data is not an undo.
- **Never overwrite inside the trash.** Two files deleted from different
  directories routinely share a name; `uniqueTarget` gives the second its own.
- **The sidecar and the item move together.** Under the freedesktop spec a trashed
  item without its `.trashinfo` is one the *desktop* cannot restore. If the
  sidecar cannot be written, `MoveToTrash` moves the item back and fails.

## Work Guidance

Two costs of trashing are real and must stay visible in the UI rather than being
smoothed over:

1. **It does not free disk space.** The item stays on the same volume. Someone
   deleting because a disk is full needs the permanent delete.
2. **It is per-volume.** An item on an external disk goes to that disk's trash.

The Windows path is the least verifiable here: it is written against the API
contract and cross-compiled, but not exercised on a real Windows machine. Treat a
bug report there as more credible than the code.

## Verification

    go test ./internal/trash/...
    CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build ./internal/trash/

Tests run against a temporary `HOME`, never the real trash. They cover the round
trip, directories, name collisions inside the trash, the refusal to overwrite on
restore, and — on the XDG platforms — that the sidecar is written, escaped and
removed.

## Child DOX Index

None.
