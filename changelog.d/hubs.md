## Added

- `make changelog` assembles `CHANGELOG.md`'s Unreleased section from one
  fragment file per task under `changelog.d/`, so two tasks landing at once no
  longer conflict in the changelog. `make ready-to-release` refuses while a
  fragment is unassembled.
- `scripts/task-file-overlap.sh` names the files two task branches both touch,
  which is the check the "no two live tasks share a file" rule needed.

## Changed

- `docs/attention-signals.md` states every signal once, generated from the
  registered findings, instead of a hand-written table per category that every
  change had to edit. The ideas no code emits yet moved to a "Not yet
  implemented" list on the same page.
- Each resource type's demo row count, issue badge and state-coverage
  allowlist now live beside its fixtures instead of in two shared test files.
