# QA User Stories: EBS Volumes and Snapshots as New Resource Types

Covers GitHub issue #66: adding EBS Volumes and EBS Snapshots as two new top-level resource types with list, detail, YAML views, and child views for cross-resource navigation.

All stories are written from a black-box perspective against the design spec and `views.yaml`. AWS CLI equivalents are cited so testers can verify data parity.

---

## A. EBS Volumes List View

### A.1 Main Menu Entry

| ID | Story | Expected |
|----|-------|----------|
| A.1.1 | I launch a9s and view the main menu. | A new entry for EBS Volumes (e.g., "EBS Volumes") appears in the resource type list with a command alias (e.g., `:ebs` or `:vol`). |
| A.1.2 | I select the EBS Volumes entry and press Enter. | The view transitions to the EBS Volumes list view. A loading spinner appears while data is fetched. |
| A.1.3 | I type the command alias (e.g., `:ebs`) in command mode and press Enter. | The view navigates directly to the EBS Volumes list from any other view. |

**AWS comparison:**
```
aws ec2 describe-volumes --query 'Volumes[].{Name:Tags[?Key==`Name`]|[0].Value,VolumeId:VolumeId,State:State}'
```

### A.2 Column Layout

| ID | Story | Expected |
|----|-------|----------|
| A.2.1 | Volume data loads and the table renders. | Ten columns appear: Name, Volume ID, State, Size (GiB), Type, IOPS, Encrypted, Attached To, AZ, Created. Column headers are bold blue (`#7aa2f7`), with no separator line below them. |
| A.2.2 | I verify the Name column data. | Each cell shows the value of the `Name` tag (e.g., `prod-data-vol`, `jenkins-home`). |
| A.2.3 | I verify the Volume ID column data. | Each cell shows the `Volume.VolumeId` field value (e.g., `vol-0abc123def456789a`). |
| A.2.4 | I verify the State column data. | Each cell shows the `Volume.State` field value (e.g., `in-use`, `available`, `creating`, `deleting`, `error`). |
| A.2.5 | I verify the Size (GiB) column data. | Each cell shows the `Volume.Size` field value (e.g., `100`, `500`, `1000`). |
| A.2.6 | I verify the Type column data. | Each cell shows the `Volume.VolumeType` field value (e.g., `gp3`, `gp2`, `io2`, `io1`, `st1`, `sc1`, `standard`). |
| A.2.7 | I verify the IOPS column data. | Each cell shows the `Volume.Iops` field value (e.g., `3000`, `16000`). |
| A.2.8 | I verify the Encrypted column data. | Each cell shows the `Volume.Encrypted` field value (`true` or `false`). |
| A.2.9 | I verify the Attached To column data. | Each cell shows the `Volume.Attachments[0].InstanceId` field value (e.g., `i-0abc123def456789a`). Unattached volumes show a blank cell. |
| A.2.10 | I verify the AZ column data. | Each cell shows the `Volume.AvailabilityZone` field value (e.g., `us-east-1a`). |
| A.2.11 | I verify the Created column data. | Each cell shows the `Volume.CreateTime` field value formatted as a readable timestamp. |
| A.2.12 | Columns are space-aligned, not pipe-separated. | Columns are padded with whitespace. No vertical pipe characters between columns. |

**AWS comparison:**
```
aws ec2 describe-volumes --query 'Volumes[].{Name:Tags[?Key==`Name`]|[0].Value,VolumeId:VolumeId,State:State,Size:Size,Type:VolumeType,IOPS:Iops,Encrypted:Encrypted,AttachedTo:Attachments[0].InstanceId,AZ:AvailabilityZone,Created:CreateTime}'
```
Expected fields visible: Name, Volume ID, State, Size (GiB), Type, IOPS, Encrypted, Attached To, AZ, Created

### A.3 Frame and Title

| ID | Story | Expected |
|----|-------|----------|
| A.3.1 | Volume data loads successfully. | The frame title shows the resource type short name and count, e.g., `ebs-volumes(47)` centered between dashes. |
| A.3.2 | The frame title count matches total volumes. | The number in parentheses equals the total count of volumes returned by `describe-volumes`. |
| A.3.3 | Frame uses thin box-drawing characters. | Standard frame style consistent with all other resource lists. Border color is dim (`#414868`). |

### A.4 Loading State

