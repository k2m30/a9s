# Pagination Interaction Design Spec

Issue: #110
Version: 1.0
Target: a9s v3.24+

---

## 1. Design Decisions (Opinionated)

This spec makes concrete recommendations, not options. The rationale is provided
for each decision so the implementation team knows *why*, not just *what*.

### Guiding Principles

1. **Client-side only.** AWS APIs do not support server-side text search. Every API
   call returns a page of results in creation order (or an opaque order). Filtering
   and sorting always operate on loaded data.
2. **Explicit is better than magic.** Never silently load more pages. The user
   pressed M to load more; the app should not page-in behind the scenes.
3. **Communicate partial data clearly.** The `+` suffix in frame titles is the
   primary signal. Reinforce it in the load-more hint and filter context.
4. **k9s precedent.** k9s operates on a fully-loaded resource set (one `kubectl`
   call returns everything). Since a9s paginates, every interaction that touches
   data scope must acknowledge the partial-load boundary.

---

## 2. Frame Title Format (Reference)

The frame title embedded in the top border is the primary pagination signal.
These formats are already implemented and remain unchanged:

| State                          | Format                           | Example                    |
|--------------------------------|----------------------------------|----------------------------|
| All loaded, no filter          | `name(count)`                    | `ec2(42)`                  |
| All loaded, filter active      | `name(filtered/total)`           | `ec2(3/42)`                |
| Truncated, no filter           | `name(count+)`                   | `ct-events(200+)`          |
| Truncated, loading more        | `name(count+ loading...)`        | `ct-events(200+ loading...)`|
| Truncated, filter active       | `name(filtered/total+)`          | `ct-events(3/200+)`        |

---

## 3. Filtering (`/`) in Paginated Views

### Decision: Client-side only, with clear "partial data" communication

Filtering always operates on already-loaded resources. It never triggers API calls.
This is the correct choice because:

- AWS APIs have no server-side text search across arbitrary fields
- Auto-loading pages during filter would be slow and confusing (typing "prod"
  triggers 3 sequential API calls for "p", "pr", "pro"?)
- Users of k9s, kubectl, and similar tools expect instant client-side filtering

### Frame Title with Active Filter + Pagination

When a filter is active on a paginated view that has more pages, the title format
is `name(filtered/loaded+)`. The `+` tells the user that loaded items are a subset:

```
┌──────────────── ct-events(3/200+) ────────────────────────────────────────────┐
```

This reads as: "3 items match your filter out of 200 loaded, and more exist on
the server."

### Load-More Hint with Active Filter

When a filter is active and the list is truncated, the bottom hint changes to
indicate that loading more pages may reveal additional matches:

```
│ EVENT NAME              TIME                 USER            SOURCE           │
│ CreateBucket            2024-03-17 08:12     admin           s3.amazonaws.com │
│ DeleteBucket            2024-03-17 07:55     admin           s3.amazonaws.com │
│ PutBucketPolicy         2024-03-16 22:30     ci-deploy       s3.amazonaws.com │
│── M: load more (filter applies to loaded data only) ──                       │
└──────────────────────────────────────────────────────────────────────────────┘
```

When no filter is active, the standard hint is shown:

```
│── M: load more ──                                                            │
```

When loading is in progress:

```
│── loading... ──                                                              │
```

### Behavior When M Is Pressed During Active Filter

Pressing M while a filter is active loads the next page of raw (unfiltered)
results, appends them to the full dataset, then re-applies the filter. The user
sees the filter count and loaded count both update:

Before M: `ct-events(3/200+)`, hint: `── M: load more ──`
During load: `ct-events(3/200+ loading...)`, hint: `── loading... ──`
After M: `ct-events(5/400+)` (new matches found in page 2)

This is the natural behavior: `allResources` grows, `applySortAndFilter()` runs,
filter count updates. No special code needed.

### Wireframe: Filter Active on Truncated List

```
 a9s v0.5.0  prod:us-east-1                                       /bucket
┌────────────────────────── ct-events(3/200+) ─────────────────────────────────┐
│ EVENT NAME              TIME                 USER            SOURCE           │
│ CreateBucket            2024-03-17 08:12     admin           s3.amazonaws.com │
│ DeleteBucket            2024-03-17 07:55     admin           s3.amazonaws.com │
│ PutBucketPolicy         2024-03-16 22:30     ci-deploy       s3.amazonaws.com │
│                                                                              │
│                                                                              │
│                                                                              │
│                                                                              │
│                                                                              │
│── M: load more (filter applies to loaded data only) ──                       │
└──────────────────────────────────────────────────────────────────────────────┘
```

