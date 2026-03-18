# Architecture Audit: Root Cause Analysis of Production Bugs

**Date:** 2026-03-18
**Auditor:** Claude Opus 4.6 (code review)
**Scope:** 4 critical bugs shipping despite green test suite

---

## Bug 1: S3 folders not navigable

**Symptom:** Pressing Enter on a folder (CommonPrefix like `enterprise/`) shows the detail view instead of navigating into the prefix.

### Root Cause

**File:** `/Users/k2m30/projects/a9s/internal/tui/views/resourcelist.go`, lines 144-158

```go
case key.Matches(msg, m.keys.Enter), key.Matches(msg, m.keys.Describe):
    if r := m.SelectedResource(); r != nil {
        // S3 bucket list: Enter drills into the bucket to show objects
        if m.typeDef.ShortName == "s3" && m.s3Bucket == "" {
            bucketName := r.ID
            return m, func() tea.Msg {
                return messages.S3EnterBucketMsg{BucketName: bucketName}
            }
        }
        return m, func() tea.Msg {
            return messages.NavigateMsg{
                Target:   messages.TargetDetail,
                Resource: r,
            }
        }
    }
```

The Enter handler only checks `m.typeDef.ShortName == "s3" && m.s3Bucket == ""` to decide whether to drill into a bucket. Once inside a bucket (i.e., when `s3Bucket != ""`), ALL Enter presses go to `NavigateMsg{Target: TargetDetail}` -- even for folders.

Folders are returned by `FetchS3Objects` in `/Users/k2m30/projects/a9s/internal/aws/s3.go` with `Status: "folder"` (line 118), but the Enter handler never checks `r.Status == "folder"`. The `S3NavigatePrefixMsg` message type exists in `/Users/k2m30/projects/a9s/internal/tui/messages/messages.go` (line 110-114) and is handled in `/Users/k2m30/projects/a9s/internal/tui/app.go` (line 291-296), but **nothing ever sends it**.

### Why Tests Miss It

The test fixtures in `/Users/k2m30/projects/a9s/tests/unit/fixtures_test.go` (function `fixtureS3Objects()`, line 65-79) contain only ONE object -- a file. There are zero folder (CommonPrefix) fixtures. The `TestQA_S3_C2_ObjectDetail_EnterSendsDetail` test at `/Users/k2m30/projects/a9s/tests/unit/qa_s3_test.go` line 819 verifies that Enter on an object sends `TargetDetail`, which is correct for files but wrong for folders. No test creates a resource with `Status: "folder"` and verifies that Enter sends `S3NavigatePrefixMsg`.

### Fix

In `resourcelist.go` line 144, after the S3 bucket check, add a folder check:

```go
case key.Matches(msg, m.keys.Enter), key.Matches(msg, m.keys.Describe):
    if r := m.SelectedResource(); r != nil {
        if m.typeDef.ShortName == "s3" && m.s3Bucket == "" {
            bucketName := r.ID
            return m, func() tea.Msg {
                return messages.S3EnterBucketMsg{BucketName: bucketName}
            }
        }
        // S3 objects: folders navigate into the prefix
        if m.typeDef.ShortName == "s3_objects" && r.Status == "folder" {
            bucket := m.s3Bucket
            prefix := r.ID
            return m, func() tea.Msg {
                return messages.S3NavigatePrefixMsg{Bucket: bucket, Prefix: prefix}
            }
        }
        return m, func() tea.Msg {
            return messages.NavigateMsg{
                Target:   messages.TargetDetail,
                Resource: r,
            }
        }
    }
```

But wait -- the `ResourceListModel` does not expose `s3Bucket` publicly, and it is set in `NewS3ObjectsList`. The `s3_objects` shortName check alone is sufficient to know we are inside a bucket, but we need the bucket name. Either add a `S3Bucket()` accessor method, or store the bucket name on the resource. The `s3Bucket` field is already on the model (line 46), so extract it via a new public method.

### Test Gap

Add fixtures with folders and test:
- Enter on a folder resource (`Status: "folder"`) sends `S3NavigatePrefixMsg` (not `NavigateMsg{TargetDetail}`)
- Enter on a file resource (`Status: "file"`) sends `NavigateMsg{TargetDetail}`
- `d` on a folder sends the same as Enter (detail or navigate -- needs design decision)

---

