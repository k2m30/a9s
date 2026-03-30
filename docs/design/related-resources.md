# Related Resources: Two-Column Detail View

Issue: #64
Version: 4.0
Target: a9s v3.28+
Status: Design

---

## 1. Overview

Related resources are embedded INSIDE the detail view as a two-column layout.
The detail view evolves from a passive text display into an interactive
navigation hub. There is no separate "related resources" view -- everything
lives in the detail view.

### Left Column -- Enhanced Detail (field-navigable)

The current detail view, but with forward-reference fields made navigable.
Fields containing resource IDs/ARNs that point to other resource types known
to a9s are highlighted and can be opened with Enter.

### Right Column -- Reverse & Algorithmic Related

A flat list of resource types that reference THIS resource (reverse lookups)
and algorithmic connections (naming conventions, multi-hop). Same visual
pattern as the main menu: type name with optional count, dim if unavailable.

### Key Binding: `r` (lowercase)

`r` toggles the right column ON/OFF. Default state: ON (visible when
entering detail view). The toggle state persists across navigation -- if the
user hides the column, it stays hidden when entering a related resource's
detail.

CFN Stack Resources key remap: `r` -> `R` (same as v3.0 spec, unchanged).

---

## 2. Design Decisions and Pushback

### 2.1 Left Column Cursor Model

**Problem:** The current detail view is a `bubbles/viewport` -- a scrolling
text display with no concept of "selected field". The proposal requires
per-field cursor navigation (j/k moves between fields, Enter on a navigable
field opens that resource). This is a fundamental change from viewport to
selectable-list semantics.

**Decision:** The left column becomes a **field list** -- not a free viewport.
Each line is a selectable item. The cursor (row highlight) is always visible
and moves with j/k. The cursor appears on ALL rows (navigable and plain
alike) because restricting cursor movement to only navigable fields would
create confusing "jump" behavior where j/k skips 15 lines to the next
navigable field. Instead, the cursor moves one row at a time like the
resource list, and Enter only acts on navigable fields.

Why not skip non-navigable fields? Consider an EC2 detail view with 40+
fields but only 8 navigable ones. Skipping would mean j/k teleports the
cursor unpredictably, losing the user's scroll position context. The
consistent one-row-at-a-time movement preserves spatial awareness.

**Cursor rendering:** Full-width row highlight matching the table row
selected style (`#7aa2f7` background, `#1a1b26` foreground, bold). This is
the same cursor used everywhere else in a9s.

**Enter behavior on non-navigable fields:** No-op. No flash message, no
error. The user can tell the field is not navigable because it lacks the
navigable styling.

### 2.2 Navigable Field Styling

**Decision:** Navigable field VALUES (not keys) are rendered with underline
styling in the accent color `#7aa2f7`.

Considered alternatives:

| Option | Verdict | Reason |
|--------|---------|--------|
| Underline accent on value | **Chosen** | Web-convention link affordance. Underline is the universal "this is clickable" signal. Only the VALUE is underlined, not the key label. Subtle enough to not overwhelm the detail view. |
| Prefix arrow (e.g., `> VpcId: vpc-0aaa`) | Rejected | Adds visual noise. Breaks alignment of key-value pairs. |
| Different background on the value | Rejected | Creates a patchwork of colored boxes in what should be a clean key-value display. |
| Bold value only | Rejected | Indistinguishable from section headers. |
| Color-only (e.g., green value) | Rejected | Green already means "running/ok" in the status palette. Any other color would conflict with existing semantics. |

When the cursor is on a navigable field, the underline disappears (the
full-row selection highlight takes over). The underline is only visible on
navigable fields that are NOT currently selected.

### 2.3 Right Column Width

The right column displays resource type names with optional counts. The
content is predetermined -- it is always one of the ~66 known resource type
names from `docs/design/resources-groupping.md`.

Longest resource type names with counts:

| Name | Width |
|------|-------|
| `CloudFormation Stacks (12)` | 28 chars |
| `CloudWatch Log Groups (8)` | 27 chars |
| `Auto Scaling Groups (15)` | 25 chars |
| `Elastic Beanstalk` | 18 chars |
| `CloudTrail Events` | 18 chars |

With 2 chars left padding (indent) and 1 char right padding:

**Fixed width: 32 characters.**

This accommodates the longest entry ("CloudFormation Stacks (12)" = 28
chars + 2 indent + 1 right pad = 31, rounded to 32 for breathing room).

### 2.4 Column Separator

**Decision:** A single thin vertical line (`│` U+2502) that changes
color based on which column is focused. No gap, no double border. The
separator occupies exactly 1 character column.

- **Left column focused:** separator in dim color `#414868` (unfocused
  border color). The separator recedes visually, keeping attention on
  the left column content.
- **Right column focused:** separator in accent color `#7aa2f7` (focused
  border color). The separator "lights up" toward the right column,
  drawing the eye to the focused side.

This is a 1-character-wide visual cue that costs no screen real estate.

```
Left focused:   ... PrivateIpAddress: 10.0.48.175  │  Security Groups (3)
                                                    ^ dim #414868

Right focused:  ... PrivateIpAddress: 10.0.48.175  │  Security Groups (3)
                                                    ^ accent #7aa2f7
```

The separator is drawn at the same height as all content rows. The top
and bottom borders span the full width unbroken -- the separator appears
only between the side borders.

### 2.5 Narrow Terminal Threshold

**Threshold: 100 columns.**