| ID | Story | Expected |
|----|-------|----------|
| A.4.1 | While volume data is being fetched. | Frame title shows the resource short name (no count yet). A spinner and loading message appear centered in the frame. |
| A.4.2 | Spinner is animated. | The spinner dot pattern cycles visually in blue (`#7aa2f7`). |
| A.4.3 | After data loads, spinner is replaced by the table. | Column headers and data rows appear. Frame title updates with the volume count. |

### A.5 Status Coloring (Entire Row)

| ID | Story | Expected |
|----|-------|----------|
| A.5.1 | A volume has State `in-use`. | The entire row renders in GREEN (`#9ece6a`). |
| A.5.2 | A volume has State `available`. | The entire row renders in YELLOW (`#e0af68`). This signals a potential orphaned volume (not attached to any instance). |
| A.5.3 | A volume has State `creating`. | The entire row renders in YELLOW (`#e0af68`) or Cyan, indicating a transitional state. |
| A.5.4 | A volume has State `deleting`. | The entire row renders in RED (`#f7768e`). |
| A.5.5 | A volume has State `error`. | The entire row renders in RED (`#f7768e`). |
| A.5.6 | I select a colored row. | The selected row shows full-width blue background (`#7aa2f7`) with dark foreground (`#1a1b26`), overriding the status color. |
| A.5.7 | An account has volumes in `in-use`, `available`, and `deleting` states. | Looking at the list, each row is visually distinct: in-use volumes are green, available volumes are yellow (orphan warning), and deleting volumes are red. |

