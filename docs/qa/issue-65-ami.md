# QA User Stories: AMI (Amazon Machine Images) as a New Resource Type

Covers GitHub issue #65: adding AMIs as a new top-level resource type with list, detail, YAML views, and two child views (AMI -> EBS Snapshots, AMI -> EC2 Instances).

All stories are written from a black-box perspective against the design spec and `views.yaml`. AWS CLI equivalents are cited so testers can verify data parity.

---

## A. AMI List View

### A.1 Main Menu Entry

| ID | Story | Expected |
|----|-------|----------|
| A.1.1 | I launch a9s and view the main menu. | A new entry for AMIs (e.g., "AMIs" or "Amazon Machine Images") appears in the resource type list with a command alias (e.g., `:ami`). |
| A.1.2 | I select the AMI entry and press Enter. | The view transitions to the AMI list view. A loading spinner appears while data is fetched. |
| A.1.3 | I type `:ami` in command mode and press Enter. | The view navigates directly to the AMI list view from any other view. |

**AWS comparison:**
```
aws ec2 describe-images --owners self --query 'Images[].{Name:Name,ImageId:ImageId,State:State}'
```

### A.2 Column Layout

| ID | Story | Expected |
|----|-------|----------|
| A.2.1 | AMI data loads and the table renders. | Eight columns appear: Name, Image ID, State, Architecture, Platform, Root Device Type, Creation Date, Public. Column headers are bold blue (`#7aa2f7`), with no separator line below them. |
| A.2.2 | I verify the Name column data. | Each cell shows the `Image.Name` field value (e.g., `my-app-v1.2.3`, `amazon-linux-2-base`). |
| A.2.3 | I verify the Image ID column data. | Each cell shows the `Image.ImageId` field value (e.g., `ami-0abc123def456789a`). |
| A.2.4 | I verify the State column data. | Each cell shows the `Image.State` field value (e.g., `available`, `pending`, `failed`, `deregistered`). |
| A.2.5 | I verify the Architecture column data. | Each cell shows the `Image.Architecture` field value (e.g., `x86_64`, `arm64`). |
| A.2.6 | I verify the Platform column data. | Each cell shows the `Image.PlatformDetails` field value (e.g., `Linux/UNIX`, `Windows`). |
| A.2.7 | I verify the Root Device Type column data. | Each cell shows the `Image.RootDeviceType` field value (e.g., `ebs`, `instance-store`). |
| A.2.8 | I verify the Creation Date column data. | Each cell shows the `Image.CreationDate` field value formatted as a readable timestamp. |
| A.2.9 | I verify the Public column data. | Each cell shows the `Image.Public` field value (e.g., `true` or `false`). |
| A.2.10 | Columns are space-aligned, not pipe-separated. | Columns are padded with whitespace; no vertical pipe characters appear between columns. |

**AWS comparison:**
```
aws ec2 describe-images --owners self --query 'Images[].{Name:Name,ImageId:ImageId,State:State,Arch:Architecture,Platform:PlatformDetails,RootDeviceType:RootDeviceType,Created:CreationDate,Public:Public}'
```
Expected fields visible: Name, Image ID, State, Architecture, Platform, Root Device Type, Creation Date, Public

### A.3 Frame and Title

| ID | Story | Expected |
|----|-------|----------|
| A.3.1 | AMI data loads successfully. | The frame title shows the resource type short name and count, e.g., `ami(12)` centered between dashes in the top border. |
| A.3.2 | The frame title count matches the total number of self-owned AMIs. | The number in parentheses equals the total count of AMIs returned by `describe-images --owners self`. |
| A.3.3 | Frame uses thin box-drawing characters. | Top: corner + dashes + title + dashes + corner. Sides: vertical bars. Bottom: corner + dashes + corner. Border color is dim (`#414868`). |

### A.4 Loading State

| ID | Story | Expected |
|----|-------|----------|
| A.4.1 | While AMI data is being fetched. | Frame title shows `ami` (no count yet). A spinner and loading message (e.g., "Fetching AMIs...") appear centered in the frame. |
| A.4.2 | Spinner is animated. | The spinner dot pattern cycles visually in blue (`#7aa2f7`). |
| A.4.3 | After data loads, spinner is replaced by the table. | Column headers and data rows appear. Frame title updates with the AMI count. |

### A.5 Status Coloring (Entire Row)

