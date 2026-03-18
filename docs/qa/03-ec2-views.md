# QA-03: EC2 Instance Views

Exhaustive black-box user stories for the EC2 Instance List, EC2 Detail, and EC2 YAML views.

AWS CLI equivalent: `aws ec2 describe-instances` returning `.Reservations[].Instances[]`

---

## A. EC2 Instance List View

### A.1 Column Layout

| # | Story | Expected |
|---|-------|----------|
| A.1.1 | Open the EC2 instance list from the main menu | Six columns appear in this exact order: Instance ID, State, Type, Private IP, Public IP, Launch Time |
| A.1.2 | Compare column header text to views.yaml labels | Headers read exactly "Instance ID", "State", "Type", "Private IP", "Public IP", "Launch Time" |
| A.1.3 | Observe column widths | Instance ID occupies ~20 chars, State ~12, Type ~14, Private IP ~16, Public IP ~16, Launch Time ~22 |
| A.1.4 | Verify Instance ID column data | Each cell shows the `InstanceId` field value (e.g. `i-0abc123def456789a`) from the AWS response |
| A.1.5 | Verify State column data | Each cell shows the `State.Name` field value (e.g. `running`, `stopped`, `pending`, `terminated`) |
| A.1.6 | Verify Type column data | Each cell shows the `InstanceType` field value (e.g. `t3.medium`, `m5.xlarge`) |
| A.1.7 | Verify Private IP column data | Each cell shows the `PrivateIpAddress` field value (e.g. `10.0.1.42`) |
| A.1.8 | Verify Public IP column data | Each cell shows the `PublicIpAddress` field value (e.g. `54.123.45.67`) |
| A.1.9 | Verify Launch Time column data | Each cell shows the `LaunchTime` field value formatted as a timestamp |
| A.1.10 | Column headers are styled bold blue (#7aa2f7) | All six header labels render in bold with blue foreground |
| A.1.11 | No separator line exists below column headers | The first data row immediately follows the header row with no underline, rule, or divider |
| A.1.12 | Columns are space-aligned, not pipe-separated | Columns are padded with whitespace; no vertical pipe characters appear between columns |

### A.2 Frame and Title

| # | Story | Expected |
|---|-------|----------|
| A.2.1 | Frame title shows resource type and count | Top border reads e.g. `ec2-instances(42)` centered between dashes |
| A.2.2 | Frame title count matches total instances returned by AWS | The number in parentheses equals the total number of EC2 instances from `describe-instances` |
| A.2.3 | Frame uses thin box-drawing characters | Top: `\u250c\u2500\u2500 title \u2500\u2500\u2510`, sides: `\u2502`, bottom: `\u2514\u2500\u2500\u2518`; border color is dim (#414868) |
| A.2.4 | Frame fills all vertical space below the header | Content area uses `termHeight - 3` rows (header + top border + bottom border = 3 overhead lines) |

### A.3 Header Bar

| # | Story | Expected |
|---|-------|----------|
| A.3.1 | Header left shows app identity and AWS context | Left side reads: `a9s` (blue bold accent) + ` v0.x.x` (dim) + `  profile:region` (bold) |
| A.3.2 | Header right shows help hint in normal mode | Right side reads `? for help` in dim gray (#565f89) |
| A.3.3 | Header is a single unframed line above the frame | No border, no separator between header and frame top border |

### A.4 Status Coloring (Entire Row)

| # | Story | Expected |
|---|-------|----------|
| A.4.1 | Instance with State `running`: entire row is green | All six cell values in the row render in green (#9ece6a) |
| A.4.2 | Instance with State `stopped`: entire row is red | All six cell values in the row render in red (#f7768e) |
| A.4.3 | Instance with State `pending`: entire row is yellow | All six cell values in the row render in yellow (#e0af68) |
| A.4.4 | Instance with State `terminated`: entire row is dimmed | All six cell values in the row render in dim gray (#565f89) |
| A.4.5 | Instance with State `shutting-down`: observe behavior | Row should render in a status-appropriate color (likely yellow for transitional states or dim) |
| A.4.6 | Instance with an unrecognized/unknown state value | Row renders in plain white (#c0caf5) |
| A.4.7 | Coloring applies to the whole row, not just the State cell | The Instance ID, Type, IPs, and Launch Time cells all share the same status color as the State cell |

### A.5 Row Selection

| # | Story | Expected |
|---|-------|----------|
| A.5.1 | The currently selected row has a distinct visual treatment | Selected row shows full-width blue background (#7aa2f7) with dark foreground (#1a1b26), bold text |
| A.5.2 | Selection overrides status coloring | A selected `running` (green) row displays with blue background, not green; status color is suppressed |
| A.5.3 | Selection overrides dim styling on terminated rows | A selected `terminated` row displays with blue background, not dim gray |
| A.5.4 | Exactly one row is selected at any time | Only one row has the blue highlight; all others show their status color |
| A.5.5 | On initial load, the first row is selected | Cursor starts at the topmost row in the list |

### A.6 Navigation

| # | Story | Expected |
|---|-------|----------|
| A.6.1 | Press `j` or Down arrow: cursor moves down one row | Selection highlight shifts to the next row below |
| A.6.2 | Press `k` or Up arrow: cursor moves up one row | Selection highlight shifts to the row above |
| A.6.3 | Press `g`: cursor jumps to the first row | Selection highlight moves to the topmost row |
| A.6.4 | Press `G`: cursor jumps to the last row | Selection highlight moves to the bottommost row |
| A.6.5 | Press `j` on the last row: cursor wraps to the first row | Selection wraps around to the top of the list |
| A.6.6 | Press `k` on the first row: cursor wraps to the last row | Selection wraps around to the bottom of the list |
| A.6.7 | Press `h` or Left arrow: columns scroll left | The visible column window shifts left, revealing any hidden left-side columns |
| A.6.8 | Press `l` or Right arrow: columns scroll right | The visible column window shifts right, revealing any hidden right-side columns |
| A.6.9 | Column headers scroll in sync with data columns | When scrolling horizontally, column headers and row data shift together |
| A.6.10 | When all columns fit in the terminal width, h/l have no effect | Horizontal scroll is a no-op if no columns are hidden |
| A.6.11 | Scrolling through a list longer than the visible area | The table scrolls vertically to keep the selected row visible within the frame |

### A.7 Sort

| # | Story | Expected |
|---|-------|----------|
| A.7.1 | Press `N`: rows sort by name (Instance ID) ascending | Rows reorder alphabetically by Instance ID; the Instance ID column header shows `\u2191` indicator |
| A.7.2 | Press `N` again: sort toggles to descending | Rows reorder in reverse; the header shows `\u2193` indicator |
| A.7.3 | Press `S`: rows sort by status (State) ascending | Rows reorder alphabetically by state value; the State header shows `\u2191` |
| A.7.4 | Press `S` again: sort toggles to descending | Rows reorder in reverse; the header shows `\u2193` |
| A.7.5 | Press `A`: rows sort by age (Launch Time) ascending | Rows reorder by launch time (oldest first); the Launch Time header shows `\u2191` |
| A.7.6 | Press `A` again: sort toggles to descending | Rows reorder (newest first); the header shows `\u2193` |
| A.7.7 | Sort indicator appears on exactly one column at a time | Switching from N-sort to S-sort removes `\u2191`/`\u2193` from Instance ID and adds it to State |
| A.7.8 | Sort indicator is appended directly to column header text | No space between header text and the arrow: e.g. `Instance ID\u2191`, not `Instance ID \u2191` |
| A.7.9 | After sorting, the selected row follows the data | If instance `i-abc` was selected before sort, it remains selected after reorder |

### A.8 Filter

| # | Story | Expected |
|---|-------|----------|
| A.8.1 | Press `/`: filter mode activates | Header right side changes from `? for help` to `/\u2588` (amber bold #e0af68) |
| A.8.2 | Type filter text: matching rows shown, non-matching hidden | Only rows where any cell contains the search text (case-insensitive substring) are displayed |
| A.8.3 | Frame title updates to show matched/total count | Title changes to e.g. `ec2-instances(3/42)` |
| A.8.4 | Filter by Instance ID substring: `i-0abc` | Only instances whose ID contains `i-0abc` appear |
| A.8.5 | Filter by state: type `running` | Only running instances appear; stopped/pending/terminated are hidden |
| A.8.6 | Filter by instance type: type `t3` | Only instances of type `t3.*` appear |
| A.8.7 | Filter by IP address: type `10.0.1` | Only instances whose Private IP or Public IP contains `10.0.1` appear |
| A.8.8 | Filter matching is case-insensitive | Typing `RUNNING` matches rows showing `running` |
| A.8.9 | Filter with no matches: empty table | Zero rows displayed; frame title shows `ec2-instances(0/42)` |
| A.8.10 | Press `Backspace` during filter: last character removed | Filter text shortens; matching rows update instantly |
| A.8.11 | Press `Esc` during filter: filter cleared, all rows restored | Header reverts to `? for help`; frame title reverts to `ec2-instances(42)`; all rows visible again |
| A.8.12 | No matched-text highlighting inside row cells | Matching rows are shown but the matching substring is not highlighted or underlined within cell text |
| A.8.13 | Row coloring is unchanged during filter | Filtered running rows are still green, stopped still red, etc. |
| A.8.14 | Selection resets to first visible row after filter narrows results | If the previously selected row is filtered out, selection moves to the first matching row |

### A.9 Command Mode

| # | Story | Expected |
|---|-------|----------|
| A.9.1 | Press `:`: command mode activates | Header right side changes to `:\u2588` (amber bold #e0af68) |
| A.9.2 | Type `main` and press Enter: navigate to main menu | View transitions to the main resource-type menu |
| A.9.3 | Type `s3` and press Enter: navigate to S3 list | View transitions to the S3 bucket list |
| A.9.4 | Press `Tab` during command input: autocomplete suggestion accepted | The partial command is completed (e.g. `se` -> `secrets`) |
| A.9.5 | Press `Esc` during command mode: command cancelled | Header reverts to `? for help`; no navigation occurs; EC2 list remains |
| A.9.6 | Type an unknown command and press Enter: error flash | Header right briefly shows an error message in red (#f7768e), then reverts |

### A.10 Actions from List

| # | Story | Expected |
|---|-------|----------|
| A.10.1 | Press `Enter` on a selected instance: detail view opens | View transitions to the EC2 detail view for that instance |
| A.10.2 | Press `d` on a selected instance: detail view opens | Same behavior as Enter; detail view shows the selected instance |
| A.10.3 | Press `y` on a selected instance: YAML view opens | View transitions to the YAML view for that instance |
| A.10.4 | Press `c` on a selected instance: instance ID copied to clipboard | The Instance ID (or Name/ARN) is copied; header right briefly shows `Copied!` in green (#9ece6a) |
| A.10.5 | `Copied!` flash message auto-clears after ~2 seconds | Header right reverts to `? for help` after the flash duration |
| A.10.6 | Press `Esc`: navigate back to main menu | View transitions back to the main resource-type menu |
| A.10.7 | Press `?`: help screen appears | Frame content is replaced by the multi-column help screen |
| A.10.8 | Press any key to close help: list view restored | EC2 instance list reappears with selection and scroll position intact |
| A.10.9 | Press `Ctrl+R`: data refreshes from AWS | A loading spinner briefly appears; data is re-fetched; list repopulates with current state |

### A.11 Edge Cases: Missing Data

| # | Story | Expected |
|---|-------|----------|
| A.11.1 | Instance has no public IP (PublicIpAddress is null/empty) | The Public IP cell is blank/empty; no crash, no placeholder text like "N/A" unless that is the design |
| A.11.2 | Instance has no Name tag | The instance still appears in the list; the Instance ID column shows the raw `i-xxx` ID |
| A.11.3 | Terminated instances appear in the list | Terminated instances are visible (not filtered out) but rendered with dim styling (#565f89) |
| A.11.4 | Instance with both Private and Public IPs populated | Both IP columns show their respective values correctly |
| A.11.5 | Instance with a very long Instance ID | The value is truncated to fit within the 20-character column width; no layout overflow |
| A.11.6 | Launch Time displays in a readable format | Timestamp is rendered in a human-readable format (e.g. `2024-01-15 09:22`) not raw ISO 8601 |

### A.12 Edge Cases: Empty and Error States

| # | Story | Expected |
|---|-------|----------|
| A.12.1 | AWS account has zero EC2 instances | Frame title shows `ec2-instances(0)`; content area shows a centered message suggesting to refresh or change region |
| A.12.2 | AWS API call fails (no credentials, network error, permission denied) | An error message flashes in the header right side in red (#f7768e); the error is persistent until navigation |
| A.12.3 | Very large number of instances (hundreds) | All instances load; vertical scrolling works; no truncation of the instance list |

### A.13 Loading State

| # | Story | Expected |
|---|-------|----------|
| A.13.1 | While EC2 data is being fetched | Frame title shows `ec2-instances` (no count yet); a spinner and "Fetching EC2 instances..." appear centered in the frame |
| A.13.2 | Spinner is animated | The spinner dot pattern cycles visually in blue (#7aa2f7) |
| A.13.3 | After data loads, spinner is replaced by the table | Column headers and data rows appear; frame title updates with instance count |

### A.14 Responsive Behavior

| # | Story | Expected |
|---|-------|----------|
| A.14.1 | Terminal width < 60 columns | An error message is displayed: "Terminal too narrow. Please resize." |
| A.14.2 | Terminal width 60-79 columns | Only 2 columns are shown (likely Instance ID and State); no horizontal scroll hint |
| A.14.3 | Terminal width 80-119 columns | Standard layout with 4 visible columns (Instance ID, State, Type, and one more) |
| A.14.4 | Terminal width 120+ columns | All 6 configured columns are visible |
| A.14.5 | Terminal height < 7 lines | An error message is displayed: "Terminal too short. Please resize." |
| A.14.6 | Terminal is resized while viewing EC2 list | Layout reflows dynamically; columns and rows adjust to the new dimensions |

---

## B. EC2 Detail View

### B.1 Entry and Frame

| # | Story | Expected |
|---|-------|----------|
| B.1.1 | Press `Enter` or `d` on an EC2 instance in the list | Detail view opens; the frame title shows the instance ID (e.g. `i-0abc123def456789a`) centered in the top border |
| B.1.2 | Frame replaces the table in the same layout position | The detail view occupies the same frame area below the header; no additional chrome appears |
| B.1.3 | Header bar is unchanged | Left side still shows `a9s v0.x.x  profile:region`; right side shows `? for help` |

### B.2 Fields Displayed

| # | Story | Expected |
|---|-------|----------|
| B.2.1 | InstanceId field is shown | A line reads with key `InstanceId` and the instance's ID value (e.g. `i-0abc123def456789a`) |
| B.2.2 | State field is shown | A line reads with key `State` and shows the state object or state name (e.g. `running`) |
| B.2.3 | State value for `running` is colored green | The value text renders in green (#9ece6a) |
| B.2.4 | State value for `stopped` is colored red | The value text renders in red (#f7768e) |
| B.2.5 | State value for `pending` is colored yellow | The value text renders in yellow (#e0af68) |
| B.2.6 | State value for `terminated` is colored dim | The value text renders in dim gray (#565f89) |
| B.2.7 | InstanceType field is shown | Key `InstanceType`, value e.g. `t3.medium` |
| B.2.8 | ImageId field is shown | Key `ImageId`, value e.g. `ami-0abcdef01234567` |
| B.2.9 | VpcId field is shown | Key `VpcId`, value e.g. `vpc-0123456789abcdef0` |
| B.2.10 | SubnetId field is shown | Key `SubnetId`, value e.g. `subnet-0123456789abcde` |
| B.2.11 | PrivateIpAddress field is shown | Key `PrivateIpAddress`, value e.g. `10.0.1.42` |
| B.2.12 | PublicIpAddress field is shown | Key `PublicIpAddress`, value e.g. `54.123.45.67` |
| B.2.13 | SecurityGroups field is shown | Key `SecurityGroups` appears, followed by the array of security group entries |
| B.2.14 | LaunchTime field is shown | Key `LaunchTime`, value shows the instance launch timestamp |
| B.2.15 | Architecture field is shown | Key `Architecture`, value e.g. `x86_64` or `arm64` |
| B.2.16 | Platform field is shown | Key `Platform`, value e.g. `windows` or empty/absent for Linux |
| B.2.17 | Tags field is shown | Key `Tags` appears, followed by the array of tag key-value pairs |
| B.2.18 | All 13 configured detail fields from views.yaml appear | No configured field is missing from the detail output |
| B.2.19 | Fields appear in the order specified in views.yaml | InstanceId first, then State, InstanceType, ImageId, VpcId, SubnetId, PrivateIpAddress, PublicIpAddress, SecurityGroups, LaunchTime, Architecture, Platform, Tags last |

### B.3 Key-Value Formatting

| # | Story | Expected |
|---|-------|----------|
| B.3.1 | Keys are styled in blue (#7aa2f7) | Every field label on the left side renders in blue |
| B.3.2 | Values are styled in plain white (#c0caf5) | Every field value on the right side renders in the default light color |
| B.3.3 | Keys are left-aligned with a fixed width (~22 characters) | All keys align neatly; values start at the same horizontal position |
| B.3.4 | Section headers are styled in yellow/orange (#e0af68), bold | If the detail view groups fields under section headings (e.g. Identity, Network, State, Tags), those headings are yellow bold |
| B.3.5 | Sub-fields use 2-space indent | Nested items under a section or array field are indented by 2 spaces relative to their parent |

### B.4 Array Fields: SecurityGroups

| # | Story | Expected |
|---|-------|----------|
| B.4.1 | Instance has one security group | SecurityGroups shows a single entry with GroupId and GroupName |
| B.4.2 | Instance has multiple security groups | SecurityGroups shows multiple entries, each on its own line(s), with GroupId and GroupName for each |
| B.4.3 | SecurityGroups renders as multi-line, not comma-separated on one line | Each security group occupies its own line(s) with proper indentation |
| B.4.4 | Each security group shows both GroupId and GroupName | Both `GroupId` (e.g. `sg-0abc123`) and `GroupName` (e.g. `web-server-sg`) are visible per entry |

### B.5 Array Fields: Tags

| # | Story | Expected |
|---|-------|----------|
| B.5.1 | Instance has multiple tags | Tags section shows each tag as a key-value pair (e.g. `Name: api-prod-01`, `Environment: production`) |
| B.5.2 | Tags render as multi-line entries | Each tag occupies its own line with Key and Value displayed |
| B.5.3 | Instance has zero tags | Tags section is empty or shows an indicator that no tags exist; no crash |
| B.5.4 | Tag with an empty value | The tag key is shown with a blank or empty value; no crash |
| B.5.5 | Tags with long values | Long tag values are either truncated or wrap depending on the wrap toggle state |

### B.6 Scrolling

| # | Story | Expected |
|---|-------|----------|
| B.6.1 | Press `j` or Down arrow: content scrolls down one line | The viewport shifts down, revealing content below |
| B.6.2 | Press `k` or Up arrow: content scrolls up one line | The viewport shifts up, revealing content above |
| B.6.3 | Press `g`: content jumps to the top | The viewport scrolls to the very first line of detail content |
| B.6.4 | Press `G`: content jumps to the bottom | The viewport scrolls to the very last line of detail content |
| B.6.5 | Detail content fits within the frame without scrolling | j/k have no visible effect; no scroll indicators appear |
| B.6.6 | Detail content exceeds the frame height | Scroll indicators appear (e.g. dim text like "\u2191 12 lines above") showing how much content is off-screen |
| B.6.7 | At the very top, pressing `k` has no further effect | No crash; content remains at the top |
| B.6.8 | At the very bottom, pressing `j` has no further effect | No crash; content remains at the bottom |

### B.7 Word Wrap Toggle

| # | Story | Expected |
|---|-------|----------|
| B.7.1 | Press `w`: word wrap toggles on | Long values that exceed the frame width now wrap to the next line instead of being clipped |
| B.7.2 | Press `w` again: word wrap toggles off | Long values are clipped/truncated at the frame boundary; horizontal overflow is hidden |
| B.7.3 | Wrap toggle affects array fields (SecurityGroups, Tags) | Multi-line array entries respect the current wrap setting |

### B.8 Actions from Detail

| # | Story | Expected |
|---|-------|----------|
| B.8.1 | Press `y`: switch to YAML view for this instance | View transitions to the YAML representation of the same instance |
| B.8.2 | Press `c`: copy detail content to clipboard | Full detail text is copied; header right briefly shows `Copied!` in green |
| B.8.3 | Press `Esc`: return to EC2 instance list | View transitions back to the list with the same instance still selected |
| B.8.4 | Press `?`: help screen appears | Detail content is replaced by the help screen |
| B.8.5 | Press `Ctrl+R`: data refreshes | Detail content is re-fetched from AWS and redisplayed |

### B.9 Edge Cases

| # | Story | Expected |
|---|-------|----------|
| B.9.1 | Instance has no PublicIpAddress (null) | The PublicIpAddress line shows a blank value or is omitted gracefully; no crash |
| B.9.2 | Instance has no Platform field (Linux instances) | The Platform line shows a blank value or is omitted; no crash |
| B.9.3 | Instance has no SecurityGroups (empty array) | SecurityGroups section is empty or shows an empty indicator; no crash |
| B.9.4 | Instance has a very large number of tags (e.g. 50) | All tags render; scrolling is required to see them all |
| B.9.5 | State field is a nested object (contains Code and Name) | The detail view shows the state information meaningfully (at minimum State.Name), not a raw object reference |

---

## C. EC2 YAML View

### C.1 Entry and Frame

| # | Story | Expected |
|---|-------|----------|
| C.1.1 | Press `y` from the EC2 list or detail view | YAML view opens for the selected instance |
| C.1.2 | Frame title shows the instance ID plus "yaml" suffix | Top border reads e.g. `i-0abc123def456789a yaml` centered between dashes |
| C.1.3 | Header bar is unchanged | Left side: `a9s v0.x.x  profile:region`; right side: `? for help` |

### C.2 Content: Full YAML Dump

| # | Story | Expected |
|---|-------|----------|
| C.2.1 | YAML view shows the complete EC2 instance object | All fields from the `describe-instances` response for this instance are present, not just the 13 detail fields |
| C.2.2 | Top-level keys are present | Keys like `InstanceId`, `InstanceType`, `State`, `ImageId`, `LaunchTime`, `BlockDeviceMappings`, `NetworkInterfaces`, `Tags`, `SecurityGroups`, `Placement`, etc. all appear |
| C.2.3 | Nested objects render with proper YAML indentation | e.g. `State:` on one line, then `  Name: running` and `  Code: 16` indented below |
| C.2.4 | Arrays render with YAML list syntax | e.g. `Tags:` followed by `- Key: Name` / `  Value: api-prod-01` entries using the `- ` prefix |
| C.2.5 | Content matches `aws ec2 describe-instances --instance-ids i-xxx --output yaml` | The YAML structure and field names correspond to the AWS CLI YAML output for the same instance |
| C.2.6 | Deeply nested structures render correctly | Fields like `BlockDeviceMappings[].Ebs.VolumeId` or `NetworkInterfaces[].Attachment.AttachmentId` appear at the correct nesting depth |

### C.3 Syntax Coloring

| # | Story | Expected |
|---|-------|----------|
| C.3.1 | YAML keys are colored blue (#7aa2f7) | Every key name (e.g. `InstanceId:`, `State:`, `Name:`) renders in blue |
| C.3.2 | String values are colored green (#9ece6a) | String values (e.g. `i-0abc123def456789a`, `running`, `t3.medium`) render in green |
| C.3.3 | Numeric values are colored orange (#ff9e64) | Numeric values (e.g. `0`, `16`, `8`) render in orange |
| C.3.4 | Boolean values are colored purple (#bb9af7) | Boolean values (`true`, `false`) render in purple |
| C.3.5 | Null values are colored dim (#565f89) | Null/nil/empty values (`null`, `~`) render in dim gray |
| C.3.6 | Indent/tree connector lines are dim (#414868) | Vertical tree connector characters (\u2502) used for visual indentation render in dim gray |
| C.3.7 | Coloring applies consistently across all nesting levels | A key at depth 4 is the same blue as a key at depth 0; a string at any depth is green |

### C.4 Specific Value Type Verification

| # | Story | Expected |
|---|-------|----------|
| C.4.1 | `AmiLaunchIndex: 0` | Key `AmiLaunchIndex` is blue; value `0` is orange (numeric) |
| C.4.2 | `Architecture: x86_64` | Key `Architecture` is blue; value `x86_64` is green (string) |
| C.4.3 | `EbsOptimized: true` | Key `EbsOptimized` is blue; value `true` is purple (boolean) |
| C.4.4 | `DeleteOnTermination: false` | Key is blue; value `false` is purple (boolean) |
| C.4.5 | `KernelId: null` or absent field with null | Key is blue; value `null` is dim gray |
| C.4.6 | `LaunchTime: 2024-01-15T09:22:31Z` | Key is blue; value (timestamp string) is green |
| C.4.7 | `State.Code: 16` | Key is blue; value `16` is orange (numeric) |
| C.4.8 | `SourceDestCheck: true` | Key is blue; value `true` is purple |

### C.5 Scrolling

| # | Story | Expected |
|---|-------|----------|
| C.5.1 | Press `j` or Down arrow: content scrolls down one line | YAML content scrolls downward |
| C.5.2 | Press `k` or Up arrow: content scrolls up one line | YAML content scrolls upward |
| C.5.3 | Press `g`: jump to top of YAML | Viewport scrolls to the first line of the YAML output |
| C.5.4 | Press `G`: jump to bottom of YAML | Viewport scrolls to the last line of the YAML output |
| C.5.5 | YAML content is taller than the frame | Scroll indicators appear showing lines above/below the visible area |
| C.5.6 | Full EC2 instance YAML can be hundreds of lines | All content is accessible via scrolling; no truncation of the YAML output |

### C.6 Actions from YAML

| # | Story | Expected |
|---|-------|----------|
| C.6.1 | Press `Esc`: return to previous view | If entered from list, returns to list; if entered from detail, returns to detail (view stack pop) |
| C.6.2 | Press `c`: copy YAML content to clipboard | Full YAML text is copied; header right briefly shows `Copied!` in green |
| C.6.3 | Press `?`: help screen appears | YAML content is replaced by the help screen |
| C.6.4 | Press `Ctrl+R`: data refreshes | YAML content is re-fetched and re-rendered |

### C.7 Edge Cases

| # | Story | Expected |
|---|-------|----------|
| C.7.1 | Instance with no tags (empty Tags array) | YAML shows `Tags: []` or `Tags:` with no children; no crash |
| C.7.2 | Instance with no public IP | `PublicIpAddress` key shows `null` (dim) or is absent; no crash |
| C.7.3 | Instance with no Platform field | `Platform` key shows `null` (dim) or is absent; no crash |
| C.7.4 | Instance with many BlockDeviceMappings | All block devices render with correct nesting; scrolling reveals all entries |
| C.7.5 | Instance with many NetworkInterfaces | All network interfaces render fully; deep nesting (Attachment, Association, PrivateIpAddresses) is correctly indented |
| C.7.6 | Very wide YAML lines (long string values) | Long lines are either truncated at frame width or wrap, consistent with the view's behavior |
| C.7.7 | Instance with SecurityGroups array | Each security group object renders with both `GroupId` and `GroupName` fields properly nested |

---

## D. Cross-View Navigation Flows

| # | Story | Expected |
|---|-------|----------|
| D.1 | Main Menu -> EC2 List -> Detail -> YAML -> Esc -> Detail -> Esc -> List -> Esc -> Main Menu | Each Esc pops one level from the view stack; final Esc returns to main menu |
| D.2 | EC2 List -> YAML (via `y`) -> Esc -> EC2 List | Pressing `y` from the list goes directly to YAML; Esc returns to the list, not to detail |
| D.3 | EC2 List -> Detail (via `d`) -> YAML (via `y`) -> Esc -> Detail -> Esc -> List | Full three-level deep navigation and back |
| D.4 | Select instance A, open detail, press Esc, select instance B, open detail | Detail shows instance B's data, not instance A's |
| D.5 | Open detail for a `running` instance, press Esc, navigate to a `stopped` instance, open detail | The detail view correctly shows the newly selected instance's data and state |
| D.6 | EC2 List with active filter -> press Enter on a filtered row -> Detail shows that instance | The detail view opens for the correct instance even when the list is filtered |
| D.7 | EC2 List with active filter -> press `y` on a filtered row -> YAML shows that instance | The YAML view opens for the correct instance even when the list is filtered |
| D.8 | EC2 Detail -> press `y` -> YAML view -> press Esc -> back to Detail (not list) | View stack correctly returns to the previous view in the stack |

---

## E. AWS Data Fidelity

| # | Story | Expected |
|---|-------|----------|
| E.1 | Instance count in frame title matches `aws ec2 describe-instances` total | The number of instances shown equals the count of objects in `.Reservations[].Instances[]` |
| E.2 | Instance IDs in the list match the AWS API response | Every `InstanceId` value corresponds to an actual instance returned by `describe-instances` |
| E.3 | State values match the API response exactly | State.Name values (`running`, `stopped`, `pending`, `terminated`, `shutting-down`, `stopping`) match the API |
| E.4 | Private/Public IP addresses match the API response | IP values are not reformatted, truncated, or altered from the API response |
| E.5 | Launch times correspond to the API response | The `LaunchTime` timestamp can be mapped back to the raw API value for each instance |
| E.6 | Security group data in detail matches the API response | GroupId and GroupName values match those returned by `describe-instances` for the selected instance |
| E.7 | Tag data in detail matches the API response | All Key-Value pairs from the API Tags array appear in the detail view |
| E.8 | YAML output contains all fields from the API response | No fields are silently dropped or omitted from the YAML dump |
