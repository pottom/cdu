# DOX framework

- DOX is highly performant AGENTS.md hierarchy installed here
- Agent must follow DOX instructions across any edits

## Core Contract

- AGENTS.md files are binding work contracts for their subtrees
- Work products, source materials, instructions, records, assets, and durable docs must stay understandable from the nearest applicable AGENTS.md plus every parent AGENTS.md above it

## Read Before Editing

1. Read the root AGENTS.md
2. Identify every file or folder you expect to touch
3. Walk from the repository root to each target path
4. Read every AGENTS.md found along each route
5. If a parent AGENTS.md lists a child AGENTS.md whose scope contains the path, read that child and continue from there
6. Use the nearest AGENTS.md as the local contract and parent docs for repo-wide rules
7. If docs conflict, the closer doc controls local work details, but no child doc may weaken DOX

Do not rely on memory. Re-read the applicable DOX chain in the current session before editing.

## Update After Editing

Every meaningful change requires a DOX pass before the task is done.

Update the closest owning AGENTS.md when a change affects:

- purpose, scope, ownership, or responsibilities
- durable structure, contracts, workflows, or operating rules
- required inputs, outputs, permissions, constraints, side effects, or artifacts
- user preferences about behavior, communication, process, organization, or quality
- AGENTS.md creation, deletion, move, rename, or index contents

Update parent docs when parent-level structure, ownership, workflow, or child index changes. Update child docs when parent changes alter local rules. Remove stale or contradictory text immediately. Small edits that do not change behavior or contracts may leave docs unchanged, but the DOX pass still must happen.

## Hierarchy

- Root AGENTS.md is the DOX rail: project-wide instructions, global preferences, durable workflow rules, and the top-level Child DOX Index
- Child AGENTS.md files own domain-specific instructions and their own Child DOX Index
- Each parent explains what its direct children cover and what stays owned by the parent
- The closer a doc is to the work, the more specific and practical it must be

## Child Doc Shape

- Create a child AGENTS.md when a folder becomes a durable boundary with its own purpose, rules, responsibilities, workflow, materials, or quality standards
- Work Guidance must reflect the current standards of the project or user instructions; if there are no specific standards or instructions yet, leave it empty
- Verification must reflect an existing check; if no verification framework exists yet, leave it empty and update it when one exists

Default section order:
- Purpose
- Ownership
- Local Contracts
- Work Guidance
- Verification
- Child DOX Index

## Style

- Keep docs concise, current, and operational
- Document stable contracts, not diary entries
- Put broad rules in parent docs and concrete details in child docs
- Prefer direct bullets with explicit names
- Do not duplicate rules across many files unless each scope needs a local version
- Delete stale notes instead of explaining history
- Trim obvious statements, repeated rules, misplaced detail, and warnings for risks that no longer exist

## Closeout

1. Re-check changed paths against the DOX chain
2. Update nearest owning docs and any affected parents or children
3. Refresh every affected Child DOX Index
4. Remove stale or contradictory text
5. Run existing verification when relevant
6. Report any docs intentionally left unchanged and why

## User Preferences

- Respond in Hungarian. Code, identifiers, comments, commit messages and docs stay in English.
- No commit, PR, or generated file may credit Claude or any AI tool as author or co-author. No `Co-Authored-By` or "Generated with" trailers.
- Pause for review after a milestone rather than running the whole brief end to end.

---

# Project: cdu

## Purpose

