# Current State Audit

## 1. Application Layout (Text Mockup of Current UI)

### 1.1 Main Menu View

```
a9s | profile: staging | us-east-1                                 v0.4.5
main

  AWS Resources

    > S3 Buckets
      EC2 Instances
      RDS Instances
      ElastiCache Redis
      DocumentDB Clusters
      EKS Clusters
      Secrets Manager

  Press : for commands, ? for help









Ready
```

### 1.2 Resource List View (EC2 Example)

```
a9s | profile: staging | us-east-1                                 v0.4.5
EC2 Instances (12)
  Instance ID          State        Type           Private IP       Public IP        Launch Time
  --------------------  ----------  --------------  ----------------  ----------------  ----------------------
> i-0a1b2c3d4e5f67890  running     t3.medium       10.0.1.42        54.23.45.67      2025-11-15 09:30:45
  i-0b2c3d4e5f678901a  running     t3.large        10.0.1.43                         2025-12-01 14:22:33
  i-0c3d4e5f67890123b  stopped     m5.xlarge       10.0.2.10                         2024-08-20 11:15:00














Ready
```

### 1.3 Detail View (EC2 Example)

```
a9s | profile: staging | us-east-1                                 v0.4.5
EC2 Instances > detail

  i-0a1b2c3d4e5f67890

  InstanceId: i-0a1b2c3d4e5f67890
  State:
    Code: 16
    Name: running
  InstanceType: t3.medium
  ImageId: ami-0abcdef1234567890
  VpcId: vpc-12345678
  SubnetId: subnet-abcdef12
  PrivateIpAddress: 10.0.1.42
  PublicIpAddress: 54.23.45.67
  SecurityGroups:
    - GroupId: sg-12345678
      GroupName: web-server-sg
  LaunchTime: 2025-11-15 09:30:45
  Architecture: x86_64
  Tags:
    - Key: Name
      Value: web-server-01


Ready
```

---

## 2. Strengths (What Works Well)

### S1. Solid Foundation Architecture
**Severity**: N/A (positive)
**Details**: The Bubble Tea v2 model is correctly implemented with Init/Update/View. Message passing for async AWS API calls is well-structured. The separation of AWS clients, resource models, views, and UI components shows thoughtful organization.

### S2. Config-Driven Views
**Severity**: N/A (positive)
**Details**: The `views.yaml` configuration system allows users to customize columns and detail fields per resource type. The fallback to built-in defaults is robust. This is a significant differentiator -- k9s does not have this level of view customization.

### S3. Vim-Style Navigation
**Severity**: N/A (positive)
**Details**: `j/k/g/G` navigation, `/` for filter, `:` for commands -- these match established conventions. Users familiar with vim, k9s, or lazygit will feel at home immediately.

### S4. S3 Hierarchical Browsing
**Severity**: N/A (positive)
**Details**: Drilling into S3 buckets > prefixes > objects with breadcrumb tracking is a well-executed feature. No other TUI tool does this for AWS S3.

### S5. Context-Sensitive Help
**Severity**: N/A (positive)
**Details**: The help overlay shows different keybindings per view type. This is the right pattern -- showing all keys everywhere creates noise.

### S6. Command Autocomplete
**Severity**: N/A (positive)
**Details**: The `:` command mode shows dimmed autocomplete suggestions. This reduces the learning curve for discovering available commands.

### S7. Reflection-Based Field Extraction
**Severity**: N/A (positive)
**Details**: The `fieldpath` package uses reflection to extract values from raw AWS SDK structs. This means the detail view can show any field path without manual mapping code. This is a powerful and extensible design.

### S8. NO_COLOR Compliance
**Severity**: N/A (positive)
**Details**: The styles package respects `NO_COLOR` environment variable, stripping all ANSI formatting. Good accessibility practice.

---

## 3. Weaknesses and Issues

### Note on Known Bugs
The 15 bugs listed in the 003-fix-ui-bugs spec are excluded from this audit as they are already being addressed. This audit focuses on structural/design issues beyond those bugs.

---

