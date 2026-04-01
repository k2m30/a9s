# Related Resources: Two-Column Detail View

Issue: #64
Version: 4.3
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

**Key binding implementation:** The detail view uses a NEW `ToggleRelated`
binding in `keys.Map`, separate from the existing `Resources` binding.
Both use the physical key `r` but are distinct bindings dispatched in
different view contexts:

| Binding | Struct Field | Physical Key | Active Context |
|---------|-------------|-------------|----------------|
| `Resources` | `keys.Map.Resources` | `r` | Resource list views (child-view trigger) |
| `ToggleRelated` | `keys.Map.ToggleRelated` | `r` | Detail view (two-column toggle) |

There is no code conflict because the detail view and resource list view
are mutually exclusive -- only one is active at a time. The detail view's
`Update()` matches `ToggleRelated`, never `Resources`. The resource list
view's `Update()` matches `Resources`, never `ToggleRelated`.

**CFN Stack Resources remap:** The existing `Resources` child-view
binding for CloudFormation stacks uses `Key: "r"` in its `ChildViewDef`.
To avoid collision with `ToggleRelated` in the detail view,
the CFN stack resource type definition changes `Key: "r"` to `Key: "R"`
(uppercase). A new `keys.Map.StackResources` binding is added:

| Binding | Struct Field | Physical Key | Active Context |
|---------|-------------|-------------|----------------|
| `StackResources` | `keys.Map.StackResources` | `R` | CFN stack resource list (child-view trigger) |

This is a concrete `ChildViewDef` change in the CFN stack resource type
registration, not a behavioral change -- the same child view is triggered,
just with a shifted key to free `r` for the related column toggle.

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

The first row of the right column is a `RELATED` header label:

```
  RELATED                   -- header, dim #565f89, not selectable
  Security Groups (3)     -- available, cheap count known
  Target Groups           -- available, expensive (no count)
  CloudWatch Alarms       -- available, expensive (no count)
  EKS Node Groups         -- dim (none found)
  CloudTrail Events       -- always available, no count
```

The `RELATED` header uses dim color `#565f89`, no bold, no accent. It is
a passive label, not a navigation target. The cursor skips it (same
behavior as dim/unavailable rows). When Tab moves focus to the right
column, the cursor lands on the first resource type row, not the header.

No blank spacer line between the header and the first resource type row.

The `RELATED` header appears only in the two-column (side-by-side)
layout. In stacked mode (< 100 cols), the existing `-- Related ---`
separator line serves as the header.

Row indent: 2 spaces from the separator (left padding inside right column).

### 5.3 Row States

| State | Visual | Cursor |
|-------|--------|--------|
| Header (`RELATED`) | Dim text `#565f89` | Cursor skips |
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
| `/` | Activate search (left column text) | Filter right column list |
| `n` | Next search match | -- (not applicable) |
| `N` | Previous search match | -- (not applicable) |
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
|[SELECTED] InstanceId:          i-0abc123def456789a         [/]| [DIM]RELATED[/]                                  |
| InstanceType:        t3.large                               | [DIM]Target Groups[/]                            |
| State:               running                                |   Auto Scaling Groups                          |
| VpcId:               [UNDERLINE]vpc-0aaa111bbb222cc[/]      |   CloudWatch Alarms                            |
| SubnetId:            [UNDERLINE]subnet-0bbb222ccc333dd[/]   |   EKS Node Groups                              |
| SecurityGroups:                                             |   CloudFormation Stacks                        |
|     - GroupId:       [UNDERLINE]sg-0ccc333ddd444ee[/]       | [DIM]Elastic Beanstalk[/]                        |
|     - GroupId:       [UNDERLINE]sg-0ddd444eee555ff[/]       |   EBS Snapshots                                |
| IamInstanceProfile:                                         |   Elastic IPs                                  |
|     Arn:             [UNDERLINE]arn:aws:iam::role/web[/]    |   CloudTrail Events                            |
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
- Right column: `RELATED` header (dim `#565f89`) as first row, not selectable.
  Resource types below with silent loading. Some available (normal text),
  some dim (no results found). CloudTrail Events always last.
- Separator: thin `│` in dim `#414868` (left column focused)