## Bug 2: `d` key enters S3 bucket instead of showing detail

**Symptom:** Pressing `d` (Describe/Detail) on an S3 bucket drills into the bucket objects instead of showing the bucket's detail view.

### Root Cause

**File:** `/Users/k2m30/projects/a9s/internal/tui/views/resourcelist.go`, line 144

```go
case key.Matches(msg, m.keys.Enter), key.Matches(msg, m.keys.Describe):
```

Enter and Describe (`d`) are handled in the **same case branch**. There is no distinction between the two keys. Both Enter and `d` on an S3 bucket will trigger `S3EnterBucketMsg` (drill into objects).

The intended design is:
- **Enter** = drill into (navigate into bucket objects)
- **d** = describe/detail (show bucket metadata like ARN, region, creation date)

### Why Tests Miss It

`TestQA_S3_C1_BucketDetail_ViaDetailCommand` in `/Users/k2m30/projects/a9s/tests/unit/qa_s3_test.go` line 798-815 explicitly documents the bug as expected behavior:

```go
// The 'd' key in the current implementation is handled the same as Enter
// for S3 buckets. Verify the behavior.
...
// For S3 buckets, both Enter and d produce S3EnterBucketMsg
_, isBucketMsg := msg.(messages.S3EnterBucketMsg)
_, isNavMsg := msg.(messages.NavigateMsg)
if !isBucketMsg && !isNavMsg {
```

The test accepts BOTH `S3EnterBucketMsg` and `NavigateMsg` as valid. It was written to match the broken behavior rather than assert the correct behavior.

### Fix

Split the Enter/Describe case into two separate branches:

```go
case key.Matches(msg, m.keys.Enter):
    if r := m.SelectedResource(); r != nil {
        // S3 bucket list: Enter drills into the bucket
        if m.typeDef.ShortName == "s3" && m.s3Bucket == "" {
            bucketName := r.ID
            return m, func() tea.Msg {
                return messages.S3EnterBucketMsg{BucketName: bucketName}
            }
        }
        // S3 objects: folders navigate into prefix
        if m.typeDef.ShortName == "s3_objects" && r.Status == "folder" {
            bucket := m.s3Bucket
            prefix := r.ID
            return m, func() tea.Msg {
                return messages.S3NavigatePrefixMsg{Bucket: bucket, Prefix: prefix}
            }
        }
        // Everything else: show detail
        return m, func() tea.Msg {
            return messages.NavigateMsg{Target: messages.TargetDetail, Resource: r}
        }
    }
case key.Matches(msg, m.keys.Describe):
    if r := m.SelectedResource(); r != nil {
        // d always shows detail, regardless of resource type
        return m, func() tea.Msg {
            return messages.NavigateMsg{Target: messages.TargetDetail, Resource: r}
        }
    }
```

### Test Gap

- Test that `d` on an S3 bucket sends `NavigateMsg{TargetDetail}` (NOT `S3EnterBucketMsg`)
- Test that Enter on an S3 bucket sends `S3EnterBucketMsg`
- Rewrite `TestQA_S3_C1_BucketDetail_ViaDetailCommand` to assert the correct behavior

---

## Bug 3: views.yaml config-driven detail rendering is broken

**Symptom:** Detail view shows only Fields map keys (key, last_modified, size, storage_class) instead of config-driven fields from views.yaml. The config-driven rendering path picks the wrong ViewDef.

### Root Cause

**File:** `/Users/k2m30/projects/a9s/internal/tui/views/detail.go`, lines 146-172

```go
func (m DetailModel) renderFromConfig(kv func(string, string) string) []string {
    for _, vd := range m.viewConfig.Views {  // <-- iterates ALL ViewDefs
        if len(vd.Detail) == 0 {
            continue
        }
        var lines []string
        for _, path := range vd.Detail {
            val := fieldpath.ExtractSubtree(m.res.RawStruct, path)
            if val == "" {
                continue
            }
            ...
        }
        if len(lines) > 0 {
            return lines  // <-- returns FIRST match, not the RIGHT match
        }
    }
    return nil
}
```

The function iterates over `m.viewConfig.Views` (a `map[string]ViewDef`) in non-deterministic order and returns the first ViewDef whose detail paths produce any non-empty output from the resource's `RawStruct`. Since many AWS structs share common field names (e.g., `Name`, `Status`, `Version`, `Endpoint`), a ViewDef for EKS or Secrets (which have paths like `Name`, `Status`) can match against an EC2 instance's struct, returning only those generic fields instead of EC2-specific fields like `InstanceId`, `State`, `InstanceType`, etc.

