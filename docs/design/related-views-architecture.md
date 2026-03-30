# Related Views Architecture

Issue: #119
Version: 2.0
Date: 2026-03-29
Status: Design
Aligns with: `docs/design/related-resources.md` v4.2

---

## 0. Flat-to-Folder Decision

### Current State

| Directory | Files | Projected After Related Views |
|-----------|------:|------------------------------:|
| `internal/aws/` | 101 | ~167 (+66 `{source}_related.go`) |
| `tests/unit/` | 203 | ~269 (+66 `aws_{source}_related_test.go`) |
| `internal/demo/` | 39 | ~45 (+6 `related_{category}.go`) |

### Decision: KEEP FLAT

Folder-per-resource reorganization is **rejected**. The cost/benefit ratio is
unfavorable at this time.

### Arguments Against Reorganization

**1. Go package design makes folders expensive.**

Go equates directories with packages. `internal/aws/ec2/` would be
`package ec2`, not `package aws`. This creates cascading problems:

- **Circular dependency**: `internal/aws/` defines `ServiceClients` which
  all fetchers access via `c.EC2`, `c.RDS`, etc. If fetchers move to
  sub-packages, they import `internal/aws` for the client. But
  `internal/aws` must blank-import all sub-packages to trigger their
  `init()` registration. This is a circular import.

- **Shared helpers become cross-package**: `formatEpochMillis()`,
  `formatBytes()`, `ptrString()` live in package `aws` today. Moving
  fetchers to sub-packages means either duplicating these helpers or
  creating a shared `aws/internal/` package.

- **66+ blank imports**: A registration file must import all sub-packages
  to trigger init(). This is fragile and noisy.

- **Interface visibility**: `interfaces.go` defines all AWS API interfaces
  in package `aws`. Sub-packages need these for type assertions. The
  interfaces would need to stay in the parent package, creating an
  asymmetry where some code is in `aws/` and some in `aws/ec2/`.

**2. Agent token waste is overestimated.**

The CLAUDE.md already mandates targeted access patterns:
```
Glob("internal/aws/{resource}*.go")   -- returns 1-3 files, not 101
Grep("mock.*{Interface}", "tests/unit/mocks_test.go")  -- targeted
```

A flat directory with 167 files where agents use `Glob("internal/aws/ec2*.go")`
returns 2-3 files. The same query against `internal/aws/ec2/*.go` returns the
same 2-3 files. The token savings are negligible.

Where agents DO waste tokens is reading entire cross-cutting files
(`mocks_test.go`, `qa_detail_v220_test.go`). But reorganizing by folders
doesn't help -- those files are cross-cutting BY DESIGN.

**3. Massive migration cost.**

- Every import path in 200+ test files changes
- Both `a9s-add-resource` and `a9s-add-child-view` skills rewrite completely
- All agent file references in CLAUDE.md and skill files change
- In-flight work disrupted
- Risk of subtle breakage from moved `init()` registrations

**4. Go ecosystem precedent supports flat.**

Large Go projects use flat packages extensively:
- Terraform providers: flat `internal/provider/` with 500+ files
- Kubernetes: flat `pkg/apis/` packages
- Go standard library: `net/http` is one flat package

### Mitigation: Filename Conventions

Instead of folders, enforce filename prefixes that keep related files
adjacent and globbable:

| Pattern | File naming | Glob |
|---------|-------------|------|
| Top-level fetcher | `internal/aws/ec2.go` | `ec2*.go` |
| Related resolvers | `internal/aws/ec2_related.go` | `ec2*.go` |
| Child fetcher | `internal/aws/ecs_tasks.go` | `ecs*.go` |
| Related tests | `tests/unit/aws_ec2_related_test.go` | `*ec2*` |

### Revisit Trigger

Reconsider reorganization if:
- `internal/aws/` exceeds 250 files (currently ~167 projected)
- Agent token metrics show >30% waste on directory scanning
- Go adds support for sub-package `init()` without circular imports

---

## 1. Registry Pattern

### 1.1 Related View Definitions

```go
// internal/resource/related.go

// RelationshipCategory classifies the cost and display behavior.
type RelationshipCategory int

const (
    RelForward     RelationshipCategory = iota // IDs from parent's Fields -- cheap
    RelReverse                                  // Cross-service lookup -- expensive
    RelAlgorithmic                              // Naming convention / multi-hop
    RelCloudTrail                               // Universal -- always available
)

// RelationshipPriority determines sort order in the related-types list.
type RelationshipPriority int

const (
    PriorityP0         RelationshipPriority = iota // Critical relationships
    PriorityP1                                      // Important relationships
    PriorityP2                                      // Secondary relationships
    PriorityCloudTrail                              // Always sorted last
)

// RelatedViewDef describes a single relationship from a source to a target type.
type RelatedViewDef struct {
    // TargetType is the ShortName of the related resource type (e.g., "sg").
    TargetType string
    // DisplayName overrides the target's Name in the related-types list.
    // Empty means use the target ResourceTypeDef.Name or child type name.
    DisplayName string
    // Category determines whether an inline count is shown.
    Category RelationshipCategory
    // Priority determines sort position (P0 first, CloudTrail last).
    Priority RelationshipPriority
}

// RelatedCheckResult contains the result of a background availability check.
type RelatedCheckResult struct {
    Available bool
    Count     int      // -1 means "available but no count shown"
    IDs       []string // optional: known target resource IDs (forward relationships)
}

// RelatedChecker checks whether related resources exist for a source resource.
// For forward relationships, this parses Fields (no API call).
// For reverse/algorithmic, this makes API calls.
type RelatedChecker func(ctx context.Context, clients interface{}, source Resource) (RelatedCheckResult, error)

// RelatedFetcher retrieves the actual related resources for navigation.
// Called when the user presses Enter on a related type in the right column.
// Returns full Resource objects suitable for display in a ResourceListModel.
type RelatedFetcher func(ctx context.Context, clients interface{}, source Resource) ([]Resource, error)
```

### 1.2 Navigable Field Definitions

Navigable field detection is registry-driven. Each resource type registers
which fields in its detail view are navigable and what target resource type
they point to. This is pure data -- no per-resource code in the detail view.

