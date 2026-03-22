# Phase 2: TUI Design — Child View Screens

**Agent:** tui-designer
**Constraint:** Do NOT read any *.go files. Only read files under `docs/design/` and `docs/design/child-views/`.
**Prerequisite:** Phase 1 must be complete. Read `docs/design/child-views/devops-research.md` for the approved resource list.

## Context

a9s is a read-only AWS TUI. Read `docs/design/design.md` for the existing design system — all visual rules (colors, borders, key bindings, view stack) are already defined and inherited automatically by every view. You do NOT need to re-specify any of that.

Two child views already exist as reference implementations — **read both before designing new ones**:
- `docs/design/child-views/s3-objects.md` — S3 bucket → object list (with prefix/folder navigation)
- `docs/design/child-views/r53-records.md` — Route 53 hosted zone → DNS record list

Your designs must follow the exact same structure and level of detail as these two files.

The existing `views.yaml` file (`.a9s/views.yaml`) defines every view as:
- **list**: column name → `path` (AWS SDK struct field) + `width`
- **detail**: ordered list of fields to show in detail view

Child views follow the same pattern. See the `s3_objects` and `r53_records` entries in `views.yaml` for the exact format.

**Important**: `.a9s/views_reference.yaml` lists every available AWS SDK struct field path for each resource type. When designing child views, the field paths in your `views.yaml` entries must come from the actual AWS SDK response struct for that child resource. Use web search to find the SDK struct fields for child resources (e.g., `lambda.GetFunctionEventInvokeConfigOutput`, `cloudwatchlogs.LogStream`) since `views_reference.yaml` only covers parent resources currently implemented.

## Your Task

Design child views for all MUST-HAVE and SHOULD-HAVE resources from the approved research.

### Per child view, produce:

#### 1. views.yaml entry
The `list` and `detail` config, same format as `s3_objects` in `.a9s/views.yaml`. This is the primary deliverable — it defines what columns appear and what fields the detail view shows. Use web search to look up the actual AWS SDK response struct field names.

#### 2. Navigation spec
- **Entry**: how the user reaches this view (Enter on parent resource list)
- **Frame title format**: e.g., `lambda-invocations(25) — my-function`
- **View stack**: Esc returns to parent. If this child has its own children, describe the chain.
- **New key bindings**: only if this child view needs keys beyond the standard set (j/k/g/G/Enter/d/y/c/Esc/filter/sort). Most won't.

#### 3. ASCII wireframe
One wireframe per view, same notation as `design.md` sections 4.1–4.8. Realistic data — names a DevOps engineer would recognize. Show normal + selected row states. Include status-colored rows only if the child resource has status semantics.

#### 4. AWS API
- Exact read-only API call(s) that populate this view
- Pagination notes if relevant
- Latency warnings if applicable (e.g., CloudWatch GetLogEvents can be slow for large groups)

#### 5. Nested children (if applicable)
If this child view itself has children (e.g., Lambda invocations → log events), describe each level with the same 4 items above. Each nesting level = separate design file + separate GitHub issue.

### Output files

One file per child view:
```
docs/design/child-views/{parent-shortname}-{child-name}.md
```

Plus a summary `docs/design/child-views/README.md` with:
1. Overview table: parent → child → tier → status (all "planned")
2. Dependency order (which children depend on other children existing first)
3. Recommended implementation order

### GitHub issues

For every child view screen (including nested children as separate issues), create a GitHub issue:

- **Title**: `feat: child view — {parent} → {child name}`
- **Labels**: `enhancement`, `child-view`, and tier label (`must-have` or `should-have`)
- **Body**:
  ```
  ## Summary
  {One paragraph: what this child view shows and why it's valuable}

  ## DevOps Use Case
  {The scenario(s) from the research doc}

  ## Design
  Link to: docs/design/child-views/{filename}.md

  ## AWS API
  {API call(s) needed — read-only only}

  ## views.yaml
  {The list + detail config for this child view}

  ## Acceptance Criteria
  - [ ] Accessible via Enter from {parent} resource list
  - [ ] Frame title: {format}
  - [ ] Columns: {list}
  - [ ] Esc returns to {parent} list
  - [ ] views.yaml entry added
  - [ ] Read-only — no write API calls

  ## Dependencies
  {List any issues this depends on, or "None"}
  ```

## Design Principles

- **Inherit everything**: colors, borders, key bindings, status-to-color mapping — all come from `design.md`. Do not re-specify them.
- **views.yaml is the spec**: the column layout and detail fields are the core design artifact. Wireframes illustrate them.
- **4–6 columns max**: terminal space is precious. Additional data is in detail (d) and YAML (y) views.
- **No new chrome**: no sidebars, split panes, tabs. The view stack (list → detail/YAML/child) is the only navigation model.
- **Realistic data**: wireframes use plausible AWS resource names, IDs, timestamps.