| ID | Story | Expected |
|----|-------|----------|
| A.5.1 | An AMI has State `available`. | The entire row renders in GREEN (`#9ece6a`). |
| A.5.2 | An AMI has State `pending`. | The entire row renders in YELLOW (`#e0af68`). |
| A.5.3 | An AMI has State `failed`. | The entire row renders in RED (`#f7768e`). |
| A.5.4 | An AMI has State `deregistered`. | The entire row renders in DIM (`#565f89`) or RED, depending on implementation. |
| A.5.5 | I select a colored row. | The selected row shows full-width blue background (`#7aa2f7`) with dark foreground (`#1a1b26`), overriding the status color. |

**AWS comparison:**
```
aws ec2 describe-images --owners self --query 'Images[].{Name:Name,State:State}'
```

### A.6 Row Selection and Navigation

| ID | Story | Expected |
|----|-------|----------|
| A.6.1 | Press `j` or Down arrow. | Cursor moves down one row. |
| A.6.2 | Press `k` or Up arrow. | Cursor moves up one row. |
| A.6.3 | Press `g`. | Cursor jumps to the first row. |
| A.6.4 | Press `G`. | Cursor jumps to the last row. |
| A.6.5 | Press `j` on the last row. | Cursor wraps to the first row. |
| A.6.6 | Press `k` on the first row. | Cursor wraps to the last row. |
| A.6.7 | Press `h` or Left arrow. | Columns scroll left, revealing hidden left-side columns. |
| A.6.8 | Press `l` or Right arrow. | Columns scroll right, revealing hidden right-side columns. |
| A.6.9 | On initial load, the first row is selected. | Cursor starts at the topmost row. |
| A.6.10 | Press `pgdn` or `ctrl+d`. | Cursor moves down one page. |
| A.6.11 | Press `pgup` or `ctrl+u`. | Cursor moves up one page. |

### A.7 Sort

| ID | Story | Expected |
|----|-------|----------|
| A.7.1 | Press `N`. | Rows sort by name (Name column) ascending. The Name column header shows an up arrow indicator. |
| A.7.2 | Press `N` again. | Sort toggles to descending. The indicator changes to a down arrow. |
| A.7.3 | Press `S`. | Rows sort by status (State column) ascending. |
| A.7.4 | Press `A`. | Rows sort by age (Creation Date column) ascending (oldest first). |
| A.7.5 | Press `A` again. | Sort toggles to descending (newest first). |

### A.8 Filter

| ID | Story | Expected |
|----|-------|----------|
| A.8.1 | Press `/` and type `arm64`. | Only AMIs with Architecture containing `arm64` appear. Frame title shows filtered count (e.g., `ami(3/12)`). |
| A.8.2 | Press `/` and type `available`. | Only AMIs with State `available` appear. |
| A.8.3 | Press `/` and type the beginning of an AMI name. | Only AMIs whose name matches the filter appear. |
| A.8.4 | Press `Esc` during filter. | Filter is cleared. All AMIs are visible again. Frame title reverts to total count. |
| A.8.5 | Filter with no matches. | Zero rows displayed. Frame title shows `ami(0/12)`. |

### A.9 Actions from List

| ID | Story | Expected |
|----|-------|----------|
| A.9.1 | Press `Enter` or `d` on a selected AMI. | Detail view opens for that AMI. |
| A.9.2 | Press `y` on a selected AMI. | YAML view opens for that AMI. |
| A.9.3 | Press `c` on a selected AMI. | The AMI ID (e.g., `ami-0abc123def456789a`) is copied to clipboard. Header right briefly shows `Copied!` in green (`#9ece6a`). |
| A.9.4 | Press `Esc`. | View navigates back to the main menu. |
| A.9.5 | Press `?`. | Help screen appears with AMI-specific context. |
| A.9.6 | Press `Ctrl+R`. | Data refreshes from AWS. A loading spinner briefly appears. |

### A.10 Empty State

| ID | Story | Expected |
|----|-------|----------|
| A.10.1 | AWS account has zero self-owned AMIs. | Frame title shows `ami(0)`. Content area shows a centered message suggesting to refresh or check the region. |
| A.10.2 | I switch to a region with no self-owned AMIs and press `Ctrl+R`. | The AMI list refreshes to zero results. |

### A.11 Edge Cases

| ID | Story | Expected |
|----|-------|----------|
| A.11.1 | An AMI has no Name (Name field is null). | The Name cell is blank. The AMI still appears in the list with its Image ID visible. |
| A.11.2 | An AMI has a very long name (e.g., 80+ characters). | The name is truncated to fit the column width. No layout overflow. |
| A.11.3 | Creation Date displays in a readable format. | Timestamp is rendered in human-readable format (e.g., `2024-01-15 09:22`), not raw ISO 8601. |
| A.11.4 | An AMI with Public set to true. | The Public column shows `true`. |
| A.11.5 | An AMI with Public set to false. | The Public column shows `false`. |
| A.11.6 | API error (e.g., AccessDenied for ec2:DescribeImages). | A red error flash appears in the header. The AMI list is empty. The application does not crash. |
| A.11.7 | Account has many AMIs (hundreds). | All AMIs load. Vertical scrolling works. No truncation. |