```go
// internal/resource/related.go

// NavigableFieldDef describes a field in a resource's detail view that can
// be navigated to (Enter opens the target resource's detail).
type NavigableFieldDef struct {
    // FieldPath is the dot-separated path in the detail view key labels.
    // Examples: "VpcId", "SecurityGroups.GroupId", "IamInstanceProfile.Arn"
    // For array items, the path matches the leaf field name.
    FieldPath string
    // TargetType is the ShortName of the resource this field points to.
    TargetType string
    // IDExtract describes how to get the target resource ID from the field value.
    // "direct" means the field value IS the ID (most common: vpc-xxx, sg-xxx).
    // "arn-resource" means extract the resource portion of an ARN.
    IDExtract string // "direct" | "arn-resource"
}

// navigableFieldRegistry maps source type ShortName to its navigable fields.
var navigableFieldRegistry = map[string][]NavigableFieldDef{}

// RegisterNavigableFields registers which detail-view fields are navigable
// for a given source resource type. Called from init() in {source}_related.go.
func RegisterNavigableFields(sourceType string, fields []NavigableFieldDef) {
    navigableFieldRegistry[sourceType] = fields
}

// GetNavigableFields returns the navigable field definitions for a resource type.
func GetNavigableFields(sourceType string) []NavigableFieldDef {
    return navigableFieldRegistry[sourceType]
}

// IsFieldNavigable checks if a specific field path is navigable for a source type.
// Returns the NavigableFieldDef if found, nil otherwise.
func IsFieldNavigable(sourceType, fieldPath string) *NavigableFieldDef {
    for i, f := range navigableFieldRegistry[sourceType] {
        if f.FieldPath == fieldPath {
            return &navigableFieldRegistry[sourceType][i]
        }
    }
    return nil
}
```

### 1.3 Related View Registry

```go
// internal/resource/related.go

// relatedEntry bundles a definition with its checker and fetcher.
type relatedEntry struct {
    Def     RelatedViewDef
    Check   RelatedChecker
    Fetch   RelatedFetcher // nil for CloudTrail (special navigation)
}

// relatedRegistry maps source type ShortName to its related entries.
var relatedRegistry = map[string][]relatedEntry{}

// RegisterRelated adds a related view entry for a source resource type.
// Called from init() in each aws/{source}_related.go file.
func RegisterRelated(sourceType string, def RelatedViewDef, check RelatedChecker, fetch RelatedFetcher) {
    relatedRegistry[sourceType] = append(relatedRegistry[sourceType], relatedEntry{
        Def: def, Check: check, Fetch: fetch,
    })
}

// GetRelatedDefs returns all related view definitions for a source type.
// Returns nil if no related views are registered.
func GetRelatedDefs(sourceType string) []RelatedViewDef {
    entries := relatedRegistry[sourceType]
    if len(entries) == 0 {
        return nil
    }
    defs := make([]RelatedViewDef, len(entries))
    for i, e := range entries {
        defs[i] = e.Def
    }
    return defs
}

// GetRelatedChecker returns the checker for a specific source->target relationship.
func GetRelatedChecker(sourceType, targetType string) RelatedChecker {
    for _, e := range relatedRegistry[sourceType] {
        if e.Def.TargetType == targetType {
            return e.Check
        }
    }
    return nil
}

// GetRelatedFetcher returns the fetcher for a specific source->target relationship.
func GetRelatedFetcher(sourceType, targetType string) RelatedFetcher {
    for _, e := range relatedRegistry[sourceType] {
        if e.Def.TargetType == targetType {
            return e.Fetch
        }
    }
    return nil
}

// GetRelatedEntries returns the full entries (def+check+fetch) for a source type.
// Used by the detail view to construct the right column with checkers.
func GetRelatedEntries(sourceType string) []relatedEntry {
    return relatedRegistry[sourceType]
}

// UnregisterRelated removes all related entries for a source type. Tests only.
func UnregisterRelated(sourceType string) {
    delete(relatedRegistry, sourceType)
}
```

### 1.4 Helper Constructors

Common relationship patterns get pre-built checker/fetcher constructors
to minimize per-resource boilerplate:

```go
// internal/resource/related_helpers.go

// ForwardSingleField creates a checker for a forward relationship
// where one target ID is in a known Fields key.
func ForwardSingleField(fieldKey string) RelatedChecker {
    return func(_ context.Context, _ interface{}, source Resource) (RelatedCheckResult, error) {
        id := source.Fields[fieldKey]
        if id == "" {
            return RelatedCheckResult{Available: false}, nil
        }
        return RelatedCheckResult{Available: true, Count: 1, IDs: []string{id}}, nil
    }
}

// ForwardCommaSeparated creates a checker for a forward relationship
// where multiple target IDs are comma-separated in a Fields key.
func ForwardCommaSeparated(fieldKey string) RelatedChecker {
    return func(_ context.Context, _ interface{}, source Resource) (RelatedCheckResult, error) {
        val := source.Fields[fieldKey]
        if val == "" {
            return RelatedCheckResult{Available: false}, nil
        }
        ids := strings.Split(val, ", ")
        return RelatedCheckResult{Available: true, Count: len(ids), IDs: ids}, nil
    }
}

// ForwardFromID creates a checker that uses Resource.ID as the target ID.
func ForwardFromID() RelatedChecker {
    return func(_ context.Context, _ interface{}, source Resource) (RelatedCheckResult, error) {
        if source.ID == "" {
            return RelatedCheckResult{Available: false}, nil
        }
        return RelatedCheckResult{Available: true, Count: 1, IDs: []string{source.ID}}, nil
    }
}

// AlwaysAvailable creates a checker that always reports available.
// Used for CloudTrail (every resource gets it).
func AlwaysAvailable() RelatedChecker {
    return func(_ context.Context, _ interface{}, _ Resource) (RelatedCheckResult, error) {
        return RelatedCheckResult{Available: true, Count: -1}, nil
    }
}
```

### 1.5 CloudTrail as Universal Relationship

Every resource type gets CloudTrail Events as the last entry in the right
column. This is NOT registered per-resource -- it is injected automatically
by the detail view when building the right column item list.

```go
// In detail view right-column construction:
// Append CloudTrail as the last item (always present for every resource)
items = append(items, relatedTypeItem{
    def: resource.RelatedViewDef{
        TargetType:  "ct-events",
        DisplayName: "CloudTrail Events",
        Category:    resource.RelCloudTrail,
        Priority:    resource.PriorityCloudTrail,
    },
    // CloudTrail is always available; navigation opens ct-search
})
```

This means NO resource type needs to register CloudTrail explicitly. The
detail view handles it universally. When CloudTrail is selected and Enter is
pressed, the detail view emits a CloudTrail-specific navigation message.

### 1.6 Relationship Categories and Display Rules

| Category | Check Method | Show Count? | Count Source |
|----------|-------------|-------------|-------------|
| `RelForward` | Parse Fields (no API) | Yes `(N)` | IDs in Fields |
| `RelReverse` | API call | No | N/A |
| `RelAlgorithmic` | API call / naming convention | No | N/A |
| `RelCloudTrail` | Always available | No | N/A |

**Architect decides per relationship:**
- Each `RelatedViewDef` in the per-resource research docs includes a
  Priority (P0/P1/P2) and a "How to Find" description
- The architect maps "How to Find" to a Category:
  - "Parse from response fields" -> `RelForward`
  - "Must query another service" -> `RelReverse`
  - "Naming convention / multi-hop" -> `RelAlgorithmic`

---

## 2. Cheap vs. Expensive Classification

### Decision Matrix

The architect classifies each relationship at design time. The
classification is encoded in `RelationshipCategory` and is immutable
at runtime.

**Cheap (RelForward) -- show inline count `(N)`:**
- Source resource's API response contains the target's ID(s)
- Checker parses `Resource.Fields` -- zero API calls
- Example: EC2 -> Security Groups (IDs in `security_groups` field)
- Example: EC2 -> VPC (ID in `vpc_id` field)