| Width | Layout |
|-------|--------|
| 100+ cols | Two columns side by side (left fills remaining, right = 32 fixed) |
| 80-99 cols | Stacked: detail on top, related list below, one scrollable stream |
| 60-79 cols | Stacked, no right column name truncation |
| < 60 cols | "Terminal too narrow" |

At 100 columns: 2 border chars + 32 right column + 1 separator = 35 fixed.
Left column gets 65 characters -- sufficient for key-value display (22-char
key column + 43-char value = comfortable).

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

### 2.6 YAML View

YAML view (`y` toggle) does NOT show the right column. Full-width YAML
only, same as today. `r` in YAML view toggles back to the detail view with
the right column visible (effectively: `r` from YAML = switch to detail
view + show right column). This gives YAML view users a one-key path to
related resources without cluttering the YAML display.

### 2.7 Section Headers and Nested Fields

Section headers (e.g., `State:`, `Tags:`, `Placement:`) and their sub-fields
occupy one row each in the field list. The cursor can land on them (for
consistent j/k behavior) but Enter is a no-op because they are not navigable.

Nested navigable fields (e.g., `VpcSecurityGroups[].VpcSecurityGroupId`
inside a section) ARE navigable and receive the underline styling on their
value.

### 2.8 Array Fields with Multiple Navigable Values

When a field like `SecurityGroups` contains multiple IDs, each array item
is a separate row (already the case in the current detail view). Each item
row is independently navigable. Enter on `sg-0abc123` opens that specific
security group's detail.

---

## 3. Two-Column Layout

### 3.1 Frame Structure

The outer frame is the same manually-constructed box from `design.md`
section 2. The title is centered in the top border as usual.

Inside the frame, content is divided by a vertical separator:

```
+--------- detail -- i-0abc123 (web-prod) ---------+
|  LEFT COLUMN (detail fields)  |  RIGHT COLUMN     |
|                               |  (related types)  |
+-------------------------------+-------------------+
```

The top border has a T-junction where the separator meets it:

```
+------- detail -- i-0abc123 (web-prod) -----+------+
```

Actually, to keep things clean and avoid the complexity of T-junction
alignment with the centered title, the separator is drawn only in the
content rows, not in the top/bottom borders:

```
+---------- detail -- i-0abc123 (web-prod) ----------+
|  InstanceId: i-0abc123          | Sec Groups (3)   |
|  InstanceType: t3.large         | VPC (1)          |
|  State: running                 | Subnet (1)       |
+----------------------------------------------------+
```

The top and bottom borders span the full width unbroken. The vertical
separator appears only between the `|` side borders, as a dim pipe character.

### 3.2 Lipgloss Composition

```
// Each content row:
leftContent  := renderDetailField(field, leftW)  // padded to leftW
sepColor     := "#414868"                         // dim when left focused
if rightFocused { sepColor = "#7aa2f7" }          // accent when right focused
separator    := lipgloss.NewStyle().Foreground(lipgloss.Color(sepColor)).Render("│")
rightContent := renderRelatedRow(row, rightW)     // padded to rightW
row := borderStyle.Render("|") + leftContent + separator + rightContent + borderStyle.Render("|")
```

When the right column is hidden (`r` toggled off):
```
row := borderStyle.Render("|") + renderDetailField(field, innerW) + borderStyle.Render("|")
```

### 3.3 Column Heights

The left and right columns may have different numbers of rows. The left
column (detail fields) typically has 20-60 rows. The right column (related
types) typically has 4-27 rows.

**Behavior:** Both columns scroll independently when focused. The right
column is a fixed-height list that does not scroll with the left column.
Each column has its own scroll position.

When the left column is focused, j/k scrolls the left column. When the
right column is focused (via Tab), j/k scrolls the right column. The
unfocused column stays at its current scroll position.

If the right column has fewer rows than the visible height, the remaining
space is empty (no filler).

---

## 4. Left Column: Field-Navigable Detail

### 4.1 Navigable Field Detection

A field value is navigable if:
1. The value matches a known resource ID pattern (e.g., `vpc-`, `subnet-`,
   `sg-`, `i-`, `vol-`, `ami-`, `arn:aws:`)
2. The target resource type is registered in a9s (in the resource registry)

The set of navigable field patterns per resource type is defined at design
time in the resource type definition (similar to how `ChildViewDef` works).

### 4.2 Field Rendering

| Field Type | Key Style | Value Style | Enter Action |
|------------|-----------|-------------|-------------|
| Plain scalar | `#7aa2f7` | `#c0caf5` | No-op |
| Navigable scalar | `#7aa2f7` | `#7aa2f7` underline | Open target detail |
| Section header | `#e0af68` bold | -- | No-op |
| Sub-field plain | `#7aa2f7` | `#c0caf5` | No-op |
| Sub-field navigable | `#7aa2f7` | `#7aa2f7` underline | Open target detail |
| Array item navigable | `#7aa2f7` | `#7aa2f7` underline | Open target detail |

The cursor highlight overrides all styles when on that row.

### 4.3 Cursor Behavior

| Key | Action | Context |
|-----|--------|---------|
| `j`/`down` | Move cursor down one row | Left column focused |
| `k`/`up` | Move cursor up one row | Left column focused |
| `g` | Jump to first row | Left column focused |
| `G` | Jump to last row | Left column focused |
| `Enter` | Open target resource detail | On navigable field |
| `Enter` | No-op | On non-navigable field |
| `pgup`/`ctrl+u` | Page up | Left column focused |
| `pgdn`/`ctrl+d` | Page down | Left column focused |

### 4.4 Word Wrap

Word wrap is always ON. Long values wrap to the next line. The wrapped
continuation line is NOT a separate selectable row -- it is part of the
same field row. The cursor highlight covers all wrapped lines of the
selected field.