### 8.2 Right Column Focused (Tab pressed)

Same view, but Tab has moved focus to the right column.

```
 a9s v3.28.0  prod:us-east-1                                                                         ? for help
+----------------------------- detail -- i-0abc123 (web-prod) ------------------------------------------------+
| InstanceId:          i-0abc123def456789a                    | [DIM]RELATED[/]                                  |
| InstanceType:        t3.large                               |[SELECTED] Auto Scaling Groups       [/]       |
| State:               running                                |   CloudWatch Alarms                            |
| VpcId:               [UNDERLINE]vpc-0aaa111bbb222cc[/]      |   CloudFormation Stacks                        |
| SubnetId:            [UNDERLINE]subnet-0bbb222ccc333dd[/]   |   EBS Snapshots                                |
| SecurityGroups:                                             |   Elastic IPs                                  |
|     - GroupId:       [UNDERLINE]sg-0ccc333ddd444ee[/]       |   CloudTrail Events                            |
|     - GroupId:       [UNDERLINE]sg-0ddd444eee555ff[/]       | [DIM]Target Groups[/]                            |
| IamInstanceProfile:                                         | [DIM]EKS Node Groups[/]                          |
|     Arn:             [UNDERLINE]arn:aws:iam::role/web[/]    | [DIM]Elastic Beanstalk[/]                        |
| ImageId:             [UNDERLINE]ami-0aaa111222333[/]        |                                                |
| ...                                                         |                                                |
+-------------------------------------------------------------------------------------------------------------+
```

Notes:
- Left column: no cursor highlight, navigable fields still underlined
- Right column: `RELATED` header (dim) at top, cursor on "Auto Scaling Groups"
  (first available row, skipping the header)
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
| DBInstanceIdentifier: mydb-prod                             | [DIM]RELATED[/]                                  |
| Engine:               postgres                              |   CloudWatch Alarms                            |
| EngineVersion:        15.4                                  |   RDS Snapshots                                |
| DBInstanceStatus:     available                             |   Secrets Manager                              |
| DBInstanceClass:      db.t3.medium                          |   CloudWatch Log Groups                        |
| Endpoint:                                                   |   CloudTrail Events                            |
|     Address:          mydb-prod.abc123.rds.amazonaws.com    | [DIM]CloudFormation Stacks[/]                    |
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
|[SELECTED] VpcId:               vpc-0aaa111bbb222cc         [/]| [DIM]RELATED[/]                                  |
| CidrBlock:           10.0.0.0/16                            |   EC2 Instances                                |
| State:               available                              |   Subnets (6)                                  |
| IsDefault:           false                                  |   Security Groups (12)                         |
| DhcpOptionsId:       dopt-0abc123                           |   Route Tables (3)                             |
| InstanceTenancy:     default                                |   NAT Gateways (2)                             |
| Tags:                                                       |   Internet Gateways (1)                        |
|     - Key: Name                                             |   VPC Endpoints (4)                            |
|       Value: production-vpc                                 |   Transit Gateways                             |
|     - Key: Environment                                      |   Load Balancers                               |
|       Value: prod                                           |   Lambda Functions                             |
|                                                             |   EKS Clusters                                 |
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
| InstanceId:          i-0abc123def456789a                    | [DIM]RELATED[/]                                  |
| InstanceType:        t3.large                               | [DIM]Target Groups[/]                            |
| State:               running                                | [DIM]Auto Scaling Groups[/]                      |
| VpcId:               [UNDERLINE]vpc-0aaa111bbb222cc[/]      | [DIM]CloudWatch Alarms[/]                        |
| SubnetId:            [UNDERLINE]subnet-0bbb222ccc333dd[/]   | [DIM]EKS Node Groups[/]                          |
| ...                                                         | [DIM]CloudFormation Stacks[/]                    |
|                                                             | [DIM]Elastic Beanstalk[/]                        |
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
| SubnetId:            subnet-0bbb222ccc333dd                 | [DIM]RELATED[/]                                  |
| VpcId:               [UNDERLINE]vpc-0aaa111bbb222cc[/]      |   EC2 Instances                                |
| CidrBlock:           10.0.1.0/24                            |   NAT Gateways                                 |
| AvailabilityZone:    us-east-1a                             |   Network Interfaces                           |
| AvailableIpAddressCount: 251                                |   Route Tables                                 |
| MapPublicIpOnLaunch: false                                  |   CloudTrail Events                            |
| State:               available                              | [DIM]Load Balancers[/]                           |
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
| FunctionName:        process-orders                         | [DIM]RELATED[/]                                  |
| Runtime:             python3.11                             |   CW Log Group                                 |
| Handler:             handler.main                           |   SQS Event Sources (2)                        |
| MemorySize:          256                                    |   EventBridge Rules                            |
| Timeout:             30                                     |   SNS Subscriptions                            |
| Role:                [UNDERLINE]arn:aws:iam::role/lambda[/] |   CloudWatch Alarms                            |
| VpcConfig:                                                  |   API Gateway                                  |
|     VpcId:           [UNDERLINE]vpc-0aaa111bbb222cc[/]      |   CloudTrail Events                            |
|     SubnetIds:                                              | [DIM]S3 Notifications[/]                         |
|       - [UNDERLINE]subnet-0aaa111[/]                        | [DIM]Step Functions[/]                            |
|       - [UNDERLINE]subnet-0bbb222[/]                        | [DIM]Target Groups[/]                            |
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
| `KeyMsg(/)` | Left focused, normal | Left: search input active | Header shows "/..." input |
| `KeyMsg(/)` | Right focused, normal | Right: filter input active | Header shows "/..." input; list filters |
| `KeyMsg(Enter)` | Search input active | Left: search confirmed | Highlights shown, match counter visible |
| `KeyMsg(Esc)` | Search input active | Search cancelled | Header reverts, no highlights |
| `KeyMsg(Esc)` | Search results active | Search cleared | Highlights removed, normal mode |
| `KeyMsg(n)` | Left, search results | Next match | Cursor jumps, counter updates |
| `KeyMsg(N)` | Left, search results | Previous match | Cursor jumps, counter updates |
| `KeyMsg(Esc)` | Right, filter active | Filter cleared | All rows restored |
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
| `/` | Search left-col text / filter right-col list | Context-dependent (see section 17) |
| `n` | Next search match | Left column, search active |
| `N` | Previous search match | Left column, search active |
| `esc` | Clear search / go back | See section 17.5 |
| `?` | Help | Any |
| `ctrl+r` | Refresh all (clears search) | Any |
| `pgup`/`ctrl+u` | Page up in focused column | Either column |
| `pgdn`/`ctrl+d` | Page down in focused column | Either column |
| `ctrl+c` | Force quit | Any |

