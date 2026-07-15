# Archived skills & agents

Superseded by the declarative-catalog architecture (020-architecture-refactor). Kept for history; the workflows they describe (imperative `Register*` calls, `core/resource/types_<category>.go` files, raw-JSON fixtures) no longer exist in the codebase.

| Archived | Successor |
|----------|-----------|
| `a9s-add-resource` | `a9s-resource-spec` (spec) + `a9s-implement-resource` (implementation) |
| `a9s-add-child-view` | `a9s-implement-resource` (child views are declarative `Children`/`ChildFetcher` catalog fields) |
| `a9s-add-related-view` | `a9s-implement-resource` (related pivots are `Related []domain.RelatedDef` catalog fields; checker-authoring rules live in that skill's E1–E6) |
| `../agents-archived/a9s-fixtures.md` | `a9s-create-demo-fixture` skill (typed fakes in `core/demo/fixtures/` + `fakes/`, not raw JSON under `tests/testdata/fixtures/`) |
