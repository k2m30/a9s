# Related Resources: QA User Stories

Design spec: `docs/design/related-resources.md` v4.3
Architecture: `docs/design/related-views-architecture.md` v2.1
Issue: #64

---

## Tier 1: Common Stories (apply to ALL 66 resource types)

These stories test the two-column detail view infrastructure. They apply
uniformly to every resource type. Test runners should execute them against
a representative sample (at minimum: EC2, Lambda, VPC, SG, RDS, S3, IAM
Role, ECS Service, EKS, CloudWatch Alarm) and then spot-check the rest.

---

### Two-Column Layout

#### Story: Right column visible by default on detail entry
**Given:** the user is on any resource list with at least one resource
**When:** the user presses `d` or `Enter` to open the detail view
**Then:** a two-column layout appears: left column shows detail fields, right column shows related resource types separated by a thin vertical line

**AWS comparison:**
aws ec2 describe-instances --instance-ids i-0abc123
Expected fields visible: left column shows the resource's detail fields; right column shows related resource type names (e.g., Security Groups, VPC, CloudWatch Alarms, CloudTrail Events)

---

#### Story: Right column toggle with `r`
**Given:** the user is in a two-column detail view with the right column visible
**When:** the user presses `r`
**Then:** the right column disappears, the left column expands to fill the full frame width, and navigable field underlines remain visible

**Given:** the user is in a detail view with the right column hidden
**When:** the user presses `r` again
**Then:** the two-column layout is restored with the right column at 32 characters wide, and the separator line reappears

**AWS comparison:**
No AWS CLI equivalent -- this is a TUI navigation feature.

---

#### Story: Toggle state persists across navigation
**Given:** the user has pressed `r` to hide the right column
**When:** the user presses Enter on a navigable field to open a new resource's detail view
**Then:** the new detail view also has the right column hidden

**Given:** the user has the right column visible
**When:** the user navigates to a related resource's detail via Enter
**Then:** the new detail view also has the right column visible

**AWS comparison:**
No AWS CLI equivalent -- persistence of UI state across navigation.

---