---

## B. AMI Detail View

### B.1 Entry and Frame

| ID | Story | Expected |
|----|-------|----------|
| B.1.1 | Press `Enter` or `d` on an AMI in the list. | Detail view opens. The frame title shows the AMI ID (e.g., `ami-0abc123def456789a`) centered in the top border. |
| B.1.2 | Frame replaces the table in the same layout position. | The detail view occupies the same frame area below the header. |
| B.1.3 | Header bar is unchanged. | Left side still shows `a9s v0.x.x  profile:region`. Right side shows `? for help`. |

### B.2 Fields Displayed

| ID | Story | Expected |
|----|-------|----------|
| B.2.1 | ImageId field is shown. | A line reads with key `ImageId` and the AMI ID value. |
| B.2.2 | Name field is shown. | A line reads with key `Name` and the AMI name. |
| B.2.3 | State field is shown. | A line reads with key `State` and the state value (e.g., `available`). |
| B.2.4 | Architecture field is shown. | A line reads with key `Architecture` and the value (e.g., `x86_64`, `arm64`). |
| B.2.5 | PlatformDetails field is shown. | A line reads with key `PlatformDetails` and the value (e.g., `Linux/UNIX`). |
| B.2.6 | RootDeviceType field is shown. | A line reads with key `RootDeviceType` and the value (e.g., `ebs`). |
| B.2.7 | CreationDate field is shown. | A line reads with key `CreationDate` and the creation timestamp. |
| B.2.8 | Public field is shown. | A line reads with key `Public` and `true` or `false`. |
| B.2.9 | BlockDeviceMappings field is shown. | A section showing the block device mappings, including device names and EBS snapshot IDs. |
| B.2.10 | Description field is shown (if present). | A line reads with key `Description` and the AMI description text. |
| B.2.11 | OwnerId field is shown. | A line reads with key `OwnerId` and the AWS account ID. |
| B.2.12 | Tags field is shown. | A section showing tag key-value pairs. |

**AWS comparison:**
```
aws ec2 describe-images --image-ids ami-0abc123def456789a --query 'Images[0]'
```

### B.3 Key-Value Formatting

| ID | Story | Expected |
|----|-------|----------|
| B.3.1 | Keys are styled in blue (`#7aa2f7`). | Every field label on the left side renders in blue. |
| B.3.2 | Values are styled in plain white (`#c0caf5`). | Every field value on the right side renders in the default light color. |
| B.3.3 | State value for `available` is colored green. | The value text renders in green (`#9ece6a`). |
| B.3.4 | State value for `failed` is colored red. | The value text renders in red (`#f7768e`). |

### B.4 BlockDeviceMappings Array

| ID | Story | Expected |
|----|-------|----------|
| B.4.1 | AMI has one block device mapping. | BlockDeviceMappings shows a single entry with DeviceName, Ebs.SnapshotId, Ebs.VolumeSize, Ebs.VolumeType. |
| B.4.2 | AMI has multiple block device mappings. | Each mapping appears on its own line(s) with proper indentation. |
| B.4.3 | A block device mapping includes an EBS snapshot ID. | The SnapshotId value (e.g., `snap-0abc123def456789a`) is visible, linking the AMI to its backing storage. |

### B.5 Scrolling

| ID | Story | Expected |
|----|-------|----------|
| B.5.1 | Press `j` or Down arrow. | Detail content scrolls down one line. |
| B.5.2 | Press `k` or Up arrow. | Detail content scrolls up one line. |
| B.5.3 | Press `g`. | Content jumps to the top. |
| B.5.4 | Press `G`. | Content jumps to the bottom. |
| B.5.5 | Press `w`. | Word wrap toggles on/off. Long values wrap or truncate accordingly. |

### B.6 Actions from Detail

| ID | Story | Expected |
|----|-------|----------|
| B.6.1 | Press `y`. | Switch to YAML view for this AMI. |
| B.6.2 | Press `c`. | Copy detail content to clipboard. Header shows `Copied!`. |
| B.6.3 | Press `Esc`. | Return to AMI list with the same AMI still selected. |
| B.6.4 | Press `?`. | Help screen appears. |
| B.6.5 | Press `Ctrl+R`. | Detail data refreshes from AWS. |