### Wireframe: Filter Active, All Pages Loaded

When `IsTruncated` is false, the title drops the `+` and the hint disappears:

```
 a9s v0.5.0  prod:us-east-1                                       /bucket
┌───────────────────────── ct-events(12/1847) ─────────────────────────────────┐
│ EVENT NAME              TIME                 USER            SOURCE           │
│ CreateBucket            2024-03-17 08:12     admin           s3.amazonaws.com │
│ DeleteBucket            2024-03-17 07:55     admin           s3.amazonaws.com │
│ ...10 more matching rows...                                                  │
└──────────────────────────────────────────────────────────────────────────────┘
```

---

## 4. Ctrl+R (Refresh) Behavior

### Decision: Refresh always reloads from scratch (page 1 only)

When the user presses Ctrl+R:

1. Clear `allResources` (replace, not append)
2. Clear `pagination` state
3. Clear any active filter text
4. Reset cursor to position 0
5. Show loading spinner
6. Fetch page 1 with empty continuation token
7. When results arrive, the view shows the first page with `+` if truncated

This is the correct choice because:

- "Refresh" means "show me current state." Reloading all previously-loaded pages
  is slow and the user likely doesn't need stale data from page 47.
- k9s refresh reloads everything from scratch in a single call. Since a9s
  paginates, the equivalent is "start over from page 1."
- If the user had loaded 2000 items across 20 M-key presses, refreshing all 20
  pages would take 10+ seconds. Page 1 takes <1 second.
- The user can press M again to load more after refresh.

### Implementation

The current `handleRefresh()` already calls `fetchResources()` which passes an
empty token, and the `ResourcesLoadedMsg` arrives with `Append=false`. This
replaces `allResources` entirely. The only change needed is to also clear the
active filter:

```go
// In handleRefresh, before dispatching fetch:
if f, ok := m.activeView().(views.Filterable); ok {
    f.SetFilter("")
}
```

### State Transition

```
User presses Ctrl+R
  -> handleRefresh()
     -> Clear filter on active view
     -> Set loading state + show spinner
     -> fetchResources(rt) with empty token
  -> ResourcesLoadedMsg arrives (Append=false)
     -> allResources replaced with page 1
     -> pagination = new PaginationMeta (possibly truncated)
     -> cursor resets to 0
     -> Frame title: "ct-events(200+)" or "ec2(42)"
```

### Wireframe: After Refresh (Truncated Resource)

```
 a9s v0.5.0  prod:us-east-1                                       Refreshing...
┌──────────────────────── ct-events(200+) ─────────────────────────────────────┐
│ EVENT NAME              TIME                 USER            SOURCE           │
│ ConsoleLogin            2024-03-17 09:00     admin           signin           │
│ AssumeRole              2024-03-17 08:58     ci-deploy       sts              │
│ CreateBucket            2024-03-17 08:12     admin           s3               │
│ ...                                                                          │
│                                                                              │
│                                                                              │
│                                                                              │
│── M: load more ──                                                            │
└──────────────────────────────────────────────────────────────────────────────┘
```

---

## 5. Navigation Caching (Esc + Re-enter)

### Decision: Cached data survives back-navigation within the same session

When a user:
1. Opens a resource list (fetches page 1)
2. Presses M to load pages 2-5
3. Presses Enter to view a resource's detail
4. Presses Esc to go back to the list

The list should show all 5 pages of data, with the cursor at the same position.
This is the natural behavior of the view stack: the `ResourceListModel` stays in
the stack while detail/YAML views are pushed on top.

### When Does Cache Invalidate?

| Event                        | Cache behavior                              |
|------------------------------|---------------------------------------------|
| Esc from detail/YAML back    | Preserved (view stays in stack)             |
| Esc all the way to main menu | Destroyed (view popped from stack)          |
| Re-enter from main menu      | Fresh fetch (new view pushed)               |
| Ctrl+R (refresh)             | Destroyed (replaced with page 1)            |
| Profile switch (:ctx)        | Destroyed (entire stack cleared)            |
| Region switch (:region)      | Destroyed (entire stack cleared)            |

### No TTL on Cache

