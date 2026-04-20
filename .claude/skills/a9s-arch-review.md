---
name: a9s-arch-review
description: Deterministic architecture review — runs automated checks from docs/go-codebase-checklist.md, then the agent interprets results and scores
---

# Architecture Review Skill

This skill automates the mechanical checks from `docs/go-codebase-checklist.md` and produces a structured PASS/FAIL report. The agent then interprets the results, applies judgment to warnings, and produces a final score.

## Phase 1: Automated Checks (deterministic)

Run the architecture review script. It checks:

- **File sizes** (>500 lines, excluding tests, demo fixtures, preview tools)
- **Function sizes** (>50 lines, approximate brace-counting)
- **Package dependency violations** (views->layout, views->app, messages->tui, etc.)
- **Key binding anti-patterns** (raw string comparison, inline key.NewBinding)
- **init() location violations** (only allowed in aws/, demo/, styles/)
- **Error handling** (bare returns, unjustified ignored errors)
- **Concurrency** (manual goroutines in tui/, context.Context in structs)
- **Interface checks** (AWS single-method, View 5-method)
- **Nolint directives** (must have explanatory comments)
- **Rendering** (len() vs lipgloss.Width heuristic)
- **Bubble Tea v2 patterns** (no v1 Init signatures)
- **Project structure** (domain code under internal/, single go.mod)
- **Style constants** (no inline hex outside styles/)
- **God struct monitoring** (root Model field count)
- **Package export count** (>15 symbols)
- **Circular imports** (via go build)
- **Naked returns** (bare return in named-return functions)
- **Charm.land imports** (not github.com/charmbracelet, excluding x/ansi)
- **Resource type consistency** (fetcher count vs view definition count)

```bash
bash .claude/scripts/arch-review.sh > /tmp/arch-review.txt 2>&1
```

Read the output from `/tmp/arch-review.txt`.

## Phase 2: Toolchain Checks

Run these separately (they take longer):

```bash
make lint > /tmp/arch-lint.txt 2>&1
```

```bash
make test > /tmp/arch-tests.txt 2>&1
```

```bash
make security > /tmp/arch-vulncheck.txt 2>&1
```

```bash
make gofix > /tmp/arch-gofix.txt 2>&1
```

Read the output from each file.

## Phase 3: Agent Judgment

After reading all outputs, the agent applies judgment to produce a final report:

### Known Acceptable Exceptions

These are architectural decisions, not violations:

- `internal/aws/<service>_interfaces.go` files may individually exceed 500 lines for services with many narrow operation interfaces (EC2, IAM). Growth is linear with resource types; each interface is ~3 lines. Acceptable.
- `internal/tui/views/resourcelist.go` exceeds 500 lines -- complex view with filtering, sorting, pagination. Monitor but acceptable.
- `ResourceListModel.Update()` exceeds 50 lines -- Bubble Tea type-switch pattern, accepted exception per checklist.
- `*DefaultViews()` and `*ResourceTypes()` functions are large map/slice literals -- data declarations, not logic. Acceptable.
- AWS fetcher functions (60-130 lines) -- multi-step API calls with pagination. Review individually but often acceptable.
- `internal/tui/views` exports >15 symbols -- each view is an exported type, this is structural. Acceptable.

### Scoring

Rate each section 0-10 based on:
- **10**: All PASS, no warnings
- **8-9**: All PASS, warnings are known exceptions or trivial
- **6-7**: Minor FAILs that are not violations of core principles
- **4-5**: FAILs that indicate architectural drift
- **0-3**: Fundamental violations (circular imports, wrong dependency direction, etc.)

### Output Format

```
# Architecture Review — YYYY-MM-DD

## Automated Checks
[Paste PASS/FAIL summary from script]

## Toolchain
- Lint: PASS/FAIL (N issues)
- Tests: PASS/FAIL (N passed, N failed)
- Race: PASS/FAIL
- Vulncheck: PASS/FAIL

## Judgment Calls
[For each WARN/FAIL, explain whether it is an acceptable exception or a real issue]

## Score: X.X/10
[One-line rationale]

## Action Items
[Numbered list of things to fix, if any, ordered by severity]
```