### W1. Monolithic AppState Model
**Severity**: High
**Impact**: As more AWS services are added, `AppState` will grow unmanageably. It already has ~30 fields and ~1900 lines of code in `app.go`. Every view's state is mixed into one struct.
**Affected personas**: All (developers maintaining the codebase)
**Evidence**: `AppState` contains `S3Bucket`, `S3Prefix` (S3-specific), `ProfileSelector`, `RegionSelector` (selector-specific), `Detail`, `JSONData`, `Reveal` (view-specific), plus generic UI state. Adding a new service requires modifying this central struct.
**Suggested fix**: Refactor to a component-based architecture where each view owns its model. The root model dispatches to the active view component. See FEATURE_IMPROVEMENTS.md for details.

### W2. No Visual Status Indicators (Color-Coded States)
**Severity**: High
**Impact**: In the resource list, "running", "stopped", "available", "creating" all appear as plain text. Users cannot scan the list quickly to identify problems. In k9s, status colors are one of the most important visual cues.
**Affected personas**: DevOps (scanning for unhealthy resources), CTO (quick health overview)
**Evidence**: The `styles.go` file defines `StatusRunning`, `StatusStopped`, `StatusPending`, `StatusAvailable` colors, but they are never applied in `renderResourceList()`. The colors exist but are unused.
**Suggested fix**: Apply color to the status column based on the status value. Map known status strings (running, stopped, available, pending, creating, deleting, error, etc.) to the defined color constants.

### W3. No Resource Count Visible During Loading
**Severity**: Medium
**Impact**: When loading resources, the user sees "Loading resources..." with no indication of progress. For S3 buckets with pagination, this could take several seconds with no feedback.
**Affected personas**: All
**Evidence**: `renderResourceList()` returns `"\n  Loading resources..."` -- a static string with no spinner animation or progress count.
**Suggested fix**: Implement a spinner animation using Bubble Tea's tick mechanism. Show incremental count if the API supports pagination ("Loading... 47 items so far").

### W4. No Confirmation for Destructive Actions
**Severity**: Medium
**Impact**: Currently, the app is read-only, so this is not yet critical. However, as write operations are added (secret rotation, instance actions), the lack of a confirmation dialog pattern will become a safety issue.
**Affected personas**: All
**Evidence**: No confirmation dialog component exists in the codebase.
**Suggested fix**: Build a reusable confirmation dialog component now. Pattern: "Are you sure you want to [action]? [y/N]" with default-safe behavior.

### W5. Empty Status Bar Wastes Space
**Severity**: Medium
**Impact**: When there is no status message, the status bar shows "Ready" -- a single word on an entire terminal line. This space could show useful context like available key hints for the current view (as k9s does).
**Affected personas**: All (especially new users discovering features)
**Evidence**: `renderStatusBar()` falls through to `return styles.StatusBarStyle.Render("Ready")`.
**Suggested fix**: Show context-sensitive key hints in the status bar, similar to k9s: `<d>describe <y>yaml <c>copy </>filter <:>command <?>help`. This serves as both a status bar and a discoverability aid.

### W6. Static Data -- No Auto-Refresh
**Severity**: Medium
**Impact**: Resources are fetched once when the user navigates to a resource type. If infrastructure changes (instances started/stopped, new resources created), the user must manually press Ctrl-R to refresh. k9s auto-refreshes every few seconds.
**Affected personas**: DevOps (monitoring during incidents), Senior Dev (waiting for deployment)
**Evidence**: No tick-based refresh mechanism in `Update()`. The only refresh path is the explicit `Ctrl-R` keybinding.
**Suggested fix**: Add configurable auto-refresh with a default interval (e.g., 30 seconds). Show "last refreshed: 15s ago" in the header or status bar. Allow toggling auto-refresh on/off.

### W7. Breadcrumb Separator Uses Unicode ">" Instead of Box-Drawing
**Severity**: Low
**Impact**: The breadcrumb separator " > " is plain text. Professional TUIs use consistent visual language. The current implementation uses the Unicode single right-pointing angle quotation mark (U+203A) in the `ui/breadcrumbs.go` but the `renderBreadcrumbs()` in `app.go` uses " > " (plain ASCII). This inconsistency means the breadcrumb UI component is not being used.
**Affected personas**: All (visual polish)
**Evidence**: `app.go:1303` uses `strings.Join(m.Breadcrumbs, " > ")` directly, bypassing `ui.RenderBreadcrumbs()`.
**Suggested fix**: Use the `ui.RenderBreadcrumbs()` function consistently. Consider using a more visually distinct separator like `" / "` or the Unicode chevron `" > "` (U+203A) that is already defined.

