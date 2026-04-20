---
name: a9s-add-attention-column
description: Recipe for adding a list-view attention column to a resource type — handles Tier A (existing field), Tier B (computed/Wave-2), and the FieldUpdates promotion path. Optimized to avoid the rediscovery overhead that plagued the first Tier A sweep.
disable-model-invocation: true
---

# Adding an Attention Column

The user wants WHY a row is colored visible in the list — not just that it's colored. This skill standardizes the recipe so adding columns is mechanical, not a per-type re-derivation.

## Prerequisites

You MUST have a scoped task from the architect with:
- **ShortName** (e.g. `kms`, `redis`, `cfn`)
- **Column title** (e.g. `Rotation`, `Failover`, `Drift`)
- **Source category** (one of: A, B-fetcher, B-enricher, C-detail-only)
- **Field key OR struct path** for the value
- **Optional**: format/decorator (when raw value isn't user-friendly)
- **AWS docs reference** (the `attention-signals.md` row) for the contract

**If you don't have this, STOP.** Reply with REJECTED and ask for architect scope.

## Source category decision

```
Is the value already on the SDK list-API response (RawStruct path)?
├─ YES → Tier A:    add column with Path: in defaults_*.go
└─ NO →
   Does the fetcher already write it to Fields[]?
   ├─ YES → Tier A:    add column with Key: in defaults_*.go
   └─ NO →
      Can it be computed cheaply per-row at fetch time (Wave-1)?
      ├─ YES → Tier B-fetcher: edit fetcher to write Fields[<key>], then add column with Key:
      └─ NO →
         Does it require an additional AWS API call?
         ├─ YES → Tier B-enricher:
         │        ├─ Edit/add Wave-2 enricher to populate
         │        │  IssueEnricherResult.FieldUpdates[resourceID][<key>]
         │        ├─ Register via registerIssueEnricher(...) in the owning
         │        │  <short>_issue_enrichment.go init() block
         │        └─ Add column with Key: in defaults_*.go
         └─ Multi-line text body? → Tier C: detail-only via DetailField{Key: ..., Label: ...}
```

## Pre-flight checklist (architect does this BEFORE dispatching)

The architect's job is to eliminate per-step rediscovery. Provide:

1. **Exact file paths** the agent will edit
2. **Exact Edit operations** — `old_string` / `new_string` snippets, not prose
3. **SDK enum constants** verified via `go doc <pkg>.<Type>` — not guessed
4. **Existing fake/fixture pattern** — link a sibling test for the agent to mirror
5. **One worked example** in the same file the agent will edit

## Tier A — Path or Key already populated

### File: `internal/config/defaults_<group>.go`

Find the `<shortName>` entry's `List []ListColumn` slice. Insert the new column AFTER the primary status column:

```go
{Title: "<Title>", Path: "<SDKPath>", Width: <N>},     // path-form
{Title: "<Title>", Key: "<field_key>", Width: <N>},    // key-form (Fields[] read)
```

### Then

```sh
cd /Users/k2m30/projects/a9s
go run ./cmd/viewsgen/    # regenerate .a9s/views/<shortName>.yaml
make build && make lint && make test
```

### Test

The architect dispatches a9s-qa with a test that asserts the COLUMN HAS DATA, not that the column exists. A column-existence test is busywork; a "value is non-empty for a fixture that should trigger it" test catches real wiring breakage. Example:

```go
func TestFetch<Type>_<Field>_Populated(t *testing.T) {
    fake := <typeFake>{ /* fixture that triggers the field */ }
    resources, err := awsclient.Fetch<Type>(ctx, fake)
    // ... assert resources[0].Fields["<key>"] == "<expected>"
}
```

## Tier B-fetcher — Wave-1 computed at fetch time

### File: `internal/aws/<short>.go`

Find the resource construction site. Add the computed field to the `Fields:` literal:

```go
Fields: map[string]string{
    // ... existing keys ...
    "<key>": <computed_value>,
},
```

Update `RegisterFieldKeys` to include `<key>`.

### Then defaults_*.go and viewsgen as Tier A.

### Test

Test asserts the COMPUTATION:

```go
func TestFetch<Type>_<Field>_ComputesCorrectly(t *testing.T) {
    // Fixture: an SDK response that should produce <expected>
    // Assertion: resources[0].Fields["<key>"] == "<expected>"
}
```

Don't test the trivial round-trip; test the logic.

## Tier B-enricher — Wave-2 via FieldUpdates

### File: `internal/aws/<short>_issue_enrichment.go`

Every registered short name already has an `_issue_enrichment.go` file. Open it and replace the `NoOpIssueEnricher` registration (if currently a stub) with a real enricher, or edit the existing enricher body if one is already there.

Mirror the existing `EnrichDynamoDBPITR` / `EnrichKMSRotation` / `EnrichRedisReplicationGroup` pattern:

```go
// <short>_issue_enrichment.go
package aws

func init() {
    registerIssueEnricher("<short>", Enrich<Name>, 100)
    resource.RegisterIssueEnricherFieldKeys("<short>", []string{"<key>"})
}

func Enrich<Name>(ctx context.Context, clients *ServiceClients, resources []resource.Resource) (IssueEnricherResult, error) {
    findings := make(map[string]resource.EnrichmentFinding)
    fieldUpdates := make(map[string]map[string]string)
    if clients.<Service> == nil {
        return IssueEnricherResult{Findings: findings}, nil
    }
    // Build set of resource IDs we know about; skip orphan responses
    known := make(map[string]struct{}, len(resources))
    for _, r := range resources {
        known[r.ID] = struct{}{}
    }
    // ONE account-wide call OR per-resource fan-out (capped at EnrichmentCap)
    for /* response */ {
        if _, ok := known[id]; !ok {
            continue
        }
        fieldUpdates[id] = map[string]string{
            "<key>": <value>,
        }
    }
    return IssueEnricherResult{
        IssueCount:   <real-count or 0 for ~ findings>,
        Truncated:    <bool>,
        Findings:     findings,
        FieldUpdates: fieldUpdates,
    }, nil
}
```

If the enricher walks paginated results (e.g. ListPackages, ListSubscriptionsByTopic), follow `NextToken` to the end. **Don't `len(out.Page)` — that under-counts.**

### Register via init()

The `init()` in the same file calls `registerIssueEnricher(<short>, <fn>, <priority>)`. `registerIssueEnricher` panics on empty name, nil fn, duplicate short name, or non-positive priority. If the short name was previously a `NoOpIssueEnricher` stub, delete the stub's registration before adding the real one — two calls with the same short name will panic at package init.

### Then defaults_*.go and viewsgen as Tier A.

### Test

Two assertions:
1. `result.FieldUpdates[<id>][<key>] == <expected>` for a fixture that triggers it
2. **For paginated enrichers**: `fake.calls >= 2` to prove pagination is followed

## Tier C — Detail-only

For multi-line / verbose data:

```yaml
detail:
  - { key: <field_key>, label: "<Label>" }
```

Or in `defaults_*.go`:

```go
Detail: []DetailField{
    // ... existing entries ...
    {Key: "<field_key>", Label: "<Label>"},
},
```

The detail renderer reads `Resource.Fields[<key>]` at render time. The label appears as the row key.

## Pitfalls (from the first Tier A sweep)

1. **Dead columns**: don't add a `Path:` for a field that isn't on the fetcher's RawStruct. The column will render blank for every row. The redis Failover bug took 3 rounds to spot. **Verify with `go doc`** before writing the column.

2. **24h-cutoff vs last-status**: enrichers that gate findings on a time window (e.g. backup last 24h) must NOT also gate FieldUpdates on the same window. The "last status" column should reflect the newest job regardless of age.

3. **Truncation truncation**: `len(out.Page1)` is wrong when `NextToken != nil`. Always paginate before counting.

4. **Negative truncation in date math**: `int(time.Until(past).Hours()/24)` is `0`, not `-1`. Compare timestamps directly: `if !future.After(now) { return "expired" }`.

5. **Enum case**: SDK enums are `string` types — `string(enum)` gives the wire value (e.g. `"enabled"`). Lowercase explicitly if you want to compare.

6. **Test fakes need to embed the aggregate API**: `type myFake struct { awsclient.SNSAPI }` — then override only the methods you exercise. Without embedding, the compiler complains about missing methods.

7. **Don't write column-existence tests**: asserting a column exists in defaults that you just added is tautological. Test the value pipeline instead (fixture → fetcher → Fields[] → column).

## Verification

```sh
cd /Users/k2m30/projects/a9s
go run ./cmd/viewsgen/
make build && make lint && make test 2>&1 | tail -10
```

ALL must pass. If `expectedConfigColumnCounts` in `tests/unit/qa_name_column_first_test.go` mentions `<short>`, increment its value.

## Skip rules

- Don't refactor the fetcher just to enable a column — use the FieldUpdates path instead. The redis Failover column shipped via Wave-2 enricher rather than fetcher refactor; same pattern applies elsewhere.
- Don't add a column whose data the fetcher genuinely doesn't have AND no Wave-2 path exists. That's a future-work item; document in the defaults_*.go file with a comment indicating the prerequisite.
