---
name: a9s-external-review
description: >-
  Run the external review pass (Opus area review, CodeRabbit, /ponytail-review) on a finished a9s task
  before its commits fast-forward into main or before a release tag. Covers when a task qualifies,
  how to invoke each reviewer, and how to treat their findings and patches.
---

# External review

Local gates prove the code runs; they do not prove it is right. Three independent reviewers find different classes of defect:

- **Area review** — a `general-purpose` agent on model `opus` per concern the task changed, reviewing all code in that area across every resource type, with the prompt in [`docs/review-prompt.md`](../../../docs/review-prompt.md). Not a diff review.
- **CodeRabbit** — from the shell:

  ```bash
  coderabbit review --committed --base-commit <base>
  ```

- **`/ponytail-review`** — over-engineering only (delete/simplify); it does not hunt correctness.

## Rules

- **Once per task, before its commits fast-forward into main.** The area review is expensive: one pass per task, over every concern the task changed, never per fix, and only when the task's production diff (`core/`, `internal/`, `cmd/`, `scripts/`, tests excluded) is at least 100 changed lines or touches 3 or more production files. One-liners, wording, docs and test-only changes are not reviewed.
- **Code review only.** The prompt forbids tests, fixtures, fakes, demos, gates and state changes; the loop has already run the gates.
- **Review the committed tree**, not the working tree — `--base-commit <base>` for CodeRabbit; the area review runs on the task's committed tip.
- **A reviewer's suggested patch is a proposal, not verified code.** Read every snippet against the actual file before applying it; patches routinely reference helpers that do not exist in that file's package. The finding can be correct while the patch does not compile.
- **Point reviewers at production code.** A large mechanical test migration will drown the signal otherwise.

On PRs: `@coderabbitai ignore` where no further review is wanted; `[skip ci]` for trivial follow-ups; reviews trigger per-push, so batch small fixes into one push.