The `DetailModel` does not know what resource type it is displaying. It receives a `resource.Resource` and a full `*config.ViewsConfig` but has no resource type string to look up the correct ViewDef. The correct function `config.GetViewDef(cfg, shortName)` exists in `/Users/k2m30/projects/a9s/internal/config/config.go` line 131-150 and does the right thing -- but it is never called from `renderFromConfig`.

**Second contributing factor:** `/Users/k2m30/projects/a9s/internal/tui/app.go` line 424 creates the detail model as:

```go
d := views.NewDetail(*msg.Resource, m.viewConfig, m.keys)
```

It passes the full `m.viewConfig` (all 8 resource types) but does NOT pass the resource type string. The `NavigateMsg` struct (line 22-26 of messages.go) has a `ResourceType` field, but it is not populated when navigating to detail from the resource list -- the resource list handler at line 153-158 only sets `Target` and `Resource`, not `ResourceType`.

### Why Tests Miss It

**File:** `/Users/k2m30/projects/a9s/tests/unit/qa_detail_test.go`, lines 99-113

```go
// configForType returns a ViewsConfig containing only the ViewDef for the given
// resource type. This avoids the non-deterministic map iteration in renderFromConfig
// matching a wrong ViewDef whose paths happen to extract values from the struct.
func configForType(typeName string) *config.ViewsConfig {
    full := config.DefaultConfig()
    vd, ok := full.Views[typeName]
    if !ok {
        return full
    }
    return &config.ViewsConfig{
        Views: map[string]config.ViewDef{
            typeName: vd,
        },
    }
}
```

The test helper `configForType` **explicitly documents the bug** in its comment (line 100-101) and then **works around it** by creating a config with only ONE ViewDef. This means every test in `qa_detail_test.go` passes a single-type config, so `renderFromConfig` always iterates exactly one ViewDef -- the correct one. In production, the full config has 8 ViewDefs and the wrong one may be picked first.

Additionally, tests in `tui_content_views_test.go` (line 127-161, `TestContentDetail_ViewWithRawStructAndConfig`) also pass a hand-crafted single-type config:

```go
viewCfg := &config.ViewsConfig{
    Views: map[string]config.ViewDef{
        "ec2": {
            Detail: []string{"InstanceId", "InstanceType"},
        },
    },
}
```

The integration tests in `qa_ec2_test.go` (e.g., `TestQA_EC2_B2_DetailFieldsDisplayed` at line 780) use fixture data with `RawStruct: nil`, so the config-driven path is never entered -- they fall through to the Fields map fallback.

### Fix

Option A (minimal): Add a `ResourceType` field to `DetailModel` and pass it through from the resource list. Modify `renderFromConfig` to call `config.GetViewDef(m.viewConfig, m.resourceType)` instead of iterating all views:

```go
// In detail.go, change renderFromConfig:
func (m DetailModel) renderFromConfig(kv func(string, string) string) []string {
    vd := config.GetViewDef(m.viewConfig, m.resourceType)
    if len(vd.Detail) == 0 {
        return nil
    }
    var lines []string
    for _, path := range vd.Detail {
        val := fieldpath.ExtractSubtree(m.res.RawStruct, path)
        if val == "" {
            continue
        }
        if strings.Contains(val, "\n") {
            lines = append(lines, " "+styles.DetailSection.Render(path))
            for _, subLine := range strings.Split(val, "\n") {
                lines = append(lines, "     "+styles.DetailVal.Render(subLine))
            }
        } else {
            lines = append(lines, kv(path, val))
        }
    }
    return lines
}
```

This requires:
1. Add `resourceType string` field to `DetailModel`
2. Add it to `NewDetail()` constructor
3. Pass it from `app.go` when creating the detail view
4. The resource list needs to pass its type in the `NavigateMsg`

Option B (add type to Resource): Add a `Type` field to `resource.Resource` itself, set it during fetch, and use it in `renderFromConfig`. This avoids threading the type through messages.

### Test Gap

