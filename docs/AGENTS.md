# docs

## Purpose

Every project document lives here: the brief, the design mocks, architecture and
decision records, and the theming guide.

Standard files stay at the repository root and do NOT move here: `README.md`,
`LICENSE.md`, `NOTICE`, `AGENTS.md`, and the man page source `cdu.1.md`.

`docs/benchmarks/` and `docs/run-books.md` are inherited from gdu and are
upstream-owned — leave them alone.

## Ownership

- `gdu-charm-claude-code-prompt.md` — the brief. The requirements baseline; read
  it before planning work. Do not rewrite it to match what was built; if reality
  diverges, record the divergence in a decision record instead.
- `design/` — five HTML mocks. These are the **visual source of truth** for the
  Charm UI: `cdu-charm-mock.html` (main browser, selection, delete modal, scan
  animation) plus `cdu-1-disks.html`, `cdu-2-largest-files.html`,
  `cdu-3-markers.html`, `cdu-4-help.html`. They are previews only — translate
  them to Bubble Tea and Lipgloss, never embed a browser.
- Decision records — one file per architectural decision, with the evidence that
  drove it.
  - `adr-001-upstream-sync-strategy.md` — the core/UI boundary, why gdu cannot be
    consumed as a module, and the three-branch fork model. Read this before
    touching anything that crosses the upstream boundary.

## Local Contracts

- A decision record states the decision, the evidence, and what it rules out. It
  is not a diary; when a decision is superseded, replace it rather than appending.
- The mocks describe a browser. Anything a terminal genuinely cannot do (box
  shadow, CSS gradients, radial glows) must be recorded with the translation
  chosen instead — that is a deliverable of the brief, not an afterthought.

## Work Guidance

Documents are English, even though conversation with the user is Hungarian.

## Verification

None — these are documents. Claims about the codebase must cite a real path.

## Child DOX Index

None. `design/` holds assets, not a work boundary.