### Keys NOT Available

| Key | Why Not |
|-----|---------|
| `:` (command) | Already available globally (unchanged) |
| `d` (detail) | Already in detail view |
| `S`/`A` (sort) | Not a sortable list |

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
| Right col header (`RELATED`) | `#565f89` | -- | -- | Dim label, not selectable |
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
| Search match (non-current) | `#1a1b26` | `#e0af68` | -- | Existing: amber bg from QA-26 |
| Search match (current) | `#1a1b26` | `#ff9e64` | Bold | Existing: orange bg from QA-26 |
| Match indicator text | `#565f89` | -- | Dim | "[3/17 matches]" at frame bottom |
| Search/filter input (header) | `#e0af68` | -- | Bold | Existing: Header filter |

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

### State Preservation on Layout Transition

When `WindowSizeMsg` crosses the 100-col threshold (switching between
stacked and two-column layouts), all state is preserved: cursor
positions for both columns, search highlights and match index, filter
query and narrowed list, focused column, scroll offsets. The focused
column remains focused in both layouts. In stacked mode, Tab switches
between the detail section (top) and related section (bottom) -- the
same semantics, different visual arrangement.

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
| <h/l>    Switch col        </>      Filter list <g>       Top                                           |
| <c>      Copy value                             <G>       Bottom                                       |
| <y>      YAML view                              <pgdn>    Page down                                    |
| </>      Search                                  <pgup>    Page up                                      |
| <n>      Next match                                                                                    |
| <N>      Prev match                                                                                    |
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
| ~~IamInstanceProfile.Arn~~ | ~~`arn:aws:iam::*`~~ | ~~IAM Role detail~~ — **REMOVED**: instance profile ARNs are not role ARNs; this mapping requires `iam:GetInstanceProfile` and belongs in algorithmic relationships |

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

