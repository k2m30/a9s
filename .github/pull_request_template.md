## Summary

<!-- What changed and why. One paragraph. -->

## Related Issues

<!-- Link related issues: Fixes #123, Relates to #456 -->

## Type of Change

- [ ] Bug fix
- [ ] New feature
- [ ] Refactoring
- [ ] Documentation
- [ ] CI/CD
- [ ] Other

## Process — see [`docs/development-process.md`](../docs/development-process.md)

### Stage gates

- [ ] **Stage 1 Intake**: sized, acceptance criteria written.
- [ ] **Stage 2 Spec** (size ≥ M): spec doc landed, OR not required for this change.
- [ ] **Stage 3 Tests**: tests written before implementation; failing tests existed in commit history before the implementation commit, OR not applicable for docs-only changes.
- [ ] **Stage 4 Implementation**: only files in scope were touched; binary rebuilt with `make build`.
- [ ] **Stage 5 Review**: diff reviewed (`a9s-consistency-checker` for cross-file drift; BT v2 / security / coverage review; CodeRabbit / Codex).
- [ ] **Stage 6 Pre-push**: `make ready-to-push` green locally (or docs-only — `make mdlint` green).
- [ ] **Live integration** (only if `internal/aws/` real-account behavior changed): `A9S_CT_PROFILE=<profile> go test -tags integration ./tests/integration/ -run TestFullRelatedViewValidation -count=1 -v -timeout 600s` green.

### Definition of Done

- [ ] Acceptance criteria demonstrably met.
- [ ] Single-source-of-truth invariants intact (no dual-authoring; no permanent dual API surface).
- [ ] Read-only invariant preserved (`make verify-readonly` green).
- [ ] Docs sync respected: README regenerated if `docs/shared/` changed; `CHANGELOG.md` updated for user-visible changes; `docs/architecture.md` aligned for cross-cutting changes.
- [ ] Conventional commit message.

## Risk and Rollback

<!-- One line on blast radius and how to revert if this lands hot. -->
