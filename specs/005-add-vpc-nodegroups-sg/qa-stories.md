# QA User Stories: VPC, Security Groups, and EKS Node Groups

Feature: 005-add-vpc-nodegroups-sg
Date: 2026-03-18

Black-box user stories for three new resource types being added to a9s: VPC, Security Groups, and EKS Node Groups. All stories treat a9s as a black box -- input goes in, output comes out. No implementation knowledge is assumed.

---

## Table of Contents

1. [Main Menu Changes](#1-main-menu-changes)
2. [VPC List View](#2-vpc-list-view)
3. [VPC Detail View](#3-vpc-detail-view)
4. [VPC YAML View](#4-vpc-yaml-view)
5. [Security Groups List View](#5-security-groups-list-view)
6. [Security Groups Detail View](#6-security-groups-detail-view)
7. [Security Groups YAML View](#7-security-groups-yaml-view)
8. [EKS Node Groups List View](#8-eks-node-groups-list-view)
9. [EKS Node Groups Detail View](#9-eks-node-groups-detail-view)
10. [EKS Node Groups YAML View](#10-eks-node-groups-yaml-view)
11. [Command Mode Aliases](#11-command-mode-aliases)
12. [Edge Cases and Error Handling](#12-edge-cases-and-error-handling)
13. [Cross-View Navigation Flows](#13-cross-view-navigation-flows)
14. [No-Regression Verification](#14-no-regression-verification)

---

## 1. Main Menu Changes

### Story: Ten resource types are displayed on launch
**Given:** The user launches a9s in a terminal at least 80 columns wide and 7 lines tall.
**When:** The application finishes loading.
**Then:** The main menu lists exactly ten rows, one per resource type:
1. EC2 Instances
2. S3 Buckets
3. RDS Instances
4. ElastiCache Redis
5. DocumentDB Clusters
6. EKS Clusters
7. Secrets Manager
8. VPC
9. Security Groups
10. EKS Node Groups

(The exact ordering may vary; the key requirement is that all ten resource types appear.)

**AWS comparison:** No AWS API call is needed for the main menu -- it is a static list of resource types.

---

### Story: VPC resource type shows its shortname alias
**Given:** The main menu is displayed.
**When:** The user looks at the VPC row.
**Then:** The row shows the display name "VPC" on the left and a dimmed shortname alias `:vpc` on the right.

---

### Story: Security Groups resource type shows its shortname alias
**Given:** The main menu is displayed.
**When:** The user looks at the Security Groups row.
**Then:** The row shows the display name "Security Groups" on the left and a dimmed shortname alias `:sg` on the right.

---

### Story: EKS Node Groups resource type shows its shortname alias
**Given:** The main menu is displayed.
**When:** The user looks at the EKS Node Groups row.
**Then:** The row shows the display name "EKS Node Groups" (or similar) on the left and a dimmed shortname alias `:nodegroups` on the right.

---

### Story: Frame title count reflects all resource types
**Given:** The main menu is displayed with no filter active.
**When:** The user looks at the frame title.
**Then:** The frame title shows `resource-types(10)` centered in the top border, reflecting all ten resource types.

---

### Story: Enter on VPC opens the VPC list
**Given:** The main menu is displayed and the user has navigated to the VPC row.
**When:** The user presses `Enter`.
**Then:** The application navigates to the VPC resource list view.

---

### Story: Enter on Security Groups opens the Security Groups list
**Given:** The main menu is displayed and the user has navigated to the Security Groups row.
**When:** The user presses `Enter`.
**Then:** The application navigates to the Security Groups resource list view.

---

### Story: Enter on EKS Node Groups opens the Node Groups list
**Given:** The main menu is displayed and the user has navigated to the EKS Node Groups row.
**When:** The user presses `Enter`.
**Then:** The application navigates to the EKS Node Groups resource list view.

---

### Story: Filter on main menu matches new resource types
**Given:** The main menu is in filter mode.
**When:** The user types "vpc".
**Then:** The VPC row is visible (and possibly "Security Groups" if it also matches the substring). Other non-matching rows are hidden.

---

### Story: Filter on main menu matches Security Groups
**Given:** The main menu is in filter mode.
**When:** The user types "secur".
**Then:** The "Security Groups" row is visible. Other non-matching rows are hidden.

---

### Story: Filter on main menu matches Node Groups
**Given:** The main menu is in filter mode.
**When:** The user types "node".
**Then:** The "EKS Node Groups" row is visible. Other non-matching rows are hidden.

---

### Story: G jumps to last row which is now the 10th item
**Given:** The main menu is displayed and the first row is selected.
**When:** The user presses `G` (shift+g).
**Then:** The selection jumps to the last row (the 10th resource type in the list).

---

### Story: Cursor wraps from bottom to top across all 10 items
**Given:** The main menu is displayed and the last row (10th item) is selected.
**When:** The user presses `j` or the down-arrow key.
**Then:** The selection wraps around to the first row.

---

## 2. VPC List View

### A. Column Layout

#### VPC-LIST-01: Navigate to VPC list
**Given:** The main menu is displayed.
**When:** The user selects "VPC" and presses Enter (or types `:vpc` Enter).
**Then:** The frame title shows `vpcs(<count>)` with the total number of VPCs.
**And:** A loading spinner with "Fetching VPCs..." (or similar) appears while data loads.
**And:** Once loaded, the table displays these columns left-to-right:

| Column header | Data path | Width | AWS CLI JSON field |
|--------------|-----------|-------|--------------------|
| VPC ID | VpcId | ~20 | `.Vpcs[].VpcId` |
| Name | Tags (Name tag) | ~20 | `.Vpcs[].Tags[] where Key=Name` |
| CIDR Block | CidrBlock | ~18 | `.Vpcs[].CidrBlock` |
| State | State | ~12 | `.Vpcs[].State` |
| Is Default | IsDefault | ~10 | `.Vpcs[].IsDefault` |

**AWS comparison:**
```
aws ec2 describe-vpcs
```
Expected fields visible: VpcId, Name (from Tags), CidrBlock, State, IsDefault

---

#### VPC-LIST-02: Verify VPC ID column data
**Given:** The VPC list is displayed with at least one VPC.
**When:** The user looks at the VPC ID column.
**Then:** Each cell shows the `VpcId` field value (e.g. `vpc-0123456789abcdef0`) from the AWS response to `describe-vpcs`.

---

#### VPC-LIST-03: Verify Name column data
**Given:** The VPC list is displayed and some VPCs have a `Name` tag.
**When:** The user looks at the Name column.
**Then:** Each cell shows the value of the `Name` tag extracted from the VPC's `Tags` array. VPCs without a Name tag show an empty cell.

**AWS comparison:** The Name column value corresponds to the tag where `Key=Name` in `.Vpcs[].Tags[]`. This is a standard AWS convention, not a native VPC field.

---

#### VPC-LIST-04: Verify CIDR Block column data
**Given:** The VPC list is displayed.
**When:** The user looks at the CIDR Block column.
**Then:** Each cell shows the `CidrBlock` field value (e.g. `10.0.0.0/16`, `172.31.0.0/16`) from the AWS response.

---

#### VPC-LIST-05: Verify State column data
**Given:** The VPC list is displayed.
**When:** The user looks at the State column.
**Then:** Each cell shows the `State` field value. VPCs in AWS are typically in state `available` or `pending`.

---

#### VPC-LIST-06: Verify Is Default column data
**Given:** The VPC list is displayed and at least one VPC is the default VPC.
**When:** The user looks at the Is Default column.
**Then:** The default VPC shows `true` (or `Yes`); non-default VPCs show `false` (or `No`).

**AWS comparison:** Maps to `.Vpcs[].IsDefault` (boolean).

---

#### VPC-LIST-07: Column headers are styled bold blue
**Given:** The VPC list is displayed.
**When:** The user looks at the column headers.
**Then:** All column header labels render in bold with blue foreground (`#7aa2f7`).

---

#### VPC-LIST-08: No separator line below column headers
**Given:** The VPC list is displayed.
**When:** The user looks below the column headers.
**Then:** The first data row immediately follows the header row with no underline, rule, or divider between them.

---

### B. Status Coloring

#### VPC-LIST-09: VPC with state "available" has green row
**Given:** The VPC list is displayed with a VPC in state `available`.
**When:** The user looks at that VPC's row.
**Then:** The entire row is rendered in green (`#9ece6a`).

---

#### VPC-LIST-10: VPC with state "pending" has yellow row
**Given:** The VPC list is displayed with a VPC in state `pending` (e.g., just created).
**When:** The user looks at that VPC's row.
**Then:** The entire row is rendered in yellow (`#e0af68`).

---

#### VPC-LIST-11: Selected row overrides status color
**Given:** The VPC list is displayed and the cursor is on an `available` (green) VPC.
**When:** The user looks at the selected row.
**Then:** The selected row has a full-width blue background (`#7aa2f7`) with dark foreground (`#1a1b26`), bold text. The green status color is suppressed.

---

### C. Frame and Title

#### VPC-LIST-12: Frame title shows resource type and count
**Given:** The VPC list has loaded and the region has 5 VPCs.
**When:** The user looks at the top border.
**Then:** The frame title shows `vpcs(5)` centered between dashes.

---

#### VPC-LIST-13: Frame title count matches total VPCs from AWS
**Given:** The VPC list has loaded.
**When:** The user compares the count to `aws ec2 describe-vpcs`.
**Then:** The number in parentheses equals the total number of VPCs returned by the DescribeVpcs API.

---

### D. Navigation

#### VPC-LIST-14: Move cursor down with j
**Given:** The VPC list is displayed and the first row is selected.
**When:** The user presses `j`.
**Then:** The selection moves to the second row.

---

#### VPC-LIST-15: Move cursor up with k
**Given:** The VPC list is displayed and the second row is selected.
**When:** The user presses `k`.
**Then:** The selection moves to the first row.

---

#### VPC-LIST-16: Jump to top with g
**Given:** The VPC list is displayed and the fourth row is selected.
**When:** The user presses `g`.
**Then:** The selection jumps to the first row.

---

#### VPC-LIST-17: Jump to bottom with G
**Given:** The VPC list is displayed and the first row is selected.
**When:** The user presses `G`.
**Then:** The selection jumps to the last row.

---

#### VPC-LIST-18: Horizontal column scrolling
**Given:** The terminal width is narrow enough that not all 5 VPC columns fit.
**When:** The user presses `l` (or right arrow).
**Then:** The visible column window shifts right, revealing hidden rightmost columns.
**And:** Column headers scroll in sync with the data rows.
**When:** The user presses `h` (or left arrow).
**Then:** The visible column window shifts back left.

---

#### VPC-LIST-19: PageUp and PageDown navigation
**Given:** The VPC list has more items than fit on screen.
**When:** The user presses `PageDown` (or `ctrl+d`).
**Then:** The view scrolls down by one page.
**When:** The user presses `PageUp` (or `ctrl+u`).
**Then:** The view scrolls up by one page.

---

### E. Sort

#### VPC-LIST-20: Sort by name (N)
**Given:** The VPC list is displayed.
**When:** The user presses `N`.
**Then:** The list sorts by the Name column (or VPC ID if Name is used for sorting) ascending, and the sorted column header shows an up-arrow indicator (`Name` with appended arrow).

---

#### VPC-LIST-21: Sort by name toggles to descending
**Given:** The VPC list is sorted by name ascending.
**When:** The user presses `N` again.
**Then:** The sort toggles to descending and the indicator becomes a down-arrow.

---

#### VPC-LIST-22: Sort by status (S)
**Given:** The VPC list is displayed.
**When:** The user presses `S`.
**Then:** The list sorts by the State column ascending.

---

#### VPC-LIST-23: Sort by age (A)
**Given:** The VPC list is displayed.
**When:** The user presses `A`.
**Then:** The list sorts by age (creation time or equivalent date field) ascending.

---

#### VPC-LIST-24: Sort indicator appears on exactly one column
**Given:** The VPC list is sorted by name.
**When:** The user presses `S` to switch to status sort.
**Then:** The arrow indicator moves from the Name column to the State column.

---

### F. Filter

#### VPC-LIST-25: Filter VPCs by CIDR block
**Given:** The VPC list shows 5 VPCs.
**When:** The user presses `/` and types `10.0`.
**Then:** Only VPCs whose rows contain "10.0" (case-insensitive substring in any visible column) are displayed.
**And:** The frame title updates to `vpcs(<matched>/<total>)` (e.g. `vpcs(2/5)`).

---

#### VPC-LIST-26: Filter VPCs by name
**Given:** The VPC list shows VPCs, some of which have a Name tag containing "prod".
**When:** The user presses `/` and types `prod`.
**Then:** Only VPCs with "prod" in any visible cell (Name, VPC ID, CIDR, etc.) are displayed.

---

#### VPC-LIST-27: Filter VPCs by VPC ID
**Given:** The VPC list is displayed.
**When:** The user presses `/` and types `vpc-0abc`.
**Then:** Only VPCs whose VPC ID contains "vpc-0abc" are displayed.

---

#### VPC-LIST-28: Filter with no matches shows empty list
**Given:** The VPC list is in filter mode.
**When:** The user types "zzz" (matching no VPCs).
**Then:** No rows are displayed. The frame title shows `vpcs(0/5)`.

---

#### VPC-LIST-29: Esc clears filter
**Given:** The VPC list is in filter mode with text "prod".
**When:** The user presses `Esc`.
**Then:** The filter is cleared, all VPCs reappear, and the header right side returns to "? for help".

---

### G. Actions from List

#### VPC-LIST-30: Enter opens detail view
**Given:** The VPC list is displayed and the cursor is on a VPC row.
**When:** The user presses `Enter`.
**Then:** The detail view opens for the selected VPC.

---

#### VPC-LIST-31: d opens detail view
**Given:** The VPC list is displayed and the cursor is on a VPC row.
**When:** The user presses `d`.
**Then:** The detail view opens for the selected VPC (same behavior as Enter).

---

#### VPC-LIST-32: y opens YAML view
**Given:** The VPC list is displayed and the cursor is on a VPC row.
**When:** The user presses `y`.
**Then:** The YAML view opens for the selected VPC.

---

#### VPC-LIST-33: c copies VPC ID to clipboard
**Given:** The VPC list is displayed and the cursor is on a VPC with ID `vpc-0123456789abcdef0`.
**When:** The user presses `c`.
**Then:** The VPC ID (or Name/ARN) is copied to the system clipboard.
**And:** The header right side briefly shows "Copied!" in green bold (`#9ece6a`).
**And:** The flash message auto-clears after approximately 2 seconds.

---

#### VPC-LIST-34: Esc returns to main menu
**Given:** The VPC list is displayed.
**When:** The user presses `Esc`.
**Then:** The view transitions back to the main resource-type menu.

---

#### VPC-LIST-35: ? opens help screen
**Given:** The VPC list is displayed.
**When:** The user presses `?`.
**Then:** The help screen replaces the frame content.
**And:** Pressing any key closes the help and returns to the VPC list.

---

#### VPC-LIST-36: Ctrl+R refreshes VPC list
**Given:** The VPC list is displayed.
**When:** The user presses `Ctrl+R`.
**Then:** A loading spinner briefly appears; VPC data is re-fetched from AWS; the list repopulates with current data.

---

### H. Loading and Empty States

#### VPC-LIST-37: Loading state while fetching VPCs
**Given:** The user navigates to the VPC list.
**When:** The AWS API call is in progress.
**Then:** The frame title shows `vpcs` (no count yet), and a spinner with "Fetching VPCs..." appears centered in the frame.

---

#### VPC-LIST-38: Empty state with no VPCs
**Given:** The AWS account has no VPCs in the current region (this is unusual since most regions have a default VPC).
**When:** The VPC list finishes loading.
**Then:** The frame shows a centered empty-state message (e.g. "No VPCs found").
**And:** The frame title shows `vpcs(0)`.

---

### I. Edge Cases

#### VPC-LIST-39: VPC with no Name tag
**Given:** A VPC exists without a `Name` tag.
**When:** The VPC list is displayed.
**Then:** The VPC appears in the list with an empty Name column. All other columns (VPC ID, CIDR Block, State, Is Default) display normally.

---

#### VPC-LIST-40: Default VPC is distinguishable
**Given:** The region has both a default VPC and custom VPCs.
**When:** The VPC list is displayed.
**Then:** The Is Default column shows `true` (or `Yes`) for the default VPC and `false` (or `No`) for all others.

---

#### VPC-LIST-41: VPC with very long Name tag
**Given:** A VPC has a Name tag like "production-vpc-for-the-main-application-stack-us-east-1".
**When:** The VPC list is displayed.
**Then:** The Name value is truncated to fit within the column width. No layout overflow occurs.

---

#### VPC-LIST-42: x key does nothing on VPC list
**Given:** The VPC list is displayed.
**When:** The user presses `x`.
**Then:** Nothing happens. The `x` key (reveal) is exclusive to Secrets Manager.

---

---

## 3. VPC Detail View

### A. Entry and Frame

#### VPC-DETAIL-01: Open VPC detail view
**Given:** The VPC list is displayed and the cursor is on a VPC with ID `vpc-0123456789abcdef0`.
**When:** The user presses Enter or `d`.
**Then:** The detail view opens; the frame title shows the VPC ID (e.g. `vpc-0123456789abcdef0`) centered in the top border.

---

#### VPC-DETAIL-02: Frame replaces the table
**Given:** The VPC detail view is open.
**When:** The user looks at the layout.
**Then:** The detail view occupies the same frame area below the header. No additional chrome appears.

---

### B. Fields Displayed

#### VPC-DETAIL-03: All configured detail fields appear
**Given:** The VPC detail view is open.
**When:** The user reads the detail content.
**Then:** The following fields are displayed as key-value pairs (the exact set depends on the views.yaml configuration for VPC, but should include at minimum):

| Field | AWS CLI JSON key | Type | Example value |
|-------|-----------------|------|---------------|
| VpcId | `.Vpcs[].VpcId` | string | `vpc-0123456789abcdef0` |
| State | `.Vpcs[].State` | string | `available` |
| CidrBlock | `.Vpcs[].CidrBlock` | string | `10.0.0.0/16` |
| IsDefault | `.Vpcs[].IsDefault` | bool | `Yes` or `No` |
| InstanceTenancy | `.Vpcs[].InstanceTenancy` | string | `default` |
| DhcpOptionsId | `.Vpcs[].DhcpOptionsId` | string | `dopt-0abc123def456` |
| OwnerId | `.Vpcs[].OwnerId` | string | `123456789012` |
| CidrBlockAssociationSet | `.Vpcs[].CidrBlockAssociationSet` | array | see below |
| Ipv6CidrBlockAssociationSet | `.Vpcs[].Ipv6CidrBlockAssociationSet` | array | see below |
| Tags | `.Vpcs[].Tags` | array | see below |

**AWS comparison:**
```
aws ec2 describe-vpcs --vpc-ids vpc-0123456789abcdef0
```
Expected fields visible: VpcId, State, CidrBlock, IsDefault, InstanceTenancy, DhcpOptionsId, OwnerId, CidrBlockAssociationSet, Tags

---

#### VPC-DETAIL-04: VpcId field is shown
**Given:** The VPC detail view is open.
**When:** The user looks at the VpcId line.
**Then:** A line reads with key `VpcId` and value e.g. `vpc-0123456789abcdef0`.

---

#### VPC-DETAIL-05: State field is shown with color
**Given:** The VPC detail view is open for an `available` VPC.
**When:** The user looks at the State line.
**Then:** The value `available` is rendered in green (`#9ece6a`).

---

#### VPC-DETAIL-06: CidrBlock field is shown
**Given:** The VPC detail view is open.
**When:** The user looks at the CidrBlock line.
**Then:** A line reads with key `CidrBlock` and value e.g. `10.0.0.0/16`.

---

#### VPC-DETAIL-07: IsDefault field is shown
**Given:** The VPC detail view is open for the default VPC.
**When:** The user looks at the IsDefault line.
**Then:** The value shows `Yes` (for true). For non-default VPCs, it shows `No`.

---

#### VPC-DETAIL-08: InstanceTenancy field is shown
**Given:** The VPC detail view is open.
**When:** The user looks at the InstanceTenancy line.
**Then:** A line reads with key `InstanceTenancy` and value e.g. `default` or `dedicated`.

---

#### VPC-DETAIL-09: DhcpOptionsId field is shown
**Given:** The VPC detail view is open.
**When:** The user looks at the DhcpOptionsId line.
**Then:** A line reads with key `DhcpOptionsId` and value e.g. `dopt-0abc123def456`.

---

#### VPC-DETAIL-10: Tags field is shown
**Given:** The VPC detail view is open and the VPC has tags.
**When:** The user looks at the Tags section.
**Then:** The Tags appear as a multi-line block with each tag rendered as a Key/Value pair.

---

### C. Nested Field Rendering

#### VPC-DETAIL-11: CidrBlockAssociationSet nested fields
**Given:** The VPC detail view is open for a VPC with associated CIDR blocks.
**When:** The user looks at the CidrBlockAssociationSet section.
**Then:** Each association is rendered with its sub-fields (AssociationId, CidrBlock, CidrBlockState) indented under the parent heading.

---

#### VPC-DETAIL-12: Tags with multiple entries
**Given:** The VPC has 5 tags.
**When:** The detail view is open.
**Then:** All 5 tags render as YAML array items: `- Key: Name` / `  Value: prod-vpc` / `- Key: Environment` / `  Value: production` / etc.

---

#### VPC-DETAIL-13: Tags with zero entries
**Given:** The VPC has no tags.
**When:** The detail view is open.
**Then:** The Tags section shows empty (no entries).

---

### D. Key-Value Formatting

#### VPC-DETAIL-14: Keys are styled in blue
**Given:** The VPC detail view is open.
**When:** The user looks at the field labels.
**Then:** Every key (VpcId, State, CidrBlock, etc.) renders in blue (`#7aa2f7`).

---

#### VPC-DETAIL-15: Values are styled in plain white
**Given:** The VPC detail view is open.
**When:** The user looks at the field values.
**Then:** Non-status values render in plain white (`#c0caf5`).

---

### E. Scrolling

#### VPC-DETAIL-16: Scroll down with j
**Given:** The VPC detail view content is longer than the visible frame.
**When:** The user presses `j`.
**Then:** The content scrolls down one line.

---

#### VPC-DETAIL-17: Scroll up with k
**Given:** The VPC detail view is scrolled down.
**When:** The user presses `k`.
**Then:** The content scrolls up one line.

---

#### VPC-DETAIL-18: Jump to top with g
**Given:** The VPC detail view is scrolled down.
**When:** The user presses `g`.
**Then:** The view jumps to the very first line.

---

#### VPC-DETAIL-19: Jump to bottom with G
**Given:** The VPC detail view is open.
**When:** The user presses `G`.
**Then:** The view jumps to the very last line.

---

### F. Word Wrap Toggle

#### VPC-DETAIL-20: Wrap toggle with w
**Given:** The VPC detail view is open and a field value extends beyond the frame width.
**When:** The user presses `w`.
**Then:** Long values wrap at the frame boundary instead of being clipped.
**When:** The user presses `w` again.
**Then:** Wrap turns off; long values are clipped at the frame boundary.

---

### G. Actions from Detail

#### VPC-DETAIL-21: Switch to YAML view with y
**Given:** The VPC detail view is open.
**When:** The user presses `y`.
**Then:** The view switches to the YAML representation of the same VPC.

---

#### VPC-DETAIL-22: Copy with c
**Given:** The VPC detail view is open.
**When:** The user presses `c`.
**Then:** The full detail content is copied to the clipboard; header briefly shows "Copied!" in green.

---

#### VPC-DETAIL-23: Return to list with Esc
**Given:** The VPC detail view is open.
**When:** The user presses `Esc`.
**Then:** The view returns to the VPC list with the previously selected row still highlighted.

---

#### VPC-DETAIL-24: Help screen with ?
**Given:** The VPC detail view is open.
**When:** The user presses `?`.
**Then:** The help screen appears; pressing any key returns to the VPC detail view.

---

---

## 4. VPC YAML View

### A. Entry and Frame

#### VPC-YAML-01: Open YAML from VPC list
**Given:** The VPC list is displayed and the cursor is on a VPC with ID `vpc-0123456789abcdef0`.
**When:** The user presses `y`.
**Then:** The YAML view opens for the selected VPC.
**And:** The frame title shows `vpc-0123456789abcdef0 yaml`.

---

#### VPC-YAML-02: Open YAML from VPC detail view
**Given:** The VPC detail view is displayed.
**When:** The user presses `y`.
**Then:** The view switches to YAML view showing the same VPC data.
**And:** The frame title updates to include "yaml".

---

### B. Content

#### VPC-YAML-03: Full YAML dump of VPC resource
**Given:** The VPC YAML view is open.
**When:** The user reads the content.
**Then:** The full VPC object from the AWS SDK is rendered as YAML, including all fields not just those in the detail view. Expected top-level keys include:

```
CidrBlock: 10.0.0.0/16
CidrBlockAssociationSet:
  - AssociationId: vpc-cidr-assoc-0abc123
    CidrBlock: 10.0.0.0/16
    CidrBlockState:
      State: associated
DhcpOptionsId: dopt-0abc123def456
InstanceTenancy: default
IsDefault: false
OwnerId: "123456789012"
State: available
Tags:
  - Key: Name
    Value: prod-vpc
  - Key: Environment
    Value: production
VpcId: vpc-0123456789abcdef0
```

**AWS comparison:**
```
aws ec2 describe-vpcs --vpc-ids vpc-0123456789abcdef0 --output yaml
```

---

### C. Syntax Coloring

#### VPC-YAML-04: YAML keys are blue
**Given:** The VPC YAML view is open.
**When:** The user looks at key names.
**Then:** Keys like `VpcId:`, `CidrBlock:`, `State:`, `IsDefault:`, `Tags:` render in blue (`#7aa2f7`).

---

#### VPC-YAML-05: String values are green
**Given:** The VPC YAML view is open.
**When:** The user looks at string values.
**Then:** Values like `vpc-0123456789abcdef0`, `10.0.0.0/16`, `available`, `default` render in green (`#9ece6a`).

---

#### VPC-YAML-06: Boolean values are purple
**Given:** The VPC YAML view is open.
**When:** The user looks at the IsDefault field.
**Then:** `true` or `false` renders in purple (`#bb9af7`).

---

#### VPC-YAML-07: Null values are dim
**Given:** The VPC YAML view is open and a field is null.
**When:** The user looks at that field.
**Then:** `null` renders in dim gray (`#565f89`).

---

### D. Scrolling and Actions

#### VPC-YAML-08: Scroll YAML content
**Given:** The VPC YAML view is open.
**When:** The user presses `j`/`k`/`g`/`G`.
**Then:** The YAML content scrolls accordingly.

---

#### VPC-YAML-09: Copy full YAML
**Given:** The VPC YAML view is open.
**When:** The user presses `c`.
**Then:** The full YAML text is copied to the clipboard.
**And:** The header shows "Copied!" flash message.

---

#### VPC-YAML-10: Navigate back from YAML
**Given:** The VPC YAML view was opened from the list.
**When:** The user presses Esc.
**Then:** The view returns to the VPC list.

**Given:** The VPC YAML view was opened from the detail view.
**When:** The user presses Esc.
**Then:** The view returns to the VPC detail view (view stack pops one level).

---

#### VPC-YAML-11: Wrap toggle in YAML
**Given:** The VPC YAML view is open.
**When:** The user presses `w`.
**Then:** Long values wrap at the frame boundary.
**When:** The user presses `w` again.
**Then:** Wrap turns off.

---

### E. Edge Cases

#### VPC-YAML-12: VPC with IPv6 CIDR blocks
**Given:** A VPC has associated IPv6 CIDR blocks.
**When:** The YAML view is opened.
**Then:** The `Ipv6CidrBlockAssociationSet` array renders with all sub-fields properly indented.

---

#### VPC-YAML-13: VPC with no tags
**Given:** A VPC has no tags.
**When:** The YAML view is opened.
**Then:** The `Tags` field renders as `Tags: []` or `Tags: null`.

---

#### VPC-YAML-14: Default VPC boolean rendering
**Given:** The default VPC's YAML view is open.
**When:** The user looks at IsDefault.
**Then:** `IsDefault: true` -- key is blue, value is purple.

---

---

## 5. Security Groups List View

### A. Column Layout

#### SG-LIST-01: Navigate to Security Groups list
**Given:** The main menu is displayed.
**When:** The user selects "Security Groups" and presses Enter (or types `:sg` Enter).
**Then:** The frame title shows `security-groups(<count>)` with the total number of security groups.
**And:** A loading spinner appears while data loads.
**And:** Once loaded, the table displays these columns left-to-right:

| Column header | Data path | Width | AWS CLI JSON field |
|--------------|-----------|-------|--------------------|
| Group ID | GroupId | ~20 | `.SecurityGroups[].GroupId` |
| Group Name | GroupName | ~20 | `.SecurityGroups[].GroupName` |
| VPC ID | VpcId | ~24 | `.SecurityGroups[].VpcId` |
| Description | Description | ~30 | `.SecurityGroups[].Description` |

**AWS comparison:**
```
aws ec2 describe-security-groups
```
Expected fields visible: GroupId, GroupName, VpcId, Description

---

#### SG-LIST-02: Verify Group ID column data
**Given:** The Security Groups list is displayed with at least one security group.
**When:** The user looks at the Group ID column.
**Then:** Each cell shows the `GroupId` field value (e.g. `sg-0abc123def456789a`) from the DescribeSecurityGroups response.

---

#### SG-LIST-03: Verify Group Name column data
**Given:** The Security Groups list is displayed.
**When:** The user looks at the Group Name column.
**Then:** Each cell shows the `GroupName` field value (e.g. `web-server-sg`, `default`, `allow-ssh`).

---

#### SG-LIST-04: Verify VPC ID column data
**Given:** The Security Groups list is displayed.
**When:** The user looks at the VPC ID column.
**Then:** Each cell shows the `VpcId` field value (e.g. `vpc-0123456789abcdef0`) indicating which VPC the security group belongs to.

---

#### SG-LIST-05: Verify Description column data
**Given:** The Security Groups list is displayed.
**When:** The user looks at the Description column.
**Then:** Each cell shows the `Description` field value. Long descriptions are truncated to fit the column width.

---

#### SG-LIST-06: Column headers are styled bold blue
**Given:** The Security Groups list is displayed.
**When:** The user looks at the column headers.
**Then:** All four column header labels render in bold with blue foreground (`#7aa2f7`).

---

### B. Row Coloring

#### SG-LIST-07: Security groups do not have a running/stopped status concept
**Given:** The Security Groups list is displayed.
**When:** The user looks at the rows.
**Then:** All rows use plain text color (`#c0caf5`) since security groups do not have a lifecycle status like running/stopped/pending.

---

#### SG-LIST-08: Selected row has blue background
**Given:** The Security Groups list is displayed with a row selected.
**When:** The user looks at the selected row.
**Then:** The selected row shows full-width blue background (`#7aa2f7`) with dark foreground (`#1a1b26`), bold text.

---

### C. Frame and Title

#### SG-LIST-09: Frame title shows resource type and count
**Given:** The Security Groups list has loaded and the region has 15 security groups.
**When:** The user looks at the top border.
**Then:** The frame title shows `security-groups(15)` centered between dashes.

---

### D. Navigation

#### SG-LIST-10: Standard navigation keys work
**Given:** The Security Groups list is displayed.
**When:** The user presses `j`/`k`/`g`/`G`/`h`/`l`.
**Then:** Vertical cursor movement (`j`/`k`/`g`/`G`) and horizontal column scrolling (`h`/`l`) work identically to other resource types.

---

#### SG-LIST-11: PageUp and PageDown work
**Given:** The Security Groups list has more items than fit on screen.
**When:** The user presses `PageDown`/`PageUp` (or `ctrl+d`/`ctrl+u`).
**Then:** The view scrolls by one page in the corresponding direction.

---

### E. Sort

#### SG-LIST-12: Sort by name (N)
**Given:** The Security Groups list is displayed.
**When:** The user presses `N`.
**Then:** The list sorts by Group Name ascending with an up-arrow indicator on the column header.

---

#### SG-LIST-13: Sort by name toggles to descending
**Given:** The Security Groups list is sorted by name ascending.
**When:** The user presses `N` again.
**Then:** The sort toggles to descending with a down-arrow indicator.

---

#### SG-LIST-14: Sort by status (S)
**Given:** The Security Groups list is displayed.
**When:** The user presses `S`.
**Then:** The list sorts by the status-equivalent column (possibly Description or Group Name depending on configuration). The sort indicator appears on the sorted column.

---

#### SG-LIST-15: Sort by age (A)
**Given:** The Security Groups list is displayed.
**When:** The user presses `A`.
**Then:** The list sorts by age if a date field is available. If no date column exists, the sort may be a no-op or sort by a sensible default.

---

### F. Filter

#### SG-LIST-16: Filter security groups by name
**Given:** The Security Groups list shows 15 security groups.
**When:** The user presses `/` and types `web`.
**Then:** Only security groups whose rows contain "web" (case-insensitive substring) are displayed.
**And:** The frame title updates to `security-groups(<matched>/<total>)` (e.g. `security-groups(3/15)`).

---

#### SG-LIST-17: Filter security groups by Group ID
**Given:** The Security Groups list is displayed.
**When:** The user presses `/` and types `sg-0abc`.
**Then:** Only security groups whose Group ID contains "sg-0abc" are displayed.

---

#### SG-LIST-18: Filter security groups by VPC ID
**Given:** The Security Groups list is displayed.
**When:** The user presses `/` and types `vpc-012`.
**Then:** Only security groups associated with a VPC whose ID contains "vpc-012" are displayed.

---

#### SG-LIST-19: Filter security groups by description
**Given:** The Security Groups list is displayed.
**When:** The user presses `/` and types `allow ssh`.
**Then:** Only security groups whose description contains "allow ssh" are displayed.

---

#### SG-LIST-20: Esc clears filter
**Given:** The Security Groups list is in filter mode.
**When:** The user presses `Esc`.
**Then:** The filter is cleared, all security groups reappear, header returns to "? for help".

---

### G. Actions from List

#### SG-LIST-21: Enter opens detail view
**Given:** The Security Groups list is displayed and the cursor is on a security group.
**When:** The user presses `Enter` or `d`.
**Then:** The detail view opens for the selected security group.

---

#### SG-LIST-22: y opens YAML view
**Given:** The Security Groups list is displayed and the cursor is on a security group.
**When:** The user presses `y`.
**Then:** The YAML view opens for the selected security group.

---

#### SG-LIST-23: c copies Group ID to clipboard
**Given:** The Security Groups list is displayed and the cursor is on a security group.
**When:** The user presses `c`.
**Then:** The Group ID (or name/ARN) is copied to the clipboard.
**And:** The header briefly shows "Copied!" in green bold.

---

#### SG-LIST-24: Esc returns to main menu
**Given:** The Security Groups list is displayed.
**When:** The user presses `Esc`.
**Then:** The view transitions back to the main menu.

---

#### SG-LIST-25: ? opens help screen
**Given:** The Security Groups list is displayed.
**When:** The user presses `?`.
**Then:** The help screen appears.

---

#### SG-LIST-26: Ctrl+R refreshes security groups
**Given:** The Security Groups list is displayed.
**When:** The user presses `Ctrl+R`.
**Then:** A loading spinner appears; data is re-fetched from AWS; the list repopulates.

---

### H. Loading and Empty States

#### SG-LIST-27: Loading state
**Given:** The user navigates to the Security Groups list.
**When:** The AWS API call is in progress.
**Then:** A spinner and loading message appear centered in the frame.

---

#### SG-LIST-28: Empty state with no security groups
**Given:** The AWS account has no security groups in the current region (extremely unlikely since every VPC has a default SG).
**When:** The Security Groups list finishes loading.
**Then:** The frame shows a centered empty-state message.
**And:** The frame title shows `security-groups(0)`.

---

### I. Edge Cases

#### SG-LIST-29: Default security group appears
**Given:** A VPC has its default security group (named "default").
**When:** The Security Groups list is displayed.
**Then:** The default security group appears in the list with Group Name "default" and the description "default VPC security group" (or similar).

---

#### SG-LIST-30: Security group with very long description
**Given:** A security group has a description like "This security group allows inbound HTTP and HTTPS traffic from the internet and outbound traffic to all destinations".
**When:** The Security Groups list is displayed.
**Then:** The Description column value is truncated to fit the column width. No layout overflow.

---

#### SG-LIST-31: x key does nothing on Security Groups list
**Given:** The Security Groups list is displayed.
**When:** The user presses `x`.
**Then:** Nothing happens. The `x` key is exclusive to Secrets Manager.

---

#### SG-LIST-32: Many security groups scroll correctly
**Given:** The AWS account has 50+ security groups.
**When:** The Security Groups list is displayed.
**Then:** All security groups load; vertical scrolling works; no truncation.

---

---

## 6. Security Groups Detail View

### A. Entry and Frame

#### SG-DETAIL-01: Open Security Group detail view
**Given:** The Security Groups list is displayed and the cursor is on a security group with ID `sg-0abc123def456789a`.
**When:** The user presses Enter or `d`.
**Then:** The detail view opens; the frame title shows the Group ID (e.g. `sg-0abc123def456789a`) centered in the top border.

---

### B. Fields Displayed

#### SG-DETAIL-02: All configured detail fields appear
**Given:** The Security Group detail view is open.
**When:** The user reads the detail content.
**Then:** The following fields are displayed (the exact set depends on views.yaml, but should include at minimum):

| Field | AWS CLI JSON key | Type | Example value |
|-------|-----------------|------|---------------|
| GroupId | `.SecurityGroups[].GroupId` | string | `sg-0abc123def456789a` |
| GroupName | `.SecurityGroups[].GroupName` | string | `web-server-sg` |
| Description | `.SecurityGroups[].Description` | string | `Allow HTTP/HTTPS` |
| VpcId | `.SecurityGroups[].VpcId` | string | `vpc-0123456789abcdef0` |
| OwnerId | `.SecurityGroups[].OwnerId` | string | `123456789012` |
| IpPermissions | `.SecurityGroups[].IpPermissions` | array | Inbound rules (see below) |
| IpPermissionsEgress | `.SecurityGroups[].IpPermissionsEgress` | array | Outbound rules (see below) |
| Tags | `.SecurityGroups[].Tags` | array | Tag key-value pairs |

**AWS comparison:**
```
aws ec2 describe-security-groups --group-ids sg-0abc123def456789a
```

---

#### SG-DETAIL-03: GroupId field is shown
**Given:** The Security Group detail view is open.
**When:** The user looks at the GroupId line.
**Then:** A line shows key `GroupId` and value e.g. `sg-0abc123def456789a`.

---

#### SG-DETAIL-04: GroupName field is shown
**Given:** The Security Group detail view is open.
**When:** The user looks at the GroupName line.
**Then:** A line shows key `GroupName` and value e.g. `web-server-sg`.

---

#### SG-DETAIL-05: VpcId field is shown
**Given:** The Security Group detail view is open.
**When:** The user looks at the VpcId line.
**Then:** A line shows key `VpcId` and value e.g. `vpc-0123456789abcdef0`.

---

#### SG-DETAIL-06: Description field is shown
**Given:** The Security Group detail view is open.
**When:** The user looks at the Description line.
**Then:** The full description text is displayed (not truncated as in the list view).

---

### C. Inbound Rules (IpPermissions)

#### SG-DETAIL-07: Inbound rules are displayed
**Given:** The Security Group detail view is open and the SG has 3 inbound rules.
**When:** The user looks at the IpPermissions section.
**Then:** The `IpPermissions` heading appears, followed by each rule rendered with its sub-fields:
- IpProtocol (e.g. `tcp`, `udp`, `-1` for all)
- FromPort (e.g. `80`, `443`)
- ToPort (e.g. `80`, `443`)
- IpRanges (array of CidrIp values, e.g. `0.0.0.0/0`)
- UserIdGroupPairs (array of GroupId references)

Each rule is indented under the parent.

---

#### SG-DETAIL-08: Inbound rule with CIDR ranges
**Given:** A security group has an inbound rule allowing TCP port 443 from `0.0.0.0/0`.
**When:** The detail view is open.
**Then:** The rule renders showing `IpProtocol: tcp`, `FromPort: 443`, `ToPort: 443`, and `IpRanges` containing `CidrIp: 0.0.0.0/0`.

---

#### SG-DETAIL-09: Inbound rule referencing another security group
**Given:** A security group has an inbound rule allowing traffic from another security group (`sg-0other123`).
**When:** The detail view is open.
**Then:** The rule renders with `UserIdGroupPairs` containing `GroupId: sg-0other123`.

---

#### SG-DETAIL-10: Security group with no inbound rules
**Given:** A security group has zero inbound rules (empty IpPermissions).
**When:** The detail view is open.
**Then:** The IpPermissions section shows empty.

---

### D. Outbound Rules (IpPermissionsEgress)

#### SG-DETAIL-11: Outbound rules are displayed
**Given:** The Security Group detail view is open and the SG has outbound rules.
**When:** The user looks at the IpPermissionsEgress section.
**Then:** The outbound rules render with the same structure as inbound rules (IpProtocol, FromPort, ToPort, IpRanges, etc.).

---

#### SG-DETAIL-12: Default outbound rule allowing all traffic
**Given:** A security group has the default outbound rule (all protocols, all ports, to `0.0.0.0/0`).
**When:** The detail view is open.
**Then:** The rule renders showing `IpProtocol: -1` (meaning all protocols) and `IpRanges` containing `CidrIp: 0.0.0.0/0`.

---

### E. Security Group with Many Rules

#### SG-DETAIL-13: Security group with hundreds of rules
**Given:** A security group has 50+ inbound rules.
**When:** The detail view is open.
**Then:** All rules are rendered. The detail content extends beyond the visible frame; scrolling with `j`/`k`/`g`/`G` reveals all rules.

---

### F. Key-Value Formatting

#### SG-DETAIL-14: Keys are styled in blue
**Given:** The Security Group detail view is open.
**When:** The user looks at field labels.
**Then:** Every key renders in blue (`#7aa2f7`).

---

#### SG-DETAIL-15: Section headers are yellow/orange bold
**Given:** The Security Group detail view is open and has section headings for IpPermissions and IpPermissionsEgress.
**When:** The user looks at those headings.
**Then:** Section headers render in yellow/orange (`#e0af68`) bold.

---

### G. Scrolling

#### SG-DETAIL-16: Scroll long detail content
**Given:** The Security Group detail content is longer than the frame.
**When:** The user presses `j`/`k`/`g`/`G`.
**Then:** The content scrolls accordingly.

---

### H. Actions from Detail

#### SG-DETAIL-17: Switch to YAML view with y
**Given:** The Security Group detail view is open.
**When:** The user presses `y`.
**Then:** The view switches to YAML for the same security group.

---

#### SG-DETAIL-18: Copy with c
**Given:** The Security Group detail view is open.
**When:** The user presses `c`.
**Then:** The detail content is copied to the clipboard; header shows "Copied!" flash.

---

#### SG-DETAIL-19: Return to list with Esc
**Given:** The Security Group detail view is open.
**When:** The user presses `Esc`.
**Then:** The view returns to the Security Groups list with the same row selected.

---

#### SG-DETAIL-20: Word wrap toggle with w
**Given:** The Security Group detail view is open and a description or CIDR list is very long.
**When:** The user presses `w`.
**Then:** Long values wrap at the frame boundary.

---

### I. Edge Cases

#### SG-DETAIL-21: Security group with description containing special characters
**Given:** A security group has a description like "Allow SSH from 10.0.0.0/8 & 172.16.0.0/12".
**When:** The detail view is open.
**Then:** The description renders as-is, including the ampersand character.

---

#### SG-DETAIL-22: Security group with empty description
**Given:** A security group has an empty description.
**When:** The detail view is open.
**Then:** The Description line shows an empty value (dash placeholder or blank).

---

---

## 7. Security Groups YAML View

### A. Entry and Frame

#### SG-YAML-01: Open YAML from Security Groups list
**Given:** The Security Groups list is displayed and the cursor is on a security group with ID `sg-0abc123def456789a`.
**When:** The user presses `y`.
**Then:** The YAML view opens; frame title shows `sg-0abc123def456789a yaml`.

---

#### SG-YAML-02: Open YAML from Security Group detail view
**Given:** The Security Group detail view is open.
**When:** The user presses `y`.
**Then:** The view switches to YAML view for the same security group.

---

### B. Content

#### SG-YAML-03: Full YAML dump of Security Group resource
**Given:** The Security Group YAML view is open.
**When:** The user reads the content.
**Then:** The full Security Group object is rendered as YAML, including all ingress/egress rules. Expected top-level keys include:

```
Description: Allow HTTP/HTTPS inbound
GroupId: sg-0abc123def456789a
GroupName: web-server-sg
IpPermissions:
  - FromPort: 80
    IpProtocol: tcp
    IpRanges:
      - CidrIp: 0.0.0.0/0
        Description: Allow HTTP from anywhere
    Ipv6Ranges: []
    PrefixListIds: []
    ToPort: 80
    UserIdGroupPairs: []
  - FromPort: 443
    IpProtocol: tcp
    IpRanges:
      - CidrIp: 0.0.0.0/0
    ToPort: 443
IpPermissionsEgress:
  - IpProtocol: "-1"
    IpRanges:
      - CidrIp: 0.0.0.0/0
OwnerId: "123456789012"
Tags:
  - Key: Name
    Value: web-server-sg
VpcId: vpc-0123456789abcdef0
```

**AWS comparison:**
```
aws ec2 describe-security-groups --group-ids sg-0abc123def456789a --output yaml
```

---

### C. Syntax Coloring

#### SG-YAML-04: Port numbers are orange
**Given:** The Security Group YAML view is open.
**When:** The user looks at FromPort and ToPort values.
**Then:** Numeric port values like `80`, `443`, `22` render in orange (`#ff9e64`).

---

#### SG-YAML-05: Protocol strings are green
**Given:** The Security Group YAML view is open.
**When:** The user looks at IpProtocol values.
**Then:** String values like `tcp`, `udp` render in green (`#9ece6a`). The special value `"-1"` (all protocols) also renders in green (it is a string).

---

#### SG-YAML-06: CIDR blocks are green strings
**Given:** The Security Group YAML view is open.
**When:** The user looks at CidrIp values.
**Then:** Values like `0.0.0.0/0`, `10.0.0.0/8` render in green (`#9ece6a`).

---

#### SG-YAML-07: Empty arrays render as inline brackets
**Given:** The Security Group YAML view is open and a rule has no UserIdGroupPairs.
**When:** The user looks at that field.
**Then:** It renders as `UserIdGroupPairs: []` on one line.

---

### D. Scrolling and Actions

#### SG-YAML-08: Scroll YAML content
**Given:** The Security Group YAML view is open (a SG with many rules can produce extensive YAML).
**When:** The user presses `j`/`k`/`g`/`G`/`PageUp`/`PageDown`.
**Then:** The YAML content scrolls accordingly.

---

#### SG-YAML-09: Copy full YAML
**Given:** The Security Group YAML view is open.
**When:** The user presses `c`.
**Then:** The full YAML text (including all rules) is copied to the clipboard.

---

#### SG-YAML-10: Navigate back from YAML
**Given:** The Security Group YAML view was opened from the list.
**When:** The user presses Esc.
**Then:** The view returns to the Security Groups list.

**Given:** The Security Group YAML view was opened from the detail view.
**When:** The user presses Esc.
**Then:** The view returns to the Security Group detail view.

---

#### SG-YAML-11: Wrap toggle
**Given:** The Security Group YAML view is open.
**When:** The user presses `w`.
**Then:** Long CIDR descriptions or rule details wrap at the frame boundary.

---

### E. Edge Cases

#### SG-YAML-12: Security group with no inbound rules
**Given:** A security group has zero IpPermissions.
**When:** The YAML view is opened.
**Then:** `IpPermissions: []` renders as an inline empty array.

---

#### SG-YAML-13: Security group with many rules produces long YAML
**Given:** A security group has 50 inbound rules and 10 outbound rules.
**When:** The YAML view is opened.
**Then:** All rules are present in the YAML output. Scrolling is required to view the entire content.

---

#### SG-YAML-14: Security group with IPv6 ranges
**Given:** A security group has rules with Ipv6Ranges.
**When:** The YAML view is opened.
**Then:** `Ipv6Ranges` renders as a YAML array with `CidrIpv6` sub-fields.

---

---

## 8. EKS Node Groups List View

### A. Column Layout

#### NG-LIST-01: Navigate to Node Groups list
**Given:** The main menu is displayed.
**When:** The user selects "EKS Node Groups" and presses Enter (or types `:nodegroups` Enter).
**Then:** The frame title shows `node-groups(<count>)` (or `eks-node-groups(<count>)`) with the total number of node groups across all clusters.
**And:** A loading spinner appears while data loads. The loading message should indicate that EKS clusters are being enumerated and then node groups are being fetched.
**And:** Once loaded, the table displays these columns left-to-right:

| Column header | Data path | Width | AWS CLI JSON field |
|--------------|-----------|-------|--------------------|
| Node Group Name | NodegroupName | ~28 | `.nodegroup.nodegroupName` |
| Cluster Name | ClusterName | ~24 | `.nodegroup.clusterName` |
| Status | Status | ~14 | `.nodegroup.status` |
| Instance Types | InstanceTypes | ~20 | `.nodegroup.instanceTypes` |
| Desired Size | ScalingConfig.DesiredSize | ~12 | `.nodegroup.scalingConfig.desiredSize` |

**AWS comparison:**
```
aws eks list-clusters
aws eks list-nodegroups --cluster-name <each-cluster>
aws eks describe-nodegroup --cluster-name <cluster> --nodegroup-name <ng>
```
Expected fields visible: nodegroupName, clusterName, status, instanceTypes, scalingConfig.desiredSize

---

#### NG-LIST-02: Verify Node Group Name column data
**Given:** The Node Groups list is displayed.
**When:** The user looks at the Node Group Name column.
**Then:** Each cell shows the `nodegroupName` field value (e.g. `prod-workers`, `system-nodegroup`).

---

#### NG-LIST-03: Verify Cluster Name column data
**Given:** The Node Groups list is displayed with node groups from multiple clusters.
**When:** The user looks at the Cluster Name column.
**Then:** Each cell shows the `clusterName` of the EKS cluster the node group belongs to.

---

#### NG-LIST-04: Verify Status column data
**Given:** The Node Groups list is displayed.
**When:** The user looks at the Status column.
**Then:** Each cell shows the node group status (e.g. `ACTIVE`, `CREATING`, `UPDATING`, `DELETING`, `CREATE_FAILED`, `DELETE_FAILED`, `DEGRADED`).

---

#### NG-LIST-05: Verify Instance Types column data
**Given:** The Node Groups list is displayed.
**When:** The user looks at the Instance Types column.
**Then:** Each cell shows the instance types configured for the node group (e.g. `t3.medium`, `m5.large`). If multiple instance types are configured, they appear comma-separated or as the first type with an indication of more.

---

#### NG-LIST-06: Verify Desired Size column data
**Given:** The Node Groups list is displayed.
**When:** The user looks at the Desired Size column.
**Then:** Each cell shows the `desiredSize` from the scaling configuration (e.g. `3`, `5`, `10`).

---

#### NG-LIST-07: Column headers are styled bold blue
**Given:** The Node Groups list is displayed.
**When:** The user looks at the column headers.
**Then:** All five column header labels render in bold blue (`#7aa2f7`).

---

### B. Status Coloring

#### NG-LIST-08: ACTIVE node groups have green rows
**Given:** The Node Groups list is displayed with node groups in ACTIVE status.
**When:** The user looks at those rows.
**Then:** The entire row is rendered in green (`#9ece6a`).

---

#### NG-LIST-09: CREATING node groups have yellow rows
**Given:** The Node Groups list includes a node group with CREATING status.
**When:** The user looks at that row.
**Then:** The entire row is rendered in yellow (`#e0af68`).

---

#### NG-LIST-10: DELETING node groups have red rows
**Given:** The Node Groups list includes a node group with DELETING status.
**When:** The user looks at that row.
**Then:** The entire row is rendered in red (`#f7768e`).

---

#### NG-LIST-11: CREATE_FAILED and DELETE_FAILED node groups have red rows
**Given:** The Node Groups list includes a node group with CREATE_FAILED or DELETE_FAILED status.
**When:** The user looks at that row.
**Then:** The entire row is rendered in red (`#f7768e`).

---

#### NG-LIST-12: DEGRADED node groups have yellow rows
**Given:** The Node Groups list includes a node group with DEGRADED status.
**When:** The user looks at that row.
**Then:** The entire row is rendered in yellow (`#e0af68`) (transitional/warning state).

---

#### NG-LIST-13: UPDATING node groups have yellow rows
**Given:** The Node Groups list includes a node group with UPDATING status.
**When:** The user looks at that row.
**Then:** The entire row is rendered in yellow (`#e0af68`).

---

#### NG-LIST-14: Selected row overrides status color
**Given:** The Node Groups list is displayed and the cursor is on an ACTIVE (green) row.
**When:** The user looks at the selected row.
**Then:** The selected row has a full-width blue background, overriding the green.

---

### C. Aggregation Across Clusters

#### NG-LIST-15: Node groups from all clusters aggregated
**Given:** There are 3 EKS clusters: cluster-a with 2 node groups, cluster-b with 1 node group, and cluster-c with 3 node groups.
**When:** The Node Groups list finishes loading.
**Then:** All 6 node groups appear in a single flat list. The Cluster Name column identifies which cluster each node group belongs to.
**And:** The frame title shows `node-groups(6)`.

---

#### NG-LIST-16: Node groups from a single cluster
**Given:** There is only 1 EKS cluster with 2 node groups.
**When:** The Node Groups list finishes loading.
**Then:** Both node groups appear in the list with the same Cluster Name.

---

### D. Frame and Title

#### NG-LIST-17: Frame title shows total count across all clusters
**Given:** The Node Groups list has loaded with 6 node groups across 3 clusters.
**When:** The user looks at the top border.
**Then:** The frame title shows `node-groups(6)`.

---

### E. Navigation

#### NG-LIST-18: Standard navigation keys work
**Given:** The Node Groups list is displayed.
**When:** The user presses `j`/`k`/`g`/`G`/`h`/`l`.
**Then:** Vertical and horizontal navigation works identically to other resource types.

---

### F. Sort

#### NG-LIST-19: Sort by name (N)
**Given:** The Node Groups list is displayed.
**When:** The user presses `N`.
**Then:** The list sorts by Node Group Name ascending with an up-arrow indicator.

---

#### NG-LIST-20: Sort by status (S)
**Given:** The Node Groups list is displayed.
**When:** The user presses `S`.
**Then:** The list sorts by the Status column.

---

#### NG-LIST-21: Sort by age (A)
**Given:** The Node Groups list is displayed.
**When:** The user presses `A`.
**Then:** The list sorts by creation time (age).

---

### G. Filter

#### NG-LIST-22: Filter node groups by name
**Given:** The Node Groups list shows 6 node groups.
**When:** The user presses `/` and types `prod`.
**Then:** Only node groups whose rows contain "prod" (case-insensitive) are displayed.
**And:** The frame title updates to `node-groups(<matched>/<total>)`.

---

#### NG-LIST-23: Filter node groups by cluster name
**Given:** The Node Groups list shows node groups from multiple clusters.
**When:** The user presses `/` and types `cluster-a`.
**Then:** Only node groups belonging to cluster-a are shown (because "cluster-a" appears in the Cluster Name column).

---

#### NG-LIST-24: Filter node groups by instance type
**Given:** The Node Groups list is displayed.
**When:** The user presses `/` and types `t3`.
**Then:** Only node groups with `t3` in the Instance Types column are displayed.

---

#### NG-LIST-25: Esc clears filter
**Given:** The Node Groups list is in filter mode.
**When:** The user presses `Esc`.
**Then:** The filter is cleared and all node groups reappear.

---

### H. Actions from List

#### NG-LIST-26: Enter opens detail view
**Given:** The Node Groups list is displayed and the cursor is on a node group.
**When:** The user presses `Enter` or `d`.
**Then:** The detail view opens for the selected node group.

---

#### NG-LIST-27: y opens YAML view
**Given:** The Node Groups list is displayed and the cursor is on a node group.
**When:** The user presses `y`.
**Then:** The YAML view opens for the selected node group.

---

#### NG-LIST-28: c copies Node Group identifier
**Given:** The Node Groups list is displayed and the cursor is on a node group.
**When:** The user presses `c`.
**Then:** The node group name (or ARN) is copied to the clipboard.
**And:** The header briefly shows "Copied!" in green bold.

---

#### NG-LIST-29: Esc returns to main menu
**Given:** The Node Groups list is displayed.
**When:** The user presses `Esc`.
**Then:** The view transitions back to the main menu.

---

#### NG-LIST-30: ? opens help screen
**Given:** The Node Groups list is displayed.
**When:** The user presses `?`.
**Then:** The help screen appears.

---

#### NG-LIST-31: Ctrl+R refreshes node groups
**Given:** The Node Groups list is displayed.
**When:** The user presses `Ctrl+R`.
**Then:** Data is re-fetched from AWS (re-enumerating clusters and their node groups); the list updates.

---

### I. Loading and Empty States

#### NG-LIST-32: Loading state
**Given:** The user navigates to the Node Groups list.
**When:** The AWS API calls are in progress (listing clusters, then listing and describing node groups).
**Then:** A spinner and loading message appear centered in the frame.

---

#### NG-LIST-33: Empty state - no EKS clusters exist
**Given:** The AWS account has no EKS clusters in the current region.
**When:** The Node Groups list finishes loading.
**Then:** The frame shows a centered empty-state message (e.g. "No Node Groups found").
**And:** The frame title shows `node-groups(0)`.
**And:** No error is displayed -- an empty list is the expected response when there are no clusters.

---

#### NG-LIST-34: Empty state - clusters exist but no managed node groups
**Given:** The AWS account has 2 EKS clusters but neither has any managed node groups (they use self-managed nodes or Fargate).
**When:** The Node Groups list finishes loading.
**Then:** The frame shows a centered empty-state message.
**And:** The frame title shows `node-groups(0)`.

---

### J. Edge Cases

#### NG-LIST-35: Node group with multiple instance types
**Given:** A node group is configured with instance types `["t3.medium", "t3.large", "t3.xlarge"]`.
**When:** The Node Groups list is displayed.
**Then:** The Instance Types column shows all types or a representative value (e.g. `t3.medium, t3.large, ...` or `t3.medium` truncated to column width).

---

#### NG-LIST-36: x key does nothing on Node Groups list
**Given:** The Node Groups list is displayed.
**When:** The user presses `x`.
**Then:** Nothing happens. The `x` key is exclusive to Secrets Manager.

---

#### NG-LIST-37: Node group names are unique per cluster but may repeat across clusters
**Given:** Two different clusters each have a node group named "workers".
**When:** The Node Groups list is displayed.
**Then:** Both "workers" entries appear in the list, distinguishable by their Cluster Name column values.

---

---

## 9. EKS Node Groups Detail View

### A. Entry and Frame

#### NG-DETAIL-01: Open Node Group detail view
**Given:** The Node Groups list is displayed and the cursor is on a node group named "prod-workers" in cluster "my-cluster".
**When:** The user presses Enter or `d`.
**Then:** The detail view opens; the frame title shows the node group name (e.g. `prod-workers`) centered in the top border.

---

### B. Fields Displayed

#### NG-DETAIL-02: All configured detail fields appear
**Given:** The Node Group detail view is open.
**When:** The user reads the detail content.
**Then:** The following fields are displayed (the exact set depends on views.yaml, but should include at minimum):

| Field | AWS CLI JSON key | Type | Example value |
|-------|-----------------|------|---------------|
| NodegroupName | `.nodegroup.nodegroupName` | string | `prod-workers` |
| ClusterName | `.nodegroup.clusterName` | string | `my-cluster` |
| Status | `.nodegroup.status` | string | `ACTIVE` |
| NodegroupArn | `.nodegroup.nodegroupArn` | string | `arn:aws:eks:us-east-1:123456789012:nodegroup/my-cluster/prod-workers/...` |
| InstanceTypes | `.nodegroup.instanceTypes` | array | `[t3.medium, t3.large]` |
| AmiType | `.nodegroup.amiType` | string | `AL2_x86_64` |
| DiskSize | `.nodegroup.diskSize` | int | `20` |
| ScalingConfig | `.nodegroup.scalingConfig` | nested | see below |
| Subnets | `.nodegroup.subnets` | array | `[subnet-0abc, subnet-0def]` |
| Labels | `.nodegroup.labels` | map | `{role: worker}` |
| Taints | `.nodegroup.taints` | array | see below |
| NodeRole | `.nodegroup.nodeRole` | string | `arn:aws:iam::123456789012:role/node-role` |
| Tags | `.nodegroup.tags` | map | `{Environment: production}` |
| CreatedAt | `.nodegroup.createdAt` | time | `2024-01-15 09:22:31` |

**AWS comparison:**
```
aws eks describe-nodegroup --cluster-name my-cluster --nodegroup-name prod-workers
```

---

#### NG-DETAIL-03: NodegroupName field is shown
**Given:** The Node Group detail view is open.
**When:** The user looks at the NodegroupName line.
**Then:** A line shows key `NodegroupName` and value e.g. `prod-workers`.

---

#### NG-DETAIL-04: ClusterName field is shown
**Given:** The Node Group detail view is open.
**When:** The user looks at the ClusterName line.
**Then:** A line shows key `ClusterName` and value e.g. `my-cluster`.

---

#### NG-DETAIL-05: Status field is shown with color
**Given:** The Node Group detail view is open for an ACTIVE node group.
**When:** The user looks at the Status line.
**Then:** The value `ACTIVE` is rendered in green (`#9ece6a`).

---

#### NG-DETAIL-06: InstanceTypes field is shown
**Given:** The Node Group detail view is open.
**When:** The user looks at the InstanceTypes line.
**Then:** The array of instance types is displayed (e.g. `t3.medium, t3.large` or as a YAML array).

---

#### NG-DETAIL-07: DiskSize field is shown
**Given:** The Node Group detail view is open.
**When:** The user looks at the DiskSize line.
**Then:** A line shows key `DiskSize` and value e.g. `20` (integer, in GB).

---

#### NG-DETAIL-08: AmiType field is shown
**Given:** The Node Group detail view is open.
**When:** The user looks at the AmiType line.
**Then:** A line shows key `AmiType` and value e.g. `AL2_x86_64`, `AL2_ARM_64`, `BOTTLEROCKET_x86_64`, or `CUSTOM`.

---

### C. Nested Field Rendering

#### NG-DETAIL-09: ScalingConfig nested fields
**Given:** The Node Group detail view is open.
**When:** The user looks at the ScalingConfig section.
**Then:** ScalingConfig renders as a multi-line block:
```
  ScalingConfig:
    DesiredSize: 3
    MaxSize: 5
    MinSize: 1
```

---

#### NG-DETAIL-10: Subnets array
**Given:** The Node Group has 3 subnets.
**When:** The detail view is open.
**Then:** The Subnets section renders as a YAML array:
```
  Subnets:
    - subnet-0abc123
    - subnet-0def456
    - subnet-0ghi789
```

---

#### NG-DETAIL-11: Labels map
**Given:** The Node Group has labels `{role: worker, tier: frontend}`.
**When:** The detail view is open.
**Then:** Labels render as key-value pairs:
```
  Labels:
    role: worker
    tier: frontend
```

---

#### NG-DETAIL-12: Taints array
**Given:** The Node Group has taints configured.
**When:** The detail view is open.
**Then:** Taints render as a YAML array with sub-fields:
```
  Taints:
    - Key: dedicated
      Value: gpu
      Effect: NO_SCHEDULE
```

---

#### NG-DETAIL-13: No labels (empty map)
**Given:** The Node Group has no labels.
**When:** The detail view is open.
**Then:** The Labels section shows empty.

---

#### NG-DETAIL-14: No taints (empty array)
**Given:** The Node Group has no taints.
**When:** The detail view is open.
**Then:** The Taints section shows empty.

---

### D. Scrolling and Actions

#### NG-DETAIL-15: Scroll long detail content
**Given:** The Node Group detail content is longer than the frame.
**When:** The user presses `j`/`k`/`g`/`G`.
**Then:** The content scrolls accordingly.

---

#### NG-DETAIL-16: Switch to YAML view with y
**Given:** The Node Group detail view is open.
**When:** The user presses `y`.
**Then:** The view switches to YAML for the same node group.

---

#### NG-DETAIL-17: Copy with c
**Given:** The Node Group detail view is open.
**When:** The user presses `c`.
**Then:** The detail content is copied to the clipboard; header shows "Copied!" flash.

---

#### NG-DETAIL-18: Return to list with Esc
**Given:** The Node Group detail view is open.
**When:** The user presses `Esc`.
**Then:** The view returns to the Node Groups list with the same row selected.

---

#### NG-DETAIL-19: Word wrap toggle with w
**Given:** The Node Group detail view is open and the NodegroupArn is very long.
**When:** The user presses `w`.
**Then:** The long ARN wraps at the frame boundary.

---

### E. Edge Cases

#### NG-DETAIL-20: Node group with CUSTOM AMI type
**Given:** A node group uses a custom AMI (AmiType = CUSTOM).
**When:** The detail view is open.
**Then:** AmiType shows `CUSTOM`. The launch template details may also be visible if configured.

---

#### NG-DETAIL-21: Node group with zero desired size
**Given:** A node group has ScalingConfig.DesiredSize = 0.
**When:** The detail view is open.
**Then:** DesiredSize shows as empty string or `0` depending on zero-value handling.

---

---

## 10. EKS Node Groups YAML View

### A. Entry and Frame

#### NG-YAML-01: Open YAML from Node Groups list
**Given:** The Node Groups list is displayed and the cursor is on "prod-workers".
**When:** The user presses `y`.
**Then:** The YAML view opens; frame title shows `prod-workers yaml`.

---

#### NG-YAML-02: Open YAML from Node Group detail view
**Given:** The Node Group detail view is open.
**When:** The user presses `y`.
**Then:** The view switches to YAML for the same node group.

---

### B. Content

#### NG-YAML-03: Full YAML dump of Node Group resource
**Given:** The Node Group YAML view is open.
**When:** The user reads the content.
**Then:** The full Node Group object from the DescribeNodegroup response is rendered as YAML. Expected top-level keys include:

```
AmiType: AL2_x86_64
CapacityType: ON_DEMAND
ClusterName: my-cluster
CreatedAt: 2024-01-15T09:22:31Z
DiskSize: 20
Health:
  Issues: []
InstanceTypes:
  - t3.medium
Labels:
  role: worker
LaunchTemplate:
  Id: lt-0abc123
  Version: "1"
ModifiedAt: 2024-03-10T14:00:00Z
NodeRole: arn:aws:iam::123456789012:role/node-role
NodegroupArn: arn:aws:eks:us-east-1:123456789012:nodegroup/my-cluster/prod-workers/abc123
NodegroupName: prod-workers
ReleaseVersion: 1.28.5-20240227
Resources:
  AutoScalingGroups:
    - Name: eks-prod-workers-abc123
  RemoteAccessSecurityGroup: sg-0abc123
ScalingConfig:
  DesiredSize: 3
  MaxSize: 5
  MinSize: 1
Status: ACTIVE
Subnets:
  - subnet-0abc123
  - subnet-0def456
Tags:
  Environment: production
Taints: []
UpdateConfig:
  MaxUnavailable: 1
Version: "1.28"
```

**AWS comparison:**
```
aws eks describe-nodegroup --cluster-name my-cluster --nodegroup-name prod-workers --output yaml
```

---

### C. Syntax Coloring

#### NG-YAML-04: Numeric values are orange
**Given:** The Node Group YAML view is open.
**When:** The user looks at DiskSize and ScalingConfig values.
**Then:** `DiskSize: 20` -- value `20` is orange. `DesiredSize: 3`, `MaxSize: 5`, `MinSize: 1` -- all orange.

---

#### NG-YAML-05: String values are green
**Given:** The Node Group YAML view is open.
**When:** The user looks at string fields.
**Then:** Values like `AL2_x86_64`, `ON_DEMAND`, `ACTIVE`, `my-cluster`, `prod-workers` all render in green.

---

#### NG-YAML-06: Empty arrays render as inline brackets
**Given:** The Node Group YAML view is open and the node group has no taints.
**When:** The user looks at the Taints field.
**Then:** `Taints: []` renders on one line.

---

#### NG-YAML-07: Nested ScalingConfig is properly indented
**Given:** The Node Group YAML view is open.
**When:** The user looks at ScalingConfig.
**Then:** Sub-fields DesiredSize, MaxSize, MinSize are indented 2 spaces under ScalingConfig.

---

### D. Scrolling and Actions

#### NG-YAML-08: Scroll YAML content
**Given:** The Node Group YAML view is open.
**When:** The user presses `j`/`k`/`g`/`G`.
**Then:** The YAML content scrolls accordingly.

---

#### NG-YAML-09: Copy full YAML
**Given:** The Node Group YAML view is open.
**When:** The user presses `c`.
**Then:** The full YAML text is copied to the clipboard.

---

#### NG-YAML-10: Navigate back from YAML
**Given:** The Node Group YAML view was opened from the list.
**When:** The user presses Esc.
**Then:** The view returns to the Node Groups list.

**Given:** The Node Group YAML view was opened from the detail view.
**When:** The user presses Esc.
**Then:** The view returns to the Node Group detail view.

---

#### NG-YAML-11: Wrap toggle
**Given:** The Node Group YAML view is open.
**When:** The user presses `w`.
**Then:** Long values (NodegroupArn, NodeRole) wrap at the frame boundary.

---

### E. Edge Cases

#### NG-YAML-12: Node group with launch template
**Given:** A node group uses a custom launch template.
**When:** The YAML view is opened.
**Then:** The `LaunchTemplate` section renders with `Id`, `Name`, and `Version` sub-fields.

---

#### NG-YAML-13: Node group health issues
**Given:** A node group has health issues (e.g. DEGRADED status with issues).
**When:** The YAML view is opened.
**Then:** The `Health.Issues` array renders with sub-fields like `Code`, `Message`, `ResourceIds`.

---

#### NG-YAML-14: Node group with taints
**Given:** A node group has taints `[{Key: dedicated, Value: gpu, Effect: NO_SCHEDULE}]`.
**When:** The YAML view is opened.
**Then:** Taints render as:
```
Taints:
  - Effect: NO_SCHEDULE
    Key: dedicated
    Value: gpu
```

---

---

## 11. Command Mode Aliases

### A. VPC Aliases

#### ALIAS-VPC-01: Navigate to VPC with :vpc
**Given:** Any view is displayed and the user enters command mode.
**When:** The user types `vpc` and presses Enter.
**Then:** The application navigates to the VPC resource list view.

---

#### ALIAS-VPC-02: Navigate to VPC with :vpcs
**Given:** Any view is displayed and the user enters command mode.
**When:** The user types `vpcs` and presses Enter.
**Then:** The application navigates to the VPC resource list view (same as `:vpc`).

---

### B. Security Groups Aliases

#### ALIAS-SG-01: Navigate to Security Groups with :sg
**Given:** Any view is displayed and the user enters command mode.
**When:** The user types `sg` and presses Enter.
**Then:** The application navigates to the Security Groups resource list view.

---

#### ALIAS-SG-02: Navigate to Security Groups with :securitygroups
**Given:** Any view is displayed and the user enters command mode.
**When:** The user types `securitygroups` and presses Enter.
**Then:** The application navigates to the Security Groups resource list view (same as `:sg`).

---

#### ALIAS-SG-03: Navigate to Security Groups with :security-groups
**Given:** Any view is displayed and the user enters command mode.
**When:** The user types `security-groups` and presses Enter.
**Then:** The application navigates to the Security Groups resource list view (same as `:sg`).

---

### C. Node Groups Aliases

#### ALIAS-NG-01: Navigate to Node Groups with :nodegroups
**Given:** Any view is displayed and the user enters command mode.
**When:** The user types `nodegroups` and presses Enter.
**Then:** The application navigates to the EKS Node Groups resource list view.

---

#### ALIAS-NG-02: Navigate to Node Groups with :ng
**Given:** Any view is displayed and the user enters command mode.
**When:** The user types `ng` and presses Enter.
**Then:** The application navigates to the EKS Node Groups resource list view (same as `:nodegroups`).

---

#### ALIAS-NG-03: Navigate to Node Groups with :node-groups
**Given:** Any view is displayed and the user enters command mode.
**When:** The user types `node-groups` and presses Enter.
**Then:** The application navigates to the EKS Node Groups resource list view (same as `:nodegroups`).

---

### D. Tab Completion

#### ALIAS-TAB-01: Tab completion for VPC
**Given:** Command mode is active and the user has typed `vp`.
**When:** The user presses `Tab`.
**Then:** The command text is completed to `vpc`.

---

#### ALIAS-TAB-02: Tab completion for Security Groups
**Given:** Command mode is active and the user has typed `sg`.
**When:** The user presses `Tab`.
**Then:** The command text remains `sg` (already complete) or the user can press Enter to navigate.

---

#### ALIAS-TAB-03: Tab completion for Node Groups
**Given:** Command mode is active and the user has typed `ng`.
**When:** The user presses `Tab`.
**Then:** The command text remains `ng` (already a valid alias) or completes to `nodegroups`.

---

### E. Existing Commands Still Work

#### ALIAS-COMPAT-01: Existing shortcuts unaffected
**Given:** Command mode is active.
**When:** The user types `ec2`, `s3`, `rds`, `redis`, `docdb`, `eks`, or `secrets` and presses Enter.
**Then:** Each navigates to its respective resource list view, unchanged from before the new resource types were added.

---

#### ALIAS-COMPAT-02: :main still returns to menu
**Given:** Any view is displayed and command mode is active.
**When:** The user types `main` and presses Enter.
**Then:** The application navigates to the main menu.

---

#### ALIAS-COMPAT-03: :q still quits
**Given:** Command mode is active.
**When:** The user types `q` and presses Enter.
**Then:** The application exits cleanly.

---

---

## 12. Edge Cases and Error Handling

### A. Permission Errors

#### EDGE-01: VPC list with insufficient IAM permissions
**Given:** The AWS credentials lack `ec2:DescribeVpcs` permission.
**When:** The user navigates to the VPC list.
**Then:** An error message appears in the header right side in red (`#f7768e`).
**And:** The application does not crash.
**And:** The error is consistent with how other resource types handle permission errors.

---

#### EDGE-02: Security Groups list with insufficient IAM permissions
**Given:** The AWS credentials lack `ec2:DescribeSecurityGroups` permission.
**When:** The user navigates to the Security Groups list.
**Then:** An error message appears in the header.
**And:** The application does not crash.

---

#### EDGE-03: Node Groups list with insufficient IAM permissions for ListClusters
**Given:** The AWS credentials lack `eks:ListClusters` permission.
**When:** The user navigates to the Node Groups list.
**Then:** An error message appears in the header.
**And:** The application does not crash.

---

#### EDGE-04: Node Groups list with permission for ListClusters but not DescribeNodegroup
**Given:** The AWS credentials allow `eks:ListClusters` and `eks:ListNodegroups` but lack `eks:DescribeNodegroup`.
**When:** The user navigates to the Node Groups list.
**Then:** An appropriate error message appears.
**And:** The application does not crash.

---

### B. Empty States

#### EDGE-05: Region with no VPCs (rare but possible in new regions)
**Given:** The current region has no VPCs.
**When:** The VPC list finishes loading.
**Then:** An empty-state message is shown. Frame title shows `vpcs(0)`.

---

#### EDGE-06: Region with no Security Groups (extremely unlikely)
**Given:** The current region has no security groups.
**When:** The Security Groups list finishes loading.
**Then:** An empty-state message is shown. Frame title shows `security-groups(0)`.

---

#### EDGE-07: Region with no EKS clusters yields empty Node Groups
**Given:** The current region has no EKS clusters.
**When:** The Node Groups list finishes loading.
**Then:** An empty-state message is shown (NOT an error). Frame title shows `node-groups(0)`.

---

#### EDGE-08: EKS clusters exist but have no managed node groups
**Given:** The region has 2 EKS clusters, both using Fargate (no managed node groups).
**When:** The Node Groups list finishes loading.
**Then:** An empty-state message is shown. Frame title shows `node-groups(0)`.

---

### C. Network and Credential Errors

#### EDGE-09: AWS credentials missing or expired
**Given:** AWS credentials are missing or expired.
**When:** The user navigates to any of the three new resource types.
**Then:** The header right side shows an error flash in red.
**And:** The error is persistent until the user navigates away or refreshes.

---

#### EDGE-10: Network timeout during VPC fetch
**Given:** The AWS API is unreachable (network issue).
**When:** The user navigates to the VPC list.
**Then:** An error message appears after a timeout period.
**And:** The application does not hang indefinitely.

---

### D. Special Data Cases

#### EDGE-11: VPC with secondary CIDR blocks
**Given:** A VPC has both a primary CIDR `10.0.0.0/16` and a secondary CIDR `10.1.0.0/16`.
**When:** The VPC list is displayed.
**Then:** The CIDR Block column shows the primary CIDR. The secondary CIDRs are visible in the detail and YAML views.

---

#### EDGE-12: Security group with description containing quotes
**Given:** A security group has a description: `Allow "special" traffic`.
**When:** The Security Groups list and detail view are displayed.
**Then:** The description renders correctly including the quote characters.

---

#### EDGE-13: Node group name with hyphens and numbers
**Given:** A node group is named `prod-workers-v2-arm64`.
**When:** The Node Groups list is displayed.
**Then:** The full name appears in the Node Group Name column.

---

#### EDGE-14: Node group with very large desired size
**Given:** A node group has DesiredSize = 100.
**When:** The Node Groups list is displayed.
**Then:** The Desired Size column shows `100`.

---

### E. Profile and Region Switch

#### EDGE-15: Switch region re-fetches new resource types
**Given:** The user is viewing the VPC list in us-east-1.
**When:** The user opens the region selector (`:region`), selects eu-west-1, and presses Enter.
**Then:** The application reconnects to the new region and the VPC list is re-fetched showing VPCs in eu-west-1.

---

#### EDGE-16: Switch profile re-fetches new resource types
**Given:** The user is viewing the Security Groups list with profile "dev".
**When:** The user switches to profile "prod" via `:ctx`.
**Then:** The application reconnects with the new profile and the Security Groups list shows security groups from the "prod" account.

---

### F. Terminal Size

#### EDGE-17: Terminal too narrow for new resource lists
**Given:** The terminal is fewer than 60 columns wide.
**When:** The user navigates to any of the three new resource types.
**Then:** The error message "Terminal too narrow. Please resize." is shown.

---

#### EDGE-18: Terminal resize while viewing new resource list
**Given:** The user is viewing the Security Groups list.
**When:** The terminal is resized.
**Then:** The UI reflows dynamically, columns adjust to the new dimensions.

---

#### EDGE-19: Narrow terminal shows reduced columns
**Given:** The terminal is between 60-79 columns wide.
**When:** The user views the VPC list.
**Then:** Only the most important columns are shown (e.g. VPC ID and State). The remaining columns can be accessed via horizontal scroll (`h`/`l`).

---

---

## 13. Cross-View Navigation Flows

### VPC Navigation

#### NAV-VPC-01: Main Menu -> VPC List -> Detail -> YAML -> Esc -> Detail -> Esc -> List -> Esc -> Main Menu
**Given:** The user is on the main menu.
**When:** The user selects VPC, presses Enter, selects a VPC, presses Enter (detail), presses `y` (YAML), then presses Esc three times.
**Then:** Each Esc pops one level: YAML -> Detail -> List -> Main Menu.

---

#### NAV-VPC-02: VPC List -> YAML (via y) -> Esc -> VPC List
**Given:** The user is on the VPC list.
**When:** The user presses `y`, then Esc.
**Then:** Returns to the VPC list (not detail).

---

#### NAV-VPC-03: VPC Detail -> YAML -> Esc -> Detail
**Given:** The user is on the VPC detail view.
**When:** The user presses `y`, then Esc.
**Then:** Returns to the VPC detail (not list).

---

### Security Groups Navigation

#### NAV-SG-01: Full navigation cycle for Security Groups
**Given:** The user is on the main menu.
**When:** The user navigates: Main Menu -> SG List -> SG Detail -> SG YAML -> Esc -> Detail -> Esc -> List -> Esc -> Main Menu.
**Then:** Each Esc pops one level correctly.

---

#### NAV-SG-02: Command mode navigation from another view
**Given:** The user is on the EC2 list view.
**When:** The user presses `:`, types `sg`, and presses Enter.
**Then:** The view transitions directly to the Security Groups list.

---

### Node Groups Navigation

#### NAV-NG-01: Full navigation cycle for Node Groups
**Given:** The user is on the main menu.
**When:** The user navigates: Main Menu -> NG List -> NG Detail -> NG YAML -> Esc -> Detail -> Esc -> List -> Esc -> Main Menu.
**Then:** Each Esc pops one level correctly.

---

#### NAV-NG-02: Command mode navigation from another view
**Given:** The user is on the RDS list view.
**When:** The user presses `:`, types `ng`, and presses Enter.
**Then:** The view transitions directly to the Node Groups list.

---

### Cross-Resource Navigation

#### NAV-CROSS-01: Navigate between new resource types via command mode
**Given:** The user is on the VPC list.
**When:** The user presses `:`, types `sg`, and presses Enter.
**Then:** The view transitions to the Security Groups list.
**When:** The user presses `:`, types `nodegroups`, and presses Enter.
**Then:** The view transitions to the Node Groups list.

---

#### NAV-CROSS-02: Navigate from new resource type to existing resource type
**Given:** The user is on the Security Groups list.
**When:** The user presses `:`, types `ec2`, and presses Enter.
**Then:** The view transitions to the EC2 Instances list.

---

#### NAV-CROSS-03: Filter then Enter opens correct resource
**Given:** The VPC list has active filter showing 2 of 5 VPCs, and the cursor is on the second filtered VPC.
**When:** The user presses Enter.
**Then:** The detail view opens for the correct VPC (the second filtered result), not the second VPC in the unfiltered list.

---

#### NAV-CROSS-04: Select resource A, view detail, return, select resource B
**Given:** The user is on the Security Groups list.
**When:** The user selects SG-A, opens detail, presses Esc, navigates to SG-B, opens detail.
**Then:** The detail view shows SG-B's data, not SG-A's.

---

---

## 14. No-Regression Verification

These stories verify that existing resource types continue to function identically after the three new resource types are added.

### REGR-01: EC2 Instances still accessible
**Given:** The main menu is displayed after the feature update.
**When:** The user selects EC2 Instances and presses Enter.
**Then:** The EC2 instance list loads and displays correctly, with all six original columns.

---

### REGR-02: S3 Buckets still accessible
**Given:** The main menu is displayed.
**When:** The user types `:s3` and presses Enter.
**Then:** The S3 bucket list loads and functions normally.

---

### REGR-03: RDS Instances still accessible
**Given:** The main menu is displayed.
**When:** The user navigates to RDS Instances.
**Then:** The RDS list loads with all seven columns functioning correctly.

---

### REGR-04: ElastiCache Redis still accessible
**Given:** The main menu is displayed.
**When:** The user navigates to ElastiCache Redis.
**Then:** The Redis cluster list loads and functions normally.

---

### REGR-05: DocumentDB Clusters still accessible
**Given:** The main menu is displayed.
**When:** The user navigates to DocumentDB Clusters.
**Then:** The DocumentDB list loads and functions normally.

---

### REGR-06: EKS Clusters still accessible
**Given:** The main menu is displayed.
**When:** The user navigates to EKS Clusters.
**Then:** The EKS cluster list loads and functions normally, independent of the Node Groups feature.

---

### REGR-07: Secrets Manager still accessible (including reveal)
**Given:** The main menu is displayed.
**When:** The user navigates to Secrets Manager.
**Then:** The Secrets list loads normally. The `x` key still reveals secrets. All other interactions work.

---

### REGR-08: Existing command aliases unchanged
**Given:** Command mode is active.
**When:** The user types any of the original commands: `ec2`, `s3`, `rds`, `redis`, `docdb`, `eks`, `secrets`, `main`, `root`, `ctx`, `region`, `q`, `quit`.
**Then:** Each command behaves exactly as before the feature addition.

---

### REGR-09: Filter on main menu with 10 resource types
**Given:** The main menu is displayed with 10 resource types.
**When:** The user presses `/` and types `e`.
**Then:** Resource types matching "e" are shown (EC2, ElastiCache Redis, EKS Clusters, Secrets Manager, Security Groups, EKS Node Groups, and possibly others). The filter/total count in the frame title is correct relative to 10 total.

---

### REGR-10: Help screen includes new resource shortnames
**Given:** The user presses `?` from any view.
**When:** The help screen is displayed.
**Then:** The help content includes the new command aliases (`:vpc`, `:sg`, `:nodegroups`) if applicable, or at minimum the existing key bindings remain unchanged.

---

---

## Story Count Summary

| Section | Stories |
|---------|---------|
| Main Menu Changes | 13 |
| VPC List View | 42 |
| VPC Detail View | 24 |
| VPC YAML View | 14 |
| Security Groups List View | 32 |
| Security Groups Detail View | 22 |
| Security Groups YAML View | 14 |
| EKS Node Groups List View | 37 |
| EKS Node Groups Detail View | 21 |
| EKS Node Groups YAML View | 14 |
| Command Mode Aliases | 13 |
| Edge Cases and Error Handling | 19 |
| Cross-View Navigation Flows | 10 |
| No-Regression Verification | 10 |
| **Total** | **285** |

---

## AWS CLI Command Reference

| Resource Type | List Command | Describe Command |
|--------------|-------------|-----------------|
| VPC | `aws ec2 describe-vpcs` | `aws ec2 describe-vpcs --vpc-ids <id>` |
| Security Groups | `aws ec2 describe-security-groups` | `aws ec2 describe-security-groups --group-ids <id>` |
| EKS Node Groups | `aws eks list-clusters` + `aws eks list-nodegroups --cluster-name <name>` | `aws eks describe-nodegroup --cluster-name <cluster> --nodegroup-name <ng>` |

### Field Mapping Reference

| views.yaml path | AWS CLI JSON field | Example value |
|-----------------|-------------------|---------------|
| VpcId | `.Vpcs[].VpcId` | `vpc-0123456789abcdef0` |
| CidrBlock | `.Vpcs[].CidrBlock` | `10.0.0.0/16` |
| State (VPC) | `.Vpcs[].State` | `available` |
| IsDefault | `.Vpcs[].IsDefault` | `true` |
| InstanceTenancy | `.Vpcs[].InstanceTenancy` | `default` |
| GroupId | `.SecurityGroups[].GroupId` | `sg-0abc123def456789a` |
| GroupName | `.SecurityGroups[].GroupName` | `web-server-sg` |
| VpcId (SG) | `.SecurityGroups[].VpcId` | `vpc-0123456789abcdef0` |
| Description (SG) | `.SecurityGroups[].Description` | `Allow HTTP/HTTPS` |
| IpPermissions | `.SecurityGroups[].IpPermissions` | Array of inbound rules |
| IpPermissionsEgress | `.SecurityGroups[].IpPermissionsEgress` | Array of outbound rules |
| NodegroupName | `.nodegroup.nodegroupName` | `prod-workers` |
| ClusterName (NG) | `.nodegroup.clusterName` | `my-cluster` |
| Status (NG) | `.nodegroup.status` | `ACTIVE` |
| InstanceTypes (NG) | `.nodegroup.instanceTypes` | `["t3.medium"]` |
| ScalingConfig.DesiredSize | `.nodegroup.scalingConfig.desiredSize` | `3` |
| AmiType | `.nodegroup.amiType` | `AL2_x86_64` |
| DiskSize | `.nodegroup.diskSize` | `20` |
| Subnets (NG) | `.nodegroup.subnets` | `["subnet-0abc"]` |
| Labels | `.nodegroup.labels` | `{"role": "worker"}` |
| Taints | `.nodegroup.taints` | `[{"key":"dedicated","value":"gpu","effect":"NO_SCHEDULE"}]` |