**AWS comparison:**
```
aws ec2 describe-volumes --query 'Volumes[].{ID:VolumeId,State:State}'
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
| A.6.7 | Press `h` or Left arrow. | Columns scroll left (useful since 10 columns will exceed most terminal widths). |
| A.6.8 | Press `l` or Right arrow. | Columns scroll right, revealing hidden columns like AZ and Created. |
| A.6.9 | On initial load, the first row is selected. | Cursor starts at the topmost row. |
| A.6.10 | Press `pgdn` or `ctrl+d`. | Cursor moves down one page. |
| A.6.11 | Press `pgup` or `ctrl+u`. | Cursor moves up one page. |

### A.7 Sort

| ID | Story | Expected |
|----|-------|----------|
| A.7.1 | Press `N`. | Rows sort by name (Name column) ascending. The Name column header shows an up arrow indicator. |
| A.7.2 | Press `N` again. | Sort toggles to descending. The indicator changes to a down arrow. |
| A.7.3 | Press `S`. | Rows sort by status (State column) ascending. |
| A.7.4 | Press `A`. | Rows sort by age (Created column) ascending (oldest first). |
| A.7.5 | Press `A` again. | Sort toggles to descending (newest first). |
| A.7.6 | Sort indicator appears on exactly one column. | Switching from N-sort to S-sort removes the indicator from Name and adds it to State. |

### A.8 Filter

| ID | Story | Expected |
|----|-------|----------|
| A.8.1 | Press `/` and type `available`. | Only volumes with State `available` appear. These are potential orphans. Frame title shows filtered count. |
| A.8.2 | Press `/` and type `gp3`. | Only volumes with Type `gp3` appear. |
| A.8.3 | Press `/` and type an instance ID prefix. | Only volumes attached to instances matching the filter appear. |
| A.8.4 | Press `/` and type a volume name. | Only volumes whose Name tag matches the filter appear. |
| A.8.5 | Press `Esc` during filter. | Filter is cleared. All volumes are visible again. |
| A.8.6 | Filter with no matches. | Zero rows displayed. Frame title shows e.g., `ebs-volumes(0/47)`. |

### A.9 Actions from List

| ID | Story | Expected |
|----|-------|----------|
| A.9.1 | Press `Enter` or `d` on a selected volume. | Detail view opens for that volume. |
| A.9.2 | Press `y` on a selected volume. | YAML view opens for that volume. |
| A.9.3 | Press `c` on a selected volume. | The Volume ID (e.g., `vol-0abc123def456789a`) is copied to clipboard. Header right briefly shows `Copied!` in green. |
| A.9.4 | Press `Esc`. | View navigates back to the main menu. |
| A.9.5 | Press `?`. | Help screen appears. |
| A.9.6 | Press `Ctrl+R`. | Data refreshes from AWS. |

### A.10 Empty State and Edge Cases

| ID | Story | Expected |
|----|-------|----------|
| A.10.1 | AWS account has zero EBS volumes. | Frame title shows `ebs-volumes(0)`. Content area shows a centered message. |
| A.10.2 | A volume has no Name tag. | The Name cell is blank. The volume still appears in the list with its Volume ID visible. |
| A.10.3 | A volume is not attached to any instance (Attachments is empty). | The Attached To cell is blank. |
| A.10.4 | A volume has IOPS of null (e.g., for magnetic/standard volumes). | The IOPS cell is blank or shows a dash. No crash. |
| A.10.5 | A volume with a very long Name tag. | The name is truncated to fit the column width. |
| A.10.6 | Account has hundreds of volumes. | All volumes load. Vertical scrolling works. No truncation. |
| A.10.7 | API error (e.g., AccessDenied for ec2:DescribeVolumes). | A red error flash appears in the header. The list is empty. The application does not crash. |

---

## B. EBS Volumes Detail View

### B.1 Entry and Frame

| ID | Story | Expected |
|----|-------|----------|
| B.1.1 | Press `Enter` or `d` on a volume in the list. | Detail view opens. The frame title shows the Volume ID (e.g., `vol-0abc123def456789a`) centered in the top border. |
| B.1.2 | Frame replaces the table in the same layout position. | The detail view occupies the same frame area below the header. |
| B.1.3 | Header bar is unchanged. | Left side still shows `a9s v0.x.x  profile:region`. Right side shows `? for help`. |

### B.2 Fields Displayed

| ID | Story | Expected |
|----|-------|----------|
| B.2.1 | VolumeId field is shown. | A line reads with key `VolumeId` and the volume ID. |
| B.2.2 | State field is shown. | A line reads with key `State` and the state value. |
| B.2.3 | Size field is shown. | A line reads with key `Size` and the volume size in GiB. |
| B.2.4 | VolumeType field is shown. | A line reads with key `VolumeType` and the type (e.g., `gp3`). |
| B.2.5 | Iops field is shown. | A line reads with key `Iops` and the IOPS value. |
| B.2.6 | Encrypted field is shown. | A line reads with key `Encrypted` and `true` or `false`. |
| B.2.7 | AvailabilityZone field is shown. | A line reads with key `AvailabilityZone` and the AZ. |
| B.2.8 | Attachments field is shown. | A section showing attachment details, including InstanceId, Device, and State. |
| B.2.9 | CreateTime field is shown. | A line reads with key `CreateTime` and the creation timestamp. |
| B.2.10 | Tags field is shown. | A section showing tag key-value pairs. |
| B.2.11 | Throughput field is shown (for gp3 volumes). | A line reads with key `Throughput` and the throughput value. |
| B.2.12 | KmsKeyId field is shown (for encrypted volumes). | A line reads with key `KmsKeyId` and the KMS key ARN. |

**AWS comparison:**
```
aws ec2 describe-volumes --volume-ids vol-0abc123def456789a --query 'Volumes[0]'
```

### B.3 Key-Value Formatting

| ID | Story | Expected |
|----|-------|----------|
| B.3.1 | Keys are styled in blue (`#7aa2f7`). | Every field label renders in blue. |
| B.3.2 | Values are styled in plain white (`#c0caf5`). | Every field value renders in the default color. |
| B.3.3 | State value `in-use` is colored green. | The value text renders in green (`#9ece6a`). |
| B.3.4 | State value `available` is colored yellow. | The value text renders in yellow (`#e0af68`). |
| B.3.5 | State value `error` is colored red. | The value text renders in red (`#f7768e`). |

### B.4 Attachments Array

| ID | Story | Expected |
|----|-------|----------|
| B.4.1 | Volume is attached to one instance. | Attachments shows a single entry with InstanceId, Device (e.g., `/dev/sda1`), State (e.g., `attached`), AttachTime, and DeleteOnTermination. |
| B.4.2 | Volume is not attached (Attachments is empty). | Attachments section shows an empty state or is absent. No crash. |
| B.4.3 | Volume has multiple attachments (multi-attach enabled). | Each attachment appears on its own line(s) with proper indentation. |

### B.5 Scrolling and Actions

| ID | Story | Expected |
|----|-------|----------|
| B.5.1 | Press `j`/`k`/`g`/`G`. | Standard scrolling behavior. |
| B.5.2 | Press `w`. | Word wrap toggles. |
| B.5.3 | Press `y`. | Switch to YAML view. |
| B.5.4 | Press `c`. | Copy detail content to clipboard. |
| B.5.5 | Press `Esc`. | Return to the volumes list. |
| B.5.6 | Press `Ctrl+R`. | Detail refreshes from AWS. |