There is no time-based cache expiration. The data is valid for the lifetime of the
view on the stack. If the user wants fresh data, they press Ctrl+R. This is
consistent with k9s behavior where data auto-refreshes on an interval, but a9s
is intentionally read-on-demand (no background polling).

### Implementation

This already works correctly with the current view stack architecture. The
`ResourceListModel` persists in `m.stack` with all its `allResources` and
`pagination` state. No code changes needed for basic back-navigation.

The one edge case is `handleNavigate(TargetResourceList)`: this always creates a
*new* `ResourceListModel` and fetches page 1. It does not try to reuse an existing
view. This is correct -- navigating via `:ec2` from the main menu always starts
fresh.

### Wireframe: Back-Navigation Preserved State

Before drilling into detail (user loaded 3 pages):

```
 a9s v0.5.0  prod:us-east-1                                       ? for help
┌──────────────────────── ct-events(600+) ─────────────────────────────────────┐
│ EVENT NAME              TIME                 USER            SOURCE           │
│ ...cursor at row 450...                                                      │
│ CreateFunction          2024-03-15 14:22     ci-deploy       lambda           │
│── M: load more ──                                                            │
└──────────────────────────────────────────────────────────────────────────────┘
```

After Esc from detail (exact same state):

```
 a9s v0.5.0  prod:us-east-1                                       ? for help
┌──────────────────────── ct-events(600+) ─────────────────────────────────────┐
│ EVENT NAME              TIME                 USER            SOURCE           │
│ ...cursor at row 450...                                                      │
│ CreateFunction          2024-03-15 14:22     ci-deploy       lambda           │
│── M: load more ──                                                            │
└──────────────────────────────────────────────────────────────────────────────┘
```

---

## 6. Sorting in Paginated Views

### Decision: Client-side sort on loaded data, with visual indicator

Sorting always operates on `allResources` (then filter is re-applied). Sorting
200 items out of potentially 10,000 is technically "wrong" (the global sort order
is unknowable without loading everything), but this is acceptable because:

- k9s sorts only what's loaded (from a single `kubectl` response)
- AWS APIs don't support arbitrary server-side sorting
- The sort indicator in the column header already tells users the sort column
- The `+` in the frame title already tells users the data is partial

No additional warning is needed. The `+` suffix is sufficient. Users who care about
seeing the globally oldest/newest items will press M to load everything first, then
sort.

### Sort + Pagination + Filter Interaction

Sort applies to `allResources`, then filter narrows to `filteredResources`.
When M loads more items, `applySortAndFilter()` re-sorts and re-filters the
combined dataset. The cursor position is preserved by index (not by resource ID),
which may shift the selected resource. This is acceptable -- the same behavior
occurs when sorting a non-paginated list.

### Wireframe: Sorted Truncated List

```
 a9s v0.5.0  prod:us-east-1                                       ? for help
┌──────────────────────── ct-events(200+) ─────────────────────────────────────┐
│ EVENT NAME              TIME↓                USER            SOURCE           │
│ ConsoleLogin            2024-03-17 09:00     admin           signin           │
│ AssumeRole              2024-03-17 08:58     ci-deploy       sts              │
│ CreateBucket            2024-03-17 08:12     admin           s3               │
│ DeleteBucket            2024-03-17 07:55     admin           s3               │
│ PutBucketPolicy         2024-03-16 22:30     ci-deploy       s3               │
│ ...                                                                          │
│── M: load more ──                                                            │
└──────────────────────────────────────────────────────────────────────────────┘
```

Note: the sort indicator `↓` on TIME column and `+` on the count co-exist. No
conflict, no extra warning.

---

## 7. Load-More Hint States

The bottom-of-table indicator is a single dim line. It appears only when
`pagination.IsTruncated == true`. It occupies one row of the visible window
(already implemented in `View()`).

| State                             | Hint text                                                |
|-----------------------------------|----------------------------------------------------------|
| Truncated, no filter, idle        | `── M: load more ──`                                    |
| Truncated, no filter, loading     | `── loading... ──`                                      |
| Truncated, filter active, idle    | `── M: load more (filter applies to loaded data only) ──`|
| Truncated, filter active, loading | `── loading... ──`                                      |
| Not truncated                     | (no hint shown)                                         |

### Implementation Change

The current `View()` method uses a static hint string. The change is minimal:

```go
if showLoadMore {
    sb.WriteString("\n")
    var hint string
    if m.loadingMore {
        hint = "── loading... ──"
    } else if m.filterText != "" {
        hint = "── M: load more (filter applies to loaded data only) ──"
    } else {
        hint = "── M: load more ──"
    }
    sb.WriteString(styles.DimText.Render(hint))
}
```

---

## 8. Help Screen Context

The help screen already adapts based on `GetHelpContext()`, which returns
`HelpFromResourceListPaginated` when the list is truncated. The M key is shown
only in the paginated context. No changes needed.

---

## 9. Edge Cases

### 9.1 Empty Filter Results on Partial Data

If the user types a filter that matches 0 items from 200 loaded:

```
 a9s v0.5.0  prod:us-east-1                                       /xyz123
┌──────────────────────── ct-events(0/200+) ───────────────────────────────────┐
│ No resources found                                                           │
│                                                                              │
│                                                                              │
│── M: load more (filter applies to loaded data only) ──                       │
└──────────────────────────────────────────────────────────────────────────────┘
```

The title shows `0/200+` and the hint reminds the user to try loading more.

### 9.2 Esc Clears Filter Before Popping View

Already implemented: pressing Esc when a filter is active clears the filter and
stays on the same view. Pressing Esc again pops the view. This two-step behavior
is important for paginated views because the user may want to clear a filter and
continue browsing their loaded data without losing it.

### 9.3 Rapid M Presses

The `loadingMore` guard prevents double-fetching. The second M press is a no-op
while `loadingMore == true`. Already tested in
`TestResourceList_LoadMore_WhenAlreadyLoading_Noop`.

### 9.4 API Error During Load More

If fetching the next page fails, `APIErrorMsg` is delivered. The current handler
shows a flash error and clears the loading state. The existing loaded data is
preserved. The user can press M again to retry.

However, `loadingMore` must also be cleared on error. Current implementation only
clears it in `ResourcesLoadedMsg`. This is a bug to fix:

```go
case messages.APIErrorMsg:
    // Clear loadingMore state so M key works again after error
    if rl, ok := m.activeView().(*views.ResourceListModel); ok {
        rl.ClearLoading()
    }
    // ... existing error flash handling
```

### 9.5 G (Jump to Bottom) on Truncated List

Pressing G moves the cursor to the last loaded item. It does NOT auto-load more
pages. The load-more hint at the bottom is visible, telling the user to press M.

### 9.6 Auto-Loading When Scrolling Past Bottom

Not implemented. Not recommended. The user must explicitly press M. This is
consistent with k9s (no auto-pagination) and prevents runaway API calls in
large accounts (e.g., CloudTrail with millions of events).

---

## 10. State Machine Summary

```
                    ┌─────────────┐
                    │   LOADING   │ (initial fetch, spinner visible)
                    └──────┬──────┘
                           │ ResourcesLoadedMsg (Append=false)
                           v
              ┌────────────────────────┐
              │  LOADED (page 1)       │
              │  title: name(N+)       │<──────────────────────┐
              │  hint: M: load more    │                       │
              └────┬───────┬───────┬───┘                       │
                   │       │       │                            │
          M key    │  /key │  Ctrl+R                           │
                   │       │       │                            │
                   v       v       v                            │
         ┌─────────┐ ┌──────────┐ ┌─────────────┐             │
         │ LOADING │ │ FILTERED │ │   LOADING   │             │
         │  MORE   │ │ f/N+     │ │  (refresh)  │─────────────┘
         └────┬────┘ └──────────┘ └─────────────┘
              │ ResourcesLoadedMsg (Append=true)
              v
    ┌──────────────────────────┐
    │  LOADED (pages 1..K)     │
    │  title: name(N2+) or     │
    │         name(N2)         │
    │  hint: M or (none)       │
    └──────────────────────────┘
```

---

## 11. Key Binding Table (Pagination Context)

