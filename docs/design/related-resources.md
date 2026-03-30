# Related Resources View Design Spec

Issue: #64
Version: 3.0
Target: a9s v3.26+
Status: Design

---

## 1. Overview

From any resource's detail view or YAML view, the user presses `r` (lowercase)
to open a "Related Resources" list. This shows a navigable single-column list
of **resource types** (not instances) related to the current resource. The list
enables cross-resource navigation -- the core capability that transforms a9s
from a resource browser into an infrastructure investigation tool.

### Design Principle: Reuse the Main Menu Pattern

The related-resources view is **visually identical** to the main menu in its
rendering approach. The main menu shows resource types with availability:

- Resource type name on the left
- Dim alias on the right (e.g., `:ec2`)
- Entire row dimmed if the resource type has no data (cursor skips)
- Normal rendering if the resource type has data

The related-resources view works the same way:

- Related type name on the left
- Optional count in parentheses inline after the name (e.g., `Security Groups (3)`)
- Entire row dimmed if no related resources exist (cursor skips)
- Normal rendering if related resources exist
- NO "Checking..." text, NO "Unavailable" text, NO spinners per row
- NO "Search >" indicators

Background checking happens silently. When the view opens, all rows that have
not yet been resolved start dim. As background checks complete, rows either
become available (normal color) or stay dim. The user sees rows "light up"
as results arrive.

### Three-Level Navigation Flow

```
Detail/YAML View         Related Types List        Filtered Resource List    Detail View
(EC2 i-abc123)     -->  Security Groups (3)   -->  sg list (filtered to  --> sg-xxx detail
  press r                VPC                       this EC2's SGs)          (full features)
                         Subnet
                         CloudTrail Events
```

### Smart Enter Behavior

- **Multiple related instances** (e.g., Security Groups -- an EC2 can have
  several): Enter opens a **filtered resource list** showing only the related
  instances. Standard list with all functionality (filter, sort, copy, detail,
  YAML).
- **Exactly one related instance** (e.g., VPC -- an EC2 is in one VPC): Enter
  goes **directly to the detail view** of that resource, skipping the list.
- **CloudTrail Events**: Opens the filtered ct-search view (per #114
  resource-to-cloudtrail.md), using the same `T` key behavior but invoked from
  the list. CloudTrail Events is just another row in the list -- no special
  visual treatment.

### Relationship Sources

