# internal/dup

## Purpose

Finds byte-identical files in a scanned tree. Backs the `F` key and the duplicate
screen.

## Ownership

cdu-owned. A new directory, so it never conflicts on an upstream merge.

## Local Contracts

- **This is the one place in cdu that reads file contents.** Everything else
  stats — that is the whole reason cdu is fast — so this is opt-in, runs off the
  render loop as a `tea.Cmd`, and is cancellable. Nothing here may be called from
  the render path.
- **Work is proportional to the candidates, not the tree.** Files are bucketed by
  size first, which is free — the sizes are already in memory — and only a size
  shared by two or more files is ever read. A file with a size nobody else has is
  never opened. Keep that order: it is what makes the feature affordable.
- **Honest content comparison, never a guess from stat.** Two different files can
  share a size to the byte, and the result feeds a `d`. So matched files are
  SHA-256 of their full contents — cryptographic, because a collision here would
  tell someone two different files are the same, and those are not odds worth a
  delete.
- **Hard links are not duplicates.** Two names for one inode share their bytes on
  disk; deleting one frees nothing. `dedupeInodes` keeps one file per hard-linked
  inode before hashing, so `Reclaimable` never counts space a delete would not
  free. A file that is not hard-linked has inode 0 here and is always kept.
- **An unreadable file is skipped, never assumed equal.** A permission error must
  not make cdu claim a match it could not verify. The finder drops the file and
  carries on.
- **Empty files are ignored.** They are all trivially identical, in their
  hundreds, and deleting one frees nothing — noise, not a finding.
- **Cancel is checked between files.** It cannot interrupt a single file mid-hash,
  but one file is a fine granularity: a cancel lands within one read. It shares
  the model's scan-cancel flag; the two searches never run at once.
- **Groups come back most-reclaimable first** — the order you would act in.

## Verification

    go test ./internal/dup/...

The suite uses real files under `t.TempDir` and a real analyzer scan, so it
exercises the actual `GetSize` / `GetMultiLinkedInode` / `GetPath` the finder
relies on: byte-identical vs. same-size-only, hard links (real `os.Link`), a real
copy beside a hard link, empty files, an unreadable file, cancellation, and the
reclaimable arithmetic.

## Child DOX Index

None.