---

## C. AMI YAML View

### C.1 Entry and Frame

| ID | Story | Expected |
|----|-------|----------|
| C.1.1 | Press `y` from the AMI list or detail view. | YAML view opens for the selected AMI. |
| C.1.2 | Frame title shows the AMI ID plus "yaml" suffix. | Top border reads e.g., `ami-0abc123def456789a yaml` centered between dashes. |

### C.2 Content

| ID | Story | Expected |
|----|-------|----------|
| C.2.1 | YAML view shows the complete AMI object. | All fields from the `describe-images` response for this AMI are present, including ImageId, Name, State, Architecture, PlatformDetails, RootDeviceType, BlockDeviceMappings, Tags, etc. |
| C.2.2 | BlockDeviceMappings renders as a YAML list. | Each block device mapping appears as a YAML list item with nested Ebs properties (SnapshotId, VolumeSize, VolumeType, DeleteOnTermination, Encrypted). |
| C.2.3 | Tags render as a YAML list. | Each tag appears as a list item with Key and Value fields. |

**AWS comparison:**
```
aws ec2 describe-images --image-ids ami-0abc123def456789a --output yaml
```

### C.3 Syntax Coloring

| ID | Story | Expected |
|----|-------|----------|
| C.3.1 | YAML keys are colored blue (`#7aa2f7`). | Every key name (e.g., `ImageId:`, `Architecture:`) renders in blue. |
| C.3.2 | String values are colored green (`#9ece6a`). | String values (e.g., `ami-0abc123def456789a`, `x86_64`) render in green. |
| C.3.3 | Boolean values are colored purple (`#bb9af7`). | Boolean values (`true`, `false`) for fields like `Public` and `EbsOptimized` render in purple. |
| C.3.4 | Numeric values are colored orange (`#ff9e64`). | Numeric values (e.g., volume sizes like `8`, `100`) render in orange. |
| C.3.5 | Null values are colored dim (`#565f89`). | Null/nil values render in dim gray. |

### C.4 Actions from YAML

| ID | Story | Expected |
|----|-------|----------|
| C.4.1 | Press `Esc`. | Return to previous view (list or detail, depending on entry point). |
| C.4.2 | Press `c`. | Copy YAML content to clipboard. Header shows `Copied!`. |
| C.4.3 | Press `?`. | Help screen appears. |
| C.4.4 | Press `Ctrl+R`. | YAML content refreshes from AWS. |

---

## D. Child View: AMI -> EBS Snapshots

This child view shows the EBS snapshots that back the selected AMI, derived from its BlockDeviceMappings.

### D.1 Entry and Navigation

| ID | Story | Expected |
|----|-------|----------|
| D.1.1 | I select an AMI in the list and press the child-view trigger key (e.g., `e` or Enter). | The view transitions to a list of EBS snapshots associated with this AMI's block device mappings. A loading spinner appears while data is fetched. |
| D.1.2 | Frame title includes the AMI context. | The frame title shows the child view type and the parent AMI name or ID (e.g., `ebs-snapshots(3) -- ami-0abc123`). |
| D.1.3 | Press `Esc`. | Return to the AMI list with the same AMI still selected. |

**AWS comparison:**
```
aws ec2 describe-images --image-ids ami-0abc123 --query 'Images[0].BlockDeviceMappings[].Ebs.SnapshotId'
aws ec2 describe-snapshots --snapshot-ids snap-aaa snap-bbb snap-ccc
```

### D.2 Column Layout

| ID | Story | Expected |
|----|-------|----------|
| D.2.1 | Snapshot data loads. | Columns show snapshot-relevant fields: Snapshot ID, State, Volume Size, Description, and/or Device Name. |
| D.2.2 | I verify snapshot IDs match the AMI's BlockDeviceMappings. | Each snapshot ID in the list corresponds to a SnapshotId from the AMI's block device mappings. |

### D.3 Empty State

| ID | Story | Expected |
|----|-------|----------|
| D.3.1 | The AMI has no EBS-backed block device mappings (instance-store only). | The child view shows zero rows with an appropriate message (e.g., "No EBS snapshots"). |
| D.3.2 | The AMI's block device mappings reference snapshots that no longer exist. | An error or empty result is shown gracefully. No crash. |

### D.4 Actions from Child View

| ID | Story | Expected |
|----|-------|----------|
| D.4.1 | Press `d` on a snapshot in the child list. | Detail view opens for that snapshot, showing its full properties. |
| D.4.2 | Press `y` on a snapshot. | YAML view opens for that snapshot. |
| D.4.3 | Press `c` on a snapshot. | The snapshot ID is copied to clipboard. |