---

## C. EBS Volumes YAML View

### C.1 Entry and Content

| ID | Story | Expected |
|----|-------|----------|
| C.1.1 | Press `y` from the volumes list or detail view. | YAML view opens for the selected volume. |
| C.1.2 | Frame title shows the Volume ID plus "yaml" suffix. | Top border reads e.g., `vol-0abc123def456789a yaml`. |
| C.1.3 | YAML view shows the complete volume object. | All fields from `describe-volumes` are present: VolumeId, State, Size, VolumeType, Iops, Encrypted, AvailabilityZone, Attachments, CreateTime, Tags, Throughput, MultiAttachEnabled, etc. |

**AWS comparison:**
```
aws ec2 describe-volumes --volume-ids vol-0abc123def456789a --output yaml
```

### C.2 Syntax Coloring

| ID | Story | Expected |
|----|-------|----------|
| C.2.1 | `Size: 100` | Key `Size` is blue; value `100` is orange (numeric). |
| C.2.2 | `Encrypted: true` | Key `Encrypted` is blue; value `true` is purple (boolean). |
| C.2.3 | `VolumeType: gp3` | Key `VolumeType` is blue; value `gp3` is green (string). |
| C.2.4 | `KmsKeyId: null` (unencrypted volume) | Key is blue; value `null` is dim gray. |

### C.3 Actions

| ID | Story | Expected |
|----|-------|----------|
| C.3.1 | Press `Esc`. | Return to previous view. |
| C.3.2 | Press `c`. | Copy YAML content to clipboard. |
| C.3.3 | Press `Ctrl+R`. | YAML refreshes from AWS. |

---

## D. Child View: EBS Volume -> Snapshots

This child view shows snapshots created from the selected volume.

### D.1 Entry and Navigation

| ID | Story | Expected |
|----|-------|----------|
| D.1.1 | I select a volume in the list and press the child-view trigger key (e.g., `e`). | The view transitions to a list of EBS snapshots created from this volume. A loading spinner appears while data is fetched. |
| D.1.2 | Frame title includes the volume context. | The frame title shows the child type and the parent volume ID or name (e.g., `ebs-snapshots(5) -- vol-0abc123`). |
| D.1.3 | Press `Esc`. | Return to the volumes list with the same volume still selected. |

**AWS comparison:**
```
aws ec2 describe-snapshots --filters "Name=volume-id,Values=vol-0abc123" --owner-ids self
```

### D.2 Column Layout

| ID | Story | Expected |
|----|-------|----------|
| D.2.1 | Snapshot data loads. | Columns show snapshot fields: Name, Snapshot ID, State, Volume ID, Size (GiB), Encrypted, Description, Started, Progress. |
| D.2.2 | I verify all listed snapshots have Volume ID matching the parent. | Every snapshot in the child list was created from the parent volume. |

### D.3 Status Coloring

| ID | Story | Expected |
|----|-------|----------|
| D.3.1 | A snapshot has State `completed`. | The row renders in GREEN (`#9ece6a`). |
| D.3.2 | A snapshot has State `pending`. | The row renders in YELLOW (`#e0af68`). |
| D.3.3 | A snapshot has State `error`. | The row renders in RED (`#f7768e`). |

### D.4 Empty State

| ID | Story | Expected |
|----|-------|----------|
| D.4.1 | No snapshots were created from this volume. | The child view shows zero rows with an appropriate message (e.g., "No snapshots found"). |

### D.5 Actions from Child View

| ID | Story | Expected |
|----|-------|----------|
| D.5.1 | Press `d` on a snapshot in the child list. | Detail view opens for that snapshot. |
| D.5.2 | Press `y` on a snapshot. | YAML view opens for that snapshot. |
| D.5.3 | Press `c` on a snapshot. | The Snapshot ID is copied to clipboard. |

---

## E. Child View: EBS Volume -> Attached Instance

This child view allows jumping to the EC2 instance that the volume is attached to.

### E.1 Entry and Navigation

| ID | Story | Expected |
|----|-------|----------|
| E.1.1 | I select an in-use volume and press the child-view trigger key for the attached instance. | The view transitions to show the EC2 instance that this volume is attached to. |
| E.1.2 | Frame title includes the instance context. | The frame title shows the instance ID. |
| E.1.3 | Press `Esc`. | Return to the volumes list. |