### W8. Header Does Not Use the UI Component
**Severity**: Low
**Impact**: `app.go` has `renderHeader()` which duplicates logic from `ui.RenderHeader()`. This means changes to the header component do not propagate to the actual rendered header.
**Affected personas**: Developers maintaining the codebase
**Evidence**: `ui/header.go` defines `RenderHeader()` but `app.go:1280` has its own `renderHeader()` method.
**Suggested fix**: Delegate to `ui.RenderHeader()` from `app.go`.

### W9. No Page Up / Page Down Support
**Severity**: Medium
**Impact**: Scrolling through hundreds of S3 objects or EC2 instances one line at a time is tedious. Power users expect PgUp/PgDn or Ctrl-U/Ctrl-D (vim half-page scroll).
**Affected personas**: DevOps (browsing large resource lists)
**Evidence**: The `KeyMap` struct has no PgUp/PgDn bindings. Only `j/k` (single line) and `g/G` (top/bottom) exist.
**Suggested fix**: Add `Ctrl-U` / `Ctrl-D` for half-page scroll, and `Ctrl-F` / `Ctrl-B` for full-page scroll. Also support `PgUp` / `PgDn` keys.

### W10. No Mouse Support
**Severity**: Low
**Impact**: While keyboard-first is the correct philosophy, Bubble Tea v2 supports mouse events. Clicking a resource to select it, scrolling with the mouse wheel -- these are free wins for discoverability.
**Affected personas**: New users, CTO persona (less keyboard-oriented)
**Evidence**: No `tea.MouseMsg` handling in `Update()`.
**Suggested fix**: Add basic mouse support: click to select row, scroll wheel for navigation. Keep keyboard as primary.

### W11. Filter Does Not Show Match Highlighting
**Severity**: Medium
**Impact**: When filtering, matching resources are shown but the matched substring is not highlighted. In k9s, the matching portion of text is highlighted in a different color, making it easy to see why a resource matched.
**Affected personas**: All
**Evidence**: `FilterResources()` in `filter.go` returns matching resources but does not indicate which field or substring matched.
**Suggested fix**: Highlight the matching substring in the rendered table row using a contrasting color.

### W12. Command Mode Has No Visual Distinction
**Severity**: Medium
**Impact**: When the user enters command mode with `:`, the status bar changes to show `:text` but there is no strong visual indicator that the app is in a different mode. vim uses a prominent command line; k9s shows a distinct command bar.
**Affected personas**: New users
**Evidence**: Command mode rendering in `renderStatusBar()` uses `styles.StatusBarStyle.Render(display)` -- the same faint style as normal status.
**Suggested fix**: Use a more prominent style for command mode: bold text, different background, or border to make the mode switch visually obvious.

### W13. Error Messages Lack Actionable Guidance
**Severity**: Medium
**Impact**: Error messages like "Error fetching ec2: ..." show the raw error. Users need guidance on what to do.
**Affected personas**: All (especially less experienced users)
**Evidence**: `APIErrorMsg` handler prepends "Error fetching [type]: " to the raw error. The expired token case does suggest "aws sso login", which is good -- but other errors do not provide actionable guidance.
**Suggested fix**: Map common AWS error codes to actionable messages. AccessDenied: "Check IAM permissions for this profile." Throttling: "Too many requests. Will retry in [N] seconds." UnrecognizedClient: "AWS credentials invalid. Run: aws configure."

### W14. No Visual Indication of Sortable Columns
**Severity**: Medium
**Impact**: Users do not know which columns support sorting or which column is currently sorted. k9s shows sort arrows on column headers.
**Affected personas**: DevOps, Senior Dev
**Evidence**: `SortByName/Status/Age` keybindings exist but the header row in `renderResourceList()` shows no sort indicator.
**Suggested fix**: Show a sort arrow (up/down triangle) on the currently sorted column header. Bold or highlight sortable column headers.

### W15. Copy Feedback Is Only in Status Bar
**Severity**: Low
**Impact**: After pressing `c`, the status bar shows "Copied: [id]" but if the user is focused on the resource list, they may not notice the status bar change. A brief flash or highlight on the copied row would be more noticeable.
**Affected personas**: All
**Evidence**: Copy handler in `handleResourceListKeys()` sets `m.StatusMessage`.
**Suggested fix**: Consider a brief visual flash on the selected row after copy, in addition to the status message.