**Expensive (RelReverse, RelAlgorithmic) -- no count shown:**
- Must query another AWS service to determine existence
- Checker makes API call(s) -- latency varies
- Example: EC2 -> Target Groups (must iterate TGs)
- Example: Lambda -> CloudWatch Log Group (naming convention lookup)

**Why not show counts for expensive lookups?**
- Reverse lookups may be slow (seconds per check)
- The count for reverse lookups isn't meaningful for inline display
  because it may change between check time and navigation time
- Keeping expensive rows count-free sets user expectations correctly

### Cheap Count Display Format

Inline count appears immediately after the display name with a space:

```
Security Groups (3)
VPC (1)
Subnet (1)
```

Count = -1 (from AlwaysAvailable) means the row shows no count but
renders as available (normal text, cursor can land).

---

## 3. Background Availability Checking

### 3.1 Session-Scoped Cache

Availability results are cached in memory for the session. No disk
persistence -- related availability is per-resource-instance and too
granular for disk caching.

```go
// On root Model:
type relatedCacheEntry struct {
    Available bool
    Count     int
    IDs       []string // for forward: known target IDs
}

// Cache key: "{source_type}:{source_id}:{target_type}"
relatedCache map[string]relatedCacheEntry
```

**Cache behavior:**
- **Hit**: Return cached result immediately (row shows available/dim instantly)
- **Miss**: Start background check, row starts dim
- **Profile/region switch**: Clear entire related cache
- **ctrl+r in detail view**: Clear cache entries for current source resource,
  restart all checks
- **TTL**: None -- session-scoped means cache dies with the process

### 3.2 Check Dispatch Pattern

Background checks are dispatched as `tea.Cmd` when the detail view is
entered (pushed onto the view stack). The detail view's Init() or the
app-level handler that pushes the detail view starts the checks.

```
1. Detail view entered for resource X
2. For each RelatedViewDef registered for X's type:
   a. Check cache -> if hit, set availability immediately
   b. If miss and forward: run synchronously (no API call)
   c. If miss and reverse/algorithmic: add to check queue
3. Fire first N concurrent probes (N=4, see section 3.9)
4. Each probe completion:
   a. Update cache
   b. Send RelatedTypeCheckedMsg to update right column
   c. Fire next queued probe
5. All probes done -> all rows resolved
```

### 3.3 Generation Counter

A generation counter prevents stale check results from applying:

```go
// On root Model:
relatedCheckGen int // incremented when entering a detail view for a different resource

// Each RelatedTypeCheckedMsg carries the gen. Results with stale gen are dropped.
```

Incrementing gen happens:
- When entering a detail view for a DIFFERENT resource
- When ctrl+r refreshes the current detail view
- On profile/region switch (which clears the cache anyway)

### 3.4 Forward Checks Are Synchronous

Forward relationship checkers parse `Resource.Fields` with zero API calls.
They complete instantly. The check dispatch can run them synchronously
during right-column construction (no need for async tea.Cmd):

```go
// When building right column items:
for _, entry := range entries {
    if entry.Def.Category == resource.RelForward {
        result, _ := entry.Check(ctx, nil, source) // no clients needed
        item.SetAvailability(result.Available, result.Count)
        cache[cacheKey] = result
    } else {
        // Queue for background check
        checkQueue = append(checkQueue, entry)
    }
}
```

This means forward relationships show their counts immediately when the
detail view opens. Reverse/algorithmic rows start dim and light up as
background checks complete.

### 3.5 Message Types for Background Checking

```go
// internal/tui/messages/messages.go -- add:

// RelatedTypeCheckedMsg reports one related type's background check result.
// This is a NEW type, distinct from AvailabilityCheckedMsg (which is for
// main-menu availability probes). The fields are different: this includes
// SourceID for cache keying and does not include Truncated.
type RelatedTypeCheckedMsg struct {
    SourceType string // source resource ShortName
    SourceID   string // source resource ID (for cache key)
    TargetType string // target resource ShortName
    Available  bool
    Count      int
    IDs        []string // optional: target resource IDs
    Err        error    // non-nil means check failed -- row stays dim
    Gen        int      // generation counter -- drop if stale
}
```

### 3.6 Ctrl+R Refresh Behavior

When the active view is a detail view and ctrl+r is pressed:

1. Re-fetch the source resource detail
2. Increment `relatedCheckGen`
3. Clear related cache entries for the current source resource
4. Set all right-column rows to dim (unchecked)
5. Restart all checks from scratch

This is handled in `handleRefresh()` in `app_handlers.go`, adding
a case for `*views.DetailModel` that includes related-check restart.

### 3.7 Rate Limiting and Throttle Protection

**Problem**: Related-view background checks fire multiple API calls
against potentially the same AWS service. For example, EC2-related checks
might issue `DescribeInstances` (for ASG reverse), `DescribeSecurityGroups`
(for SG-referencing reverse), and `DescribeAlarms` (for CW reverse) -- all
within seconds. With rapid navigation (esc/enter/esc/enter), these calls
stack up and risk hitting AWS API rate limits.