**AWS comparison:**
```
aws ec2 describe-instances --instance-ids i-0abc123 --query 'Reservations[].Instances[].{ID:InstanceId,State:State.Name}'
```

### E.2 Edge Cases

| ID | Story | Expected |
|----|-------|----------|
| E.2.1 | I try to navigate to the attached instance of an unattached (available) volume. | The navigation does not occur (the child view trigger is disabled or shows an error), since there is no attached instance. No crash. |
| E.2.2 | The attached instance has been terminated but the volume still references it. | An error or empty result is shown gracefully. No crash. |

---

## F. EBS Snapshots List View

### F.1 Main Menu Entry

| ID | Story | Expected |
|----|-------|----------|
| F.1.1 | I launch a9s and view the main menu. | A new entry for EBS Snapshots (e.g., "EBS Snapshots") appears with a command alias (e.g., `:snap`). |
| F.1.2 | I select the EBS Snapshots entry and press Enter. | The view transitions to the EBS Snapshots list view. A loading spinner appears. |
| F.1.3 | I type the command alias (e.g., `:snap`) in command mode and press Enter. | The view navigates directly to the snapshots list. |

**AWS comparison:**
```
aws ec2 describe-snapshots --owner-ids self --query 'Snapshots[].{Name:Tags[?Key==`Name`]|[0].Value,SnapshotId:SnapshotId,State:State}'
```

### F.2 Column Layout

| ID | Story | Expected |
|----|-------|----------|
| F.2.1 | Snapshot data loads and the table renders. | Nine columns appear: Name, Snapshot ID, State, Volume ID, Size (GiB), Encrypted, Description, Started, Progress. Column headers are bold blue (`#7aa2f7`). |
| F.2.2 | I verify the Name column data. | Each cell shows the value of the `Name` tag. |
| F.2.3 | I verify the Snapshot ID column data. | Each cell shows the `Snapshot.SnapshotId` field value (e.g., `snap-0abc123def456789a`). |
| F.2.4 | I verify the State column data. | Each cell shows the `Snapshot.State` field value (e.g., `completed`, `pending`, `error`, `recoverable`, `recovering`). |
| F.2.5 | I verify the Volume ID column data. | Each cell shows the `Snapshot.VolumeId` field value (e.g., `vol-0abc123`). |
| F.2.6 | I verify the Size (GiB) column data. | Each cell shows the `Snapshot.VolumeSize` field value. |
| F.2.7 | I verify the Encrypted column data. | Each cell shows `true` or `false`. |
| F.2.8 | I verify the Description column data. | Each cell shows the `Snapshot.Description` field value. |
| F.2.9 | I verify the Started column data. | Each cell shows the `Snapshot.StartTime` field value formatted as a readable timestamp. |
| F.2.10 | I verify the Progress column data. | Each cell shows the `Snapshot.Progress` field value (e.g., `100%`, `45%`). |
| F.2.11 | Columns are space-aligned, not pipe-separated. | Standard column formatting. |

**AWS comparison:**
```
aws ec2 describe-snapshots --owner-ids self --query 'Snapshots[].{Name:Tags[?Key==`Name`]|[0].Value,SnapshotId:SnapshotId,State:State,VolumeId:VolumeId,Size:VolumeSize,Encrypted:Encrypted,Description:Description,Started:StartTime,Progress:Progress}'
```
Expected fields visible: Name, Snapshot ID, State, Volume ID, Size (GiB), Encrypted, Description, Started, Progress

### F.3 Frame and Title

| ID | Story | Expected |
|----|-------|----------|
| F.3.1 | Snapshot data loads successfully. | Frame title shows the resource type and count, e.g., `ebs-snapshots(23)` centered between dashes. |
| F.3.2 | The frame title count matches total self-owned snapshots. | The number in parentheses equals the count returned by `describe-snapshots --owner-ids self`. |

### F.4 Loading State

| ID | Story | Expected |
|----|-------|----------|
| F.4.1 | While snapshot data is being fetched. | Frame title shows the resource short name (no count). Spinner and loading message centered in frame. |
| F.4.2 | After data loads. | Column headers and rows appear. Frame title updates with count. |

### F.5 Status Coloring (Entire Row)