---

## 17. Search Interaction (Cross-View Search, #89)

The cross-view search component (QA-26) was designed for viewport-based
views (YAML, logs, policy documents). The two-column detail view
introduces a field-list cursor model that requires careful integration.

This section resolves all design questions about how search interacts
with the two-column layout.

### 17.1 Decision: `/` Behavior Depends on Focused Column

`/` is context-sensitive based on which column has focus:

| Focus | `/` behavior | Rationale |
|-------|-------------|-----------|
| Left column | **Text search** (QA-26 style) | The left column displays field keys and values -- the same content as the old viewport-based detail view. Search within field text is useful for finding specific values in large detail views (40+ fields). |
| Right column | **List filter** (QA-11 style) | The right column is a navigable list of resource type names, identical in structure to the main menu. The main menu uses `/` as a filter. Consistency demands the right column follow the same pattern. |

This is the "focused column only" model. The alternative of searching
both columns simultaneously was rejected: it would require a merged
match index across two independent scroll positions, and `n`/`N`
jumping between columns would be disorienting. The focused-column-only
model is simpler and maps to established patterns already in a9s.

The alternative of disabling search entirely in the two-column view
(YAML-view-only) was also considered, but rejected. Users with 40-60
field detail views genuinely need `/` to locate specific values. The
field-list cursor model (j/k one-row-at-a-time) makes scrolling through
many fields tedious without a search shortcut.

### 17.2 Left Column: Text Search

When the left column is focused and the user presses `/`:

1. The header right side changes to `/<cursor>` (amber `#e0af68`, bold)
2. The user types a search query (e.g., "10.0", "vpc", "running")
3. Enter confirms the search
4. All matches in field keys AND values are highlighted:
   - Non-current matches: amber background `#e0af68`, dark fg `#1a1b26`
   - Current match: orange background `#ff9e64`, dark fg `#1a1b26`, bold
5. The match indicator `[1/N matches]` appears at the bottom of the
   left column content area (above the frame bottom border), dim `#565f89`
6. The cursor jumps to the field row containing the first match
7. `n`/`N` advances/retreats to the next/previous match, moving the
   cursor to the field row containing that match

**Adaptation from viewport search:** The key difference from the
viewport-based search (QA-26) is that the cursor jumps to the matched
row rather than scrolling a viewport offset. This is because the left
column is a field list with per-row cursor, not a free-scrolling
viewport. The visual result is the same: the user sees the matched text
centered on screen.

**Match scope:** Search matches span the full rendered text of each
field row, including the key label and value. Searching for "Vpc" would
match both the key "VpcId:" and any value containing "Vpc". This is
case-insensitive, consistent with QA-26-I01.

**Interaction with navigable fields:** Search highlighting coexists
with navigable-field underline styling. When a match falls on a
navigable field's value, the search highlight (amber/orange background)
takes precedence over the underline. The underline reappears when the
search is cleared. When the cursor lands on a row with a navigable
field (via `n`/`N`), Enter still opens the target resource. Search mode
does not change Enter's behavior -- Enter always means "open navigable
field" in the left column. There is no "jump to match" Enter action;
`n`/`N` handles match navigation.

**Word wrap interaction:** Word wrap is always ON in the left column
(section 4.4). Matches on wrapped continuation lines are highlighted
normally. The cursor lands on the field's first line; the entire
field (including wrapped lines) is highlighted with the cursor style.

### 17.3 Right Column: List Filter

When the right column is focused and the user presses `/`:

1. The header right side changes to `/<cursor>` (amber `#e0af68`, bold)
2. The user types a filter term (e.g., "cloud", "sg")
3. Matching happens live as the user types (immediate filtering, no
   Enter required) -- same as main menu filter behavior (QA-11)
4. The right column list narrows to show only resource types whose names
   contain the filter substring (case-insensitive)
5. Dim (unavailable) rows that match the filter ARE shown (still dim)
6. The frame title does NOT change (unlike the resource list which shows
   `(matched/total)`, the right column is too small for that)