- Test that `renderFromConfig` with the FULL config (all 8 ViewDefs) and an EC2 `RawStruct` renders EC2-specific fields (InstanceId, State, etc.), not generic fields
- Remove or fix `configForType` -- tests must use the full config to catch this class of bug
- Test each resource type's detail view with the full production config

---

## Bug 4: EC2 shows only Tags in detail view

**Symptom:** EC2 detail view shows only the Tags field instead of all configured fields (InstanceId, State, InstanceType, etc.).

### Root Cause

This is a specific manifestation of Bug 3. Here is how it happens:

When `renderFromConfig` iterates the full config's `Views` map in non-deterministic order, it may encounter the `"eks"` ViewDef first. The EKS detail paths are:

```yaml
detail:
  - Name
  - Version
  - Status
  - Endpoint
  - PlatformVersion
  - Arn
  - RoleArn
  - KubernetesNetworkConfig
```

Against an `ec2types.Instance` struct, `Name` extracts nothing (EC2 Instance has no `Name` field), `Version` extracts nothing, `Status` extracts nothing -- so EKS produces zero lines and is skipped.

But if the `"secrets"` ViewDef is iterated first, its paths include `Name` and `Tags` -- `Tags` DOES exist on `ec2types.Instance` and extracts to a non-empty YAML string. So secrets produces 1 line (Tags) and returns immediately.

Or if `"s3"` is iterated first, its detail paths `["BucketArn", "BucketRegion", "CreationDate"]` all fail against an EC2 struct, so it is skipped. But `"ec2"` might or might not come next -- it depends on Go's map iteration order.

The point is: **which fields are displayed depends on which ViewDef Go's map iterator visits first**, and this is non-deterministic. In some runs you might see the correct fields; in others you see Tags only; in others you see Name+Description (from secrets).

This is the same root cause as Bug 3. The fix is identical.

### Why Tests Miss It

Same as Bug 3. The `configForType("ec2")` helper strips out all other ViewDefs, so the EC2 ViewDef is always the one that matches.

The integration tests in `qa_ec2_test.go` (e.g., `TestQA_EC2_B2_DetailFieldsDisplayed` line 780) use fixture data with `RawStruct: nil`, so they test the Fields-map fallback path, not the config-driven path. The Fields map does contain all the expected values, so those tests pass. In production, resources fetched from AWS have `RawStruct` set (it is the actual AWS SDK struct), so the config-driven path IS entered -- and it breaks.

### Fix

Same as Bug 3. Thread the resource type through to `DetailModel` and use `config.GetViewDef` instead of iterating all views.

### Test Gap

- Create an EC2 detail model with a real `ec2types.Instance` RawStruct AND the full production config (not `configForType`), and assert that EC2-specific fields like `InstanceId`, `InstanceType`, `PrivateIpAddress` are present
- Do the same for every resource type

---

## Summary of Systemic Issues

### 1. Tests were written to match broken behavior, not to specify correct behavior

The clearest evidence is `configForType()` which explicitly documents the `renderFromConfig` map iteration bug and then works around it. The test for `d` on S3 buckets also accepts the wrong behavior as valid.

### 2. Test fixtures lack RawStruct, hiding the config-driven code path

The fixtures in `fixtures_test.go` set only `Fields` (map of strings) and `RawStruct: nil`. This means all detail view tests exercise only the fallback rendering path (Fields map), never the config-driven path (RawStruct + ViewDef). In production, AWS fetchers populate `RawStruct` with actual SDK structs.

### 3. S3 test fixtures lack folder entries

`fixtureS3Objects()` returns only one file. There are zero folder (CommonPrefix) entries. This means folder navigation code is completely untested.

### 4. Detail model has no knowledge of resource type

`DetailModel` receives a `Resource` and a full `ViewsConfig` but has no way to determine which ViewDef applies. This is an architectural gap -- the resource type should be threaded through from the list view to the detail view.

---

## Priority Order for Fixes

1. **Bug 3+4 (config-driven detail broken)** -- Highest impact. Affects ALL resource types in production. Fix: add `resourceType` to `DetailModel`, use `GetViewDef` instead of iterating all views.
2. **Bug 2 (d key on S3 buckets)** -- Split Enter and Describe into separate case branches.
3. **Bug 1 (S3 folder navigation)** -- Add folder check in Enter handler, send `S3NavigatePrefixMsg`.
4. **Test infrastructure** -- Remove `configForType` workaround, add RawStruct to fixtures, add folder fixtures.