### W16. Profile and Region Selectors Lack Filter
**Severity**: Medium
**Impact**: Users with many AWS profiles (10+) must scroll through all of them. No filter capability in the profile/region selectors.
**Affected personas**: DevOps (manages many accounts)
**Evidence**: `ProfileSelectModel` and `RegionSelectModel` have no filter field or filtering logic.
**Suggested fix**: Add `/` filter support to profile and region selectors, reusing the same filter pattern as the main menu and resource list.

### W17. Resource List Table Is Custom-Rendered, Not Using bubble-table
**Severity**: Low (architectural debt)
**Impact**: The `resourcelist.go` file defines `ResourceListModel` using `evertras/bubble-table`, but `app.go:renderResourceList()` ignores it entirely and renders tables manually. This means two table implementations exist but only one is used.
**Affected personas**: Developers maintaining the codebase
**Evidence**: `resourcelist.go` imports `github.com/evertras/bubble-table` and builds a table model, but `app.go` never calls `ResourceListModel.View()`. Instead, it builds rows manually in `renderResourceList()`.
**Suggested fix**: Either commit to the custom renderer (and remove the unused bubble-table code) or migrate to the bubble-table component. The custom renderer provides more control for features like horizontal scrolling and status coloring, so it is likely the better choice. Remove the dead code.

### W18. No ARN Display or Copy
**Severity**: Medium
**Impact**: ARNs are the universal identifier in AWS. Users frequently need to copy an ARN to paste into IAM policies, CLI commands, or Terraform configs. Currently, `c` copies the resource ID (e.g., instance ID) but not the ARN.
**Affected personas**: DevOps, Senior Dev
**Evidence**: Copy handler uses `selected.ID` which maps to the primary identifier, not the ARN. No ARN field is shown in list views.
**Suggested fix**: Add ARN to detail views. Consider a separate keybinding (e.g., `Shift-C` or `a` for ARN copy) or make `c` context-aware to copy the most useful identifier.

### W19. No Keyboard Shortcut to Jump Directly to Resource Types
**Severity**: Low
**Impact**: To switch from viewing EC2 to RDS, the user must either press Escape to go back to the main menu and navigate there, or type `:rds` in command mode. k9s allows direct type switching with just the `:` command, but also supports aliases like `:deploy`, `:svc` etc. The current command set is good but could be faster.
**Affected personas**: Power users
**Evidence**: Command mode supports resource type names but requires entering command mode first. No single-key shortcuts for common resource types.
**Suggested fix**: Consider number shortcuts on the main menu (1-7 for the seven resource types) or allowing `:` + first letter for unique prefixes.

### W20. No Indication of AWS Connection Status
**Severity**: Medium
**Impact**: If AWS credentials expire mid-session, the user only discovers this when they try to fetch data and get an error. There is no persistent indicator of connection health.
**Affected personas**: All
**Evidence**: Initial connection status is shown as a status message that gets replaced by subsequent messages. No persistent indicator.
**Suggested fix**: Add a connection status indicator in the header bar. Green dot for connected, red for disconnected/expired. Show elapsed time since last successful API call.

---

## 4. Architecture Observations

### 4.1 State Management Complexity

The `AppState` struct uses value semantics (methods on `AppState`, not `*AppState`), which means every `Update()` call copies the entire state. For the current size this is fine, but as the state grows (more views, more cached data), this could become a performance concern. The `applyFilter()`, `sortResources()`, and `updateBreadcrumbs()` methods use pointer receivers but are called on value copies in `Update()`, which is correct (the mutations happen on the copy that is returned) but can be confusing.

### 4.2 Data Flow

```
User Input (KeyPressMsg)
    |
    v
AppState.Update() -- dispatches to handleXxxKeys()
    |
    v
May return tea.Cmd (async AWS API call)
    |
    v
API response arrives as tea.Msg (ResourcesLoadedMsg, APIErrorMsg, etc.)
    |
    v
AppState.Update() processes response, updates state
    |
    v
AppState.View() re-renders everything
```

This is the standard Bubble Tea flow and is correctly implemented.

### 4.3 Testing Infrastructure

The test suite is extensive with unit tests for every major component, integration tests for AWS API calls, and specific bug regression tests. This is a strong foundation for making UI changes confidently.