#### Story: Column separator changes color with focus
**Given:** the user is in a two-column detail view with the left column focused
**When:** the user looks at the vertical separator between the two columns
**Then:** the separator is rendered in dim color (#414868)

**Given:** the user presses Tab to switch focus to the right column
**When:** the user looks at the vertical separator
**Then:** the separator is rendered in accent color (#7aa2f7)

**AWS comparison:**
No AWS CLI equivalent -- visual focus indicator.

---

#### Story: Frame borders span full width unbroken
**Given:** the user is in a two-column detail view
**When:** the user looks at the top and bottom borders of the frame
**Then:** both borders span the full terminal width without interruption; the column separator appears only in content rows between the side borders

**AWS comparison:**
No AWS CLI equivalent -- frame rendering.

---

#### Story: Right column fixed width of 32 characters
**Given:** the terminal is at least 100 columns wide
**When:** the user opens a detail view with the right column visible
**Then:** the right column occupies exactly 32 characters, the separator occupies 1 character, and the left column fills the remaining width

**AWS comparison:**
No AWS CLI equivalent -- layout geometry.

---

### Focus Switching

#### Story: Tab switches focus between columns
**Given:** the user is in a two-column detail view with the left column focused
**When:** the user presses Tab
**Then:** focus moves to the right column: the cursor appears on the first available (non-dim) row in the right column, and the left column cursor disappears

**Given:** the right column is focused
**When:** the user presses Tab again
**Then:** focus returns to the left column at the previously selected field row

**AWS comparison:**
No AWS CLI equivalent -- TUI navigation.

---

#### Story: h/l switches focus between columns
**Given:** the user is in a two-column detail view with the left column focused
**When:** the user presses `l`
**Then:** focus moves to the right column (same as Tab)

**Given:** the right column is focused
**When:** the user presses `h`
**Then:** focus moves to the left column (same as Tab back)

**Given:** the left column is already focused
**When:** the user presses `h`
**Then:** nothing happens (already on the leftmost column)

**Given:** the right column is already focused
**When:** the user presses `l`
**Then:** nothing happens (already on the rightmost column)

**AWS comparison:**
No AWS CLI equivalent -- vim-style column switching.

---

#### Story: Tab does nothing when right column is hidden
**Given:** the user has hidden the right column with `r`
**When:** the user presses Tab
**Then:** nothing happens; the cursor stays in the left column

**AWS comparison:**
No AWS CLI equivalent.

---

#### Story: Focus restores cursor position when switching back
**Given:** the user is in the left column with the cursor on field row 15
**When:** the user presses Tab to focus the right column, scrolls down to row 4, then presses Tab again
**Then:** the left column cursor returns to field row 15; the right column remembers row 4 for the next Tab press

**AWS comparison:**
No AWS CLI equivalent -- cursor position memory.

---

### Left Column Navigation

#### Story: Cursor moves one row at a time with j/k
**Given:** the user is in the detail view with the left column focused and the cursor on any field row
**When:** the user presses `j` or Down
**Then:** the cursor moves down one row, including section headers, non-navigable fields, and navigable fields alike

**When:** the user presses `k` or Up
**Then:** the cursor moves up one row

**AWS comparison:**
aws ec2 describe-instances --instance-ids i-0abc123
The detail view shows all fields from the describe output. j/k scrolls through them one at a time.

---

#### Story: Jump to top and bottom with g/G
**Given:** the user is in the left column with the cursor somewhere in the middle
**When:** the user presses `g`
**Then:** the cursor jumps to the first field row

**When:** the user presses `G`
**Then:** the cursor jumps to the last field row

**AWS comparison:**
No AWS CLI equivalent -- keyboard navigation.

---

#### Story: Page up/down with pgup/pgdn and ctrl+u/ctrl+d
**Given:** the user is in the left column with many fields (40+)
**When:** the user presses PgDn or Ctrl+D
**Then:** the cursor moves down by approximately one visible page

**When:** the user presses PgUp or Ctrl+U
**Then:** the cursor moves up by approximately one visible page

**AWS comparison:**
No AWS CLI equivalent -- pagination within detail view.

---

#### Story: Word wrap is always on
**Given:** the user is viewing a resource with a field value longer than the left column width (e.g., a long ARN or policy document excerpt)
**When:** the value overflows the column width
**Then:** the value wraps to the next visual line; the wrapped continuation is part of the same field row, not a separate selectable row; the cursor highlight covers all wrapped lines when the field is selected

**AWS comparison:**
aws iam get-role --role-name my-role
Long ARNs and policy documents in the output would exceed column width.

---

### Navigable Fields (Left Column)

#### Story: Navigable field values are underlined in accent color
**Given:** the user is viewing an EC2 instance detail
**When:** the user looks at a field whose value is a known resource ID (e.g., VpcId: vpc-0aaa111bbb222cc)
**Then:** the value portion is rendered with underline styling in accent color (#7aa2f7); the key label is NOT underlined

**Given:** the cursor is on a navigable field
**When:** the cursor highlight covers the row
**Then:** the underline disappears (the full-row selection highlight takes over)

**When:** the cursor moves off the navigable field
**Then:** the underline reappears on that field's value

**AWS comparison:**
aws ec2 describe-instances --instance-ids i-0abc123
Expected navigable fields: VpcId (vpc-*), SubnetId (subnet-*), SecurityGroups[].GroupId (sg-*), ImageId (ami-*), BlockDeviceMappings[].Ebs.VolumeId (vol-*), NetworkInterfaces[].NetworkInterfaceId (eni-*)
Note: IamInstanceProfile.Arn is NOT navigable — instance profile ARNs are not role ARNs, so direct navigation would route to the wrong resource. The EC2-to-Role relationship is algorithmic (requires iam:GetInstanceProfile API call).

---

#### Story: Enter on a navigable field opens target resource detail
**Given:** the user is in the left column with the cursor on a navigable field (e.g., VpcId: vpc-0aaa111)
**When:** the user presses Enter
**Then:** a new detail view is pushed onto the view stack showing the VPC resource's detail, with its own right column of related types

**AWS comparison:**
aws ec2 describe-vpcs --vpc-ids vpc-0aaa111
The detail view for the VPC shows its fields and its own related resource types.

**Demo dependency:** requires EC2 demo fixtures to contain VPC IDs that exist in VPC demo fixtures.

---

#### Story: Enter on a non-navigable field is a no-op
**Given:** the user is in the left column with the cursor on a plain field (e.g., InstanceType: t3.large)
**When:** the user presses Enter
**Then:** nothing happens; no flash message, no error, no navigation

**AWS comparison:**
No AWS CLI equivalent -- the field "t3.large" is not a resource ID.

---

#### Story: Section headers are selectable but not navigable
**Given:** the user is in the left column and the cursor is on a section header (e.g., "Placement:" or "Tags:")
**When:** the user presses Enter
**Then:** nothing happens; section headers are not navigable

**When:** the user presses j/k
**Then:** the cursor moves normally through the section header row

**AWS comparison:**
No AWS CLI equivalent -- section headers group nested fields.

---

#### Story: Array items with navigable values are independently navigable
**Given:** the user is viewing an EC2 detail with multiple SecurityGroups
**When:** the user moves the cursor to the first GroupId (sg-0ccc333)
**Then:** that value is underlined; pressing Enter opens the sg-0ccc333 security group detail

**When:** the user moves the cursor to the second GroupId (sg-0ddd444)
**Then:** that value is also underlined; pressing Enter opens the sg-0ddd444 security group detail

**AWS comparison:**
aws ec2 describe-instances --instance-ids i-0abc123 --query 'Reservations[].Instances[].SecurityGroups[]'
Each security group ID is a separate navigable target.

**Demo dependency:** requires EC2 demo fixtures to contain SG IDs that exist in SG demo fixtures.

---

#### Story: Navigable fields work with right column hidden
**Given:** the user has pressed `r` to hide the right column
**When:** the user presses Enter on a navigable field (e.g., SubnetId: subnet-0bbb222)
**Then:** the target resource's detail view opens, identical to when the right column is visible

**AWS comparison:**
aws ec2 describe-subnets --subnet-ids subnet-0bbb222
Right column visibility does not affect left column Enter behavior.

**Demo dependency:** requires EC2 demo fixtures to contain Subnet IDs that exist in Subnet demo fixtures.

---

### Right Column Content and Display

#### Story: Right column shows reverse and algorithmic relationships only
**Given:** the user is viewing an EC2 instance detail
**When:** the user looks at the right column
**Then:** the right column lists resource types like Target Groups, Auto Scaling Groups, CloudWatch Alarms, EKS Node Groups, CloudFormation Stacks, EBS Snapshots, Elastic IPs, and CloudTrail Events; it does NOT list VPC, Subnet, Security Groups, or AMIs (those are forward relationships visible as navigable fields in the left column)

**AWS comparison:**
There is no single AWS CLI command that shows all reverse relationships. Each would require a separate describe/list call against a different service.

---

#### Story: CloudTrail Events always appears last in right column
**Given:** the user opens any resource's detail view (any of the 66 types)
**When:** the user looks at the right column
**Then:** "CloudTrail Events" appears as the last row, always available (never dim), regardless of which resource type is being viewed

**AWS comparison:**
aws cloudtrail lookup-events --lookup-attributes AttributeKey=ResourceName,AttributeValue={resource-id}
CloudTrail is universal -- every resource type has associated audit events.

---

#### Story: Right column sort order follows priority
**Given:** the user opens a detail view for a resource with multiple related types
**When:** the user looks at the right column ordering
**Then:** P0 relationships appear first (alphabetically within P0), then P1 (alphabetically), then P2 (alphabetically), then CloudTrail Events last

**AWS comparison:**
No AWS CLI equivalent -- display ordering is a UI design choice.

---

#### Story: Relationships with known counts show inline count
**Given:** the user is viewing a resource where some right-column entries have counts (i.e., the checker returned Count >= 0)
**When:** the check completes (instantly for forward, asynchronously for reverse)
**Then:** the row displays the resource type name followed by the count in parentheses, e.g., "Security Groups (3)", "Subnets (6)", "SQS Event Sources (2)"

**Note:** Both forward and reverse relationships can show counts. Forward relationships always have counts (parsed from Fields, zero API calls). Reverse relationships show counts when the checker provides them (e.g., VPC -> Subnets via DescribeSubnets filter, Lambda -> SQS Event Sources via ListEventSourceMappings).

**AWS comparison:**
No AWS CLI equivalent -- counts are derived from parsing the resource's API response or from bounded reverse-lookup API calls.

---

#### Story: Relationships without counts show name only
**Given:** the user is viewing a resource with right-column entries whose checkers returned Count = -1 (no count available)
**When:** the check completes (background API call returns)
**Then:** the row shows only the resource type name without any count, e.g., "CloudWatch Alarms" (not "CloudWatch Alarms (5)")

**Note:** Whether a count is shown depends on the checker's return value, not the relationship category. A reverse checker that cannot cheaply enumerate results returns Count = -1 and the row shows no count. A reverse checker that can cheaply enumerate (e.g., filter by vpc-id) returns Count >= 0 and the row shows the count.

**AWS comparison:**
No AWS CLI equivalent -- checkers decide per-relationship whether to enumerate or just confirm existence.

---

### Right Column Scrolling

#### Story: Right column scrolls independently when focused
**Given:** the right column is focused and has more rows than the visible height (e.g., VPC with ~18 related types in a terminal with 15 visible rows)
**When:** the user presses j/k
**Then:** the right column scrolls independently; the left column stays at its current scroll position

**When:** rows are hidden above or below the visible area
**Then:** a dim scroll indicator appears: "^ N more" at the top or "v N more" at the bottom

**AWS comparison:**
No AWS CLI equivalent -- independent column scrolling.

---

#### Story: Right column g/G jumps to first/last available row
**Given:** the right column is focused with multiple available (non-dim) rows
**When:** the user presses `g`
**Then:** the cursor jumps to the first available row

**When:** the user presses `G`
**Then:** the cursor jumps to the last available row

**AWS comparison:**
No AWS CLI equivalent -- keyboard navigation within the related types list.

---

### Right Column States and Background Checking

#### Story: All right column rows start dim during initial load
**Given:** the user enters a detail view for the first time for a given resource
**When:** the detail view first renders
**Then:** all right-column rows appear in dim text (#565f89); forward relationship rows immediately resolve and light up; reverse/algorithmic rows remain dim until their background checks complete

**AWS comparison:**
No AWS CLI equivalent -- progressive loading indicator.

---

#### Story: Rows light up silently as background checks complete
**Given:** the user is in a detail view with dim right-column rows
**When:** a background availability check for a related type completes successfully (resources found)
**Then:** that row changes from dim to normal text (#c0caf5) without any spinner, flash message, or visual transition

**When:** a background check completes and finds no related resources
**Then:** that row stays dim

**AWS comparison:**
No AWS CLI equivalent -- asynchronous availability probing.

---

#### Story: Cursor skips dim rows in right column
**Given:** the right column is focused and some rows are dim (unavailable or still checking)
**When:** the user presses j/k to move the cursor
**Then:** the cursor skips over dim rows and lands only on available (non-dim) rows

**When:** all right-column rows are dim (all still checking or all unavailable)
**Then:** there is no selectable row; the right column shows only dim text

**AWS comparison:**
No AWS CLI equivalent -- cursor behavior.

---

#### Story: Ctrl+R refreshes detail and re-checks related resources
**Given:** the user is in a detail view with some right-column rows resolved
**When:** the user presses Ctrl+R
**Then:** the detail fields refresh from the API, all right-column rows reset to dim, and all background availability checks restart from scratch

**AWS comparison:**
aws ec2 describe-instances --instance-ids i-0abc123
Ctrl+R re-executes the describe call and re-probes all reverse relationships.

---

#### Story: Results are cached for the session
**Given:** the user viewed the detail for resource X and all checks completed
**When:** the user presses Esc to go back, then re-enters resource X's detail
**Then:** right-column rows show their cached availability instantly (no dim flicker, no re-checking)

**When:** the user presses Ctrl+R in the detail view
**Then:** the cache is cleared for this resource and all checks restart

**AWS comparison:**
No AWS CLI equivalent -- session-scoped caching.

---

#### Story: Cache clears on profile or region switch
**Given:** the user has cached related-resource availability for several resources
**When:** the user switches AWS profile or region
**Then:** the entire related-resource cache is cleared; entering any detail view triggers fresh checks

**AWS comparison:**
aws configure set profile staging
Switching profiles means different resources, so cached availability is stale.

---

#### Story: Background check failure degrades gracefully
**Given:** a background reverse-relationship check encounters an error (e.g., API throttling after retries exhausted)
**When:** the check result returns with an error
**Then:** the affected right-column row stays dim (same visual as "no related resources found"); no error flash message appears; the user can press Ctrl+R to retry later

**AWS comparison:**
No AWS CLI equivalent -- graceful degradation on API errors.

---

### Right Column Enter Behavior

#### Story: Enter on a right-column type with count=1 opens detail directly
**Given:** the right column is focused and the cursor is on a related type showing count 1 (e.g., "VPC (1)")
**When:** the user presses Enter
**Then:** the target resource's detail view is pushed onto the view stack (no intermediate list view)

**AWS comparison:**
aws ec2 describe-vpcs --vpc-ids vpc-0aaa111
When there is exactly one related resource, navigate directly to its detail.

**Demo dependency:** requires demo fixtures to have valid cross-references so the single resource can be fetched.

---

#### Story: Enter on a right-column type with count>1 opens filtered list
**Given:** the right column is focused and the cursor is on a related type showing count greater than 1 (e.g., "Security Groups (3)")
**When:** the user presses Enter
**Then:** a resource list view is pushed showing only the related resources, with the frame title indicating the filter scope (e.g., "sg(3) -- i-0abc123 (web-prod)")

**AWS comparison:**
aws ec2 describe-security-groups --group-ids sg-0ccc333 sg-0ddd444 sg-0eee555
When there are multiple related resources, show them as a filtered list.

**Demo dependency:** requires demo fixtures to have multiple valid cross-references.

---

#### Story: Enter on CloudTrail Events opens pre-filtered search
**Given:** the right column is focused and the cursor is on "CloudTrail Events"
**When:** the user presses Enter
**Then:** the CloudTrail search view opens pre-filtered for the current resource's ID or name

**AWS comparison:**
aws cloudtrail lookup-events --lookup-attributes AttributeKey=ResourceName,AttributeValue=i-0abc123
CloudTrail search scoped to the current resource.

---

#### Story: Enter on a dim right-column row is impossible
**Given:** the right column is focused
**When:** dim (unavailable) rows exist in the list
**Then:** the cursor cannot land on dim rows, so pressing Enter on a dim row is not possible

**AWS comparison:**
No AWS CLI equivalent -- cursor behavior prevents interaction with unavailable types.

---

### Copy Behavior

#### Story: Copy from left column copies field value
**Given:** the left column is focused and the cursor is on a field (e.g., InstanceId: i-0abc123def456789a)
**When:** the user presses `c`
**Then:** the field value "i-0abc123def456789a" is copied to the clipboard; the header shows "Copied!" flash in green (#9ece6a) for approximately 2 seconds

**AWS comparison:**
No AWS CLI equivalent -- clipboard integration.

---

#### Story: Copy from right column copies resource type name
**Given:** the right column is focused and the cursor is on "Auto Scaling Groups"
**When:** the user presses `c`
**Then:** the text "Auto Scaling Groups" is copied to the clipboard; the header shows "Copied!" flash

**AWS comparison:**
No AWS CLI equivalent -- clipboard integration.

---

### Navigation Chaining and View Stack

#### Story: Chained navigation through forward fields
**Given:** the user is in an EC2 instance detail view
**When:** the user presses Enter on VpcId to open VPC detail, then in VPC detail presses Enter on a navigable field (if any), continuing to chain
**Then:** each Enter pushes a new detail view onto the stack; Esc at each level pops back one view; the full chain unwinds correctly

**AWS comparison:**
aws ec2 describe-instances --instance-ids i-0abc123  (shows VpcId)
aws ec2 describe-vpcs --vpc-ids vpc-0aaa111  (shows VPC fields)
Each navigation step is equivalent to a separate describe command.

**Demo dependency:** requires EC2 fixtures to reference VPC IDs that exist, and VPC fixtures to contain navigable field values that also exist as fixtures.

---

#### Story: Chained navigation through right column
**Given:** the user is in an EC2 detail view
**When:** the user presses Tab to focus the right column, selects "CloudWatch Alarms", and presses Enter
**Then:** a filtered alarm list (or alarm detail if count=1) is pushed onto the view stack

**When:** in the alarm detail, the user again navigates to a related resource via either column
**Then:** that resource's detail is pushed onto the stack; Esc unwinds the entire chain step by step

**AWS comparison:**
aws cloudwatch describe-alarms --alarm-names ...
aws ec2 describe-instances --instance-ids ...
Navigation chains through multiple resource types.

---

#### Story: Mixed navigation between left and right columns
**Given:** the user is in an EC2 detail
**When:** the user presses Enter on sg-0ccc333 (left column, navigable field) to open the SG detail
**Then:** in the SG detail, the right column shows SG's reverse relationships (EC2 Instances, RDS Instances, ELBs, etc.)

**When:** the user presses Tab, selects "EC2 Instances", and presses Enter
**Then:** a filtered EC2 list is pushed (instances using this SG)

**AWS comparison:**
aws ec2 describe-security-groups --group-ids sg-0ccc333
aws ec2 describe-instances --filters Name=instance.group-id,Values=sg-0ccc333
Mixed left-column (forward) and right-column (reverse) navigation.

**Demo dependency:** requires SG fixtures to cross-reference EC2 fixtures, and EC2 fixtures to cross-reference SG fixtures.

---

#### Story: Depth indicator shows when stack exceeds 4
**Given:** the user has navigated through 5 or more levels (e.g., EC2 detail -> VPC detail -> Subnet list -> Subnet detail -> ENI detail)
**When:** the user looks at the header
**Then:** the version number is replaced by `[5]` (or whatever the current depth), e.g., "a9s [5]  prod:us-east-1"

**When:** the user presses Esc to pop back to depth 4
**Then:** the header reverts to showing the version number, e.g., "a9s v3.28.0  prod:us-east-1"

**AWS comparison:**
No AWS CLI equivalent -- view stack depth indicator.

---

#### Story: Esc from detail view goes back to previous view
**Given:** the user is in a detail view (opened from a resource list)
**When:** the user presses Esc (with no search or filter active)
**Then:** the view pops back to the resource list that preceded it, with the cursor on the same resource

**AWS comparison:**
No AWS CLI equivalent -- view stack navigation.

---

### Search and Filter

#### Story: `/` in left column activates text search
**Given:** the left column is focused in the detail view
**When:** the user presses `/`
**Then:** the header right side changes to `/<cursor>` in amber bold (#e0af68)

**When:** the user types "vpc" and presses Enter
**Then:** all occurrences of "vpc" in field keys and values are highlighted: the current match in orange background (#ff9e64, bold), other matches in amber background (#e0af68); the cursor jumps to the first match; a match indicator like `[1/3 matches]` appears at the bottom of the left column content area in dim text

**AWS comparison:**
No AWS CLI equivalent -- text search within detail output. Comparable to piping `aws ec2 describe-instances` output through `grep -i vpc`.

---

#### Story: n/N navigates between search matches
**Given:** a search is active in the left column with 3 matches
**When:** the user presses `n`
**Then:** the cursor jumps to the next match; the match indicator updates to `[2/3 matches]`; the new current match is highlighted in orange, the previous becomes amber

**When:** the user presses `N`
**Then:** the cursor jumps to the previous match; the match indicator updates accordingly

**When:** the user presses `n` on the last match
**Then:** the cursor wraps to the first match

**AWS comparison:**
No AWS CLI equivalent -- match navigation.

---

#### Story: Search highlights coexist with navigable field underlines
**Given:** a search for "vpc" is active and a match falls on a navigable field value (e.g., VpcId: vpc-0aaa111)
**When:** the user views the field
**Then:** the search highlight (amber/orange background) takes precedence over the navigable underline

**When:** the search is cleared (Esc)
**Then:** the navigable underline reappears on that field's value

**Given:** the cursor lands on a navigable field via `n`/`N` search navigation
**When:** the user presses Enter
**Then:** the target resource's detail opens (Enter always means "open navigable field" regardless of search state)

**AWS comparison:**
No AWS CLI equivalent -- interaction between search and navigation.

---

#### Story: `/` in right column activates list filter
**Given:** the right column is focused in the detail view
**When:** the user presses `/`
**Then:** the header right side changes to `/<cursor>` in amber bold

**When:** the user types "cloud"
**Then:** the right column immediately narrows to show only resource types containing "cloud" (e.g., CloudWatch Alarms, CloudFormation Stacks, CloudTrail Events); non-matching types are hidden; dim rows that match the filter are shown but remain dim

**When:** the user presses Esc
**Then:** the filter clears and all rows are restored

**AWS comparison:**
No AWS CLI equivalent -- filtering a list of related resource types.

---

#### Story: Search/filter state persists across column switches
**Given:** the user has an active search in the left column (highlights visible)
**When:** the user presses Tab to switch to the right column
**Then:** the search highlights remain visible in the left column; `n`/`N` are inactive; the header reverts to "? for help"

**When:** the user presses Tab again to return to the left column
**Then:** the search highlights are still there; `n`/`N` resume working; the header does NOT re-show the search input

**Given:** the user has an active filter in the right column (list narrowed)
**When:** the user presses Tab to switch to the left column
**Then:** the filtered right column list stays narrowed; the header reverts to "? for help"

**When:** the user presses Tab back to the right column
**Then:** the filter is still active; the narrowed list is preserved

**AWS comparison:**
No AWS CLI equivalent -- per-column state persistence.

---

#### Story: Header state reverts when switching to column without search
**Given:** the left column has an active search and the header shows "? for help" (search was confirmed, not in input mode)
**When:** the user switches to the right column which has no filter
**Then:** the header continues to show "? for help"

**Given:** the right column has an active filter and the header shows "/cloud"
**When:** the user switches to the left column which has no search
**Then:** the header reverts to "? for help" (the filter input text is preserved internally but hidden)

**AWS comparison:**
No AWS CLI equivalent -- header context sensitivity.

---

#### Story: Esc layering with search and filter
**Given:** the user is typing a search query in the left column (search input active)
**When:** the user presses Esc
**Then:** the search input is cancelled; no highlights appear; the header reverts to "? for help"

**Given:** search results are visible (highlights shown, not in input mode)
**When:** the user presses Esc
**Then:** the search highlights are cleared; the view returns to normal mode

**When:** the user presses Esc again
**Then:** the detail view pops back to the previous view (standard back navigation)

**Given:** the right column is focused with an active filter
**When:** the user presses Esc
**Then:** the filter clears and all rows are restored

**When:** the user presses Esc again
**Then:** the detail view pops back to the previous view

**AWS comparison:**
No AWS CLI equivalent -- multi-layered escape behavior.

---

### YAML View Interaction

#### Story: YAML view shows no right column
**Given:** the user is in a two-column detail view
**When:** the user presses `y` to switch to YAML view
**Then:** the YAML view appears at full width with no right column, no separator, and no related resource types visible

**AWS comparison:**
aws ec2 describe-instances --instance-ids i-0abc123 --output yaml
YAML view shows the raw resource data without the related-resources column.

---

#### Story: `r` in YAML view does nothing
**Given:** the user is in the YAML view
**When:** the user presses `r`
**Then:** nothing happens; `r` has no effect in YAML view (the toggle is only meaningful in the detail view)

**AWS comparison:**
No AWS CLI equivalent -- YAML view ignores the column toggle.

---

### Responsive Layout

#### Story: Stacked layout below 100 columns
**Given:** the terminal is between 60 and 99 columns wide
**When:** the user opens a detail view
**Then:** the layout is stacked: detail fields appear on top, a dim separator line "-- Related ---" divides them from the related types list below; both sections are in a single scrollable stream

**When:** the user presses Tab
**Then:** focus switches between the detail section (top) and the related section (bottom)

**AWS comparison:**
No AWS CLI equivalent -- responsive layout for narrow terminals.

---

#### Story: Side-by-side layout at 100+ columns
**Given:** the terminal is 100 or more columns wide
**When:** the user opens a detail view
**Then:** the layout is two columns side by side: left column fills remaining width, right column is 32 characters wide, separated by a thin vertical line

**AWS comparison:**
No AWS CLI equivalent -- responsive layout for wide terminals.

---

#### Story: State preserved when crossing the 100-column threshold
**Given:** the user is in a two-column detail view at 120 columns wide with the cursor on field 12, a search active with 3 matches, and the right column filter narrowed to "cloud"
**When:** the terminal is resized to 90 columns (below the 100-col threshold)
**Then:** the layout switches to stacked mode; the cursor remains on field 12; the search highlights and match index are preserved; the filter on the related section stays active; the focused section remains focused

**When:** the terminal is resized back to 120 columns
**Then:** the layout switches back to two-column mode; all state is preserved identically

**AWS comparison:**
No AWS CLI equivalent -- state preservation across layout transitions.

---

### Help Screen

#### Story: Help screen shows two-column detail key bindings
**Given:** the user is in a two-column detail view
**When:** the user presses `?`
**Then:** the help screen appears with four columns: DETAIL, RELATED, NAVIGATION, HOTKEYS; the DETAIL column shows Enter (Open link), Esc (Go back), h/l (Switch col), c (Copy value), y (YAML view), / (Search), n (Next match), N (Prev match); the RELATED column shows Tab (Switch col), r (Toggle col), / (Filter list)

**When:** the user presses any key
**Then:** the help screen closes and the detail view is restored

**AWS comparison:**
No AWS CLI equivalent -- help overlay.

---

### Concurrency and Rate Limiting

#### Story: Maximum 4 concurrent background checks
**Given:** the user enters a detail view for a resource with 12 reverse/algorithmic related types
**When:** the background checks begin
**Then:** at most 4 checks run concurrently; as each completes, the next queued check starts immediately; all 12 complete progressively

**AWS comparison:**
No AWS CLI equivalent -- internal rate limiting for API calls.

---

#### Story: Background checks have per-check timeout
**Given:** a background reverse-relationship check is running
**When:** the check takes longer than 10 seconds
**Then:** the check times out; the affected row stays dim; no error is shown to the user

**AWS comparison:**
No AWS CLI equivalent -- timeout protection for slow API calls.

---

### CFN Stack Resources Key Remap

#### Story: CloudFormation stack resources triggered by `R` (uppercase)
**Given:** the user is on a CloudFormation stack in the resource list
**When:** the user presses `R` (uppercase)
**Then:** the stack resources child view opens (same behavior as before, but now with uppercase R instead of lowercase r)

**When:** the user presses `r` (lowercase) in the CFN stack detail view
**Then:** the right column toggles on/off (related resources toggle, not stack resources)

**AWS comparison:**
aws cloudformation list-stack-resources --stack-name my-stack
The `R` key triggers the stack resources drill-down; `r` toggles the related column.

---

---

## Tier 2: Resource-Specific Stories

These stories cover unique algorithmic relationships, multi-hop lookups,
and naming-convention-based connections that require resource-specific
verification. Only resources with non-trivial logic are covered here.

---

## Lambda

### Story: CW Log Group via naming convention
**Given:** the user opens the detail view for a Lambda function named "process-orders"
**When:** the right column availability checks complete
**Then:** "CW Log Group" appears as an available (non-dim) row in the right column if a log group named `/aws/lambda/process-orders` exists

**When:** the user navigates to the CW Log Group via the right column
**Then:** the log group detail opens showing the `/aws/lambda/process-orders` log group

**AWS comparison:**
aws logs describe-log-groups --log-group-name-prefix /aws/lambda/process-orders
Lambda log groups follow the naming convention `/aws/lambda/{function-name}`.

**Demo dependency:** requires Lambda demo fixture with name "process-orders" (or similar) and a matching CloudWatch Log Group fixture with the name `/aws/lambda/process-orders`.

---

### Story: Lambda LoggingConfig override
**Given:** the user opens the detail view for a Lambda function that has a custom `LoggingConfig.LogGroup` set (overriding the default `/aws/lambda/{name}` convention)
**When:** the right column resolves the CW Log Group relationship
**Then:** the navigated log group matches the custom `LoggingConfig.LogGroup` value, not the default naming convention

**AWS comparison:**
aws lambda get-function-configuration --function-name my-func --query 'LoggingConfig.LogGroup'
If LoggingConfig.LogGroup is set, it overrides the default `/aws/lambda/{name}` convention.

**Demo dependency:** requires a Lambda demo fixture with a custom LoggingConfig.LogGroup value and a corresponding log group fixture.

---

### Story: SQS Event Source Mappings
**Given:** the user opens the detail view for a Lambda function that polls SQS queues
**When:** the right column resolves "SQS Event Sources"
**Then:** the row appears as available, showing the count of event source mappings (e.g., "SQS Event Sources (2)")

**When:** the user selects "SQS Event Sources (2)" and presses Enter
**Then:** a filtered SQS list opens showing the two queues this Lambda polls

**AWS comparison:**
aws lambda list-event-source-mappings --function-name process-orders
Returns SQS queue ARNs that trigger this Lambda.

**Demo dependency:** requires Lambda fixtures to have event source mappings pointing to SQS fixtures that exist.

---

### Story: Lambda execution role as forward navigable field
**Given:** the user is viewing a Lambda function detail
**When:** the user looks at the "Role:" field
**Then:** the Role ARN value is underlined in accent color (navigable)

**When:** the user presses Enter on the Role field
**Then:** the IAM Role detail view opens for that role, showing the role's attached policies in its own right column

**AWS comparison:**
aws lambda get-function-configuration --function-name process-orders --query 'Role'
aws iam get-role --role-name lambda-execution-role
Navigation from Lambda to its execution role's detail.

**Demo dependency:** requires Lambda fixtures to reference IAM Role ARNs that exist in Role fixtures.

---

## EC2

### Story: EBS Snapshots via attached volumes (multi-hop)
**Given:** the user opens the detail view for an EC2 instance with attached EBS volumes
**When:** the right column resolves "EBS Snapshots"
**Then:** the row becomes available if any snapshots exist for the attached volumes

**When:** the user navigates to EBS Snapshots via the right column
**Then:** a list of snapshots is shown, filtered to those created from the instance's attached volume IDs

**AWS comparison:**
aws ec2 describe-instances --instance-ids i-0abc123 --query 'Reservations[].Instances[].BlockDeviceMappings[].Ebs.VolumeId'
aws ec2 describe-snapshots --filters Name=volume-id,Values=vol-xxx,vol-yyy
Two-step lookup: instance -> volume IDs -> snapshots of those volumes.

**Demo dependency:** requires EC2 fixtures with volume IDs that have corresponding snapshot fixtures.

---

### Story: Elastic IP association check
**Given:** the user opens the detail view for an EC2 instance
**When:** the right column resolves "Elastic IPs"
**Then:** the row becomes available if an Elastic IP is associated with this instance; it stays dim if the instance only has an auto-assigned public IP

**AWS comparison:**
aws ec2 describe-addresses --filters Name=instance-id,Values=i-0abc123
Checks whether the instance's public IP is an Elastic IP or auto-assigned.

**Demo dependency:** requires at least one EC2 fixture to have an associated Elastic IP fixture.

---

### Story: Auto Scaling Group reverse lookup
**Given:** the user opens the detail view for an EC2 instance managed by an ASG
**When:** the right column resolves "Auto Scaling Groups"
**Then:** the row becomes available

**When:** the user navigates to it
**Then:** the ASG detail opens showing the ASG that manages this instance

**AWS comparison:**
aws autoscaling describe-auto-scaling-instances --instance-ids i-0abc123
Returns the ASG name directly for a managed instance.

**Demo dependency:** requires at least one EC2 fixture whose instance ID appears in an ASG fixture's instance list.

---

### Story: EKS Node Group detection via tags
**Given:** the user opens the detail view for an EC2 instance that is an EKS worker node
**When:** the right column resolves "EKS Node Groups"
**Then:** the row becomes available if the instance has `eks:nodegroup-name` and `eks:cluster-name` tags

**Given:** the instance does not have EKS tags
**Then:** the "EKS Node Groups" row stays dim

**AWS comparison:**
aws ec2 describe-instances --instance-ids i-0abc123 --query 'Reservations[].Instances[].Tags'
Check for eks:nodegroup-name and eks:cluster-name tags.

**Demo dependency:** requires at least one EC2 fixture with EKS node group tags matching an existing EKS Node Group fixture.

---

## ECS Service

### Story: Load Balancer via target group chain (multi-hop)
**Given:** the user opens the detail view for an ECS service with a load balancer configuration
**When:** the right column resolves "Load Balancers"
**Then:** the row becomes available if the multi-hop lookup succeeds: service -> targetGroupArn (from `loadBalancers[]`) -> DescribeTargetGroups -> LoadBalancerArns[]

**When:** the user navigates to Load Balancers
**Then:** the ELB detail opens for the load balancer fronting this service

**AWS comparison:**
aws ecs describe-services --cluster my-cluster --services my-svc --query 'services[].loadBalancers[].targetGroupArn'
aws elbv2 describe-target-groups --target-group-arns arn:aws:...
aws elbv2 describe-load-balancers --load-balancer-arns ...
Three-step chain: ECS Service -> TG ARN -> TG -> ELB.

**Demo dependency:** requires ECS Service fixtures to reference Target Group ARNs that exist in TG fixtures, and TG fixtures to reference ELB ARNs that exist in ELB fixtures.

---

### Story: CW Log Group via task definition (multi-hop)
**Given:** the user opens the detail view for an ECS service
**When:** the right column resolves "CloudWatch Log Groups"
**Then:** the row becomes available if the multi-hop lookup succeeds: service -> taskDefinition ARN -> DescribeTaskDefinition -> containerDefinitions[].logConfiguration.options["awslogs-group"]

**AWS comparison:**
aws ecs describe-services --cluster my-cluster --services my-svc --query 'services[].taskDefinition'
aws ecs describe-task-definition --task-definition arn:aws:... --query 'taskDefinition.containerDefinitions[].logConfiguration.options'
Two-step lookup to find the log group from the task definition.

**Demo dependency:** requires ECS Service fixtures to reference task definitions with log configuration pointing to existing log group fixtures.

---

### Story: ECR Repository via task definition image URI (multi-hop)
**Given:** the user opens the detail view for an ECS service
**When:** the right column resolves "ECR Repositories"
**Then:** the row becomes available if the task definition's container image URI matches an ECR repository pattern ({account}.dkr.ecr.{region}.amazonaws.com/{repo}:{tag})

**AWS comparison:**
aws ecs describe-task-definition --task-definition arn:aws:... --query 'taskDefinition.containerDefinitions[].image'
Parse the image URI to extract the ECR repository name.

**Demo dependency:** requires ECS Service fixtures to reference task definitions with ECR image URIs matching existing ECR fixtures.

---

## IAM Role

### Story: Lambda functions using this role (reverse)
**Given:** the user opens the detail view for an IAM Role
**When:** the right column resolves "Lambda Functions"
**Then:** the row becomes available if any Lambda functions have their `Role` field matching this role's ARN

**AWS comparison:**
aws lambda list-functions --query 'Functions[?Role==`arn:aws:iam::123456789012:role/my-role`]'
Reverse lookup: find all Lambdas using this role.

**Demo dependency:** requires Lambda fixtures to reference Role ARNs that match IAM Role fixtures.

---

### Story: EC2 instances using this role (reverse via instance profiles)
**Given:** the user opens the detail view for an IAM Role
**When:** the right column resolves "EC2 Instances"
**Then:** the row becomes available if any EC2 instances have an instance profile associated with this role

**AWS comparison:**
aws iam list-instance-profiles-for-role --role-name my-role
aws ec2 describe-instances --filters Name=iam-instance-profile.arn,Values=arn:aws:iam::...
Two-step: role -> instance profiles -> instances using those profiles.

**Demo dependency:** requires EC2 fixtures with instance profile ARNs that map back to IAM Role fixtures.

---

### Story: Trust policy principal navigation
**Given:** the user opens the detail view for an IAM Role
**When:** the right column resolves "Trusted Principals"
**Then:** the analysis of the trust policy shows which AWS services (e.g., lambda.amazonaws.com, ecs-tasks.amazonaws.com) and which accounts/roles can assume this role

**AWS comparison:**
aws iam get-role --role-name my-role --query 'Role.AssumeRolePolicyDocument'
Parse the trust policy to identify who can assume this role.

---

## S3

### Story: CloudFront distributions using this bucket as origin (reverse)
**Given:** the user opens the detail view for an S3 bucket
**When:** the right column resolves "CloudFront Distributions"
**Then:** the row becomes available if any CloudFront distribution has an origin with DomainName matching this bucket (e.g., "{bucket}.s3.amazonaws.com" or "{bucket}.s3.{region}.amazonaws.com")

**AWS comparison:**
aws cloudfront list-distributions --query 'DistributionList.Items[].Origins.Items[?contains(DomainName, `my-bucket`)]'
Reverse lookup: find distributions whose origins reference this bucket.

**Demo dependency:** requires S3 fixtures whose bucket names appear in CloudFront fixture origin configurations.

---

### Story: Event notifications (Lambda, SQS, SNS triggers)
**Given:** the user opens the detail view for an S3 bucket with event notifications configured
**When:** the right column resolves event notification relationships
**Then:** the rows for Lambda Functions, SQS Queues, or SNS Topics become available if the bucket's notification configuration references them

**AWS comparison:**
aws s3api get-bucket-notification-configuration --bucket my-bucket
Returns LambdaFunctionConfigurations[], QueueConfigurations[], TopicConfigurations[] with target ARNs.

**Demo dependency:** requires S3 fixtures with notification configurations referencing existing Lambda/SQS/SNS fixtures.

---

## Route 53

### Story: DNS records pointing to ELBs (alias record parsing)
**Given:** the user opens the detail view for a Route 53 hosted zone
**When:** the right column resolves "Load Balancers"
**Then:** the row becomes available if any alias records in this zone have a HostedZoneId matching known ELB hosted zone IDs (e.g., Z35SXDOTRQ7X7K for us-east-1 ALB) and a DNSName matching an ELB DNS name

**AWS comparison:**
aws route53 list-resource-record-sets --hosted-zone-id Z123456
Parse alias records to find those pointing to ELB endpoints.

**Demo dependency:** requires R53 fixtures with alias records that reference ELB DNS names present in ELB fixtures.

---

### Story: DNS records pointing to CloudFront distributions
**Given:** the user opens the detail view for a Route 53 hosted zone
**When:** the right column resolves "CloudFront Distributions"
**Then:** the row becomes available if any alias records have HostedZoneId=Z2FDTNDATAQYW2 (CloudFront's fixed hosted zone ID)

**AWS comparison:**
aws route53 list-resource-record-sets --hosted-zone-id Z123456
Alias records with CloudFront's fixed hosted zone ID indicate CF distributions.

**Demo dependency:** requires R53 fixtures with alias records pointing to CloudFront's hosted zone ID and matching CloudFront fixtures.

---

### Story: DNS records pointing to S3 website endpoints
**Given:** the user opens the detail view for a Route 53 hosted zone
**When:** the right column resolves "S3 Buckets"
**Then:** the row becomes available if alias records or CNAME records point to S3 website endpoints ({bucket}.s3-website-{region}.amazonaws.com)

**AWS comparison:**
aws route53 list-resource-record-sets --hosted-zone-id Z123456
Parse records for S3 website endpoint patterns.

**Demo dependency:** requires R53 fixtures with records pointing to S3 website endpoints matching S3 fixtures.

---

## CloudTrail Events

### Story: Navigate to affected resource from event
**Given:** the user opens the detail view for a CloudTrail event (e.g., TerminateInstances)
**When:** the right column resolves "Affected Resource"
**Then:** the row becomes available; the relationship is derived from parsing `Resources[].ResourceType` and `Resources[].ResourceName` (or `requestParameters` when Resources[] is empty) and mapping to an a9s resource type

**When:** the user navigates to the affected resource
**Then:** the detail view for the affected resource opens (e.g., the EC2 instance that was terminated)

**AWS comparison:**
aws cloudtrail lookup-events --lookup-attributes AttributeKey=EventName,AttributeValue=TerminateInstances
The Resources[] field in the event identifies what was affected.

**Demo dependency:** requires CloudTrail event fixtures with Resources[] fields that reference existing resource fixtures.

---

### Story: Navigate to IAM role from event's userIdentity
**Given:** the user opens the detail view for a CloudTrail event where the actor assumed a role
**When:** the right column resolves "IAM Role"
**Then:** the row becomes available; the role ARN is parsed from `userIdentity.sessionContext.sessionIssuer.arn`

**When:** the user navigates to the IAM Role
**Then:** the role's detail view opens

**AWS comparison:**
aws cloudtrail lookup-events ... --query 'Events[].CloudTrailEvent' | jq '.userIdentity.sessionContext.sessionIssuer.arn'
Parse the acting role from the event's identity chain.

**Demo dependency:** requires CloudTrail event fixtures with userIdentity.sessionContext.sessionIssuer.arn matching existing IAM Role fixtures.

---

## CloudWatch Alarm

### Story: Navigate to monitored resource from alarm dimensions
**Given:** the user opens the detail view for a CloudWatch alarm with an InstanceId dimension
**When:** the right column resolves "Monitored Resource"
**Then:** the row becomes available; the alarm's Dimensions[] are parsed to identify the resource type (InstanceId -> EC2) and resource ID

**When:** the user navigates to the monitored resource
**Then:** the EC2 instance detail opens

**AWS comparison:**
aws cloudwatch describe-alarms --alarm-names my-alarm --query 'MetricAlarms[].Dimensions'
Dimension name determines resource type: InstanceId=EC2, DBInstanceIdentifier=RDS, FunctionName=Lambda, etc.

**Demo dependency:** requires alarm fixtures with Dimensions[] that reference existing resource fixtures (e.g., EC2 instance IDs, RDS identifiers).

---

### Story: Dimension-to-resource mapping covers major types
**Given:** alarms exist with various dimension types
**When:** the right column resolves the monitored resource for each
**Then:** the following dimension mappings are supported:
- InstanceId -> EC2 Instances
- DBInstanceIdentifier -> DB Instances (RDS)
- FunctionName -> Lambda Functions
- LoadBalancer -> Load Balancers
- TargetGroup -> Target Groups
- TableName -> DynamoDB Tables
- QueueName -> SQS Queues
- CacheClusterId -> ElastiCache Redis
- AutoScalingGroupName -> Auto Scaling Groups
- FileSystemId -> EFS
- NatGatewayId -> NAT Gateways

**AWS comparison:**
aws cloudwatch describe-alarms --query 'MetricAlarms[].Dimensions'
Each alarm dimension type maps to a specific AWS resource type.

**Demo dependency:** requires alarm fixtures with multiple dimension types, each referencing existing resources of the corresponding type.

---

## Security Group

### Story: ENI-based universal reverse lookup
**Given:** the user opens the detail view for a Security Group
**When:** the right column resolves "Network Interfaces"
**Then:** the row becomes available showing ENIs attached to this SG; the ENI lookup is the universal reverse relationship that covers all VPC resources using this SG (EC2, RDS, ELB, Lambda, ECS, EKS, VPC Endpoints)

**AWS comparison:**
aws ec2 describe-network-interfaces --filters Name=group-id,Values=sg-0abc123
Single API call returns all ENIs using this SG, regardless of owning service.

**Demo dependency:** requires SG fixtures whose IDs appear in ENI fixtures' security group lists.

---

### Story: Cross-referenced security groups
**Given:** the user opens the detail view for a Security Group
**When:** the right column resolves "Security Groups" (self-referencing type)
**Then:** the row becomes available if other SGs have rules referencing this SG as a source or destination

**AWS comparison:**
aws ec2 describe-security-group-rules --filters Name=referenced-group-info.group-id,Values=sg-0abc123
Finds rules in OTHER SGs that reference this SG.

**Demo dependency:** requires SG fixtures with rules that reference other SG fixture IDs.

---

## VPC

### Story: VPC has the most reverse relationships
**Given:** the user opens the detail view for a VPC
**When:** the right column fully resolves
**Then:** the right column shows up to ~18 related resource types including: EC2 Instances, Subnets, Security Groups, Route Tables, NAT Gateways, Internet Gateways, VPC Endpoints, Transit Gateways, Load Balancers, Lambda Functions, EKS Clusters, DB Instances, ElastiCache, CloudTrail Events (and potentially more)

**When:** there are more rows than fit in the visible height
**Then:** a dim scroll indicator appears at the bottom (e.g., "v 2 more")

**AWS comparison:**
No single AWS CLI command shows all resources in a VPC. Each type requires a separate describe call with vpc-id filter.

**Demo dependency:** requires VPC fixture IDs to be referenced across EC2, Subnet, SG, NAT, IGW, ELB, Lambda, EKS, and RDS fixtures.

---

## EKS

### Story: CW Log Group via naming convention
**Given:** the user opens the detail view for an EKS cluster named "production"
**When:** the right column resolves "CloudWatch Log Groups"
**Then:** the row becomes available if a log group named `/aws/eks/production/cluster` exists and the cluster has control plane logging enabled

**AWS comparison:**
aws eks describe-cluster --name production --query 'cluster.logging'
aws logs describe-log-groups --log-group-name-prefix /aws/eks/production/cluster
EKS control plane logs follow the naming convention `/aws/eks/{cluster-name}/cluster`.

**Demo dependency:** requires EKS fixtures with cluster names that match CloudWatch Log Group fixtures following the `/aws/eks/{name}/cluster` convention.

---

### Story: Node Groups reverse lookup
**Given:** the user opens the detail view for an EKS cluster
**When:** the right column resolves "EKS Node Groups"
**Then:** the row becomes available showing node groups belonging to this cluster

**AWS comparison:**
aws eks list-nodegroups --cluster-name production
Lists all node groups associated with the cluster.

**Demo dependency:** requires EKS fixtures whose cluster names match Node Group fixtures' cluster names.

---

## CloudFront

### Story: Origin parsing for S3 and ELB
**Given:** the user opens the detail view for a CloudFront distribution
**When:** the right column resolves origin-based relationships
**Then:** "S3 Buckets" becomes available if any origin's DomainName matches an S3 pattern ({bucket}.s3.amazonaws.com or {bucket}.s3.{region}.amazonaws.com); "Load Balancers" becomes available if any origin's DomainName matches an ELB DNS name

**AWS comparison:**
aws cloudfront get-distribution --id E12345 --query 'Distribution.DistributionConfig.Origins.Items[].DomainName'
Parse origin domain names to identify S3 buckets and ELBs.

**Demo dependency:** requires CloudFront fixtures with origin DomainNames matching S3 bucket names or ELB DNS names in their respective fixtures.

---

### Story: Route 53 reverse lookup for distributions
**Given:** the user opens the detail view for a CloudFront distribution
**When:** the right column resolves "Route 53 Hosted Zones"
**Then:** the row becomes available if any R53 hosted zone has alias records with HostedZoneId=Z2FDTNDATAQYW2 pointing to this distribution's domain name

**AWS comparison:**
aws route53 list-resource-record-sets --hosted-zone-id Z123456
Search for alias records with CloudFront's fixed hosted zone ID.

**Demo dependency:** requires R53 fixtures with alias records referencing CloudFront distribution domain names.

---

---

## Cross-Cutting Stories

---

### Story: Two-column detail view for every resource type
**Given:** any of the 66 resource types in a9s
**When:** the user enters the detail view for any resource of that type
**Then:** the left column shows resource-specific detail fields; the right column shows resource-type-specific related types (or just CloudTrail Events if no other relationships are defined); no resource type crashes or shows an empty right column without at least CloudTrail Events

**AWS comparison:**
aws {service} describe-{resource} ...
Every resource type has a describe command and every resource has CloudTrail events.

---

### Story: Forward navigable fields are resource-type-specific
**Given:** different resource types
**When:** the user opens each one's detail view
**Then:** each resource type has its own set of navigable fields based on its API response: EC2 has VpcId, SubnetId, SecurityGroups; RDS has VpcSecurityGroups, KmsKeyId, DBSubnetGroup.Subnets; Lambda has Role, VpcConfig.VpcId, VpcConfig.SubnetIds; VPC has very few (mostly just DhcpOptionsId which is not in a9s); the navigable field set is defined per resource type, not auto-detected from all fields

**AWS comparison:**
Each resource type's describe output contains different cross-references. Not all fields that look like IDs are navigable -- only those whose target resource types are registered in a9s.

---

### Story: Right column relationships are resource-type-specific
**Given:** different resource types
**When:** the user looks at the right column for each
**Then:** each shows different related types: EC2 shows Target Groups, ASGs, Alarms, etc.; Lambda shows CW Log Group, SQS Event Sources, EventBridge Rules, etc.; VPC shows EC2 Instances, Subnets, SGs, NAT GWs, etc.; CloudTrail Events appears for all of them as the last row

**AWS comparison:**
Each resource type has different reverse dependencies in AWS. There is no single API that lists them all.

---

### Story: No resource type shows duplicate relationships across columns
**Given:** any resource type in a9s
**When:** the user examines both the left column (navigable fields) and right column (related types)
**Then:** no resource type appears in both columns: forward relationships are in the left column only, reverse/algorithmic relationships are in the right column only

**AWS comparison:**
No AWS CLI equivalent -- architectural invariant preventing duplication.
