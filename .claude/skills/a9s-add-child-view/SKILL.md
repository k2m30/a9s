---
name: a9s-add-child-view
description: Blueprint for adding a new child view to a9s — 3-phase workflow (architect → tests → implement) with exact file manifests and hard-won lessons
disable-model-invocation: true
---

# Adding a New Child View

**Workflow: Architect → Tests → Implement. Never skip or reorder.**

## Phase 1: Architect Spec (a9s-architect agent)

The architect reads the design spec and parent fetcher, then produces an **exact file manifest** for the QA and coder agents. This prevents context drain — downstream agents receive only the manifest, not the full design spec.

### Architect must determine:

1. **Parent analysis** — read `internal/aws/{parent}.go`:
   - What is `Resource.ID`? (often a name, NOT an ARN)
   - What Fields keys exist? Does the parent store the ARN? If not, add it.
   - Which `ServiceClients` field is needed? Already exists?

2. **ContextKeys mapping** — the #1 source of bugs:
   - If the child API needs an ARN, the parent MUST have the ARN in Fields
   - `"ID"` → `Resource.ID` (often a name — verify!)
   - `"Name"` → `Resource.Name`
   - `"field_key"` → `Resource.Fields["field_key"]`
   - `"@parent.x"` → inherited from parent context (Pattern C nesting)
   - **Write a test** that verifies the parent fetcher populates the required field

3. **Field formatting rules** (apply to ALL fields):
   - `*int64` epoch-ms timestamps → use `formatEpochMillis()` in fetcher, `key:` in config
   - `*int64` byte counts → use `formatBytes()` in fetcher, `key:` in config
   - String SDK fields → can use `path:` in config (reads from RawStruct directly)
   - Computed/formatted fields → MUST use `key:` in config (reads from Fields map)
   - **Never use `path:` for timestamps or byte counts** — they show raw numbers

4. **File manifest** — exact list of files to CREATE/EDIT/APPEND with specific content

### Architect output format:

```
CHILD VIEW SPEC: {child_shortname}
Parent: {parent_shortname} | Pattern: {A/B/C/D} | Key: {enter/e/L/r/s}
API: {service}:{APICall} with {ParentParam}
Client field: c.{ServiceField} (exists: yes/no)

CONTEXT KEYS:
  {context_key} ← Fields["{field_key}"] (verified: parent stores ARN at line N)

COLUMNS (all use key: in config):
  {key} | {Title} | {width} | formatter: {none/formatEpochMillis/formatBytes}

DETAIL PATHS:
  {Path1}, {Path2}, ... (longest: {N} chars → keyW will auto-size)

FILES TO CREATE:
  internal/aws/{child_type}.go
  tests/unit/aws_{child_type}_test.go

FILES TO EDIT:
  internal/aws/interfaces.go — append {InterfaceName}
  internal/resource/types.go — add Children to {parent}, add {ChildType}Columns()
  internal/config/defaults.go — add "{child_shortname}" entry
  .a9s/views.yaml — add {child_shortname} section
  cmd/refgen/main.go — add entry (if SDK struct)
  tests/unit/mocks_test.go — append mock
  tests/unit/qa_detail_child_views_test.go — append 2 tests
  tests/unit/qa_yaml_child_views_test.go — append 3 tests
  tests/unit/qa_list_rawstruct_child_views_test.go — append 1 test
  internal/demo/fixtures_{category}.go — register child demo
```

## Phase 2: Tests FIRST (a9s-qa agent)

The QA agent receives the architect's manifest and writes ALL tests. Tests MUST fail to compile because the implementation doesn't exist yet.

### Test files to create/modify:

**1. Mock:** `tests/unit/mocks_test.go` (APPEND)
- For paginated APIs, use `outputs []*{Output}` slice with `callIdx` counter
- For single-response APIs, use `output *{Output}`

**2. Fetcher tests:** `tests/unit/aws_{child_shortname}_test.go` (CREATE)
- Happy path: correct ID, Name, Status, all Fields, RawStruct
- Empty response: empty slice, no error
- API error: error propagation
- Pagination (if applicable): multiple pages collected, stops at cap
- Timestamp formatting: known epoch ms → expected formatted string
- Byte formatting: known bytes → expected human-readable string
- Nil fields: no panic, empty strings
- RawStruct: original SDK struct preserved
- **Parent context test**: verify the correct context key is used (e.g., ARN not name)

**3. Detail tests:** `tests/unit/qa_detail_child_views_test.go` (APPEND)
- ViewContainsExpectedFields
- NilFields (no panic)
- **Long field names not truncated** (if any path > 22 chars)
- **Formatted timestamps in detail** (not raw epoch ms)

**4. YAML + List tests:** (APPEND to existing files)
- YAML view contains fields, frame title, no ANSI in raw content
- List rawstruct renders correctly

**5. Run `go test`** — confirm compilation failure (expected).

## Phase 3: Implementation (a9s-coder agent)