7. Esc clears the filter and restores all rows

This is identical to how `/` works in the main menu and resource list.
No matched-text highlighting inside row cells. The filter simply hides
non-matching rows.

**Why filter instead of search?** The right column has at most ~18 rows
(VPC has the most). Highlighting matches within 18 short labels adds
no value -- the user can see all the labels at a glance. Filtering is
more useful: it narrows the list when looking for a specific type.

### 17.4 Switching Columns with Active Search/Filter

When the user has an active search (left) or filter (right) and
switches columns with Tab or h/l:

| From | To | Behavior |
|------|----|----------|
| Left (search active) | Right | Search highlights persist in left column. Right column enters normal mode (no filter). `n`/`N` are inactive until user switches back to left column. |
| Right (filter active) | Left | Filter persists in right column (filtered list stays narrowed). Left column enters normal mode. |
| Left (search active) | Right, then back to left | Search highlights and match position are restored. `n`/`N` resume from where they left off. |
| Right (filter active) | Left, then back to right | Filter is still active. The narrowed list is preserved. |

The search/filter state is per-column and persists across focus switches.
This avoids the confusing "matches disappear" problem when switching
columns: they do not disappear, they simply become non-interactive
while the other column is focused.

**Header state on column switch:** When switching from a column with
active search/filter to a column without, the header right side reverts
to `? for help` (normal state). The search/filter query text is
preserved internally but hidden from the header. When switching back to
the column with active search/filter, the header does NOT re-show the
input -- instead, the highlights/filter remain visible as a reminder
that search/filter is active. To re-enter search input mode, the user
presses `/` again.

### 17.5 Esc Layering in Search/Filter Context

Esc has three possible meanings in the two-column detail view. They are
resolved by priority:

| State | Esc action | Next state |
|-------|-----------|------------|
| Search input active (typing in header) | Cancel search input | Normal mode (no highlights) |
| Search results active (highlights visible) | Clear search highlights | Normal mode |
| Filter active (right column narrowed) | Clear filter, restore all rows | Normal mode |
| Normal mode (no search, no filter) | Go back (pop view stack) | Previous view |

This means:
- If search input is showing: first Esc cancels the input.
- If highlights are visible: first Esc clears them. Second Esc goes back.
- If filter is active: first Esc clears it. Second Esc goes back.

This matches the established pattern from QA-26-G04 (two Esc presses
to exit view when search is active).

### 17.6 `n`/`N` Key Conflict Analysis

In the detail view (two-column), `n` and `N` have no prior bindings:

| Key | Resource list binding | Detail view binding (before) | Detail view binding (now) |
|-----|----------------------|-----------------------------|-----------------------------|
| `n` | -- | -- | Next search match (left col, search active) |
| `N` | Sort by name | -- | Previous search match (left col, search active) |

`N` (uppercase) is bound to "Sort by name" in list views only. It has
never been bound in the detail view. There is no conflict.

When search is NOT active, `n`/`N` are no-ops in the detail view.
They only activate when search results are visible in the left column.

When the right column is focused, `n`/`N` are no-ops regardless of
whether the left column has active search highlights. This prevents
confusion: match navigation only works when the search column (left)
has focus.

### 17.7 `r` Key Conflict Analysis

The physical key `r` is already bound to `keys.Map.Resources` (child-view
trigger, active in resource list views). The detail view introduces a new
`keys.Map.ToggleRelated` binding on the same physical key. There is no
runtime conflict because the two bindings are dispatched in mutually
exclusive view contexts:

| Binding | Struct Field | Physical Key | View Context |
|---------|-------------|-------------|--------------|
| `Resources` | `keys.Map.Resources` | `r` | Resource list view |
| `ToggleRelated` | `keys.Map.ToggleRelated` | `r` | Detail view |

The detail view's `Update()` matches against `ToggleRelated`; the
resource list view's `Update()` matches against `Resources`. Neither
view imports the other's binding.