There is no `w` toggle. The YAML view (`y`) serves the "see raw
untruncated data" use case -- it shows full-width YAML with no column
layout.

### 4.5 h/l Column Switching

`h`/`l` switches focus between left and right columns. `h` moves focus
to the left column, `l` moves focus to the right column. This is an
alias for Tab/Shift+Tab. Since word wrap is always on, there is no
horizontal scrolling -- `h`/`l` always means column switching.

This is consistent vim-style navigation: left/right moves between
side-by-side columns, up/down moves within a column.

---

## 5. Right Column: Related Resource Types

### 5.1 Content

The right column shows resource types related to the current resource via:

1. **Reverse relationships** -- resources that reference this resource but
   this resource has no pointer back (from `related-resources/*.md` research)
2. **Algorithmic relationships** -- connections requiring resource-specific
   logic (naming conventions, multi-hop)

Forward relationships are NOT in the right column -- they are already
visible as navigable fields in the left column. This avoids duplication.

Exception: CloudTrail Events always appears in the right column (it is
neither a forward field nor a traditional reverse lookup, but a universal
event search scoped to this resource).

### 5.2 Row Format

Same pattern as the main menu:

```
  Security Groups (3)     -- available, cheap count known
  Target Groups           -- available, expensive (no count)
  CloudWatch Alarms       -- available, expensive (no count)
  EKS Node Groups         -- dim (none found)
  CloudTrail Events       -- always available, no count
```

Row indent: 2 spaces from the separator (left padding inside right column).

### 5.3 Row States

| State | Visual | Cursor |
|-------|--------|--------|
| Available (count known) | Normal text `#c0caf5` with `(N)` | Selectable |
| Available (no count) | Normal text `#c0caf5` | Selectable |
| Unavailable | Dim text `#565f89` | Cursor skips |
| Selected | Full-width highlight `#7aa2f7` bg | Current row |
| Checking (initial load) | Dim text `#565f89` | Cursor skips |

Rows start dim and silently "light up" as background checks complete. Same
silent-loading pattern as the previous design -- no spinners, no
"Checking..." text.

### 5.4 Sort Order

1. **P0** relationships -- alphabetical
2. **P1** relationships -- alphabetical
3. **P2** relationships -- alphabetical
4. **CloudTrail Events** -- always last

### 5.5 Enter Behavior

| Condition | Action |
|-----------|--------|
| Count = 1 | Open target resource detail directly |
| Count > 1 | Open filtered resource list |
| CloudTrail Events | Open ct-search pre-filtered for this resource |
| Dim row | No-op (cursor cannot land here) |

### 5.6 Background Availability Checking

Same pattern as v3.0:

1. All rows start dim
2. Background checks run in parallel
3. Rows become available as results arrive
4. Results cached for session (`{resource-type}:{resource-id}:{related-type}`)
5. `ctrl+r` clears cache and re-checks

### 5.7 Right Column Scroll

If the right column has more rows than the visible height, it scrolls
independently when focused. j/k moves the cursor and scrolls. A dim scroll
indicator appears at the bottom: `v N more` or at the top: `^ N more`.

---

## 6. Column Interaction

### 6.1 Focus Switching

| Key | Action |
|-----|--------|
| `Tab` | Switch focus between left and right columns |
| `Shift+Tab` | Switch focus (reverse direction, same as Tab since there are only 2) |
| `h`/`l` | Switch focus between columns (`h` = left, `l` = right) |

`h`/`l` acts as an alias for Tab/Shift+Tab. `h` switches to the left
column, `l` switches to the right column. Since word wrap is always on,
there is no horizontal scrolling -- `h`/`l` always means column
switching.

The focused column is indicated by two visual cues:
1. **Cursor position:** The cursor is visible in exactly one column at a
   time. The unfocused column has no cursor highlight.
2. **Separator color:** The thin vertical separator changes color based
   on focus (dim `#414868` when left focused, accent `#7aa2f7` when
   right focused). See section 2.4.

When Tab switches to the right column:
- The cursor appears on the first available (non-dim) row
- If the user had a previous position in the right column, it is restored
- j/k now controls the right column

When Tab switches back to the left column:
- The cursor returns to the previously selected field
- j/k now controls the left column

### 6.2 Keys by Focus Context

| Key | Left Column Focused | Right Column Focused |
|-----|--------------------|--------------------|
| `j`/`down` | Move field cursor down | Move related type cursor down |
| `k`/`up` | Move field cursor up | Move related type cursor up |
| `g` | Jump to first field | Jump to first available type |
| `G` | Jump to last field | Jump to last available type |
| `Enter` | Open navigable field target | Open related type (smart) |
| `c` | Copy current field value | Copy selected type name |
| `h`/`l` | Switch column focus (`h` = left, `l` = right) | Switch column focus (`h` = left, `l` = right) |
| `y` | Switch to YAML view | Switch to YAML view |
| `Tab` | Switch to right column | Switch to left column |
| `r` | Toggle right column off | Toggle right column off |
| `esc` | Go back (pop view stack) | Go back (pop view stack) |
| `?` | Help | Help |
| `ctrl+r` | Refresh detail + re-check related | Refresh detail + re-check related |
| `/` | Not active (detail view) | Not active (detail view) |
| `pgup`/`ctrl+u` | Page up | Page up |
| `pgdn`/`ctrl+d` | Page down | Page down |

### 6.3 Right Column Hidden