| Key     | Action                  | Context                        | Notes                              |
|---------|-------------------------|--------------------------------|------------------------------------|
| `M`     | Load next page          | Resource list, truncated       | No-op if all loaded or loading     |
| `/`     | Start filter input      | Resource list                  | Filters loaded data only           |
| `Esc`   | Clear filter / go back  | Resource list with filter      | First press clears filter          |
| `Ctrl+R`| Refresh from scratch    | Resource list                  | Clears filter, reloads page 1      |
| `N`     | Sort by name            | Resource list                  | Sorts loaded data only             |
| `I`     | Sort by ID              | Resource list                  | Sorts loaded data only             |
| `A`     | Sort by age/date        | Resource list                  | Sorts loaded data only             |
| `G`     | Jump to last loaded     | Resource list                  | Does not auto-load more            |
| `Enter` | Drill into detail       | Resource list                  | View stays in stack for back-nav   |
| `Esc`   | Go back                 | Detail/YAML (no filter)        | Returns to list with all data      |

---

## 12. Msg Types and Transitions

| Msg Type               | Trigger                    | Effect on ResourceListModel                         |
|------------------------|----------------------------|-----------------------------------------------------|
| `ResourcesLoadedMsg`   | Initial fetch / refresh    | `Append=false`: replaces allResources, resets cursor|
| `ResourcesLoadedMsg`   | M key (load more)          | `Append=true`: appends to allResources              |
| `LoadMoreMsg`          | M key pressed              | Sets `loadingMore=true`, dispatches fetch            |
| `APIErrorMsg`          | Fetch failure              | Flash error, clears loadingMore                      |
| `RefreshMsg`           | Ctrl+R                     | Clears filter, starts fresh fetch                    |
| `tea.KeyMsg("/")`      | Typing in filter           | Re-filters allResources client-side                  |
| `tea.KeyMsg("N/I/A")`  | Sort key                   | Re-sorts allResources, re-applies filter             |

---

## 13. Implementation Checklist

Changes needed to implement this spec:

### Must Do (correctness)

- [ ] **`resourcelist.go` View()**: Change load-more hint to include filter context
      hint when `m.filterText != ""` and `showLoadMore`
- [ ] **`app_handlers.go` handleRefresh()**: Clear active filter before dispatching
      fetch (add `SetFilter("")` call)
- [ ] **`app_handlers.go` handleAPIError()**: Clear `loadingMore` on the active
      ResourceListModel when an API error arrives during pagination

### Nice to Have (polish)

- [ ] **`resourcelist.go` View()**: When filter matches 0 items and list is
      truncated, show `"No resources found"` plus the load-more hint (currently
      shows "No resources found" without the hint because the early return
      skips the hint rendering)

### Already Working (no changes)

- [x] Frame title format with `+` suffix
- [x] M key guard against double-fetch
- [x] Append vs replace behavior
- [x] Cursor preservation on append
- [x] Back-navigation preserves loaded data (view stack)
- [x] Sort on loaded data
- [x] Filter on loaded data
- [x] Help context adapts for paginated views
- [x] Profile/region switch clears stack

---

## 14. Wireframe Gallery

### 14.1 Paginated List (Truncated, No Filter)

```
 a9s v0.5.0  prod:us-east-1                                       ? for help
┌──────────────────────── ct-events(200+) ─────────────────────────────────────┐
│ EVENT NAME↑             TIME                 USER            SOURCE           │
│ AssumeRole              2024-03-17 08:58     ci-deploy       sts              │
│ ConsoleLogin            2024-03-17 09:00     admin           signin           │
│ CreateBucket            2024-03-17 08:12     admin           s3               │
│ CreateFunction          2024-03-16 14:22     ci-deploy       lambda           │
│ DeleteBucket            2024-03-17 07:55     admin           s3               │
│ DescribeInstances       2024-03-17 08:45     monitoring      ec2              │
│ GetSecretValue          2024-03-17 08:30     app-service     secretsmanager   │
│ PutBucketPolicy         2024-03-16 22:30     ci-deploy       s3               │
│ RunInstances            2024-03-16 20:15     ci-deploy       ec2              │
│ StopInstances           2024-03-16 19:00     admin           ec2              │
│── M: load more ──                                                            │
└──────────────────────────────────────────────────────────────────────────────┘
```

### 14.2 Loading More (In Progress)