The list contains both:
1. **Forward relationships** -- derived automatically from the resource's API
   response fields (e.g., EC2's `VpcId`, `SubnetId`, `SecurityGroups[].GroupId`).
   These are the majority of entries.
2. **Reverse relationships** -- resources that reference the current resource but
   the current resource has no pointer back (e.g., Target Groups that register
   an EC2 instance). Documented in `docs/design/related-resources/*.md`.
3. **Algorithmic relationships** -- connections requiring resource-specific logic,
   naming conventions, or multi-hop lookups (e.g., Lambda -> CloudWatch Log
   Group via `/aws/lambda/{function-name}` convention).

---

## 2. Key Binding: `r` (lowercase)

### Key Binding Analysis

| Considered | Verdict | Reason |
|------------|---------|--------|
| `r` (lower) | **Chosen** | Strongest mnemonic: "related". Lowercase = fast single-keypress access from detail/YAML views. Previously used by CFN Stack Resources child view (`Key: "r"`) -- that ONE child view is remapped to `R` (uppercase) to free `r` for global use. The `ctrl+r` (refresh) fat-finger risk is minimal: `ctrl+r` requires holding ctrl, which is a fundamentally different gesture than tapping `r`. |
| `R` (upper) | Rejected | Workable but unnecessarily awkward. Requiring Shift for such a frequent navigation action adds friction. Uppercase keys in a9s are reserved for less frequent actions (`N`/`I`/`A` sort, `L` logs, `M` load more). Related-resource navigation is a core feature that should be effortless. |
| `tab` | Rejected | Used for autocomplete in command mode. Some terminals intercept it. Not discoverable as "related resources". |
| `x` | Rejected | Used by Secrets Manager for "Reveal secret". |
| `ctrl+r` | Rejected | Already used for "Refresh current view" globally. |

### CFN Stack Resources Key Remap

CFN stacks previously used `r` as the child-view key for "Stack Resources"
(`cfn_resources`). This is the ONLY child view in the entire codebase that used
`Key: "r"`. It is remapped to `R` (uppercase):

```go
// Before
{ChildType: "cfn_resources", Key: "r", ...}

// After
{ChildType: "cfn_resources", Key: "R", ...}
```

Rationale for `R`:
- Same letter, just shifted -- zero learning curve for existing CFN users.
- Follows the existing pattern where uppercase child-view keys are used for
  less-frequently-accessed views (`L` for logs).
- `R` is not used in any other context on the CFN resource list or detail views.
- CFN Stack Resources is a resource-type-specific child view (only appears on
  CFN stacks), while `r` for related-resources is global across ALL resource
  types -- the global key deserves the easier keystroke.

The help hint on CFN views updates from `r resources` to `R resources`.
The `keys.go` binding `Resources` changes from `"r"` to `"R"`.

### Scope

`r` is active in these view contexts:
- **Resource list view** (with a resource selected)
- **Detail view**
- **YAML view**

`r` is NOT active in:
- Main menu (no resource selected)
- Help screen
- Profile/region selector
- ct-search views (no related-resource navigation from event views)
- Filter/command input mode

---

## 3. Navigation History

### Design Decision: Reuse Existing View Stack

Related resource navigation reuses the existing view stack (`[]ViewState`).
There is no parallel history concept. Each navigation step pushes a new entry
onto the view stack, and `esc` pops it as usual.

Rationale:
- The view stack already handles arbitrary depth (child views go 4 levels deep).
- A parallel history concept would confuse `esc` semantics -- users would not
  know whether `esc` goes "back in related history" or "back in view stack".
- Browser-style `[`/`]` history adds complexity without clear benefit when the
  view stack already provides linear back-navigation.

### Decision: No `[`/`]` Keys

The user prompt suggested `[` and `]` for browser-style back/forward. After
analysis, this is **rejected**:

| Factor | Assessment |
|--------|------------|
| `[` vs `esc` confusion | Both go "back" but in different scopes -- users will not know which to use. |
| Forward navigation | Forward (`]`) requires remembering where you were going, which is not how TUI navigation works. You re-navigate by pressing `r` again. |
| Implementation cost | Requires a parallel forward stack alongside the view stack, with complex invalidation when the user navigates to a new branch. |
| k9s precedent | k9s has no forward navigation. `esc` is the universal "go back". |

Instead, the existing view stack provides all back-navigation via `esc`. Forward
navigation is simply pressing `r` again on the new resource.

### Breadcrumb Trail in Frame Title

When navigating through related resources, the frame title shows a condensed
breadcrumb trail so the user knows their position:

```
Depth 1:  related -- i-0abc123 (web-prod)
Depth 2:  sg-instances(3) -- sg-0abc123 (web-sg)
Depth 3:  related -- sg-0abc123 (web-sg)
```

The frame title follows the existing child-view pattern for filtered lists:
`{view-type}({count}) -- {resource-id} ({resource-name})`

For the related-types list specifically, NO count is shown in the title:
`related -- {resource-id} ({resource-name})`

The "resolved/total types" count is meaningless to the user -- they do not
care how many related type slots resolved vs. total. The list itself makes
availability visually obvious (dim vs. normal rows).

### Depth Indicator in Header

When the view stack depth exceeds 4 (which happens quickly with related
navigation), the header shows a depth indicator replacing the version text:

```
Normal (depth <= 4):   a9s v3.26.0  prod:us-east-1                  ? for help
Deep   (depth > 4):    a9s [6]      prod:us-east-1                  ? for help
```

The depth indicator `[N]` uses the dim style (`#565f89`) and replaces `vX.Y.Z`
to save horizontal space. This gives the user a sense of how deep they are
without consuming extra screen real estate.

---

## 4. Single-Column List Layout

### Design Decision: Flat Single-Column List

Related types are displayed as a flat single-column list -- the same component
pattern as the main menu. Each row shows one related resource type with an
optional count. The rendering is visually indistinguishable from the main menu.

Rationale:
- **Consistency**: Every other view in a9s is a single-column list. A
  multi-column grid would require a new component and look alien.
- **Simplicity**: The median resource has ~13 related types. A flat list of 13
  items fits comfortably on one screen without any special layout.
- **No category headers needed**: We analyzed all 66 resources. The distribution
  of related types per resource is: minimum 4 (CloudTrail Events), median ~13,
  maximum ~27 (VPC with forward + reverse + algorithmic). Even 27 items is
  manageable with standard scrolling. Category headers add visual noise and a
  rendering concept not used anywhere else.

### Sort Order

Items are sorted by priority, then alphabetically within each priority tier:

1. **P0** (critical relationships) -- sorted alphabetically
2. **P1** (important relationships) -- sorted alphabetically
3. **P2** (secondary relationships) -- sorted alphabetically
4. **CloudTrail Events** -- always last

This gives sufficient structure without explicit category grouping. The most
important relationships appear first.

### Row Format

Each row shows the resource type label on the left. For cheap API lookups, the
count appears inline immediately after the name, separated by a space. For
expensive lookups, no count is shown -- the row is either normal (available) or
dim (unavailable). The row format is identical to the main menu.

**Rows with counts** (cheap API calls -- forward relationships parsed from the
resource's own fields):

```
  Security Groups (3)
  VPC (1)
  Subnet (1)
```

**Rows without counts** (expensive reverse/algorithmic lookups):

```
  Target Groups
  CloudWatch Alarms
  CloudTrail Events
```

**Dim rows** (unavailable -- no related resources found):

```
  EKS Node Groups
  Elastic Beanstalk
```

Dim rows show only the type name in dim text (`#565f89`). No "Unavailable"
text, no status indicator. The dimness itself communicates unavailability.

### Which Lookups Are Cheap vs. Expensive

The architect determines at design time which relationships are cheap (count
shown) vs. expensive (no count):

| Lookup Type | Cost | Show Count? | Example |
|-------------|------|-------------|---------|
| Forward: IDs already in resource fields | Cheap | Yes `(N)` | EC2 -> Security Groups, VPC, Subnet |
| Forward: single known ID | Cheap | Yes `(1)` | EC2 -> AMI, Key Pair |
| Reverse: must query another service | Expensive | No | EC2 -> Target Groups, ASG |
| Algorithmic: naming convention lookup | Expensive | No | Lambda -> CW Log Group |
| CloudTrail Events | N/A | No | Always available, opens ct-search |

Counts use the same `(N)` and `(N+)` format as the main menu.

### Pagination

The list uses the same pagination pattern as resource lists:

| Key | Action |
|-----|--------|
| `j`/`down` | Move cursor down (skips dim rows) |
| `k`/`up` | Move cursor up (skips dim rows) |
| `g` | Jump to first available item |
| `G` | Jump to last available item |
| `pgup`/`ctrl+u` | Page up |
| `pgdn`/`ctrl+d` | Page down |

The frame title shows the resource identity:
`related -- i-0abc123 (web-prod)`

When items exceed the visible area, scrolling works exactly as in resource
lists. A scroll indicator appears when there are items above or below the
visible area.

---

## 5. Background Availability Checking

### Pattern: Silent Background Resolution

The related-resources list follows a silent-loading pattern:

1. **Immediate display**: When the list opens, show ALL possible related
   resource types immediately. All rows start **dim** (greyed out, cursor
   skips). This is the "loading" state -- visually identical to a list where
   everything is unavailable.

2. **Background checking**: A background process checks which related resources
   actually exist. Each type resolves independently as results arrive.

3. **Row becomes available**: When a check finds related resources (count > 0),
   the row transitions from dim to normal. The cursor can now land on it. If
   the lookup is cheap, the count appears in parentheses. If expensive, the
   row simply becomes normal with no count.

4. **Row stays dim**: When a check finds zero related resources, the row stays
   dim. No text change, no "Unavailable" label. It just remains greyed out.

5. **Error handling**: If a check fails (e.g., access denied), the row stays
   dim. Same visual treatment as "no results" -- the user does not need to
   distinguish between "none found" and "check failed".

6. **Cursor behavior**: As rows light up, the cursor automatically moves to
   the first available row (if it is currently on a dim row or if no row was
   selected yet).

7. **Session cache**: Availability results are cached for the session.
   Navigating back to the list shows cached results instantly. The cache key
   is `{resource-type}:{resource-id}:{related-type}`.

8. **Manual refresh**: `ctrl+r` clears the cache for this resource and re-runs
   all availability checks. All rows go dim again and light up as results
   arrive.

### Forward vs. Reverse Check Cost

| Relationship Type | Check Method | Cost |
|-------------------|-------------|------|
| Forward (e.g., EC2 -> VPC) | Parse resource's own Fields; VPC ID is already known. Just verify the VPC exists (one API call). | Cheap |
| Reverse (e.g., EC2 -> Target Groups) | Must query the reverse service (e.g., iterate TGs or use cached data). | Expensive |
| Algorithmic (e.g., Lambda -> CW Log Group) | Apply naming convention or multi-step lookup. | Varies |

Forward relationships are checked first (they complete quickly), then reverse
and algorithmic checks run in parallel.

### All-Dim State

When ALL related types are dim (either still checking or all unavailable),
display a centered dim message below the list:

```
  EC2 Instances
  Auto Scaling Groups
  EBS Snapshots
  CloudTrail Events

  No related resources found. Press ctrl+r to refresh or esc to go back.
```

The dim message appears only after all checks complete and all are unavailable.
While checks are still in progress, no message is shown (rows are still
lighting up). The list remains visible (not replaced) so the user can see what
was checked.

---

## 6. State Transitions

### Msg Types

| Msg | From State | To State | Notes |
|-----|-----------|----------|-------|
| `KeyMsg(r)` | Detail/YAML/ResourceList | RelatedTypesListView | Push view stack; all rows dim; start checks |
| `RelatedTypeCheckedMsg` | List:checking | List:updated | One type resolved; row becomes normal or stays dim |
| `AllRelatedTypesCheckedMsg` | List:some-checking | List:all-resolved | All background checks complete |
| `KeyMsg(enter)` | List (on single-count type) | Detail view | Direct to detail (count = 1) |
| `KeyMsg(enter)` | List (on multi-count type) | Filtered ResourceListView | Push filtered list |
| `KeyMsg(enter)` | List (on CloudTrail Events) | ct-search (pre-filtered) | Same as `T` key behavior per #114 |
| `KeyMsg(enter)` | List (on dim row) | No-op | Cursor cannot be on dim rows |
| `KeyMsg(esc)` | List | Previous view | Pop view stack |
| `KeyMsg(ctrl+r)` | List | List:all-dim | Clear cache, restart all checks |
| `KeyMsg(/)` | List | List:filter-active | Filter list by type name |
| `KeyMsg(?)` | List | Help view | Show related-resources help |
| `KeyMsg(r)` | Filtered list/detail (from list) | New List for THAT resource | Push new list; enables chaining |
| `tea.WindowSizeMsg` | List | List (reflowed) | Recalculate layout widths |

### View Stack Examples

Simple navigation:
```
push(MainMenu) -> push(ec2-list) -> push(ec2-detail:i-abc)
  -> push(related-list:i-abc) -> push(sg-list:filtered)
     -> push(sg-detail:sg-xxx)
```

Chained navigation (EC2 -> SG -> VPC):
```
push(MainMenu) -> push(ec2-list) -> push(ec2-detail:i-abc)
  -> push(related-list:i-abc) -> push(sg-list:filtered)
     -> push(sg-detail:sg-xxx)
        -> push(related-list:sg-xxx) -> push(vpc-detail:vpc-yyy)
           -> push(related-list:vpc-yyy) -> push(subnet-list:filtered)
```

Esc unwinds one step at a time, exactly as today.

---

## 7. Key Binding Table: Related Types List

### List Navigation

| Key | Action | Notes |
|-----|--------|-------|
| `j`/`down` | Move cursor down | Skips dim rows |
| `k`/`up` | Move cursor up | Skips dim rows |
| `g` | Jump to first available type | |
| `G` | Jump to last available type | |
| `pgup`/`ctrl+u` | Page up | Same as resource lists |
| `pgdn`/`ctrl+d` | Page down | Same as resource lists |
| `enter` | Open selected related type | Smart: detail if count=1, list if count>1 |
| `esc` | Go back | Pop view stack |

### Standard Keys (Inherited)

| Key | Action | Notes |
|-----|--------|-------|
| `?` | Toggle help | Shows list-specific help |
| `ctrl+r` | Refresh | Re-checks all related types (all rows go dim) |
| `/` | Filter | Filter by type name |
| `:` | Command mode | Navigate to any resource type |
| `ctrl+c` | Force quit | |
| `r` | Not active | Already on the related list |

### Keys NOT Available on List

| Key | Why Not |
|-----|---------|
| `c` (copy) | This is a type picker, not an instance view |
| `d` (detail) | No detail for a type-picker row |
| `y` (YAML) | No YAML for a type-picker row |
| `N`/`S`/`A` (sort) | List is priority-sorted, not user-sortable |
| `T` (trail) | Use the CloudTrail Events row in the list instead |
| `h`/`l` (left/right) | Single-column list, no horizontal movement |

---

## 8. ASCII Wireframes

All wireframes use the same visual vocabulary as the main menu (design.md
section 4.1). There are only three row states:

- **[SELECTED]**: Blue background `#7aa2f7`, foreground `#1a1b26`, bold
- **Normal**: Text `#c0caf5` (available, has related resources)
- **[DIM]**: Text `#565f89` (unavailable or not yet checked, cursor skips)

No status text, no spinners, no "Checking..." or "Unavailable" labels.

### 8.1 List -- Initial Load (all rows dim, checks in progress)

```
 a9s v3.26.0  prod:us-east-1                                                                         ? for help
+----------------------------------- related -- i-0abc123 (web-prod) -----------------------------------+
| [DIM] Security Groups                                                                                 |
| [DIM] VPC                                                                                             |
| [DIM] Subnet                                                                                          |
| [DIM] Elastic IPs                                                                                     |
| [DIM] Network Interfaces                                                                              |
| [DIM] Auto Scaling Groups                                                                             |
| [DIM] Target Groups                                                                                   |
| [DIM] CloudWatch Alarms                                                                               |
| [DIM] IAM Role                                                                                        |
| [DIM] EKS Node Groups                                                                                 |
| [DIM] Elastic Beanstalk                                                                               |
| [DIM] CloudTrail Events                                                                               |
|                                                                                                       |
+-------------------------------------------------------------------------------------------------------+
```

Title shows just `related` with the resource identity. All rows are dim. No
cursor is visible (no available row to land on). As checks complete, rows will
light up one by one.

### 8.2 List -- Partially Loaded (some checks complete)

```
 a9s v3.26.0  prod:us-east-1                                                                         ? for help
+----------------------------------- related -- i-0abc123 (web-prod) -----------------------------------+
| [SELECTED] Security Groups (3)                                                                        |
|    VPC (1)                                                                                             |
|    Subnet (1)                                                                                          |
| [DIM] Elastic IPs                                                                                     |
| [DIM] Network Interfaces                                                                              |
| [DIM] Auto Scaling Groups                                                                             |
| [DIM] Target Groups                                                                                   |
| [DIM] CloudWatch Alarms                                                                               |
| [DIM] IAM Role                                                                                        |
| [DIM] EKS Node Groups                                                                                 |
| [DIM] Elastic Beanstalk                                                                               |
| [DIM] CloudTrail Events                                                                               |
|                                                                                                       |
+-------------------------------------------------------------------------------------------------------+
```

Three forward relationships (cheap lookups) have resolved. They show counts
in parentheses. The cursor has moved to the first available row. The remaining
rows are still dim (checks in progress or not yet started).

### 8.3 List -- Fully Loaded (all checks complete)

```
 a9s v3.26.0  prod:us-east-1                                                                         ? for help
+----------------------------------- related -- i-0abc123 (web-prod) -----------------------------------+
| [SELECTED] Security Groups (3)                                                                        |
|    VPC (1)                                                                                             |
|    Subnet (1)                                                                                          |
|    Elastic IPs (1)                                                                                     |
|    Network Interfaces (2)                                                                              |
|    Auto Scaling Groups                                                                                 |
|    Target Groups                                                                                       |
|    CloudWatch Alarms                                                                                   |
|    IAM Role                                                                                            |
| [DIM] EKS Node Groups                                                                                 |
| [DIM] Elastic Beanstalk                                                                               |
|    CloudTrail Events                                                                                   |
|                                                                                                        |
+--------------------------------------------------------------------------------------------------------+
```

Notes:
- Title shows just `related` with the resource identity
- Forward relationships show counts: Security Groups `(3)`, VPC `(1)`, etc.
- Reverse/expensive lookups show NO count: Auto Scaling Groups, Target Groups,
  CloudWatch Alarms, IAM Role -- just normal text (available but no count)
- Dim rows: EKS Node Groups, Elastic Beanstalk (no related resources found)
- CloudTrail Events: just a normal row, no special indicator
- Cursor is on "Security Groups" -- pressing Enter opens filtered SG list
- Pressing `j` from "IAM Role" skips the two dim rows to "CloudTrail Events"

### 8.4 List -- Filter active

```
 a9s v3.26.0  prod:us-east-1                                                                         /sec
+----------------------------------- related -- i-0abc123 (web-prod) -----------------------------------+
| [SELECTED] Security Groups (3)                                                                        |
|                                                                                                        |
|                                                                                                        |
|                                                                                                        |
|                                                                                                        |
+--------------------------------------------------------------------------------------------------------+
```

Notes:
- Filter `/sec` matches "Security Groups"
- Header right shows `/sec` in amber bold with cursor
- Only matching rows are displayed
- Dim matches still appear (dim) but cursor skips them
- Title does not change when filtering -- still just `related`

### 8.5 List at 80 columns

```
 a9s v3.26.0  prod:us-east-1                                ? for help
+-------------- related -- i-0abc123 (web-prod) ------------+
| [SELECTED] Security Groups (3)                            |
|    VPC (1)                                                 |
|    Subnet (1)                                              |
|    Elastic IPs (1)                                         |
|    Network Interfaces (2)                                  |
|    Auto Scaling Groups                                     |
|    Target Groups                                           |
|    CloudWatch Alarms                                       |
|    IAM Role                                                |
| [DIM] EKS Node Groups                                     |
| [DIM] Elastic Beanstalk                                    |
|    CloudTrail Events                                       |
|                                                            |
+------------------------------------------------------------+
```

At 80 columns, the list narrows but all content still fits. Counts appear
inline after the name. Rows without counts have empty right side.

### 8.6 Smart Enter: Single Resource (VPC) -> Direct to Detail

When cursor is on "VPC (1)" and user presses Enter:

```
 a9s v3.26.0  prod:us-east-1                                                                         ? for help
+--------------------------------------- vpc-0aaa111bbb222cc -----------------------------------------------+
| VpcId:                vpc-0aaa111bbb222cc                                                                 |
| CidrBlock:            10.0.0.0/16                                                                         |
| State:                available                                                                           |
| IsDefault:            false                                                                               |
| DhcpOptionsId:        dopt-0abc123                                                                        |
| InstanceTenancy:      default                                                                             |
| Tags:                                                                                                     |
|     - Key: Name                                                                                           |
|       Value: production-vpc                                                                               |
|     - Key: Environment                                                                                    |
|       Value: prod                                                                                         |
|                                                                                                           |
+-----------------------------------------------------------------------------------------------------------+
```

No intermediate list -- jumps directly to detail because count was 1.

### 8.7 Smart Enter: Multiple Resources (Security Groups) -> Filtered List

When cursor is on "Security Groups (3)" and user presses Enter:

```
 a9s v3.26.0  prod:us-east-1                                                                         ? for help
+----------------- sg-instances(3) -- i-0abc123 (web-prod) ------------------------------------------------+
| NAME                    GROUP ID               VPC ID                   DESCRIPTION                       |
| [SELECTED]web-sg        sg-0abc111222333444    vpc-0aaa111bbb222cc      Web server security group         |
|   db-access-sg          sg-0def555666777888    vpc-0aaa111bbb222cc      Database access from web tier     |
|   monitoring-sg         sg-0ghi999000111222    vpc-0aaa111bbb222cc      Monitoring agent inbound          |
|                                                                                                           |
| 3 security groups                                                                                         |
+-----------------------------------------------------------------------------------------------------------+
```

This is a standard resource list, filtered to only show the SGs attached to
i-0abc123. All list features work: filter, sort, detail, YAML, copy, and
pressing `r` again opens a new related list for THAT security group.

### 8.8 Deep Navigation: Header with Depth Indicator

After navigating EC2 -> related -> SG list -> SG detail -> related -> VPC
detail -> related (depth 7):

```
 a9s [7]  prod:us-east-1                                                                             ? for help
+-------------------------------- related -- vpc-0aaa111 (production-vpc) --------------------------------+
| [SELECTED] Subnets (6)                                                                                 |
|    Security Groups (12)                                                                                |
|    Route Tables (3)                                                                                    |
|    NAT Gateways (2)                                                                                    |
|    Internet Gateways (1)                                                                               |
|    VPC Endpoints (4)                                                                                   |
|    Transit Gateways                                                                                    |
|    EC2 Instances                                                                                       |
|    Load Balancers                                                                                      |
|    Lambda Functions                                                                                    |
|    EKS Clusters                                                                                        |
|    DB Instances                                                                                         |
|    ElastiCache                                                                                         |
|    Target Groups                                                                                       |
| [DIM] OpenSearch                                                                                       |
| [DIM] Redshift                                                                                         |
| [DIM] MSK Clusters                                                                                     |
|    CloudTrail Events                                                                                   |
|                                                                                                        |
+--------------------------------------------------------------------------------------------------------+
```

Notes:
- Header shows `[7]` instead of version number to signal depth
- VPC has 18 related types (the heaviest case) -- all fit in a scrollable list
- P0 types (Subnets, Security Groups, Route Tables) appear first with counts
  (cheap forward lookups from VPC fields)
- P1 types that are forward (NAT GW, IGW, Endpoints) show counts
- P1 types that are reverse (EC2, Load Balancers, Lambda, etc.) show no count
  -- just normal text indicating availability
- Dim rows (OpenSearch, Redshift, MSK) -- no related resources found
- CloudTrail Events is always last, shown as a normal row
- User can press `esc` 6 times to return to the EC2 list

### 8.9 All Dim State (No Related Resources Found)

```
 a9s v3.26.0  prod:us-east-1                                                                         ? for help
+------------------------------- related -- ami-0abc123 (my-custom-ami) ---------------------------------+
| [DIM] EC2 Instances                                                                                    |
| [DIM] Auto Scaling Groups                                                                              |
| [DIM] EBS Snapshots                                                                                    |
| [DIM] CloudTrail Events                                                                                |
|                                                                                                        |
|            No related resources found. Press ctrl+r to refresh or esc to go back.                      |
|                                                                                                        |
+--------------------------------------------------------------------------------------------------------+
```

All rows dim. The message appears only after ALL checks have completed and
all returned zero results.

### 8.10 Long List with Scroll Indicator (27 related types for VPC at 80 cols)

When the list exceeds the visible frame height, a scroll indicator shows:

```
 a9s v3.26.0  prod:us-east-1                                ? for help
+------------- related -- vpc-0aaa111 (production-vpc) -----+
| [SELECTED] Subnets (6)                                   |
|    Security Groups (12)                                   |
|    Route Tables (3)                                       |
|    NAT Gateways (2)                                       |
|    Internet Gateways (1)                                  |
|    VPC Endpoints (4)                                      |
|    Transit Gateways                                       |
|    EC2 Instances                                          |
|    Load Balancers                                         |
|    Lambda Functions                                       |
|                                          v 17 more below  |
+-----------------------------------------------------------+
```

The scroll indicator uses dim text (`#565f89`) and appears at the bottom of the
visible area. `pgdn`/`ctrl+d` scrolls a full page, `j`/`down` scrolls one row.

---

## 9. Integration with Existing View Stack

### How It Fits

The related-types list is a new view type (`RelatedListView`) that pushes onto
the existing `[]ViewState` stack, alongside `ResourceListView`, `DetailView`,
`YAMLView`, `HelpView`, etc.

```go
// Conceptual view stack entry
type ViewState struct {
    ViewType    ViewType          // MainMenu, ResourceList, Detail, YAML, Help, RelatedList, ...
    ResourceDef ResourceTypeDef   // which resource type (for list/detail/yaml)
    ResourceID  string            // specific resource (for detail/yaml/related)
    Context     map[string]string // inherited context for child views
    // ... existing fields
}
```

When `r` is pressed from a detail or YAML view:
1. The current resource's type definition is inspected for forward relationships
   (by parsing its Fields for known ID patterns like `VpcId`, `SubnetId`,
   `SecurityGroups[].GroupId`).
2. The related-resources research (`docs/design/related-resources/{type}.md`)
   provides reverse and algorithmic relationships.
3. A `RelatedListView` is created with the combined list and pushed onto the
   view stack.

When Enter is pressed on a list row:
- **Count > 1**: Push a `ResourceListView` with a filter context that restricts
  the list to only related instances.
- **Count = 1**: Push a `DetailView` for that specific resource.
- **CloudTrail**: Push the `ct-search` view pre-populated with resource filters
  (same as pressing `T`).

### Filtered Resource List

The filtered list opened from the related list is a standard `ResourceListView`
with one addition: a **filter context** that tells the fetcher to return only
related instances. This is NOT the same as the `/` text filter -- it is a
server-side or in-memory scope filter.

The frame title reflects the scoping:
`sg-instances(3) -- i-0abc123 (web-prod)`

The user can still apply `/` text filter on top of the scope filter.

---

## 10. Color/Style Table

No new colors or styles are introduced. All elements use the existing Tokyo
Night Dark palette from `design.md` section 1. The related-resources view
reuses the exact same styles as the main menu.

| Element | Foreground | Background | Style | Existing Mapping |
|---------|-----------|-----------|-------|-----------------|
| Row label (available) | `#c0caf5` | -- | -- | Table row normal |
| Row label (selected) | `#1a1b26` | `#7aa2f7` | Bold | Table row selected |
| Row label (dim) | `#565f89` | -- | Dim | Table row dim |
| Count text (available) | `#c0caf5` | -- | -- | Table row normal |
| Count text (selected) | `#1a1b26` | `#7aa2f7` | Bold | Table row selected |
| Depth indicator `[N]` | `#565f89` | -- | -- | Header dim |
| No-results message | `#565f89` | -- | Dim | Status unknown |
| Frame border | `#414868` | -- | -- | Table border |
| Frame title | `#c0caf5` | -- | Bold | Frame title |
| Scroll indicator | `#565f89` | -- | Dim | Dim text |

---

## 11. Bubbles Components

| Component | Bubbles | Notes |
|-----------|---------|-------|
| List rows | Custom renderer | Same pattern as main menu |
| List scrolling | `bubbles/viewport` | If list exceeds frame height, scroll with j/k/pgup/pgdn |
| Filter input | `bubbles/textinput` | Same as existing `/` filter, in header right |
| Help screen | Custom multi-column | Same as existing help renderer |
| Filtered resource list | Existing `ResourceListView` | Reused with scope filter |

No spinner component is used. No new visual elements are introduced.

---

## 12. Responsive Behavior

### Width Behavior

The single-column list adapts to any terminal width. The label text is
left-aligned with the optional count inline immediately after the name. The
entire row is left-aligned with no right-side content.

| Terminal Width | Notes |
|----------------|-------|
| < 60 cols | "Terminal too narrow" |
| 60-79 cols | Functional, shorter gap between label and count |
| 80+ cols | Comfortable |

### Height Behavior

If the list has more rows than fit in the frame, the list scrolls vertically.
A dim scroll indicator appears at the bottom:
`v N more below`

And at the top when scrolled down:
`^ N more above`

Minimum height follows the existing design: 7 lines minimum, 3 lines overhead
(header + frame borders), so the list gets at least 4 content lines.

### Narrow Header

At narrow widths, the depth indicator compresses further:

```
Normal:     a9s v3.26.0  prod:us-east-1           ? for help
Narrow:     a9s [7]  prod:us-east-1
Very narrow: a9s prod:us-east-1
```

---

## 13. Help Screen for Related List

When `?` is pressed on the related-types list, the help screen shows:

```
 a9s v3.26.0  prod:us-east-1                                                                         ? for help
+----------------------------------------------- Help ---------------------------------------------------+
| RELATED                    GENERAL              NAVIGATION           HOTKEYS                            |
|                                                                                                        |
| <enter>  Open type         <ctrl-r> Refresh     <j>       Down       <?>  Help                         |
| <esc>    Go back           <q>      Quit        <k>       Up         <:>  Command                      |
|                            </>      Filter      <g>       Top        <r>  Related                      |
|                                                 <G>       Bottom                                        |
|                                                 <pgdn>    Page down                                     |
|                                                 <pgup>    Page up                                       |
|                                                                                                        |
|                      Press any key to close                                                             |
+--------------------------------------------------------------------------------------------------------+
```

Note: compared to the previous grid help, `h`/`l` (left/right) are removed
since there is no horizontal movement in a single-column list. `pgup`/`pgdn`
are added for pagination.

---

## 14. Open Design Questions (Resolved)

### Q1: Multi-column layout or single-column list?
**Answer (v2.0):** Single-column list. The multi-column grid was dropped in
favor of consistency with every other view in a9s. See section 4.

### Q2: Grouping: by category, flat alphabetical, or flat by priority?
**Answer (v2.0):** Flat list sorted by priority (P0 first, then P1, P2) with
CloudTrail Events always last. Category headers were dropped -- the median
resource has ~13 related types, which fits on one screen without grouping. See
section 4.

### Q3: Header/frame title format when 4+ hops deep?
**Answer:** Depth indicator `[N]` replaces version in header. Frame title
shows the immediate context only (not full breadcrumb). See section 3.

### Q4: What visual treatment for rows that are still being checked?
**Answer (v3.0):** No special treatment. Rows start dim and either become
normal when the check finds results, or stay dim. No "Checking..." text, no
spinners. The user sees rows silently "light up" as results arrive. This
matches the main menu pattern where rows are either normal or dim.

### Q5: What happens when ALL related types are unavailable?
**Answer:** List remains visible (all dim) with a dim message below after all
checks complete. See wireframe 8.9.

### Q6: Pagination for long related type lists?
**Answer (v2.0):** Yes, same pagination as resource lists. The heaviest
resources (VPC, EC2, Lambda) have 20-27 related types. Standard scrolling
with `j/k/g/G/pgup/pgdn/ctrl+u/ctrl+d`. See section 4.

### Q7: Should counts be shown for all rows?
**Answer (v3.0):** No. Counts are shown only for cheap API lookups (forward
relationships where the IDs are already in the resource's fields). Expensive
reverse and algorithmic lookups show no count -- the row is simply normal
(available) or dim (unavailable). This avoids suggesting that a9s always
knows the count, and it matches the main menu where counts come from a
background fetch that may not always have exact numbers.

---

## 15. Example: EC2 Instance Related Types

For an EC2 instance, the related types list would show (based on the API
response fields in `views_reference.yaml` and the reverse relationships in
`docs/design/related-resources/ec2.md`):

### Forward (from API response fields) -- counts shown

| Related Type | Source Field(s) | Cardinality | Count Shown? |
|-------------|----------------|-------------|--------------|
| VPC | `VpcId` | 1 | Yes `(1)` |
| Subnet | `SubnetId` | 1 | Yes `(1)` |
| Security Groups | `SecurityGroups[].GroupId` | 1+ | Yes `(N)` |
| AMI | `ImageId` | 1 | Yes `(1)` |
| EBS Volumes | `BlockDeviceMappings[].Ebs.VolumeId` | 1+ | Yes `(N)` |
| Network Interfaces | `NetworkInterfaces[].NetworkInterfaceId` | 1+ | Yes `(N)` |
| IAM Instance Profile | `IamInstanceProfile.Arn` | 0-1 | Yes `(1)` |
| Key Pair | `KeyName` | 0-1 | Yes `(1)` |

### Reverse (from ec2.md research) -- no counts shown

| Related Type | Priority | Check Method | Count Shown? |
|-------------|----------|-------------|--------------|
| Target Groups | P0 | Iterate TGs, match instance ID | No |
| Auto Scaling Groups | P0 | `DescribeAutoScalingInstances` | No |
| CloudWatch Alarms | P1 | Filter alarms by InstanceId dimension | No |
| CloudFormation Stacks | P1 | Check tags | No |
| EKS Node Groups | P1 | Check tags | No |
| Elastic Beanstalk | P2 | Check tags | No |

### Algorithmic (from ec2.md research) -- no counts shown

| Related Type | Priority | Check Method | Count Shown? |
|-------------|----------|-------------|--------------|
| EBS Snapshots | P1 | Snapshots of attached volumes | No |
| Elastic IPs | P1 | DescribeAddresses filtered by instance ID | No |

### Always Present -- no count shown

| Related Type | Notes | Count Shown? |
|-------------|-------|--------------|
| CloudTrail Events | Opens ct-search pre-filtered for this instance | No |

Total: up to 18 types for EC2, displayed as a flat list sorted by priority.

---

## 16. List Row Details

### Selected Row Rendering

The selected row gets full-width highlight, identical to the main menu:

```
| [SELECTED] Security Groups (3)                                                                        |
```

vs. when "VPC" is selected:

```
| [SELECTED] VPC (1)                                                                                    |
```

The selected row's full width gets `#7aa2f7` background with `#1a1b26`
foreground, matching the existing table row selected style. Count text, if
present, is also rendered in the selected style (not separately colored).

### Row Height

Each row is exactly one line. No wrapping, no multi-line rows.

### Inline Count Format

The count appears immediately after the resource type name, separated by a
single space (e.g., `Security Groups (3)`, `Subnets (12)`, `EBS Volumes (20+)`).
The entire row is left-aligned. Rows without counts have no additional text
after the name.
