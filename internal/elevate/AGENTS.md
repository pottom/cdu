# internal/elevate

## Purpose

Retrying a permission-denied delete with elevated privileges. When a plain remove
fails on `fs.ErrPermission`, charm offers to run it again through `sudo`.

## Ownership

cdu-owned. A new directory, so it never conflicts on an upstream merge.

- `doc.go` — the package contract.
- `elevate_unix.go` — `sudo`: `Available`, `Cached`, `RemoveCmd`, `Reason`.
- `elevate_windows.go` — every function a no-op; `Reason` explains why.

The offer, the modal, and running the command are in `charm/elevate.go`.

## Local Contracts

- **cdu never sees the password.** `RemoveCmd` builds a real `sudo rm` command and
  the caller hands it the terminal (`tea.ExecProcess`); sudo prompts for itself.
  cdu must never read, prompt for, store, or pass a password. This is the whole
  security posture of the feature — do not add a code path that weakens it.
- **Build the command, do not run it.** This package only *constructs* the
  `*exec.Cmd`. Running it is the caller's job, because only the caller can hand over
  the terminal cleanly. Keep the split.
- **`Cached` is a silent `sudo -n` pre-check**, so the offer can say whether a
  password will be asked for. It must never itself prompt.
- **Windows is an honest no-op, not a fake.** There is no `sudo`; `Available` is
  false and `Reason` says to relaunch cdu as administrator. Do not paper over the
  gap with a broken command — a delete that silently does nothing is worse.
- **The paths are passed after `--`.** A file named `-rf` is a filename, not a flag.

## Verification

    go test ./internal/elevate/...

`RemoveCmd` is tested for the plain and notice-carrying forms of the command, and
that the paths land after the `--` separator.

## Child DOX Index

None.