cdu ("charm disk usage") is a fork of [dundee/gdu](https://github.com/dundee/gdu)
that replaces gdu's interactive tview interface with one built on the Charm stack
(Bubble Tea, Lipgloss, Bubbles), and adds theming, an installer and a self-updater.
The disk-analysis engine is reused as-is, never reimplemented.

The full brief is `docs/gdu-charm-claude-code-prompt.md`; the visual source of
truth is the mocks in `docs/design/`.

## Ownership

The tree is split into two halves, and the split is the single most important
rule in this repo.

**Upstream-owned — never edit.** These come from gdu and are replaced wholesale
on every upstream sync. Editing them turns cheap merges into expensive ones.

    pkg/  tui/  stdout/  report/  internal/common/  build/build.go  default.pgo

No child AGENTS.md is placed inside them: we do not maintain that code, and any
doc written there would be describing a tree that gets overwritten from upstream.

**cdu-owned — ours to change freely.** New directories, so they cannot conflict:

    charm/  internal/theme/  internal/config/  internal/trash/
    internal/selfupdate/  build/cdu.go  scripts/  docs/  AGENTS.md
    NOTICE  UPSTREAM_VERSION

`UPSTREAM_VERSION` records which gdu tag the engine is at. It is the single source
of truth for the `+gduA.B.C` build metadata and for the upstream watcher — update
it only in a sync PR.

**Conflict surface — edit deliberately, expect merge conflicts.** These are
upstream files we must modify, and they are the only ones:

    go.mod  go.sum  cmd/cdu/main.go  cmd/cdu/app/app.go  cmd/cdu/app/app_test.go
    Makefile  .github/workflows/*  README.md  cdu.1.md

`app_test.go` is on the list because cdu changes the interactive default: gdu's
"Gui" tests inject a mocked tview application, so `runApp` sets `Classic: true` to
keep testing what they were written to test.

Adding a file to a *new* directory is always safe. Editing an upstream file is a
cost — justify it, and keep the diff minimal and localized.

## Local Contracts

- **Engine reuse.** The new UI calls the existing analyzer, tree, sorting and
  delete/empty operations. No duplicated scanning logic.
- **Export parity.** `"progname":"gdu"` in `report/export.go` and
  `tui/actions.go` must stay `gdu` — the JSON export is byte-compatible with
  gdu and ncdu. The non-interactive and export modes stay byte-for-byte
  identical to upstream.
- **`--classic` stays.** gdu's original tview UI remains reachable and unchanged.
  Ask before any change that alters classic behavior.
- **License.** `LICENSE.md` and Daniel Milde's copyright stay intact. `NOTICE`
  states the fork and its attribution. Never imply this is the official gdu.
- **Themes, not hardcoded colors.** Every visual token lives in a `Theme`;
  the `charm` theme is the default, not a constant in the render path.
- **Branch model.** Three branches, and the rename lives on the middle one:
  - `upstream` — verbatim gdu, never touched.
  - `upstream-cdu` — `upstream` plus one script-generated rename commit
    (`scripts/rename-upstream.sh`). This is what keeps import-path churn out of
    every future merge.
  - `main` — merges `upstream-cdu`, plus all cdu code. Protected: PR required,
    CI green and up to date, no force-push, no deletion.
- **Merge method depends on the PR.** Feature PRs are **squash-merged**, so the
  history stays effectively flat. Sync PRs are merged with a **real merge commit**
  — the merge base is what makes the *next* sync cheap, and squashing one would
  throw it away and force every future sync to re-resolve the whole tree. This is
  why `main` does not enforce linear history.
- **Every change lands on a prefixed branch and a PR**, never straight to `main`:
  `feat/ fix/ chore/ docs/ refactor/ test/ perf/ ci/ build/` + a short slug.
  Conventional Commits (`type: summary`) drive the changelog.
- **Versioning.** SemVer `vX.Y.Z` for cdu's own changes, with the embedded engine
  recorded as build metadata: `cdu v0.3.1+gdu5.36.1`.

## Work Guidance

- Ship vertical slices: each PR builds, runs, and is reviewable on its own.
- Go 1.26, `CGO_ENABLED=0` on every target — this rules out cgo-based libraries.
- The Bubble Tea list must stay virtualized; the render layer stays off the scan
  hot path. Speed is gdu's identity.
- Layout is recomputed from `tea.WindowSizeMsg` in `View()`. Never hardcode a
  width, column position, or bar length. Never panic on a tiny terminal.
- Non-color cues accompany every color-coded state, so meaning survives `mono`,
  `NO_COLOR` and colorblindness.

## Verification

    make test            # go test ./... — 699 tests inherited from gdu, keep green
    make lint            # golangci-lint, config .golangci.yml
    go build ./...
    make gobench         # scan throughput, must not regress

Parity check: diff `cdu --non-interactive` and `cdu -o -` against upstream `gdu`
on the same tree; the bytes must match.

## Child DOX Index

- `charm/AGENTS.md` — the default Bubble Tea interface.
- `internal/theme/AGENTS.md` — the colour tokens, the five themes, and yours.
- `internal/config/AGENTS.md` — where the config lives, and inheriting gdu's.
- `internal/trash/AGENTS.md` — recoverable deletes, per OS, CGO-free.
- `docs/AGENTS.md` — the brief, design mocks, and architecture/decision records.
- `scripts/AGENTS.md` — the upstream rename and sync tooling.

Directories from the Ownership section that do not exist yet
(`internal/selfupdate/`) get their own child doc in the PR that creates them.
