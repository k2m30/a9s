# Changelog fragments

One file per task, named after its branch: `changelog.d/<task>.md`. A task
writes only its own file, so two tasks in flight never conflict here.

A fragment holds only the sections it touches, each an `##` heading whose text
is one of `Added`, `Changed`, `Fixed`, `Removed`, followed by list items in the
same voice as `CHANGELOG.md`:

```markdown
## Added

- Volumes no backup plan selects now say so.

## Fixed

- A failed related check no longer reports a confident zero.
```

`make changelog` assembles the fragments in file-name order into the
`## [Unreleased]` section of `CHANGELOG.md`, merging same-named sections and
keeping every line already there. Nothing else writes `CHANGELOG.md` outside a
release. `make ready-to-release` refuses a tree that still has unassembled
fragments.

This README is not a fragment; the assembler ignores it.
