# scripts

## Purpose

Tooling that keeps the fork in sync with upstream gdu.

## Ownership

- `rename-upstream.sh` — rewrites an upstream gdu tree into cdu's namespace.
  Runs on the `upstream-cdu` branch, never on `main`.

## Local Contracts

`rename-upstream.sh` is the load-bearing piece of the fork's merge strategy, and
its narrow scope is the whole point. It must stay:

- **Deterministic and idempotent.** It runs again on every upstream tag. Running
  it on an already-renamed tree is a no-op.
- **Mechanical only.** Import paths and directory names. It must never touch
  `"progname":"gdu"` (export format is byte-compatible with gdu and ncdu), config
  paths, flag help, README, Makefile, or packaging — those are rewritten on `main`
  and would be clobbered on the next sync.
- **Portable.** It runs on macOS locally and on Linux in CI. No `sed -i ''`, no
  `xargs -r` — both differ between BSD and GNU.

Widening its scope re-introduces the merge pain it exists to prevent.

## Work Guidance

Syncing a new upstream release:

    git checkout upstream     && git merge vX.Y.Z          # fast-forward
    git checkout upstream-cdu && git merge upstream
    bash scripts/rename-upstream.sh
    git commit -am "chore: rename for vX.Y.Z"
    git checkout -b chore/sync-gdu-vX.Y.Z main
    git merge upstream-cdu
    # bump GduVersion in build/cdu.go to X.Y.Z — its single source
    git commit -am "chore: sync gdu engine to X.Y.Z"       # then open a PR

`watch-upstream.yml` does all of this automatically; the steps above are the manual
fallback. The `GduVersion` bump is the only version edit — there is no other copy.

Conflicts should only ever appear in the conflict surface listed in the root
AGENTS.md. A conflict anywhere else means someone edited an upstream-owned file.

Releases gate on green CI. An upstream refactor of the analyzer API can break the
build; when it does, the sync PR stays open and no release is cut.

## Verification

    bash scripts/rename-upstream.sh   # must be a no-op on an already-renamed tree
    go build ./... && go test ./...

## Child DOX Index

None.