The coder receives the architect's file manifest and makes all tests pass.

### Checklist (order matters):

**1. Interface:** `internal/aws/interfaces.go` (APPEND)
```go
type {InterfaceName} interface {
    {APICall}(ctx context.Context, params *{service}.{APICall}Input, optFns ...func(*{service}.Options)) (*{service}.{APICall}Output, error)
}
```

**2. Client field** (IF new service): `internal/aws/client.go`

**3. Child fetcher:** `internal/aws/{child_type}.go` (CREATE)
- `init()` registers: `RegisterFieldKeys`, `RegisterChildFetcher`, `RegisterChildType`
- Fetcher function with proper formatting:
  - Timestamps: `formatEpochMillis(*field)` — NEVER `fmt.Sprintf("%d", *field)`
  - Bytes: `formatBytes(*field)` — NEVER `fmt.Sprintf("%d", *field)`
  - Messages: strip newlines if content may contain `\n`
- For paginated APIs: cap at reasonable limit (e.g., `const maxResults = 500`)
- Column function returns `[]resource.Column` with proper widths

**4. Parent wiring:** `internal/resource/types.go` (EDIT)
- Add/append `Children` on parent type
- Add column function
- **ContextKeys must map to actual data** — if API needs ARN, use Fields key that has ARN

**5. Config:** `internal/config/defaults.go` (ADD)
- List columns: use `Key:` for computed fields, `Path:` only for string SDK fields
- Detail paths: include all relevant fields

**6. Views config:** `.a9s/views.yaml` (ADD)
- Mirror defaults.go but in YAML format
- Use `key:` for computed fields, `path:` for string SDK fields

**7. Refgen:** `cmd/refgen/main.go` (APPEND if SDK struct)

**8. Demo fixtures:** `internal/demo/fixtures_{category}.go`

**9. Parent fetcher** (IF needed): add missing Fields (e.g., ARN)
- Write a test for this FIRST in the parent's test file

### Verification:
```
go test ./tests/unit/ -count=1 -timeout 120s
golangci-lint run ./...
go build -o a9s ./cmd/a9s/
go run ./cmd/refgen/ > .a9s/views_reference.yaml  # if SDK struct added
```

## Pattern Variants

### Pattern A: Single Child (most common)
- Parent has 1 child. `Enter` drills in.
- Examples: Target Group Health, ASG Activities, Alarm History, ECR Images

### Pattern B: Multi-Child Parent
- 2+ children with different trigger keys.
- Examples: ECS (`Enter`→Tasks, `e`→Events, `L`→Logs), CFN (`Enter`→Events, `r`→Resources)
- Implement all children of the same parent in ONE release.

### Pattern C: Level-2 Nested
- Child has its own children. `RegisterChildType` includes `Children` slice.
- Uses `@parent.` prefix in ContextKeys.
- Examples: Log Streams→Events, Lambda Invocations→Log Lines, ELB Listeners→Rules

### Pattern D: Cross-Service
- Fetcher calls different AWS service than parent.
- Needs multiple interfaces and possibly multiple client fields.
- Examples: Lambda→Invocations (CW Logs), ECS→Container Logs (CW Logs)

## What You Do NOT Need to Change

- `app.go` — generic `handleEnterChildView` and `fetchChildResources`
- `messages.go` — `EnterChildViewMsg` handles all child navigation
- `resourcelist.go` — `handleChildKey` and `buildChildContext`
- `keys.go` — trigger keys already defined

## Hard-Won Lessons (v3.1.0)

1. **ContextKeys: ARN vs Name** — `Resource.ID` is often a name, not an ARN. If the child API needs an ARN, verify the parent populates it in Fields. Test this explicitly.

2. **`key:` vs `path:` in config** — `path:` reads raw SDK struct (epoch ms, raw bytes). `key:` reads formatted Fields values. ALWAYS use `key:` for timestamps and byte counts.

3. **Detail view Fields-first** — `renderFromConfig` checks Fields before RawStruct. Fetchers must populate Fields with ALL formatted values needed for detail display.

4. **Newlines in messages** — `PadOrTrunc` strips `\n`/`\r`, but log-like messages should be cleaned in the fetcher too.

5. **Narrow screens** — `fitColumns` shrinks the last column to remaining space (min 10 chars). Don't assume fixed terminal width.

6. **Detail key column** — `computeKeyWidth()` auto-sizes from longest field name. Long dotted paths like `Target.AvailabilityZone` are handled.

7. **Pagination caps** — large AWS resources (log groups with 8000+ streams) need pagination limits. Add `const maxResults = 500` and break when exceeded.

8. **Deprecated AWS fields** — `StoredBytes` on LogStream is deprecated (always 0). Check AWS docs before adding fields.

9. **formatBytes/formatFloat are shared utilities** in `internal/aws/log_streams.go` — reuse them, never delete.

10. **Sort by age** — `getAgeField` matches field keys containing: time, date, launch, creation, event, start, timestamp. Name new time fields accordingly.
