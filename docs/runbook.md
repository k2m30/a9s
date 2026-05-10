# a9s Operations Runbook

This is the runbook for **operational incidents that fall outside the standard Stage 5 / Stage 6 / Stage 6.5 review-and-validation pipeline**. It is the single place to look first when something is wrong with the agent fleet or the Paperclip platform layer that hosts a9s' agents — not when something is wrong with a9s' code.

For software incidents (regressions in `main`, P0/P1 bugs, real-AWS regressions), see `docs/development-process.md` § "Incident & Rollback" and § "Stage 6.5".

## Agent recovery — `error → idle`

If a Paperclip agent on the a9s roster is stuck in `error` state and is not progressing through its standard heartbeat lifecycle, the operative recovery primitive is:

```
PATCH /api/agents/{id}
{ "status": "idle" }
```

### Constraints (do not violate)

- **CEO bearer token only.** Lower-trust bearers cannot transition agent state. The CTO bearer is **not** sufficient. If the CEO is unavailable, escalate; do not attempt with another bearer.
- **Only valid for the `error → idle` transition.** Do not use this endpoint for `paused → idle`, `running → idle`, `terminated → idle`, or any other transition. Those go through their own dedicated endpoints.
- **All other agent transitions go through dedicated endpoints**:
  - `POST /api/agents/{id}/pause` — running → paused
  - `POST /api/agents/{id}/resume` — paused → running
  - `POST /api/agents/{id}/terminate` — any → terminated
- **Do not** invent ad-hoc transitions through `PATCH /api/agents/{id}`. The `error → idle` carve-out is the one documented exception; everything else uses the dedicated endpoint or files an incident.

### When to use it

Use this primitive when:

1. An agent is stuck in `error` after a heartbeat that hit a non-recoverable adapter exception (model API rejection, malformed tool call, etc.).
2. The agent's pending issues are time-sensitive (assigned PR review, release-blocking work) and waiting for a natural recovery is not acceptable.
3. The board / CEO has confirmed the underlying cause has been addressed (or determined that the cause was transient and unlikely to recur this heartbeat).

If the underlying cause is unknown, **do not unstick the agent**. Diagnose first; an agent that is unstuck without root-cause analysis will re-enter `error` on its next wake and burn budget repeatedly.

### Audit trail

Every use of this primitive is logged at the platform layer. The originating CEO bearer is recorded against the transition. Do not paper-over the diagnosis: file or update the incident issue with the timestamp of the recovery, the symptom, the suspected cause, and the gate change (if any) that prevents recurrence — same rule as `docs/development-process.md` § "Incident & Rollback".

### History

Captured here from AS-115 (2026-05-10) so future on-call decisions do not require reading the issue body. Filed by [AS-117](https://github.com/k2m30/a9s/issues) per CEO direction. Any future addition to this constraint set lands as a single PR that updates this section in the same change.