```
 a9s v0.5.0  prod:us-east-1                                       ? for help
┌──────────────── ct-events(200+ loading...) ──────────────────────────────────┐
│ EVENT NAME↑             TIME                 USER            SOURCE           │
│ AssumeRole              2024-03-17 08:58     ci-deploy       sts              │
│ ConsoleLogin            2024-03-17 09:00     admin           signin           │
│ CreateBucket            2024-03-17 08:12     admin           s3               │
│ CreateFunction          2024-03-16 14:22     ci-deploy       lambda           │
│ DeleteBucket            2024-03-17 07:55     admin           s3               │
│ DescribeInstances       2024-03-17 08:45     monitoring      ec2              │
│ GetSecretValue          2024-03-17 08:30     app-service     secretsmanager   │
│ PutBucketPolicy         2024-03-16 22:30     ci-deploy       s3               │
│ RunInstances            2024-03-16 20:15     ci-deploy       ec2              │
│ StopInstances           2024-03-16 19:00     admin           ec2              │
│── loading... ──                                                              │
└──────────────────────────────────────────────────────────────────────────────┘
```

### 14.3 After Loading All Pages

```
 a9s v0.5.0  prod:us-east-1                                       ? for help
┌──────────────────────── ct-events(1847) ─────────────────────────────────────┐
│ EVENT NAME↑             TIME                 USER            SOURCE           │
│ AssumeRole              2024-03-17 08:58     ci-deploy       sts              │
│ ConsoleLogin            2024-03-17 09:00     admin           signin           │
│ CreateBucket            2024-03-17 08:12     admin           s3               │
│ ...                                                                          │
│ (no load-more hint — all data loaded)                                        │
└──────────────────────────────────────────────────────────────────────────────┘
```

### 14.4 Filter Active on Truncated List

```
 a9s v0.5.0  prod:us-east-1                                       /bucket
┌────────────────────────── ct-events(3/200+) ─────────────────────────────────┐
│ EVENT NAME              TIME                 USER            SOURCE           │
│ CreateBucket            2024-03-17 08:12     admin           s3               │
│ DeleteBucket            2024-03-17 07:55     admin           s3               │
│ PutBucketPolicy         2024-03-16 22:30     ci-deploy       s3               │
│                                                                              │
│                                                                              │
│                                                                              │
│                                                                              │
│                                                                              │
│                                                                              │
│── M: load more (filter applies to loaded data only) ──                       │
└──────────────────────────────────────────────────────────────────────────────┘
```

### 14.5 Filter Active, Zero Matches, Truncated

```
 a9s v0.5.0  prod:us-east-1                                       /xyz123abc
┌────────────────────────── ct-events(0/200+) ─────────────────────────────────┐
│ No resources found                                                           │
│                                                                              │
│                                                                              │
│                                                                              │
│                                                                              │
│── M: load more (filter applies to loaded data only) ──                       │
└──────────────────────────────────────────────────────────────────────────────┘
```

### 14.6 After Refresh (Ctrl+R on Previously Multi-Page List)

```
 a9s v0.5.0  prod:us-east-1                                       Refreshing...
┌──────────────────────── ct-events(200+) ─────────────────────────────────────┐
│ EVENT NAME              TIME                 USER            SOURCE           │
│ ConsoleLogin            2024-03-17 09:05     admin           signin           │
│ AssumeRole              2024-03-17 09:03     ci-deploy       sts              │
│ ...fresh page 1 data, filter cleared, cursor at top...                       │
│                                                                              │
│── M: load more ──                                                            │
└──────────────────────────────────────────────────────────────────────────────┘
```

### 14.7 Non-Paginated List (Legacy Fetcher, No Changes)

```
 a9s v0.5.0  prod:us-east-1                                       ? for help
┌──────────────────────── ec2(42) ─────────────────────────────────────────────┐
│ NAME↑                STATUS      TYPE       AZ           LAUNCH TIME         │
│ api-prod-01          running     t3.medium  us-east-1a   2024-01-15 09:22    │
│ api-prod-02          running     t3.medium  us-east-1b   2024-01-15 09:25    │
│ ...                                                                          │
│ (no load-more hint — all data available)                                     │
└──────────────────────────────────────────────────────────────────────────────┘
```

---

## 15. Color Reference (Pagination-Specific Elements)

| Element                      | Foreground   | Background | Style  |
|------------------------------|--------------|------------|--------|
| Load-more hint text          | `#565f89`    | --         | Dim    |
| "loading..." in frame title  | `#c0caf5`    | --         | Bold   |
| `+` in frame title           | `#c0caf5`    | --         | Bold   |
| Filter count in title        | `#c0caf5`    | --         | Bold   |
| Frame title (all)            | `#c0caf5`    | --         | Bold   |
| Frame border                 | `#414868`    | --         | --     |

No new colors are introduced. All elements use the existing Tokyo Night palette.