**CFN Stack Resources special case:** The CFN stack resource type has a
`ChildViewDef` with `Key: "r"` that triggers the stack resources child
view. Because the detail view now uses `r` for
`ToggleRelated`, the `ChildViewDef` is changed to `Key: "R"` (uppercase
shift). A new `keys.Map.StackResources` binding for `R` is added. This
is the only resource type affected -- no other child-view trigger uses
`r`.

**`/` dual binding:** `/` is bound to both `Filter` and `Search` in
`keys.Map`. Implementation must branch on the focused column before key
matching: left column focused dispatches to `Search`, right column
focused dispatches to `Filter`. This is the same pattern used for `c`
(copy) which behaves differently per column.

### 17.8 YAML View (No Change)

YAML view does not show the right column (section 2.6). Search in YAML
view works exactly as specified in QA-26: full-width viewport search
with `/`, highlights, `n`/`N` navigation, match counter. No changes
needed for the two-column feature.

### 17.9 Stacked Layout (< 100 cols)

In stacked mode (section 2.5), the detail fields and related list are
in a single scrollable stream separated by `-- Related ---`.

- When the detail section has focus (top): `/` activates text search
  across the field rows, same as left-column search in two-column mode.
- When the related section has focus (bottom): `/` activates list
  filter, same as right-column filter in two-column mode.

Tab switches between sections. The search/filter state is per-section
and follows the same persistence rules as section 17.4.

### 17.10 Right Column Hidden (`r` toggled off)

When the right column is hidden, the view is a single-column detail
display. `/` activates text search (same as left-column search).
`n`/`N` navigate matches. This is identical to the old viewport-based
detail view behavior from QA-26, except the cursor jumps to matched
rows instead of scrolling a viewport offset.

### 17.11 Wireframe: Search Active in Two-Column View

Left column focused, search for "vpc" confirmed, 3 matches found.
Current match on VpcId value (match 1/3).

```
 a9s v3.28.0  prod:us-east-1                                                                         ? for help
+----------------------------- detail -- i-0abc123 (web-prod) ------------------------------------------------+
| InstanceId:          i-0abc123def456789a                    | [DIM]RELATED[/]                                  |
| InstanceType:        t3.large                               |   Auto Scaling Groups                          |
|[SELECTED] VpcId:               [MATCHCUR]vpc[/]-0aaa111bbb222cc        [/]|   CloudWatch Alarms                            |
| SubnetId:            subnet-0bbb222ccc333dd                 |   CloudFormation Stacks                        |
| SecurityGroups:                                             |   EBS Snapshots                                |
|     - GroupId:       sg-0ccc333ddd444ee                     |   Elastic IPs                                  |
|     - GroupId:       sg-0ddd444eee555ff                     | [DIM]EKS Node Groups[/]                          |
| IamInstanceProfile:                                         |   CloudTrail Events                            |
|     Arn:             arn:aws:iam::role/web                   |                                                |
| ImageId:             ami-0aaa111222333                       |                                                |
| [MATCH]Vpc[/]Config:                                                |                                                |
|     [MATCH]Vpc[/]Id:           vpc-0aaa111bbb222cc                  |                                                |
| ...                                                         |                                                |
| [DIM][1/3 matches][/]                                                   |                                                |
+-------------------------------------------------------------------------------------------------------------+
```

Notes:
- `[MATCHCUR]` = orange bg `#ff9e64`, dark fg, bold (current match)
- `[MATCH]` = amber bg `#e0af68`, dark fg (other matches)
- The cursor (full-row highlight) is on the VpcId field row (match 1)
- Match 2 is on "VpcConfig:" section header (key contains "Vpc")
- Match 3 is on sub-field "VpcId:" under VpcConfig
- `n` would move cursor to VpcConfig row, `N` would wrap to match 3
- Right column is unaffected by left-column search
- Navigable underlines on values are hidden while search highlights
  are active (search bg takes precedence)
- Match indicator `[1/3 matches]` at bottom-left of content area

### 17.12 Wireframe: Filter Active in Right Column

Right column focused, filter for "cloud" active, 3 of 9 types match.