Note: `RetryOnThrottle` exists in `internal/aws/retry.go` but is currently
NOT used by any fetcher or availability probe (tracked as bug #186).
Related views are the first feature that makes this risk real, because
they fire multiple concurrent checks against the same account.

**Protection layers (mandatory for all reverse/algorithmic checkers):**

#### Layer 1: Concurrency cap

Batch dispatch with max 4 concurrent probes. This is specified in
section 3.9 but is critical for rate limiting:

```go
// Fire first batch of concurrent probes (up to 4)
for i := 0; i < 4 && len(checkQueue) > 0; i++ {
    entry := checkQueue[0]
    checkQueue = checkQueue[1:]
    cmds = append(cmds, m.probeRelatedType(entry, gen))
}
// Next check fires when a RelatedTypeCheckedMsg arrives
```

#### Layer 2: Per-check timeout

Every reverse/algorithmic check runs with a context timeout. A slow or
throttled check must not block the entire queue:

```go
func (m *Model) probeRelatedType(entry checkQueueEntry, gen int) tea.Cmd {
    clients := m.clients
    return func() tea.Msg {
        ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
        defer cancel()

        checker := resource.GetRelatedChecker(entry.sourceType, entry.targetType)
        if checker == nil {
            return RelatedTypeCheckedMsg{..., Err: fmt.Errorf("no checker")}
        }
        result, err := checker(ctx, clients, entry.source)
        // ...
    }
}
```

#### Layer 3: RetryOnThrottle wrapping (MANDATORY for reverse/algorithmic)

All reverse and algorithmic checkers MUST wrap their API calls in
`RetryOnThrottle`. This is enforced by the skill -- the architect must
verify during code review.

```go
// CORRECT: reverse checker with throttle protection
func checkEC2ForSG(ctx context.Context, clients interface{}, source resource.Resource) (resource.RelatedCheckResult, error) {
    c := clients.(*ServiceClients)
    result, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*ec2.DescribeInstancesOutput, error) {
        return c.EC2.DescribeInstances(ctx, &ec2.DescribeInstancesInput{
            Filters: []ec2types.Filter{
                {Name: aws.String("instance.group-id"), Values: []string{source.ID}},
            },
            MaxResults: aws.Int32(5), // only need existence + count, not all instances
        })
    })
    if err != nil {
        return resource.RelatedCheckResult{}, err
    }
    count := 0
    for _, r := range result.Reservations {
        count += len(r.Instances)
    }
    return resource.RelatedCheckResult{Available: count > 0, Count: count}, nil
}

// WRONG: no throttle protection -- will fail under load
func checkEC2ForSG_BAD(ctx context.Context, clients interface{}, source resource.Resource) (resource.RelatedCheckResult, error) {
    c := clients.(*ServiceClients)
    result, err := c.EC2.DescribeInstances(ctx, &ec2.DescribeInstancesInput{...})
    // ...
}
```

#### Layer 4: MaxResults / early termination for existence checks

Checkers only need to know IF related resources exist and optionally
HOW MANY. They do NOT need to fetch all data. Use `MaxResults` or
equivalent pagination limits to minimize API data transfer:

```go
// CORRECT: check existence with minimal data
input := &ec2.DescribeInstancesInput{
    Filters: []ec2types.Filter{{Name: aws.String("vpc-id"), Values: []string{vpcID}}},
    MaxResults: aws.Int32(5), // enough to confirm existence + rough count
}

// WRONG: fetch all instances just to count
input := &ec2.DescribeInstancesInput{
    Filters: []ec2types.Filter{{Name: aws.String("vpc-id"), Values: []string{vpcID}}},
    // no MaxResults -- could return thousands
}
```

For checkers where the API doesn't support `MaxResults`, fetch ONE page
and treat the count as approximate. The fetcher (called on Enter) does
the full retrieval.

#### Layer 5: Graceful degradation

A throttled check (after all retries exhausted) results in the row
staying dim -- same visual treatment as "no related resources found."
The user sees no error. They can ctrl+r to retry later when rate limits
have cooled down.

```go
// In handleRelatedTypeChecked:
if msg.Err != nil {
    // Row stays dim -- no flash, no error visible to user
    return m, nextProbeCmd
}
```

#### Architect verification checklist (per resource)

When the architect scopes related views for a resource type, they MUST:

1. **Count the reverse/algorithmic checks** -- if > 5, consider ordering
   by priority (P0 checks first) so the most useful rows light up fastest
2. **Identify same-service collisions** -- if 3+ checks hit the same AWS
   service API (e.g., 3 checks all call EC2 DescribeInstances), document
   this and consider staggering
3. **Verify MaxResults** -- for each reverse checker, confirm the API
   supports MaxResults or equivalent and the checker uses it
4. **Verify RetryOnThrottle** -- every reverse/algorithmic checker must be
   wrapped. This is a code review gate, not optional.
5. **Document API call count** -- the architect handoff must state the
   total number of API calls per detail-view entry (forward: 0, reverse: 1+ each)

### 3.8 Rate Limiting Test Requirements

QA MUST write throttle-resilience tests for every reverse/algorithmic
checker. These tests are in addition to the standard checker tests.

```go
// Required test: checker retries on throttle and succeeds
func TestRelated_{Source}_{Target}_ThrottleThenSuccess(t *testing.T) { ... }

// Required test: checker degrades gracefully after max retries
func TestRelated_{Source}_{Target}_ThrottleExhausted(t *testing.T) { ... }

// Required test: checker respects context timeout
func TestRelated_{Source}_{Target}_ContextTimeout(t *testing.T) { ... }
```

These tests use a `callFn` mock pattern that allows per-call behavior.
See skill `a9s-add-related-view` for full templates.

### 3.9 Concurrency Limit

Max 4 parallel probes. This is higher than the main-menu pattern (3)
because related checks are typically lighter (MaxResults=5) and the user
is focused on a single resource's detail view, not switching rapidly.

The limit applies to the total number of in-flight background checks
for one resource's detail view. When a check completes, the next queued
check fires immediately.

---

## 4. Detail View Refactor: Field-List Model

**This section replaces the current viewport-based detail view with a
field-list model. This is the foundation that ALL subsequent sections
depend on.**

### 4.1 Current State

The current `DetailModel` uses `bubbles/viewport` -- a scrolling text
display with no concept of "selected field." Content is rendered as a
single string and set on the viewport. The viewport handles scroll.
There is no cursor, no per-field interaction.

### 4.2 New Model: Field List

The detail view becomes a **field list** where each field is a selectable
item. The viewport is removed. The detail model manages its own scroll
position, cursor, and per-field rendering.

```go
// internal/tui/views/detail.go (refactored)

type fieldRow struct {
    key         string   // field key label (e.g., "VpcId:")
    value       string   // field value (e.g., "vpc-0aaa111bbb222cc")
    isHeader    bool     // true for section headers (e.g., "State:", "Tags:")
    isSubField  bool     // true for indented sub-fields
    navigable   *resource.NavigableFieldDef // non-nil if this field is navigable
    wrapLines   int      // number of lines this field occupies after word wrap
}

type DetailModel struct {
    res          resource.Resource
    resourceType string
    viewConfig   *config.ViewsConfig
    fields       []fieldRow        // computed from resource + config
    cursor       int               // index into fields[]
    scrollOffset int               // first visible field index
    width        int
    height       int
    keys         keys.Map
    search       SearchModel

    // Two-column state
    rightColumn    rightColumnModel  // embedded sub-component
    showRight      bool              // r toggle state (persists across navigation)
    focusRight     bool              // true when right column has focus
}
```

### 4.3 Field Row Construction

Fields are built once (on SetSize or resource change) from the resource's
RawStruct using the same rendering pipeline as today (`renderFromConfig`
and `renderFromReflection`), but instead of producing a single string,
they produce `[]fieldRow`. Each row carries metadata about navigability.

```go
func (m *DetailModel) buildFieldRows() {
    m.fields = nil
    navigableDefs := resource.GetNavigableFields(m.resourceType)

    // Walk the rendered key-value pairs (same order as current detail view)
    for _, kv := range m.renderKVPairs() {
        row := fieldRow{
            key:       kv.Key,
            value:     kv.Value,
            isHeader:  kv.IsHeader,
            isSubField: kv.IsSubField,
        }
        // Check navigability against registry
        for _, nav := range navigableDefs {
            if matchesFieldPath(kv.FieldPath, nav.FieldPath) {
                row.navigable = &nav
                break
            }
        }
        // Compute word-wrap line count
        row.wrapLines = computeWrapLines(row, m.leftColumnWidth())
        m.fields = append(m.fields, row)
    }
}
```

### 4.4 Cursor and Scroll

The cursor moves over ALL rows (navigable and non-navigable alike).
j/k moves one row at a time. Enter acts only on navigable rows.

**Scroll-to-ensure-visible:** When cursor moves (j/k, n/N, g/G, pgup/pgdn),
the scroll offset adjusts to keep the cursor row fully visible. This
accounts for variable-height rows (word wrap):

```go
func (m *DetailModel) ensureCursorVisible() {
    // Sum heights of rows [0..cursor-1] to get cursor's pixel line
    cursorLine := 0
    for i := 0; i < m.cursor; i++ {
        cursorLine += m.fields[i].wrapLines
    }
    cursorHeight := m.fields[m.cursor].wrapLines

    // Adjust scrollOffset so cursor is visible
    if cursorLine < m.scrollOffset {
        m.scrollOffset = cursorLine
    }
    if cursorLine + cursorHeight > m.scrollOffset + m.visibleHeight() {
        m.scrollOffset = cursorLine + cursorHeight - m.visibleHeight()
    }
}
```

### 4.5 Word Wrap

Word wrap is always ON. There is no `w` toggle. Long values wrap to the
next visual line. The wrapped continuation is NOT a separate selectable
row -- it is part of the same field row. The cursor highlight covers all
wrapped lines of the selected field.

The YAML view (`y`) serves the "see raw untruncated data" use case.

### 4.6 View Rendering

The field list renders visible rows based on scrollOffset and
visibleHeight. Each row is rendered as key + value, with styles
applied per field type:

| Field Type | Key Style | Value Style | Enter Action |
|------------|-----------|-------------|-------------|
| Plain scalar | `#7aa2f7` | `#c0caf5` | No-op |
| Navigable scalar | `#7aa2f7` | `#7aa2f7` underline | Open target detail |
| Section header | `#e0af68` bold | -- | No-op |
| Sub-field plain | `#7aa2f7` | `#c0caf5` | No-op |
| Sub-field navigable | `#7aa2f7` | `#7aa2f7` underline | Open target detail |
| Array item navigable | `#7aa2f7` | `#7aa2f7` underline | Open target detail |

The cursor highlight overrides all styles when on that row:
full-width `#7aa2f7` background, `#1a1b26` foreground, bold.

When the cursor is on a navigable field, the underline disappears
(the full-row selection highlight takes over).

---

## 5. Two-Column Layout Infrastructure

### 5.1 Column Dimensions

Inside the detail frame, content is divided by a vertical separator:

```
+---------- detail -- i-0abc123 (web-prod) ----------+
|  LEFT COLUMN (detail fields)  |  RIGHT COLUMN      |
|                               |  (related types)   |
+----------------------------------------------------+
```

- **Right column**: fixed 32 characters (accommodates longest name
  "CloudFormation Stacks (12)" = 28 chars + padding)
- **Separator**: 1 character (`|` U+2502)
- **Left column**: remaining width (total inner width - 33)

The top and bottom borders span the full width unbroken. The vertical
separator appears only in content rows, not in the borders.

### 5.2 Column Separator

A single thin vertical line (`|` U+2502) that changes color based on
which column is focused:

- **Left column focused:** separator in dim color `#414868`
- **Right column focused:** separator in accent color `#7aa2f7`

This is a 1-character-wide visual cue that costs no screen real estate.

### 5.3 Focus Switching

| Key | Action |
|-----|--------|
| `Tab` | Switch focus between left and right columns |
| `Shift+Tab` | Switch focus (same as Tab since only 2 columns) |
| `h` | Focus left column |
| `l` | Focus right column |

`h`/`l` acts as column switching (NOT horizontal scrolling, since word
wrap is always on). This reuses the `ScrollLeft`/`ScrollRight` bindings
with column-switch semantics in the detail view context.

### 5.4 Narrow Terminal: Stacked Layout

**Threshold: 100 columns.**

| Width | Layout |
|-------|--------|
| 100+ cols | Two columns side by side |
| 60-99 cols | Stacked: detail on top, related list below |
| < 60 cols | "Terminal too narrow" |

Below 100 columns, the right column moves BELOW the detail fields in a
stacked layout. A dim section separator divides them:

```
| PrivateIpAddress: 10.0.48.175                               |
| ...                                                          |
|                                                              |
| -- Related ------------------------------------------------- |
|                                                              |
|   Security Groups (3)                                        |
|   VPC (1)                                                    |
```

In stacked mode, Tab switches between the detail section and the related
section. j/k moves within whichever section has focus.

### 5.5 State Preservation on Layout Transition

When `WindowSizeMsg` crosses the 100-col threshold (switching between
stacked and two-column layouts), ALL state is preserved:

- Cursor positions for both columns
- Search highlights and match index
- Filter query and narrowed list
- Focused column
- Scroll offsets

The focused column remains focused in both layouts.

---

## 6. Right Column Component

**The right column is NOT a separate view pushed on the stack. It is a
sub-component embedded inside the DetailModel.**

### 6.1 Struct

```go
// internal/tui/views/detail.go (or a separate detail_related.go)

type relatedTypeItem struct {
    def       resource.RelatedViewDef
    available bool
    checked   bool
    count     int // meaningful only when Category == RelForward
}

type rightColumnModel struct {
    items         []relatedTypeItem
    filteredItems []relatedTypeItem // nil when no filter active
    filterText    string
    cursor        int     // index into visible items (skips dim rows)
    scrollOffset  int
    width         int
    height        int
}
```

### 6.2 Behavior

- Flat list with dim unavailable rows
- Cursor skips dim (unavailable/unchecked) rows
- Background availability checking triggered when the detail view is
  entered (NOT on `r` press -- `r` only toggles visibility)
- Session-scoped caching via `relatedCache` on root model
- `ctrl+r` clears cache and re-checks
- `r` toggles visibility (default ON), state persists across navigation

### 6.3 Row States

| State | Visual | Cursor |
|-------|--------|--------|
| Available (count known) | Normal text `#c0caf5` with `(N)` | Selectable |
| Available (no count) | Normal text `#c0caf5` | Selectable |
| Unavailable | Dim text `#565f89` | Cursor skips |
| Selected | Full-width highlight `#7aa2f7` bg | Current row |
| Checking (initial load) | Dim text `#565f89` | Cursor skips |

Rows start dim and silently "light up" as background checks complete.
Same silent-loading pattern as the main menu -- no spinners, no
"Checking..." text.

### 6.4 Sort Order

1. **P0** relationships -- alphabetical
2. **P1** relationships -- alphabetical
3. **P2** relationships -- alphabetical
4. **CloudTrail Events** -- always last

### 6.5 Enter Behavior

| Condition | Action |
|-----------|--------|
| Count = 1 | Open target resource detail directly |
| Count > 1 | Open filtered resource list |
| CloudTrail Events | Open ct-search pre-filtered for this resource |
| Dim row | No-op (cursor cannot land here) |

### 6.6 Right Column Scroll

If the right column has more rows than the visible height, it scrolls
independently when focused. A dim scroll indicator appears: `v N more`
(bottom) or `^ N more` (top).

### 6.7 SetAvailability

The root model calls this when a `RelatedTypeCheckedMsg` arrives:

```go
func (m *rightColumnModel) SetAvailability(targetType string, available bool, count int) {
    for i, item := range m.items {
        if item.def.TargetType == targetType {
            m.items[i].available = available
            m.items[i].count = count
            m.items[i].checked = true
            return
        }
    }
}
```

### 6.8 Right Column Hidden

When `r` toggles the right column off:
- Left column expands to full width (same as today's detail view)
- Tab does nothing (only one column)
- The cursor stays in the left column
- All field-navigation behavior is unchanged
- Press `r` again to restore the right column

---

## 7. Navigable Field Detection

### 7.1 Registry-Driven Detection

Navigable field detection is driven by `NavigableFieldDef` registrations
(section 1.2). This is pure data -- the detail view does NOT contain
per-resource logic to identify navigable fields.

Each resource type registers its navigable fields in the same
`init()` function where it registers related views:

```go
// In internal/aws/ec2_related.go init():
resource.RegisterNavigableFields("ec2", []resource.NavigableFieldDef{
    {FieldPath: "VpcId", TargetType: "vpc", IDExtract: "direct"},
    {FieldPath: "SubnetId", TargetType: "subnets", IDExtract: "direct"},
    {FieldPath: "SecurityGroups.GroupId", TargetType: "sg", IDExtract: "direct"},
    {FieldPath: "ImageId", TargetType: "ami", IDExtract: "direct"},
    {FieldPath: "IamInstanceProfile.Arn", TargetType: "iam_roles", IDExtract: "arn-resource"},
    {FieldPath: "BlockDeviceMappings.Ebs.VolumeId", TargetType: "ebs", IDExtract: "direct"},
    {FieldPath: "NetworkInterfaces.NetworkInterfaceId", TargetType: "eni", IDExtract: "direct"},
})
```

### 7.2 Styling

Navigable field VALUES (not keys) are rendered with underline styling in
the accent color `#7aa2f7`. Only the value portion is underlined.

When the cursor is on a navigable field, the underline disappears (the
full-row selection highlight takes over). The underline is only visible
on navigable fields that are NOT currently selected.

### 7.3 Enter on Navigable Field

When Enter is pressed on a navigable field:

1. Extract the target resource ID from the field value using the
   `IDExtract` pattern from `NavigableFieldDef`
2. The detail view emits a message that the root model handles to push
   the target resource's detail view

```go
// Detail view emits:
return m, func() tea.Msg {
    return messages.NavigateToRelatedMsg{
        TargetType:     navDef.TargetType,
        SourceResource: m.res,
        SourceType:     m.resourceType,
        Count:          1, // navigable field = single resource
        TargetID:       extractID(fieldValue, navDef.IDExtract),
    }
}
```

### 7.4 Forward Relationships: Left Column Only

Forward relationships are visible as navigable fields in the left column.
They do NOT appear in the right column. This eliminates duplication:

- **Left column** = "what does this resource point to" (navigable underlined fields)
- **Right column** = "what points to this resource" (reverse, algorithmic, CloudTrail)

---

## 8. Key Binding Changes

### 8.1 New Bindings

```go
// internal/tui/keys/keys.go -- add to Map struct:
ToggleRelated  key.Binding // r in detail view context
StackResources key.Binding // R for CFN stack resources (was r)

// Add to Default():
ToggleRelated:  key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "toggle related")),
StackResources: key.NewBinding(key.WithKeys("R"), key.WithHelp("R", "resources")),
```

### 8.2 Modified Bindings

```go
// Resources changes from "r" to "R":
Resources: key.NewBinding(key.WithKeys("R"), key.WithHelp("R", "resources")),
```

### 8.3 Key Conflict Resolution

| Physical Key | Binding | Active Context | Notes |
|-------------|---------|---------------|-------|
| `r` | `ToggleRelated` | Detail view | NEW: toggle right column |
| `r` | `Resources` | Resource list view | REMOVED: was `r`, now `R` |
| `R` | `Resources` | Resource list view | MOVED from `r` |
| `R` | `StackResources` | CFN stack detail view | NEW: replaces old `r` |

No code conflict because detail view and resource list view are mutually
exclusive -- only one is active at a time.

### 8.4 CFN ChildViewDef Change

```go
// In internal/resource/types_cicd.go, cfn Children:
{ChildType: "cfn_resources", Key: "R", ...}  // was Key: "r"
```

### 8.5 h/l Column Switching

`h`/`l` reuses `ScrollLeft`/`ScrollRight` bindings with column-switch
semantics in the detail view. Since word wrap is always on, there is no
horizontal scrolling in the detail view -- `h`/`l` always means column
switching.

The detail view's `Update()` checks for `ScrollLeft`/`ScrollRight` and
interprets them as focus-left/focus-right when two columns are visible.

### 8.6 `/` Dual Binding

`/` is bound to both `Filter` and `Search` in `keys.Map`. The detail
view's `Update()` must branch on the focused column BEFORE key matching:

- Left column focused -> dispatch to `Search` handler (text search)
- Right column focused -> dispatch to `Filter` handler (list filter)

This is the same pattern used for `c` (copy) which behaves differently
per column.

### 8.7 Full Key Table: Detail View (Two-Column)

| Key | Left Column Focused | Right Column Focused |
|-----|--------------------|--------------------|
| `j`/`down` | Move field cursor down | Move related type cursor down |
| `k`/`up` | Move field cursor up | Move related type cursor up |
| `g` | Jump to first field | Jump to first available type |
| `G` | Jump to last field | Jump to last available type |
| `Enter` | Open navigable field target | Open related type (smart) |
| `Tab` | Switch to right column | Switch to left column |
| `h` | No-op (already left) | Switch to left column |
| `l` | Switch to right column | No-op (already right) |
| `r` | Toggle right column off | Toggle right column off |
| `y` | Switch to YAML view | Switch to YAML view |
| `c` | Copy field value | Copy type name |
| `/` | Text search | List filter |
| `n` | Next search match | No-op |
| `N` | Previous search match | No-op |
| `esc` | Clear search / go back | Clear filter / go back |
| `?` | Help | Help |
| `ctrl+r` | Refresh + re-check | Refresh + re-check |
| `pgup`/`ctrl+u` | Page up | Page up |
| `pgdn`/`ctrl+d` | Page down | Page down |
| `ctrl+c` | Force quit | Force quit |

---

## 9. Message Contracts

### 9.1 New Messages (infrastructure -- ONE TIME)

```go
// internal/tui/messages/messages.go -- add:

// RelatedTypeCheckedMsg reports one related type's background check result.
// Distinct from AvailabilityCheckedMsg: includes SourceID for cache keying,
// does not include Truncated. Used ONLY for related-view background probes.
type RelatedTypeCheckedMsg struct {
    SourceType string
    SourceID   string
    TargetType string
    Available  bool
    Count      int
    IDs        []string
    Err        error
    Gen        int
}

// NavigateToRelatedMsg requests navigation to a related resource.
// Emitted by the detail view when Enter is pressed in EITHER column
// (navigable field in left, or related type in right).
type NavigateToRelatedMsg struct {
    TargetType     string
    TargetID       string            // non-empty for navigable field (left column)
    SourceResource resource.Resource
    SourceType     string
    Count          int  // -1 or 0 = unknown, use fetcher to determine
    IsCloudTrail   bool // true = open ct-search instead of resource list
}
```

### 9.2 App.go Update Switch (infrastructure -- ONE TIME)

```go
// In app.go Update:
case messages.RelatedTypeCheckedMsg:
    return m.handleRelatedTypeChecked(msg)
case messages.NavigateToRelatedMsg:
    return m.handleNavigateToRelated(msg)
```

### 9.3 handleRelatedTypeChecked

When a `RelatedTypeCheckedMsg` arrives:

1. Check generation counter -- drop if stale
2. Update `relatedCache`
3. Forward to the active detail view's right column via
   `rightColumnModel.SetAvailability()`
4. Dispatch next queued check if any

### 9.4 handleNavigateToRelated

When a `NavigateToRelatedMsg` arrives:

```go
func (m Model) handleNavigateToRelated(msg messages.NavigateToRelatedMsg) (tea.Model, tea.Cmd) {
    if msg.IsCloudTrail {
        // Navigate to ct-search filtered by resource (future: #114)
        return m, func() tea.Msg {
            return messages.FlashMsg{Text: "CloudTrail search coming soon"}
        }
    }

    if msg.TargetID != "" {
        // Left-column navigable field: fetch single resource by ID
        return m, m.fetchSingleRelatedByID(msg)
    }

    if msg.Count == 1 {
        // Right-column single: fetch and go directly to detail
        return m, m.fetchSingleRelatedResource(msg)
    }

    // Right-column multiple: push resource list, then fetch
    return m.pushRelatedResourceList(msg)
}
```

### 9.5 Message Flow Diagram

```
Detail view entered for resource X
    |
    v
buildRightColumn() -- forward checks synchronous, reverse queued
    |
    v
dispatchRelatedChecks() -- up to 4 concurrent tea.Cmd
    |
    v
Background check completes
    | produces RelatedTypeCheckedMsg
    v
app.Update -> handleRelatedTypeChecked()
    | updates cache
    | calls rightColumn.SetAvailability()
    | dispatches next check if queued
    |
    v
User presses Enter on navigable field (left column)
    |
    v
detail.Update -> produces NavigateToRelatedMsg{TargetID: "vpc-xxx"}
    |
    v
app.Update -> handleNavigateToRelated()
    | fetches resource by ID -> pushes vpc-detail

User presses Enter on related type (right column)
    |
    v
detail.Update -> produces NavigateToRelatedMsg{Count: N}
    |
    v
app.Update -> handleNavigateToRelated()
    |-- count=1: fetch single -> push detail
    |-- count>1: push ResourceList -> fetch -> ResourcesLoadedMsg
    +-- CloudTrail: navigate to ct-search
```

### 9.6 Zero Per-Resource Changes to app.go/messages.go

The message types are generic:
- `RelatedTypeCheckedMsg` carries source/target type strings
- `NavigateToRelatedMsg` carries target type, source resource, optional target ID

App.go dispatches to the registry:
- `resource.GetRelatedChecker(sourceType, targetType)` for checks
- `resource.GetRelatedFetcher(sourceType, targetType)` for navigation

Adding related views for a new resource type requires ONLY:
1. `resource.RegisterRelated()` calls in init()
2. `resource.RegisterNavigableFields()` calls in init()
3. Checker/Fetcher function implementations

Zero changes to `app.go`, `messages.go`, or `detail.go`.

---

## 10. Search Integration

### 10.1 `/` Behavior Depends on Focused Column

| Focus | `/` behavior | Pattern |
|-------|-------------|---------|
| Left column | Text search (QA-26 style) | Search within field keys and values |
| Right column | List filter (QA-11 style) | Narrow list to matching type names |

### 10.2 Left Column: Text Search

When the left column is focused and `/` is pressed:

1. Header right side changes to `/<cursor>` (amber `#e0af68`, bold)
2. User types a search query
3. Enter confirms the search
4. All matches in field keys AND values are highlighted
5. Match indicator `[1/N matches]` at bottom of left column
6. Cursor jumps to the field row containing the first match
7. `n`/`N` advances/retreats to next/previous match

**Adaptation from viewport search:** The cursor jumps to the matched
row rather than scrolling a viewport offset. This is because the left
column is a field list with per-row cursor, not a free-scrolling viewport.

**Match scope:** Case-insensitive, spans full rendered text of each
field row (key label + value).

**Interaction with navigable fields:** Search highlighting coexists
with navigable-field underline styling. Search highlight (amber/orange
background) takes precedence over the underline. Enter still opens the
target resource regardless of search state.

### 10.3 Right Column: List Filter

When the right column is focused and `/` is pressed:

1. Header right side changes to `/<cursor>` (amber `#e0af68`, bold)
2. Matching happens live as the user types (immediate filtering)
3. Right column narrows to show only matching type names
4. Dim rows that match the filter ARE shown (still dim)
5. Esc clears the filter and restores all rows

### 10.4 Per-Column State Persistence

Search/filter state is per-column and persists across focus switches:

| From | To | Behavior |
|------|----|----------|
| Left (search active) | Right | Highlights persist. `n`/`N` inactive. |
| Right (filter active) | Left | Filter persists. Narrowed list stays. |
| Switch back | Original column | State restored. |

**Header on switch:** When switching from a column with active
search/filter to a column without, the header reverts to `? for help`.
Query text preserved internally but hidden.

### 10.5 Esc Layering

| State | Esc action | Next state |
|-------|-----------|------------|
| Search input active | Cancel search input | Normal mode |
| Search results active | Clear highlights | Normal mode |
| Filter active | Clear filter, restore rows | Normal mode |
| Normal mode | Go back (pop view stack) | Previous view |

---

## 11. Demo Mode

For forward relationships, checkers parse `Resource.Fields` which work
identically in demo mode (demo fixtures populate the same Fields maps).

For reverse relationships, register demo overrides:

```go
// internal/demo/related_{category}.go
func init() {
    resource.RegisterDemoRelatedChecker("ec2", "tg", func(...) (RelatedCheckResult, error) {
        return RelatedCheckResult{Available: true, Count: 2}, nil
    })
}
```

This follows the existing `demo.RegisterChildDemo` pattern and keeps
demo logic out of production code.

---

## 12. File Manifest (Infrastructure)

These files are created/modified ONCE to establish the related-view
infrastructure. No per-resource changes after this.

### New Files

| File | Description |
|------|-------------|
| `internal/resource/related.go` | Types, registries (RelatedViewDef + NavigableFieldDef), helper constructors |
| `internal/resource/related_helpers.go` | ForwardSingleField, ForwardCommaSeparated, etc. |

### Modified Files

| File | Change |
|------|--------|
| `internal/tui/views/detail.go` | **Major refactor**: viewport -> field-list model, two-column layout, embedded rightColumnModel, navigable field rendering, column focus |
| `internal/tui/messages/messages.go` | Add `RelatedTypeCheckedMsg`, `NavigateToRelatedMsg` |
| `internal/tui/keys/keys.go` | Add `ToggleRelated`, `StackResources`; change `Resources` to "R" |
| `internal/tui/app.go` | Add cases in Update switch, add `relatedCache` and `relatedCheckGen` to Model |
| `internal/tui/app_handlers.go` | Add related-check dispatch in detail-view entry, handle `RelatedTypeCheckedMsg`, handle `NavigateToRelatedMsg` |
| `internal/tui/views/help.go` | Update help context for detail view two-column keys |
| `internal/tui/views/resourcelist.go` | Add `SetDisplayName()` method for related-navigation frame titles |
| `internal/resource/types_cicd.go` | CFN child `Key: "r"` -> `Key: "R"` |

### Files NOT Created (v1 architecture -- removed)

These files were in the v1 architecture (separate pushed view) and are
**NOT part of v2** (embedded two-column):

| File | Why not needed |
|------|----------------|
| `internal/tui/views/relatedlist.go` | No separate `RelatedTypesListModel` -- right column is embedded in DetailModel |
| `internal/tui/app_related.go` | Handler logic moves to `app_handlers.go` additions |

### Per-Resource Files (mechanical, per `a9s-add-related-view` skill)

| File | Description |
|------|-------------|
| `internal/aws/{source}_related.go` | init() + `RegisterRelated` + `RegisterNavigableFields` + checkers + fetchers |
| `tests/unit/aws_{source}_related_test.go` | Checker, fetcher, and navigable field tests |
| `internal/demo/related_{category}.go` | Demo mode overrides (if reverse relationships) |

---

## 13. Implementation Phasing

Each phase builds on the previous. The dependency chain is strict.

### Phase 1: Registry Infrastructure (foundation)
1. `resource/related.go` -- types, registries, helpers
2. `resource/related_helpers.go` -- ForwardSingleField, etc.
3. Tests for registry and helpers

### Phase 2: Detail View Refactor (foundation)
1. Replace viewport with field-list model in `detail.go`
2. Per-row cursor, j/k navigation, scroll-to-ensure-visible
3. Variable-height word-wrapped rows (always ON, no wrap toggle)
4. All existing tests must still pass (View() output changes)

### Phase 3: Two-Column Layout Infrastructure
1. Fixed 32-char right column, left takes remaining space
2. Focus-colored separator
3. Tab and h/l column switching
4. Stacked layout below 100 cols
5. State preservation on WindowSizeMsg crossing threshold

### Phase 4: Right Column Component
1. Embedded rightColumnModel in DetailModel
2. Flat list with dim unavailable rows, cursor-skip
3. Background availability checking triggered on detail view entry
4. Session-scoped caching, ctrl+r refresh
5. `r` toggle visibility (default ON)

### Phase 5: Navigable Field Detection
1. NavigableFieldDef registry
2. Underline styling for navigable values
3. Enter on navigable field opens target detail

### Phase 6: Key Binding Changes
1. `ToggleRelated` binding (r in detail view)
2. `StackResources` binding (R for CFN)
3. CFN ChildViewDef `Key: "r"` -> `Key: "R"`
4. h/l column switching via ScrollLeft/ScrollRight
5. `/` dual binding (Search vs. Filter based on focus)

### Phase 7: New Message Types
1. `RelatedTypeCheckedMsg` in messages.go
2. `NavigateToRelatedMsg` in messages.go
3. App.go Update switch cases
4. Handler implementations in app_handlers.go

### Phase 8: Search Integration
1. Left column text search with match highlighting
2. Right column list filter
3. Per-column state persistence
4. Esc layering
5. Header state management

### Phase 9: Skill Update
1. Update `a9s-add-related-view` skill for two-column architecture
2. Ensure skill covers BOTH columns per resource

### Phase 10: Per-Resource Rollout
1. EC2 first (validates pattern end-to-end)
2. VPC, SG, Lambda, RDS (highest-value resources)
3. Remaining resources in batches of 5-10
4. All tasks are Sonnet-executable, mechanical

### Phase 11: CloudTrail Integration (after #114)
1. Wire CloudTrail row's Enter to ct-search view
2. Currently: flash "coming soon"

---

## 14. Mechanization Guarantee

Once infrastructure (Phases 1-8) is complete, per-resource tasks are
pure data registration. Completable by Sonnet reading:
- `a9s-add-related-view` skill
- `docs/design/related-resources/{shortname}.md` research doc
- `.a9s/views_reference.yaml` for field paths

No cross-file reasoning, no interface design, no message changes per
resource. Both columns are driven by registry:
- `NavigableFieldDef` for left column navigability
- `RelatedViewDef` for right column reverse/algorithmic types

**If ANY per-resource task requires architectural judgment, the
infrastructure is incomplete. File an issue and fix the infrastructure
before continuing the rollout.**

---

## 15. Architectural Decisions Log

| Decision | Rationale |
|----------|-----------|
| Keep flat directory structure | Go package design makes folders expensive; agents already use targeted globs |
| Separate registry (not on ResourceTypeDef) | Related defs are numerous (5-20+ per type); co-locating with resolvers in `{source}_related.go` is cleaner |
| Split Checker/Fetcher (not combined) | Checkers are often cheap (parse Fields); fetchers need API calls. Different concerns, different signatures |
| Forward checks run synchronously | No API calls needed -- parse Fields instantly at detail-view entry |
| CloudTrail injected by detail view, not registered | Universal to all types -- registering it 66 times is wasteful and error-prone |
| Session cache only (no disk) | Per-instance data is too granular and volatile for disk persistence |
| Right column embedded in DetailModel, not pushed on stack | Design spec v4.2: two-column detail view, not separate view |
| NavigableFieldDef registry for left-column navigability | Pure data registration per resource, no per-resource code in detail view |
| `r` toggles right column (not opens separate view) | Design spec v4.2: r is a visibility toggle, not a navigation action |
| Background checks on detail-view entry (not r press) | Right column is default-visible; checks must start immediately |
| Max 4 concurrent probes (not 3) | Related checks are lighter than main-menu probes (MaxResults=5) |
| ResourceListModel reused for related navigation | No new list view component -- push ResourceListModel with `SetDisplayName` for the frame title |
| NavigateToRelatedMsg includes TargetID | Left-column navigable fields know the exact target ID; right column may not |
| RetryOnThrottle mandatory for all reverse/algorithmic checkers | Multiple checks can hit the same AWS service API; without retry, throttled checks fail silently (#186) |
| MaxResults on existence checks | Checkers need count, not full data; minimizes API data transfer and reduces throttle risk |
| Throttle tests mandatory per reverse checker | QA must verify retry-then-success, retry-exhausted, and context-timeout for every expensive check |
| Forward relationships in left column ONLY | Avoids duplication; left = "points to", right = "pointed to by" |
| No separate relatedlist.go or app_related.go | Right column is embedded in detail view; handlers go in existing app_handlers.go |