When `r` toggles the right column off:
- Left column expands to full width (same as today's detail view)
- Tab does nothing (only one column)
- The cursor stays in the left column
- All field-navigation behavior is unchanged
- Press `r` again to restore the right column

---

## 7. Navigation

### 7.1 Enter from Left Column (Navigable Field)

Opens a new detail view for the target resource. The new detail view has
its own right column (if `r` toggle is on) with that resource's related
types.

Example chain: EC2 detail -> Enter on `VpcId` -> VPC detail (with VPC's
related types in right column) -> Enter on navigable Subnet field -> Subnet
detail -> ...

### 7.2 Enter from Right Column

- **Count > 1:** Opens a filtered resource list showing only the related
  instances. This is a standard `ResourceListView` with a scope filter.
  Frame title: `{type}({count}) -- {resource-id} ({resource-name})`
- **Count = 1:** Opens the target resource's detail view directly.
- **CloudTrail Events:** Opens ct-search pre-filtered for this resource.

### 7.3 View Stack

Navigation uses the existing view stack. Each entry is a push.

```
push(MainMenu) -> push(ec2-list) -> push(ec2-detail:i-abc)
  -> [Enter on VpcId field] push(vpc-detail:vpc-xxx)
     -> [Tab, Enter on "Subnets (6)"] push(subnet-list:filtered)
        -> push(subnet-detail:subnet-yyy)
```

Esc pops one level at a time. The `r` toggle state persists across the
stack (stored in the app model, not per-view).

### 7.4 Depth Indicator

When view stack depth exceeds 4, the header shows `[N]` replacing the
version number (unchanged from v3.0 design):

```
Normal (depth <= 4):  a9s v3.28.0  prod:us-east-1            ? for help
Deep   (depth > 4):   a9s [6]      prod:us-east-1            ? for help
```

---

## 8. ASCII Wireframes

### 8.1 Two-Column Detail View (120 cols, right column visible)

EC2 instance, left column focused, cursor on InstanceId (non-navigable).

```
 a9s v3.28.0  prod:us-east-1                                                                         ? for help
+----------------------------- detail -- i-0abc123 (web-prod) ------------------------------------------------+
|[SELECTED] InstanceId:          i-0abc123def456789a         [/]| [DIM]Target Groups               [/]        |
| InstanceType:        t3.large                               |   Auto Scaling Groups                          |
| State:               running                                |   CloudWatch Alarms                            |
| VpcId:               [UNDERLINE]vpc-0aaa111bbb222cc[/]      |   EKS Node Groups                              |
| SubnetId:            [UNDERLINE]subnet-0bbb222ccc333dd[/]   |   CloudFormation Stacks                        |
| SecurityGroups:                                             | [DIM]Elastic Beanstalk[/]                        |
|     - GroupId:       [UNDERLINE]sg-0ccc333ddd444ee[/]       |   EBS Snapshots                                |
|     - GroupId:       [UNDERLINE]sg-0ddd444eee555ff[/]       |   Elastic IPs                                  |
| IamInstanceProfile:                                         |   CloudTrail Events                            |
|     Arn:             [UNDERLINE]arn:aws:iam::role/web[/]    |                                                |
| ImageId:             [UNDERLINE]ami-0aaa111222333[/]        |                                                |
| KeyName:             prod-keypair                           |                                                |
| PrivateIpAddress:    10.0.48.175                            |                                                |
| PublicIpAddress:     203.0.113.10                           |                                                |
| LaunchTime:          2026-03-15 09:22:45                    |                                                |
| Architecture:        x86_64                                 |                                                |
| Placement:                                                  |                                                |
|     AvailabilityZone: us-east-1a                            |                                                |
|     Tenancy:         default                                |                                                |
| Tags:                                                       |                                                |
|     - Key: Name                                             |                                                |
|       Value: web-prod                                       |                                                |
|     - Key: Environment                                      |                                                |
|       Value: production                                     |                                                |
+-------------------------------------------------------------------------------------------------------------+
```

Notes:
- Left column: cursor on first row (InstanceId, non-navigable, full row highlight)
- Navigable values: VpcId, SubnetId, SecurityGroup GroupIds, IAM Arn, ImageId
  are rendered with underline in `#7aa2f7` (shown as `[UNDERLINE]` above)
- Non-navigable values: InstanceType, State, KeyName, IPs, etc. in plain `#c0caf5`
- Right column: related types with silent loading. Some available (normal text),
  some dim (no results found). CloudTrail Events always last.
- Separator: thin `│` in dim `#414868` (left column focused)

### 8.2 Right Column Focused (Tab pressed)

Same view, but Tab has moved focus to the right column.

```
 a9s v3.28.0  prod:us-east-1                                                                         ? for help
+----------------------------- detail -- i-0abc123 (web-prod) ------------------------------------------------+
| InstanceId:          i-0abc123def456789a                    |[SELECTED] Auto Scaling Groups       [/]       |
| InstanceType:        t3.large                               |   CloudWatch Alarms                            |
| State:               running                                |   CloudFormation Stacks                        |
| VpcId:               [UNDERLINE]vpc-0aaa111bbb222cc[/]      |   EBS Snapshots                                |
| SubnetId:            [UNDERLINE]subnet-0bbb222ccc333dd[/]   |   Elastic IPs                                  |
| SecurityGroups:                                             |   CloudTrail Events                            |
|     - GroupId:       [UNDERLINE]sg-0ccc333ddd444ee[/]       | [DIM]Target Groups[/]                            |
|     - GroupId:       [UNDERLINE]sg-0ddd444eee555ff[/]       | [DIM]EKS Node Groups[/]                          |
| IamInstanceProfile:                                         | [DIM]Elastic Beanstalk[/]                        |
|     Arn:             [UNDERLINE]arn:aws:iam::role/web[/]    |                                                |
| ImageId:             [UNDERLINE]ami-0aaa111222333[/]        |                                                |
| ...                                                         |                                                |
+-------------------------------------------------------------------------------------------------------------+
```

Notes:
- Left column: no cursor highlight, navigable fields still underlined
- Right column: cursor on "Auto Scaling Groups" (first available row)
- Separator: accent `#7aa2f7` (right column focused)
- Dim rows at bottom: Target Groups (none found), EKS Node Groups, EB

### 8.3 Right Column Hidden (r toggled off)

```
 a9s v3.28.0  prod:us-east-1                                                                         ? for help
+----------------------------- detail -- i-0abc123 (web-prod) ------------------------------------------------+
|[SELECTED] InstanceId:          i-0abc123def456789a                                                 [/]      |
| InstanceType:        t3.large                                                                               |
| State:               running                                                                                |
| VpcId:               [UNDERLINE]vpc-0aaa111bbb222cc[/]                                                      |
| SubnetId:            [UNDERLINE]subnet-0bbb222ccc333dd[/]                                                   |
| SecurityGroups:                                                                                             |
|     - GroupId:       [UNDERLINE]sg-0ccc333ddd444ee[/]                                                       |
|     - GroupId:       [UNDERLINE]sg-0ddd444eee555ff[/]                                                       |
| IamInstanceProfile:                                                                                         |
|     Arn:             [UNDERLINE]arn:aws:iam::role/web[/]                                                    |
| ImageId:             [UNDERLINE]ami-0aaa111222333[/]                                                        |
| ...                                                                                                         |
+-------------------------------------------------------------------------------------------------------------+
```

Full-width detail view. Same as today but with navigable field styling and
field-level cursor. Press `r` to bring back the right column.

### 8.4 RDS Instance Detail (different resource type)

Shows that forward relationships in the left column differ per resource
type, and the right column shows RDS-specific reverse relationships.

```
 a9s v3.28.0  prod:us-east-1                                                                         ? for help
+-------------------------------- detail -- mydb-prod --------------------------------------------------------+
| DBInstanceIdentifier: mydb-prod                             |   CloudWatch Alarms                            |
| Engine:               postgres                              |   RDS Snapshots                                |
| EngineVersion:        15.4                                  |   Secrets Manager                              |
| DBInstanceStatus:     available                             |   CloudWatch Log Groups                        |
| DBInstanceClass:      db.t3.medium                          |   CloudTrail Events                            |
| Endpoint:                                                   | [DIM]CloudFormation Stacks[/]                    |
|     Address:          mydb-prod.abc123.rds.amazonaws.com    |                                                |
|     Port:             5432                                  |                                                |
| MultiAZ:             true                                   |                                                |
| VpcSecurityGroups:                                          |                                                |
|     - VpcSecurityGroupId: [UNDERLINE]sg-0abc123[/]          |                                                |
|       Status:         active                                |                                                |
| DBSubnetGroup:                                              |                                                |
|     DBSubnetGroupName: prod-db-subnets                      |                                                |
|     Subnets:                                                |                                                |
|       - SubnetId:     [UNDERLINE]subnet-0aaa111[/]          |                                                |
|       - SubnetId:     [UNDERLINE]subnet-0bbb222[/]          |                                                |
| KmsKeyId:            [UNDERLINE]arn:aws:kms:...:key/abc[/]  |                                                |
| StorageEncrypted:    true                                   |                                                |
+-------------------------------------------------------------------------------------------------------------+
```

Notes:
- Forward navigable fields: VpcSecurityGroupId, SubnetIds, KmsKeyId (underlined)
- Right column: RDS-specific reverse relationships (Alarms, Snapshots, Secrets)
- Read Replicas would appear as navigable forward fields if present

### 8.5 VPC Detail (heavy right column -- many reverse relationships)

VPC has the most reverse relationships (~18 types). The right column scrolls.

```
 a9s v3.28.0  prod:us-east-1                                                                         ? for help
+---------------------------- detail -- vpc-0aaa111 (production-vpc) ------------------------------------------+
|[SELECTED] VpcId:               vpc-0aaa111bbb222cc         [/]|   EC2 Instances                              |
| CidrBlock:           10.0.0.0/16                            |   Subnets (6)                                  |
| State:               available                              |   Security Groups (12)                         |
| IsDefault:           false                                  |   Route Tables (3)                             |
| DhcpOptionsId:       dopt-0abc123                           |   NAT Gateways (2)                             |
| InstanceTenancy:     default                                |   Internet Gateways (1)                        |
| Tags:                                                       |   VPC Endpoints (4)                            |
|     - Key: Name                                             |   Transit Gateways                             |
|       Value: production-vpc                                 |   Load Balancers                               |
|     - Key: Environment                                      |   Lambda Functions                             |
|       Value: prod                                           |   EKS Clusters                                 |
|                                                             |   DB Instances                                 |
|                                                             |   ElastiCache                                  |
|                                                             | [DIM]OpenSearch[/]                               |
|                                                             | [DIM]Redshift[/]                                 |
|                                                             | [DIM]MSK Clusters[/]                             |
|                                                             |   CloudTrail Events                            |
|                                                             |                          v 2 more              |
+-------------------------------------------------------------------------------------------------------------+
```

Notes:
- VPC has very few forward navigable fields (DhcpOptionsId is not in a9s)
- Right column is long -- scroll indicator at bottom ("v 2 more")
- Left column is short (VPC has few fields), so the frame height is driven
  by the right column
- When right column is focused, j/k scrolls through the 17 types

### 8.6 Stacked Layout (narrow terminal, 80 cols)

Below 100 columns, the right column stacks below the detail fields.

```
 a9s v3.28.0  prod:us-east-1                                ? for help
+-------- detail -- i-0abc123 (web-prod) -------------------+
|[SELECTED] InstanceId:      i-0abc123def456789a         [/]|
| InstanceType:    t3.large                                 |
| State:           running                                  |
| VpcId:           [UNDERLINE]vpc-0aaa111bbb222cc[/]        |
| SubnetId:        [UNDERLINE]subnet-0bbb222ccc333dd[/]     |
| SecurityGroups:                                           |
|     - GroupId:   [UNDERLINE]sg-0ccc333ddd444ee[/]         |
|     - GroupId:   [UNDERLINE]sg-0ddd444eee555ff[/]         |
| ...                                                       |
| [DIM]-- Related -------------------------------------------[/]|
|   Auto Scaling Groups                                     |
|   CloudWatch Alarms                                       |
|   CloudFormation Stacks                                   |
| [DIM]EKS Node Groups[/]                                    |
|   CloudTrail Events                                       |
+-----------------------------------------------------------+
```

Notes:
- Single column, full width
- Dim separator line "-- Related ---" divides the two sections
- Tab switches focus between the top section (detail fields) and the
  bottom section (related types)
- Scrolls as one continuous viewport when detail fields are long

### 8.7 Initial Load (right column, all dim)

When first entering the detail view, all right column rows are dim while
background availability checks run.

```
 a9s v3.28.0  prod:us-east-1                                                                         ? for help
+----------------------------- detail -- i-0abc123 (web-prod) ------------------------------------------------+
| InstanceId:          i-0abc123def456789a                    | [DIM]Target Groups[/]                            |
| InstanceType:        t3.large                               | [DIM]Auto Scaling Groups[/]                      |
| State:               running                                | [DIM]CloudWatch Alarms[/]                        |
| VpcId:               [UNDERLINE]vpc-0aaa111bbb222cc[/]      | [DIM]EKS Node Groups[/]                          |
| SubnetId:            [UNDERLINE]subnet-0bbb222ccc333dd[/]   | [DIM]CloudFormation Stacks[/]                    |
| ...                                                         | [DIM]Elastic Beanstalk[/]                        |
|                                                             | [DIM]EBS Snapshots[/]                            |
|                                                             | [DIM]Elastic IPs[/]                              |
|                                                             | [DIM]CloudTrail Events[/]                        |
+-------------------------------------------------------------------------------------------------------------+
```

Left column is immediately usable (navigable fields are styled). Right
column rows light up as background checks complete.

### 8.8 Deep Navigation with Depth Indicator

After navigating EC2 -> VpcId -> Subnets list -> Subnet detail (depth 5):

```
 a9s [5]  prod:us-east-1                                                                             ? for help
+------------------------ detail -- subnet-0bbb222 (private-us-east-1a) --------------------------------------+
| SubnetId:            subnet-0bbb222ccc333dd                 |   EC2 Instances                                |
| VpcId:               [UNDERLINE]vpc-0aaa111bbb222cc[/]      |   NAT Gateways                                 |
| CidrBlock:           10.0.1.0/24                            |   Network Interfaces                           |
| AvailabilityZone:    us-east-1a                             |   Route Tables                                 |
| AvailableIpAddressCount: 251                                |   CloudTrail Events                            |
| MapPublicIpOnLaunch: false                                  | [DIM]Load Balancers[/]                           |
| State:               available                              |                                                |
| Tags:                                                       |                                                |
|     - Key: Name                                             |                                                |
|       Value: private-us-east-1a                             |                                                |
+-------------------------------------------------------------------------------------------------------------+
```

### 8.9 Lambda Detail (algorithmic relationships)

Lambda has algorithmic relationships (CW Log Group via naming convention,
Event Source Mappings) alongside reverse relationships.

```
 a9s v3.28.0  prod:us-east-1                                                                         ? for help
+----------------------- detail -- process-orders (process-orders) -------------------------------------------+
| FunctionName:        process-orders                         |   CW Log Group                                 |
| Runtime:             python3.11                             |   SQS Event Sources (2)                        |
| Handler:             handler.main                           |   EventBridge Rules                            |
| MemorySize:          256                                    |   SNS Subscriptions                            |
| Timeout:             30                                     |   CloudWatch Alarms                            |
| Role:                [UNDERLINE]arn:aws:iam::role/lambda[/] |   API Gateway                                  |
| VpcConfig:                                                  |   CloudTrail Events                            |
|     VpcId:           [UNDERLINE]vpc-0aaa111bbb222cc[/]      | [DIM]S3 Notifications[/]                         |
|     SubnetIds:                                              | [DIM]Step Functions[/]                            |
|       - [UNDERLINE]subnet-0aaa111[/]                        | [DIM]Target Groups[/]                            |
|       - [UNDERLINE]subnet-0bbb222[/]                        |                                                |
|     SecurityGroupIds:                                       |                                                |
|       - [UNDERLINE]sg-0abc123[/]                            |                                                |
| ...                                                         |                                                |
+-------------------------------------------------------------------------------------------------------------+
```

Notes:
- Forward navigable: Role ARN, VpcId, SubnetIds, SecurityGroupIds
- Right column: CW Log Group (algorithmic, via naming convention), SQS Event
  Sources (from ListEventSourceMappings), EventBridge Rules (reverse), etc.
- "SQS Event Sources (2)" has a count because the Lambda API provides this
  via ListEventSourceMappings (a relatively cheap call)

---

## 9. State Transitions

### Msg Types

| Msg | From State | To State | Notes |
|-----|-----------|----------|-------|
| `KeyMsg(d)` / `KeyMsg(enter)` | ResourceListView | DetailView (two-column) | Push; start right-column checks |
| `RelatedTypeCheckedMsg` | Right col: checking | Right col: updated | One type resolved |
| `AllRelatedTypesCheckedMsg` | Right col: some dim | Right col: all resolved | All checks done |
| `KeyMsg(Tab)` | Left focused | Right focused | Focus switch |
| `KeyMsg(Tab)` | Right focused | Left focused | Focus switch |
| `KeyMsg(h)` | Right focused | Left focused | Alias for Shift+Tab |
| `KeyMsg(l)` | Left focused | Right focused | Alias for Tab |
| `KeyMsg(Enter)` | Left, navigable field | New DetailView | Push target resource's detail |
| `KeyMsg(Enter)` | Left, non-navigable | No-op | Nothing happens |
| `KeyMsg(Enter)` | Right, count=1 | New DetailView | Push target detail directly |
| `KeyMsg(Enter)` | Right, count>1 | FilteredListView | Push filtered resource list |
| `KeyMsg(Enter)` | Right, CloudTrail | ct-search view | Push pre-filtered ct-search |
| `KeyMsg(r)` | Right visible | Right hidden | Left expands to full width |
| `KeyMsg(r)` | Right hidden | Right visible | Two-column layout restored |
| `KeyMsg(y)` | DetailView | YAMLView | Right column hidden in YAML |
| `KeyMsg(esc)` | DetailView | Previous view | Pop view stack |
| `KeyMsg(ctrl+r)` | DetailView | DetailView refreshed | Re-fetch detail + re-check related |
| `KeyMsg(c)` | Left focused | Copied field value | Flash "Copied!" in header |
| `KeyMsg(c)` | Right focused | Copied type name | Flash "Copied!" in header |
| `tea.WindowSizeMsg` | Two-column | Reflow | May switch to stacked below 100 cols |

### View Stack Examples

Simple: field navigation:
```
push(MainMenu) -> push(ec2-list) -> push(ec2-detail:i-abc)
  -> [Enter on VpcId] push(vpc-detail:vpc-xxx)
     -> [esc] pop back to ec2-detail:i-abc
```

Chained via right column:
```
push(MainMenu) -> push(ec2-list) -> push(ec2-detail:i-abc)
  -> [Tab, Enter on "CloudWatch Alarms"] push(alarm-list:filtered-to-i-abc)
     -> push(alarm-detail:alarm-xxx)
        -> [Enter on navigable field or Tab+Enter on right column] -> ...
```

Mixed navigation:
```
push(ec2-detail:i-abc) -> [Enter on sg-0ccc333] push(sg-detail:sg-0ccc333)
  -> [Tab, Enter on "EC2 Instances"] push(ec2-list:filtered-to-sg-0ccc333)
     -> [Enter on i-0def456] push(ec2-detail:i-0def456)
```

---

## 10. Key Binding Table

### Detail View (Two-Column)

| Key | Action | Context |
|-----|--------|---------|
| `j`/`down` | Move cursor down in focused column | Either column |
| `k`/`up` | Move cursor up in focused column | Either column |
| `g` | Jump to top of focused column | Either column |
| `G` | Jump to bottom of focused column | Either column |
| `Enter` | Open navigable field / related type | Either column |
| `Tab` | Switch column focus | Right column visible |
| `r` | Toggle right column visibility | Any |
| `y` | Switch to YAML view (full width) | Any |
| `h`/`l` | Switch column focus (`h` = left, `l` = right) | Either column |
| `c` | Copy field value / type name | Either column |
| `esc` | Go back | Any |
| `?` | Help | Any |
| `ctrl+r` | Refresh all | Any |
| `pgup`/`ctrl+u` | Page up in focused column | Either column |
| `pgdn`/`ctrl+d` | Page down in focused column | Either column |
| `ctrl+c` | Force quit | Any |

### Keys NOT Available

| Key | Why Not |
|-----|---------|
| `/` (filter) | Detail view has no filterable list of instances |
| `:` (command) | Already available globally (unchanged) |
| `d` (detail) | Already in detail view |
| `N`/`S`/`A` (sort) | Not a sortable list |

---

## 11. Color/Style Table

No new colors. Two new style combinations using existing palette values.

| Element | Foreground | Background | Style | Notes |
|---------|-----------|-----------|-------|-------|
| Detail key (all) | `#7aa2f7` | -- | -- | Existing: Detail key |
| Detail value (plain) | `#c0caf5` | -- | -- | Existing: Detail value |
| Detail value (navigable) | `#7aa2f7` | -- | Underline | NEW: accent + underline |
| Detail section header | `#e0af68` | -- | Bold | Existing: Detail section |
| Cursor row (either col) | `#1a1b26` | `#7aa2f7` | Bold | Existing: Table row selected |
| Right col row (available) | `#c0caf5` | -- | -- | Existing: Table row normal |
| Right col row (dim) | `#565f89` | -- | Dim | Existing: Table row dim |
| Right col row (selected) | `#1a1b26` | `#7aa2f7` | Bold | Existing: Table row selected |
| Column separator (left focused) | `#414868` | -- | -- | Existing: Table border (dim) |
| Column separator (right focused) | `#7aa2f7` | -- | -- | NEW: accent color when right col focused |
| Stacked separator line | `#414868` | -- | Dim | Existing: Border color |
| Frame border | `#414868` | -- | -- | Existing: Table border |
| Frame title | `#c0caf5` | -- | Bold | Existing: Frame title |
| Depth indicator `[N]` | `#565f89` | -- | -- | Existing: Header dim |
| Scroll indicator | `#565f89` | -- | Dim | Existing: Dim text |

---

## 12. Bubbles Components

| Component | Bubbles | Notes |
|-----------|---------|-------|
| Left column (field list) | Custom renderer | Field-level cursor, navigable detection |
| Right column (type list) | Custom renderer | Same pattern as main menu |
| Help screen | Custom multi-column | Existing help renderer |
| Filtered resource list | Existing `ResourceListView` | Reused with scope filter |

The left column is no longer a `bubbles/viewport`. It is a custom list
renderer that understands field types, navigability, and cursor position.
This is a breaking change from the current viewport-based detail view.

---

## 13. Responsive Behavior

### Width Breakpoints

| Terminal Width | Layout | Right Column |
|----------------|--------|-------------|
| < 60 cols | "Terminal too narrow" | N/A |
| 60-99 cols | Stacked (detail above, related below) | Below detail, full width |
| 100-119 cols | Two columns, right = 32 chars | Side by side |
| 120+ cols | Two columns, right = 32 chars | Side by side, comfortable |

### Height Breakpoints

| Terminal Height | Behavior |
|-----------------|----------|
| < 7 lines | "Terminal too short" |
| 7-14 lines | Both columns visible, very limited scroll |
| 15+ lines | Comfortable |

### Column Height Mismatch

When the left column has more rows than the right:
- Left column scrolls normally
- Right column content is top-aligned, empty space below

When the right column has more rows than visible height:
- Right column scrolls independently when focused
- Scroll indicator shows remaining rows

---

## 14. Help Screen

When `?` is pressed in the two-column detail view:

```
 a9s v3.28.0  prod:us-east-1                                                                         ? for help
+----------------------------------------------- Help ---------------------------------------------------+
| DETAIL                     RELATED              NAVIGATION           HOTKEYS                            |
|                                                                                                        |
| <enter>  Open link         <tab>    Switch col  <j>       Down       <?>  Help                         |
| <esc>    Go back           <r>      Toggle col  <k>       Up         <:>  Command                      |
| <h/l>    Switch col                             <g>       Top                                           |
| <c>      Copy value                             <G>       Bottom                                       |
| <y>      YAML view                              <pgdn>    Page down                                    |
|                                                  <pgup>    Page up                                      |
|                                                                                                        |
|                      Press any key to close                                                             |
+--------------------------------------------------------------------------------------------------------+
```

---

## 15. Example: EC2 Instance Relationship Distribution

For an EC2 instance with the two-column layout:

### Left Column (Forward Navigable Fields)

| Field | Value Pattern | Target Type |
|-------|--------------|-------------|
| VpcId | `vpc-*` | VPC detail |
| SubnetId | `subnet-*` | Subnet detail |
| SecurityGroups[].GroupId | `sg-*` | Security Group detail |
| ImageId | `ami-*` | AMI detail |
| BlockDeviceMappings[].Ebs.VolumeId | `vol-*` | EBS Volume detail |
| NetworkInterfaces[].NetworkInterfaceId | `eni-*` | ENI detail |
| IamInstanceProfile.Arn | `arn:aws:iam::*` | IAM Role detail |

### Right Column (Reverse + Algorithmic)

| Related Type | Source | Priority |
|-------------|--------|----------|
| Target Groups | Reverse: iterate TGs, match instance ID | P0 |
| Auto Scaling Groups | Reverse: DescribeAutoScalingInstances | P0 |
| CloudWatch Alarms | Reverse: filter by InstanceId dimension | P1 |
| CloudFormation Stacks | Reverse: check tags | P1 |
| EKS Node Groups | Reverse: check tags | P1 |
| Elastic Beanstalk | Reverse: check tags | P2 |
| EBS Snapshots | Algorithmic: snapshots of attached volumes | P1 |
| Elastic IPs | Algorithmic: DescribeAddresses by instance | P1 |
| CloudTrail Events | Always present | Last |

Total: ~7 forward navigable fields in left column, ~9 types in right column.
No duplication between columns.

---

## 16. Open Design Questions (Resolved)

### Q1: Should forward relationships appear in both columns?

**Answer: No.** Forward navigable fields (VpcId, SubnetId, etc.) appear
ONLY in the left column as underlined values. The right column shows ONLY
reverse and algorithmic relationships. This eliminates duplication and gives
each column a clear purpose: left = "what does this resource point to",
right = "what points to this resource".

### Q2: What about CloudTrail Events?

**Answer:** CloudTrail Events appears in the right column as the last row.
It is always available (never dim). Enter opens ct-search pre-filtered for
this resource. It is not a forward field because there is no CloudTrail
field in the resource's API response.

### Q3: How does `c` (copy) work with two columns?

**Answer:** `c` copies contextually. Left column focused: copies the
current field's value (unchanged from today). Right column focused: copies
the selected resource type name (less useful, but consistent -- the user
can navigate into the related resource and copy from there).

### Q4: What happens when right column is hidden and Enter is pressed on a navigable field?

**Answer:** Opens the target resource's detail view, same as when the right
column is visible. The right column visibility has no effect on left column
Enter behavior.

### Q5: Can the user resize the right column?

**Answer: No.** Fixed at 32 characters. The content is predetermined
resource type names, so dynamic width adds complexity without benefit.
