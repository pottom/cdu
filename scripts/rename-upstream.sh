#!/usr/bin/env bash
#
# Mechanically rewrites an upstream gdu tree into cdu's namespace.
#
# Runs on the `upstream-cdu` branch after merging a new upstream tag. It is
# deterministic and idempotent: re-running it on an already-renamed tree is a
# no-op, and running it on a freshly merged tag renames only what the merge
# brought in. Keeping the rename here rather than on `main` is what stops every
# upstream merge from conflicting across every import block in the tree.
#
# Scope is deliberately narrow — import paths and directory names only. It must
# not touch:
#   * `"progname":"gdu"` in report/export.go and tui/actions.go — the JSON export
#     format is byte-compatible with gdu/ncdu and has to stay that way.
#   * config paths, flag help, README, Makefile, packaging — those live on the
#     conflict surface and are rewritten on `main`.

set -euo pipefail

OLD_MODULE="github.com/dundee/gdu/v5"
NEW_MODULE="github.com/pottom/cdu"

cd "$(git rev-parse --show-toplevel)"

# 1. Module path in go.mod and every Go import. perl rather than sed -i, whose
#    backup-suffix syntax differs between BSD and GNU — this also runs in CI.
#    Guard on emptiness: BSD xargs runs the command even with no input.
files=$(grep -rl --include='*.go' --include='go.mod' -F "$OLD_MODULE" . || true)
if [ -n "$files" ]; then
	echo "$files" | xargs perl -pi -e "s|\Q${OLD_MODULE}\E|${NEW_MODULE}|g"
fi

# 2. Command directory and binary name, both on disk and in the import paths
#    that reference it (cmd/cdu/main.go imports cmd/cdu/app).
if [ -d cmd/gdu ]; then
	git mv cmd/gdu cmd/cdu
fi
files=$(grep -rl --include='*.go' -F "${NEW_MODULE}/cmd/gdu" . || true)
if [ -n "$files" ]; then
	echo "$files" | xargs perl -pi -e "s|\Q${NEW_MODULE}/cmd/gdu\E|${NEW_MODULE}/cmd/cdu|g"
fi

# 3. Man page source and its generated roff.
[ -f gdu.1.md ] && git mv gdu.1.md cdu.1.md
[ -f gdu.1 ] && git mv gdu.1 cdu.1

gofmt -l -w ./cmd ./pkg ./internal ./tui ./stdout ./report ./build >/dev/null

echo "renamed ${OLD_MODULE} -> ${NEW_MODULE}"