| ID | Story | Expected |
|----|-------|----------|
| F.5.1 | A snapshot has State `completed`. | The entire row renders in GREEN (`#9ece6a`). |
| F.5.2 | A snapshot has State `pending`. | The entire row renders in YELLOW (`#e0af68`). |
| F.5.3 | A snapshot has State `error`. | The entire row renders in RED (`#f7768e`). |
| F.5.4 | A snapshot has State `recoverable`. | The entire row renders in YELLOW (`#e0af68`) or an appropriate transitional color. |
| F.5.5 | A snapshot has State `recovering`. | The entire row renders in YELLOW (`#e0af68`). |
| F.5.6 | I select a colored row. | The selected row shows full-width blue background, overriding the status color. |
| F.5.7 | An account has snapshots in `completed`, `pending`, and `error` states. | Each row is colored by its state: completed are green, pending are yellow, error are red. |

### F.6 Row Selection and Navigation

| ID | Story | Expected |
|----|-------|----------|
| F.6.1 | Press `j`/`k`/`g`/`G`. | Standard vertical navigation with wrap. |
| F.6.2 | Press `h`/`l`. | Horizontal column scrolling (9 columns will exceed most terminal widths). |
| F.6.3 | Press `pgdn`/`pgup` or `ctrl+d`/`ctrl+u`. | Page up/down navigation. |

### F.7 Sort

| ID | Story | Expected |
|----|-------|----------|
| F.7.1 | Press `N`. | Rows sort by name ascending. |
| F.7.2 | Press `S`. | Rows sort by state ascending. |
| F.7.3 | Press `A`. | Rows sort by age (Started) ascending. |

### F.8 Filter

| ID | Story | Expected |
|----|-------|----------|
| F.8.1 | Press `/` and type `completed`. | Only completed snapshots appear. |
| F.8.2 | Press `/` and type a volume ID prefix. | Only snapshots from volumes matching the filter appear. |
| F.8.3 | Press `/` and type `pending`. | Only pending snapshots appear (useful for monitoring in-progress snapshots). |
| F.8.4 | Press `Esc` during filter. | Filter cleared. All snapshots visible. |

### F.9 Actions from List

| ID | Story | Expected |
|----|-------|----------|
| F.9.1 | Press `Enter` or `d` on a selected snapshot. | Detail view opens for that snapshot. |
| F.9.2 | Press `y` on a selected snapshot. | YAML view opens for that snapshot. |
| F.9.3 | Press `c` on a selected snapshot. | The Snapshot ID is copied to clipboard. Header shows `Copied!`. |
| F.9.4 | Press `Esc`. | View navigates back to the main menu. |
| F.9.5 | Press `?`. | Help screen appears. |
| F.9.6 | Press `Ctrl+R`. | Data refreshes from AWS. |

### F.10 Empty State and Edge Cases

| ID | Story | Expected |
|----|-------|----------|
| F.10.1 | AWS account has zero self-owned snapshots. | Frame title shows `ebs-snapshots(0)`. Content shows a centered message. |
| F.10.2 | A snapshot has no Name tag. | The Name cell is blank. The snapshot is still visible via its Snapshot ID. |
| F.10.3 | A snapshot's source volume has been deleted. | The Volume ID column still shows the original volume ID (even if that volume no longer exists). |
| F.10.4 | A snapshot has Progress `100%`. | The Progress column shows `100%`. |
| F.10.5 | A snapshot has Progress `45%` (still in progress). | The Progress column shows `45%`. The State column shows `pending`. The row is YELLOW. |
| F.10.6 | A snapshot has a very long Description. | The description is truncated to fit the column width. |
| F.10.7 | Account has hundreds of snapshots. | All load. Scrolling works. No truncation of the list. |
| F.10.8 | API error (e.g., AccessDenied). | Red error flash in header. Empty list. No crash. |
| F.10.9 | Default filter is `--owner-ids self`. | Only self-owned snapshots appear, not public or shared snapshots. |

---

## G. EBS Snapshots Detail View

### G.1 Entry and Frame

| ID | Story | Expected |
|----|-------|----------|
| G.1.1 | Press `Enter` or `d` on a snapshot in the list. | Detail view opens. Frame title shows the Snapshot ID. |
| G.1.2 | Header bar is unchanged. | Left side: `a9s v0.x.x  profile:region`. Right side: `? for help`. |

### G.2 Fields Displayed

