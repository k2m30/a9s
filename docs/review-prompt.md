# Review prompt

The external review pass dispatches a `general-purpose` agent on model `opus`, in the background. `<AREA>` is the concern the task changed, stated as a scope across every resource type it reaches (for example "related-panel and navigation target ID resolution for all resource types"), never the diff. A task that changes several concerns gets one agent per concern, up to four at a time. The prompt is sent verbatim with `<AREA>`, `<REPO>` (the task's worktree) and `<TASKDIR>` replaced; `<slug>` is a short name for the area.

```text
You are reviewing the Go repo at <REPO> (a read-only AWS TUI). Task:

Review ALL production code related to <AREA>. This is NOT a diff review.

Scope: every production-code path that implements, calls, or depends on <AREA>, across every AWS resource type it reaches, plus only the direct production-code dependencies necessary to verify those paths. Production code lives under core/, internal/, cmd/. Useful: `graphify query "<question>"` works in the repo; docs/resources/<shortName>.md describes each type's intended behavior.

Do NOT inspect, run, mention, or use tests, test fixtures, fakes, demos (core/demo/), coverage, git diff, or git status. Do NOT download anything, modify any repo file, or run commands that change state. The only network exception is looking up the most recent AWS documentation (WebFetch/WebSearch/context7) to verify API semantics.

Do NOT review unrelated repository code.

Report only concrete production-code defects in <AREA> — verify each against the actual code before reporting; no speculation. For each finding, provide:
- Priority (P0/P1/P2/P3)
- Exact file and line
- Trigger scenario
- User impact
- Concise fix direction

If no production-code defect exists, the report is exactly: "No findings in <AREA>."

OUTPUT: write the full report as markdown to <TASKDIR>/review/<slug>.md (the only file you may create). Your final reply to me must be ONLY one line: "<slug>: N findings (P0:a P1:b P2:c P3:d)".
```