```
 a9s v3.28.0  prod:us-east-1                                                                      /cloud
+----------------------------- detail -- i-0abc123 (web-prod) ------------------------------------------------+
| InstanceId:          i-0abc123def456789a                    | [DIM]RELATED[/]                                  |
| InstanceType:        t3.large                               |[SELECTED] CloudWatch Alarms          [/]       |
| State:               running                                |   CloudFormation Stacks                        |
| VpcId:               [UNDERLINE]vpc-0aaa111bbb222cc[/]      |   CloudTrail Events                            |
| SubnetId:            [UNDERLINE]subnet-0bbb222ccc333dd[/]   |                                                |
| SecurityGroups:                                             |                                                |
|     - GroupId:       [UNDERLINE]sg-0ccc333ddd444ee[/]       |                                                |
|     - GroupId:       [UNDERLINE]sg-0ddd444eee555ff[/]       |                                                |
| ...                                                         |                                                |
+-------------------------------------------------------------------------------------------------------------+
```

Notes:
- Header right shows `/cloud` (amber bold) -- filter input active
- Right column shows only types matching "cloud": CloudWatch Alarms,
  CloudFormation Stacks, CloudTrail Events
- Non-matching types (Target Groups, ASG, EKS, etc.) are hidden
- Left column navigable underlines remain visible (no search active)
- Separator color is accent `#7aa2f7` (right column focused)
- Esc clears the filter and restores all 9 types

---

## 18. Summary of v4.0 to v4.1 Changes

Changes from v4.0 to v4.1, prompted by cross-view search (#89):

1. **`/` is now active in the detail view** -- previously explicitly
   excluded. Behavior depends on focused column (text search vs. list
   filter).
2. **`n`/`N` added** to the detail view key bindings for search match
   navigation. No conflict with existing bindings.
3. **Esc layering** clarified: search input > search results > filter >
   go back. Matches the QA-26-G04 pattern.
4. **Search highlights in left column** coexist with navigable field
   underlines and cursor highlighting. Search bg takes precedence.
5. **Help screen wireframe** updated to include `/` Search, `n` Next,
   `N` Prev in the DETAIL column and `/` Filter in the RELATED column.
6. **State transition table** extended with search/filter messages.
7. **Color table** extended with search match colors (amber/orange bg).
8. **Two new wireframes** added: sections 17.11 (search active) and
   17.12 (right-column filter active).

---

## 19. Summary of v4.1 to v4.2 Changes

Changes from v4.1 to v4.2, prompted by TUI reviewer audit:

1. **`r` key binding clarified** -- the detail view uses a NEW
   `ToggleRelated` binding in `keys.Map`, separate from the existing
   `Resources` binding. Both use physical key `r` but are dispatched in
   mutually exclusive view contexts (section 1, "Key Binding: `r`").
2. **CFN Stack Resources remap made concrete** -- `ChildViewDef` change
   from `Key: "r"` to `Key: "R"`, with a new `keys.Map.StackResources`
   binding (section 1, "Key Binding: `r`").
3. **`r` key conflict analysis** added as section 17.7, parallel to the
   existing `n`/`N` analysis in 17.6. Documents the `Resources` vs.
   `ToggleRelated` dispatch model and the `/` dual-binding note.
4. **Header state on column switch** specified in section 17.4 -- when
   switching from a column with active search/filter to one without,
   the header reverts to `? for help`. Query text preserved internally;
   re-entering input mode requires pressing `/` again.
5. **State preservation on layout transition** added to section 13 --
   when `WindowSizeMsg` crosses the 100-col threshold, all state is
   preserved: cursor positions, search/filter state, focused column,
   scroll offsets.

---

## 20. Summary of v4.2 to v4.3 Changes

Changes from v4.2 to v4.3:

1. **`RELATED` header added to right column** -- dim `#565f89` label as
   the first row of the right column in two-column layout. Not selectable
   (cursor skips it). When Tab moves focus to the right column, cursor
   lands on the first resource type row, not the header (section 5.2).
2. **Row states table updated** -- header state added to section 5.3.
3. **Color table updated** -- `RELATED` header entry added to section 11.
4. **All two-column wireframes updated** -- sections 8.1, 8.2, 8.4, 8.5,
   8.7, 8.8, 8.9, 17.11, 17.12 now show the `RELATED` header as the
   first right-column row.
5. **Stacked mode unchanged** -- the existing `-- Related ---` separator
   already serves as the section header in stacked layout.