| ID | Story | Expected |
|----|-------|----------|
| G.2.1 | SnapshotId field is shown. | Key `SnapshotId`, value e.g., `snap-0abc123def456789a`. |
| G.2.2 | State field is shown. | Key `State`, value e.g., `completed`. |
| G.2.3 | VolumeId field is shown. | Key `VolumeId`, value e.g., `vol-0abc123`. |
| G.2.4 | VolumeSize field is shown. | Key `VolumeSize`, value e.g., `100`. |
| G.2.5 | Encrypted field is shown. | Key `Encrypted`, value `true` or `false`. |
| G.2.6 | Description field is shown. | Key `Description`, value shows the snapshot description. |
| G.2.7 | StartTime field is shown. | Key `StartTime`, value shows the timestamp. |
| G.2.8 | Progress field is shown. | Key `Progress`, value e.g., `100%`. |
| G.2.9 | OwnerId field is shown. | Key `OwnerId`, value shows the AWS account ID. |
| G.2.10 | Tags field is shown. | A section showing tag key-value pairs. |
| G.2.11 | KmsKeyId field is shown (for encrypted snapshots). | Key `KmsKeyId`, value shows the KMS key ARN. |

**AWS comparison:**
```
aws ec2 describe-snapshots --snapshot-ids snap-0abc123def456789a --query 'Snapshots[0]'
```

### G.3 Key-Value Formatting

| ID | Story | Expected |
|----|-------|----------|
| G.3.1 | Keys are styled in blue (`#7aa2f7`). | Every field label renders in blue. |
| G.3.2 | Values are styled in plain white (`#c0caf5`). | Field values render in the default color. |
| G.3.3 | State value `completed` is colored green. | The value text renders in green (`#9ece6a`). |
| G.3.4 | State value `pending` is colored yellow. | The value text renders in yellow (`#e0af68`). |
| G.3.5 | State value `error` is colored red. | The value text renders in red (`#f7768e`). |

### G.4 Scrolling and Actions

| ID | Story | Expected |
|----|-------|----------|
| G.4.1 | Press `j`/`k`/`g`/`G`. | Standard scrolling behavior. |
| G.4.2 | Press `w`. | Word wrap toggles. |
| G.4.3 | Press `y`. | Switch to YAML view. |
| G.4.4 | Press `c`. | Copy detail content to clipboard. |
| G.4.5 | Press `Esc`. | Return to the snapshots list. |
| G.4.6 | Press `Ctrl+R`. | Detail refreshes from AWS. |

---

## H. EBS Snapshots YAML View

### H.1 Entry and Content

| ID | Story | Expected |
|----|-------|----------|
| H.1.1 | Press `y` from the snapshots list or detail. | YAML view opens for the selected snapshot. |
| H.1.2 | Frame title shows the Snapshot ID plus "yaml" suffix. | Top border reads e.g., `snap-0abc123def456789a yaml`. |
| H.1.3 | YAML view shows the complete snapshot object. | All fields from `describe-snapshots`: SnapshotId, State, VolumeId, VolumeSize, Encrypted, Description, StartTime, Progress, OwnerId, Tags, etc. |

**AWS comparison:**
```
aws ec2 describe-snapshots --snapshot-ids snap-0abc123def456789a --output yaml
```

### H.2 Syntax Coloring

| ID | Story | Expected |
|----|-------|----------|
| H.2.1 | `VolumeSize: 100` | Key is blue; value `100` is orange (numeric). |
| H.2.2 | `Encrypted: true` | Key is blue; value `true` is purple (boolean). |
| H.2.3 | `State: completed` | Key is blue; value `completed` is green (string). |
| H.2.4 | `Progress: 100%` | Key is blue; value `100%` is green (string). |

### H.3 Actions

| ID | Story | Expected |
|----|-------|----------|
| H.3.1 | Press `Esc`. | Return to previous view. |
| H.3.2 | Press `c`. | Copy YAML content to clipboard. |
| H.3.3 | Press `Ctrl+R`. | YAML refreshes from AWS. |

---

## I. Child View: EBS Snapshot -> AMIs

This child view shows AMIs that use the selected snapshot as a backing store.

### I.1 Entry and Navigation

| ID | Story | Expected |
|----|-------|----------|
| I.1.1 | I select a snapshot in the list and press the child-view trigger key. | The view transitions to a list of AMIs that reference this snapshot in their BlockDeviceMappings. A loading spinner appears. |
| I.1.2 | Frame title includes the snapshot context. | The frame title shows the child type and the parent snapshot ID or name (e.g., `ami(2) -- snap-0abc123`). |
| I.1.3 | Press `Esc`. | Return to the snapshots list with the same snapshot still selected. |