---

## E. Child View: AMI -> EC2 Instances

This child view shows EC2 instances that were launched from the selected AMI.

### E.1 Entry and Navigation

| ID | Story | Expected |
|----|-------|----------|
| E.1.1 | I select an AMI in the list and press the appropriate child-view trigger key. | The view transitions to a list of EC2 instances launched from this AMI. A loading spinner appears while data is fetched. |
| E.1.2 | Frame title includes the AMI context. | The frame title shows the child view type and the parent AMI name or ID (e.g., `ec2-instances(5) -- ami-0abc123`). |
| E.1.3 | Press `Esc`. | Return to the AMI list with the same AMI still selected. |

**AWS comparison:**
```
aws ec2 describe-instances --filters "Name=image-id,Values=ami-0abc123" --query 'Reservations[].Instances[].{ID:InstanceId,State:State.Name}'
```

### E.2 Column Layout

| ID | Story | Expected |
|----|-------|----------|
| E.2.1 | Instance data loads. | Columns show EC2-relevant fields: Instance ID (or Name), State, Type, Private IP, etc. |
| E.2.2 | I verify all listed instances use the parent AMI. | Every instance in the list has ImageId matching the parent AMI's ID. |

### E.3 Status Coloring

| ID | Story | Expected |
|----|-------|----------|
| E.3.1 | A launched instance has State `running`. | The row renders in GREEN (`#9ece6a`). |
| E.3.2 | A launched instance has State `stopped`. | The row renders in RED (`#f7768e`). |
| E.3.3 | A launched instance has State `terminated`. | The row renders in DIM (`#565f89`). |

### E.4 Empty State

| ID | Story | Expected |
|----|-------|----------|
| E.4.1 | No running or recent instances were launched from this AMI. | The child view shows zero rows with a message (e.g., "No instances found"). This is a normal state for old or unused AMIs. |

### E.5 Actions from Child View

| ID | Story | Expected |
|----|-------|----------|
| E.5.1 | Press `d` on an instance in the child list. | Detail view opens for that EC2 instance. |
| E.5.2 | Press `y` on an instance. | YAML view opens for that instance. |
| E.5.3 | Press `c` on an instance. | The Instance ID is copied to clipboard. |

---

## F. Cross-View Navigation Flows

| ID | Story | Expected |
|----|-------|----------|
| F.1 | Main Menu -> AMI List -> AMI Detail -> YAML -> Esc -> Detail -> Esc -> List -> Esc -> Main Menu | Each Esc pops one level from the view stack. Final Esc returns to main menu. |
| F.2 | AMI List -> YAML (via `y`) -> Esc -> AMI List | Pressing `y` from the list goes directly to YAML. Esc returns to the list, not to detail. |
| F.3 | AMI List -> AMI Detail (via `d`) -> YAML (via `y`) -> Esc -> Detail -> Esc -> List | Full three-level deep navigation and back. |
| F.4 | AMI List -> EBS Snapshots child -> Detail -> Esc -> Child -> Esc -> AMI List | Child view navigation follows the view stack pattern. |
| F.5 | AMI List -> EC2 Instances child -> Detail -> Esc -> Child -> Esc -> AMI List | Same view stack pattern for the instances child view. |
| F.6 | Select AMI A, open detail, Esc, select AMI B, open detail. | Detail shows AMI B's data, not AMI A's. |
| F.7 | AMI List with active filter -> press Enter on a filtered row -> Detail shows that AMI. | The detail view opens for the correct AMI even when the list is filtered. |

---

## G. AWS Data Fidelity

| ID | Story | Expected |
|----|-------|----------|
| G.1 | AMI count in frame title matches `aws ec2 describe-images --owners self` total. | The number of AMIs shown equals the count returned by the API. |
| G.2 | AMI IDs in the list match the AWS API response. | Every Image ID corresponds to an actual AMI returned by `describe-images`. |
| G.3 | State values match the API response exactly. | State values (`available`, `pending`, `failed`, `deregistered`) match the API. |
| G.4 | Architecture, Platform, and Root Device Type values match the API. | Values are not reformatted or altered from the API response. |
| G.5 | Creation dates correspond to the API response. | The CreationDate timestamp can be mapped back to the raw API value. |
| G.6 | Block device mapping data in detail matches the API. | Device names, snapshot IDs, and volume properties match `describe-images`. |
| G.7 | YAML output contains all fields from the API response. | No fields are silently dropped from the YAML dump. |
| G.8 | Default filter is `--owners self`. | Only AMIs owned by the current account appear by default, not all public AMIs. |