**AWS comparison:**
```
aws ec2 describe-images --owners self --filters "Name=block-device-mapping.snapshot-id,Values=snap-0abc123"
```

### I.2 Column Layout

| ID | Story | Expected |
|----|-------|----------|
| I.2.1 | AMI data loads. | Columns show AMI fields: Name, Image ID, State, Architecture, Platform, Creation Date. |
| I.2.2 | I verify all listed AMIs reference the parent snapshot. | Every AMI in the list has the parent snapshot ID in its BlockDeviceMappings. |

### I.3 Empty State

| ID | Story | Expected |
|----|-------|----------|
| I.3.1 | No AMIs use this snapshot as a backing store. | The child view shows zero rows with a message (e.g., "No AMIs found"). This is a normal state for orphaned snapshots. |

### I.4 Actions from Child View

| ID | Story | Expected |
|----|-------|----------|
| I.4.1 | Press `d` on an AMI in the child list. | Detail view opens for that AMI. |
| I.4.2 | Press `y` on an AMI. | YAML view opens for that AMI. |
| I.4.3 | Press `c` on an AMI. | The AMI ID is copied to clipboard. |

---

## J. Cross-Resource Navigation Flows

| ID | Story | Expected |
|----|-------|----------|
| J.1 | Main Menu -> EBS Volumes -> Volume Detail -> YAML -> Esc -> Detail -> Esc -> List -> Esc -> Main Menu | Each Esc pops one level from the view stack. |
| J.2 | Main Menu -> EBS Snapshots -> Snapshot Detail -> YAML -> Esc -> Detail -> Esc -> List -> Esc -> Main Menu | Same view stack pattern for snapshots. |
| J.3 | EBS Volumes -> Snapshots child -> Snapshot Detail -> Esc -> Child -> Esc -> Volumes list | Child view navigation follows view stack. |
| J.4 | EBS Snapshots -> AMIs child -> AMI Detail -> Esc -> Child -> Esc -> Snapshots list | Child view navigation follows view stack. |
| J.5 | EBS Volumes -> Attached Instance child -> Instance Detail -> Esc -> Volumes list | Cross-resource jump to EC2 instance works. |
| J.6 | AMI List (issue #65) -> EBS Snapshots child -> Snapshot Detail -> Esc -> Child -> Esc -> AMI list | AMI-to-snapshot navigation works both directions. |
| J.7 | EBS Volumes list with active filter -> press Enter on a filtered row -> Detail shows that volume | Detail opens for the correct volume even when filtered. |
| J.8 | Select Volume A, open detail, Esc, select Volume B, open detail | Detail shows Volume B's data, not Volume A's. |

---

## K. AWS Data Fidelity

### K.1 EBS Volumes

| ID | Story | Expected |
|----|-------|----------|
| K.1.1 | Volume count in frame title matches `aws ec2 describe-volumes` total. | The number shown equals the API count. |
| K.1.2 | Volume IDs in the list match the AWS API response. | Every Volume ID corresponds to an actual volume. |
| K.1.3 | State values match the API exactly. | States (`in-use`, `available`, `creating`, `deleting`, `error`) match the API. |
| K.1.4 | Size, Type, IOPS, and Encrypted values match the API. | Values are not reformatted or altered. |
| K.1.5 | Attachment data in detail matches the API. | Instance IDs, device names, and attachment states match `describe-volumes`. |
| K.1.6 | YAML output contains all fields. | No fields are silently dropped from the YAML dump. |

### K.2 EBS Snapshots

| ID | Story | Expected |
|----|-------|----------|
| K.2.1 | Snapshot count in frame title matches `aws ec2 describe-snapshots --owner-ids self` total. | The number shown equals the API count. |
| K.2.2 | Snapshot IDs in the list match the AWS API response. | Every Snapshot ID corresponds to an actual snapshot. |
| K.2.3 | State values match the API exactly. | States (`completed`, `pending`, `error`, `recoverable`, `recovering`) match the API. |
| K.2.4 | Volume ID, Size, and Progress values match the API. | Values are not reformatted. |
| K.2.5 | YAML output contains all fields. | No fields are silently dropped. |
| K.2.6 | Default filter is `--owner-ids self`. | Only self-owned snapshots appear, not shared/public snapshots. |
