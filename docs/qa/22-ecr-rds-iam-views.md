# QA User Stories: ECR Images, RDS Events, IAM Role Policies (Child Views)

Covers three child views that open when pressing Enter on a parent resource:
- ECR Repositories --> ECR Images
- RDS Instances --> RDS Events
- IAM Roles --> Attached Policies

All stories are written from a black-box perspective against the design spec and
`views.yaml` configuration. AWS CLI equivalents are cited so testers can verify data parity.

---

## A. ECR Images View

### A.1 Loading State

| ID | Story | Expected |
|----|-------|----------|
| A.1.1 | I select an ECR repository in the ECR list and press Enter. | A spinner (animated dot) is displayed centered inside the frame. The text reads "Fetching ECR images..." (or similar). The frame title shows the repository name with no count. The header shows "? for help" on the right. |
| A.1.2 | I press keys (j, k, /, N) while the spinner is visible. | No navigation or sort occurs. Keypresses are ignored or queued until data loads. The spinner continues uninterrupted. |
| A.1.3 | The API responds successfully with image data. | The spinner disappears. The table renders with column headers and rows. The frame title updates to "ecr-images(N) -- repo-name" where N is the total image count and repo-name is the repository I entered from. |
| A.1.4 | The API responds with an error (e.g., RepositoryNotFoundException, no credentials). | The spinner disappears. A red error flash message appears in the header right side. The frame content area shows an appropriate empty or error state. |

**AWS comparison:**
```
aws ecr describe-images --repository-name payment-api
```
Expected fields visible: Tag(s), Digest, Pushed At, Size, Scan Status, Findings

### A.2 Empty State

| ID | Story | Expected |
|----|-------|----------|
| A.2.1 | The API returns zero images for the repository. | The frame title reads "ecr-images(0) -- repo-name". The content area shows a centered message (e.g., "No images found") with a hint to refresh. No column headers are shown (or headers are shown with no data rows). |
| A.2.2 | I press ctrl+r on the empty state. | The loading spinner appears again while the refresh request is in flight. |

**AWS comparison:**
```
aws ecr describe-images --repository-name empty-repo
# Returns {"imageDetails": []}
```

### A.3 Column Layout

| ID | Story | Expected |
|----|-------|----------|
| A.3.1 | Images load and the table renders. | Six columns are displayed: "Tag(s)" (width 24), "Digest" (width 16), "Pushed At" (width 22), "Size" (width 12), "Scan Status" (width 14), "Findings" (width 20). Column headers are bold, colored blue (#7aa2f7), with no separator line below them. |
| A.3.2 | I verify column data against `aws ecr describe-images --repository-name REPO`. | "Tag(s)" maps to `.imageDetails[].imageTags` (comma-separated). "Digest" maps to the first 12 characters of `.imageDetails[].imageDigest` after the `sha256:` prefix. "Pushed At" maps to `.imageDetails[].imagePushedAt`. "Size" maps to `.imageDetails[].imageSizeInBytes` (displayed in human-readable format, e.g., "245 MB"). "Scan Status" maps to `.imageDetails[].imageScanStatus.status`. "Findings" maps to `.imageDetails[].imageScanFindingsSummary.findingSeverityCounts` (formatted as "0C 2H 5M"). |
| A.3.3 | An image has tags that together exceed 24 characters (e.g., "v2.3.1, latest, staging"). | The tag string is truncated to fit the 24-character column width. No row wrapping occurs. |
| A.3.4 | The terminal is narrower than the combined column widths (24+16+22+12+14+20=108 plus borders). | The rightmost column(s) are hidden (not truncated mid-value). Horizontal scroll with h/l is available to reveal hidden columns. |

**AWS comparison:**
```
aws ecr describe-images --repository-name payment-api --output table
```
Expected fields visible: Tag(s), Digest, Pushed At, Size, Scan Status, Findings

### A.4 Frame Title

| ID | Story | Expected |
|----|-------|----------|
| A.4.1 | 47 images are loaded for repository "payment-api". | The frame top border shows the title centered: "ecr-images(47) -- payment-api" with equal-length dashes on both sides. |
| A.4.2 | A filter is active and matches 5 of 47 images. | The frame title reads "ecr-images(5/47) -- payment-api". |
| A.4.3 | A filter is active and matches 0 images. | The frame title reads "ecr-images(0/47) -- payment-api". The content area is empty (no rows). |

### A.5 Navigation

| ID | Story | Expected |
|----|-------|----------|
| A.5.1 | I press j (or down-arrow) with the first image selected. | The selection cursor moves to the second image. The previously selected row loses the blue highlight. The new row gains the full-width blue background (#7aa2f7) with dark foreground (#1a1b26), bold. |
| A.5.2 | I press k (or up-arrow) with the second image selected. | The selection cursor moves back to the first image. |
| A.5.3 | I press g. | The selection jumps to the very first image in the list. |
| A.5.4 | I press G. | The selection jumps to the very last image in the list. |
| A.5.5 | I press PageDown (or ctrl+d). | The selection moves down by one page of visible rows. If fewer rows remain below than a page, the cursor lands on the last row. |
| A.5.6 | I press PageUp (or ctrl+u). | The selection moves up by one page of visible rows. If fewer rows remain above than a page, the cursor lands on the first row. |
| A.5.7 | I press j on the last row. | The behavior depends on wrap configuration. If wrapping, cursor moves to the first row. If not, cursor stays on the last row. |
| A.5.8 | There are more images than fit on screen. I scroll past the visible area. | The table scrolls to keep the selected row visible. The column headers remain in place. |

### A.6 Horizontal Scroll

| ID | Story | Expected |
|----|-------|----------|
| A.6.1 | Terminal width is 120 columns and all six columns fit within the frame. | All columns are visible. h/l does nothing visible or is a no-op. |
| A.6.2 | Terminal width is 80 columns and not all columns fit. | The rightmost columns (Scan Status, Findings) are hidden. Pressing l reveals them while hiding leftmost columns. Pressing h reverses. Column headers scroll in sync with data columns. |

### A.7 Sorting

| ID | Story | Expected |
|----|-------|----------|
| A.7.1 | I press N on the images list. | Rows are sorted by tag name in ascending order. The "Tag(s)" column header shows a sort indicator: an up-arrow appended directly (e.g., "Tag(s)^"). |
| A.7.2 | I press N again. | Sort order toggles to descending. The indicator changes to a down-arrow. |
| A.7.3 | I press A on the images list. | Rows are sorted by push date in ascending order (oldest first). The "Pushed At" column header shows the up-arrow indicator. |
| A.7.4 | I press A again. | Sort order toggles to descending (newest first). The indicator changes to a down-arrow. |
| A.7.5 | I sort by name, then apply a filter. | The filtered subset remains sorted by name. The sort indicator persists on the column header. |
| A.7.6 | I sort by name, then refresh with ctrl+r. | After data reloads, the sort order and direction are preserved. The indicator remains. |

### A.8 Filter

| ID | Story | Expected |
|----|-------|----------|
| A.8.1 | I press /. | The header right side changes from "? for help" to "/|" (amber/bold, with cursor). Filter mode is active. |
| A.8.2 | I type "v2.3" in filter mode. | The header right shows "/v2.3|". Only rows whose tag string contains "v2.3" (case-insensitive) are displayed. The frame title updates to "ecr-images(M/N) -- repo-name" where M is the matched count. |
| A.8.3 | I press Escape in filter mode. | The filter is cleared. All rows reappear. The frame title reverts to "ecr-images(N) -- repo-name". The header right reverts to "? for help". |
| A.8.4 | I type "staging" and only one image has a "staging-a1b2c3d4" tag. | Only that one image is displayed. The frame title shows "ecr-images(1/47) -- repo-name". |
| A.8.5 | I type a filter string that matches no images. | Zero rows are displayed. The frame title shows "ecr-images(0/N) -- repo-name". |

### A.9 Row Coloring — Vulnerability Severity

| ID | Story | Expected |
|----|-------|----------|
| A.9.1 | An image has CRITICAL findings (e.g., "1C 3H 12M"). | The entire row is rendered in RED (#f7768e). |
| A.9.2 | An image has HIGH findings but no CRITICAL (e.g., "0C 2H 5M"). | The entire row is rendered in YELLOW (#e0af68). |
| A.9.3 | An image has only MEDIUM or LOW findings (e.g., "0C 0H 3M"). | The entire row is rendered in PLAIN text color (#c0caf5). |
| A.9.4 | An image has zero findings of all severities (clean scan). | The entire row is rendered in PLAIN text color (#c0caf5). |
| A.9.5 | An image is untagged (no tags, shows "<untagged>"). | The entire row is rendered in DIM (#565f89). |
| A.9.6 | An image's scan status is "FAILED". | The entire row is rendered in RED (#f7768e). |
| A.9.7 | An image's scan status is "IN_PROGRESS" or "PENDING". | The entire row is rendered in YELLOW (#e0af68). |
| A.9.8 | I select a row with CRITICAL findings. | The selected row has full-width blue background (#7aa2f7) with dark foreground (#1a1b26), bold. The red coloring is overridden by the selection highlight. |
| A.9.9 | I move selection away from the CRITICAL findings row. | The row reverts to RED (#f7768e) coloring based on its finding severity. |

**AWS comparison:**
```
aws ecr describe-images --repository-name payment-api \
  --query 'imageDetails[].{tags:imageTags,scan:imageScanStatus.status,findings:imageScanFindingsSummary.findingSeverityCounts}'
```

### A.10 Multiple Tags

| ID | Story | Expected |
|----|-------|----------|
| A.10.1 | An image has multiple tags: ["v2.3.1", "latest"]. | The Tag(s) column shows "v2.3.1, latest" (comma-separated). |
| A.10.2 | An image has a single tag: ["v2.3.0"]. | The Tag(s) column shows "v2.3.0". |
| A.10.3 | An image has no tags (untagged image). | The Tag(s) column shows "<untagged>". |
| A.10.4 | An image has many tags that exceed the 24-character column width. | The combined tag string is truncated at 24 characters. The full tag list is visible in the detail view (d) or YAML view (y). |

**AWS comparison:**
```
aws ecr describe-images --repository-name payment-api \
  --query 'imageDetails[].imageTags'
```

### A.11 Digest Column

| ID | Story | Expected |
|----|-------|----------|
| A.11.1 | An image has digest "sha256:a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6". | The Digest column shows "a1b2c3d4e5f6" (first 12 characters after the "sha256:" prefix). |
| A.11.2 | The full digest is needed. | I press d or y on the image row. The detail or YAML view shows the full ImageDigest value. |

### A.12 Scan Status Variations

| ID | Story | Expected |
|----|-------|----------|
| A.12.1 | Image scanning is enabled and scan is complete. | The Scan Status column shows "COMPLETE". The Findings column shows the formatted severity counts (e.g., "0C 2H 5M"). |
| A.12.2 | Image scanning is enabled but the scan is still in progress. | The Scan Status column shows "IN_PROGRESS". The Findings column is empty or shows a dash. The entire row is YELLOW (#e0af68). |
| A.12.3 | Image scanning is not enabled for this repository. | The Scan Status column shows a dash or empty value. The Findings column also shows a dash. The row is PLAIN colored. |
| A.12.4 | The scan completed but failed. | The Scan Status column shows "FAILED". The Findings column is empty or shows a dash. The entire row is RED (#f7768e). |

**AWS comparison:**
```
aws ecr describe-images --repository-name payment-api \
  --query 'imageDetails[].{digest:imageDigest,scanStatus:imageScanStatus.status,findings:imageScanFindingsSummary}'
```

### A.13 Image Size

| ID | Story | Expected |
|----|-------|----------|
| A.13.1 | An image is 245 MB (245000000 bytes). | The Size column shows "245 MB" or equivalent human-readable format. It does NOT show raw bytes "245000000". |
| A.13.2 | An image is very large (2.1 GB). | The Size column shows "2.1 GB" or equivalent human-readable format, fitting within the 12-character column width. |
| A.13.3 | An image is small (5.2 MB). | The Size column shows "5.2 MB" or similar. |

**AWS comparison:**
```
aws ecr describe-images --repository-name payment-api \
  --query 'imageDetails[].imageSizeInBytes'
# Returns raw bytes; a9s converts to human-readable
```

### A.14 Copy Key (c) — Full Image URI

| ID | Story | Expected |
|----|-------|----------|
| A.14.1 | I select a tagged image (e.g., tag "v2.3.1" in repo "payment-api") and press c. | The full image URI is copied to the system clipboard in the format `{accountId}.dkr.ecr.{region}.amazonaws.com/payment-api:v2.3.1`. A green flash message "Copied!" appears in the header right side. |
| A.14.2 | I select an image with multiple tags (e.g., "v2.3.1, latest") and press c. | The URI uses the first tag: `{accountId}.dkr.ecr.{region}.amazonaws.com/payment-api:v2.3.1`. |
| A.14.3 | I select an untagged image (no tags) and press c. | The URI uses the digest format: `{accountId}.dkr.ecr.{region}.amazonaws.com/payment-api@sha256:a1b2c3d4e5f6...`. |
| A.14.4 | After ~2 seconds. | The "Copied!" flash message auto-clears and the header right reverts to "? for help". |
| A.14.5 | I paste from clipboard into another application. | The pasted text matches the full image URI exactly. This URI can be used directly in a Kubernetes deployment spec or ECS task definition. |

**AWS comparison:**
```
# No single CLI command returns the full URI; it is constructed from:
# Account ID: aws sts get-caller-identity --query 'Account'
# Region: current profile region
# Repository: selected repo name
# Tag/Digest: from describe-images output
# Format: {accountId}.dkr.ecr.{region}.amazonaws.com/{repo}:{tag}
```

### A.15 Detail Key (d)

| ID | Story | Expected |
|----|-------|----------|
| A.15.1 | I select an image and press d. | The detail view opens for the selected image. The frame title shows the image digest or primary tag. |
| A.15.2 | I verify the detail fields match views.yaml ecr_images detail config. | The detail view shows key-value pairs for: ImageDigest, ImageTags, ImagePushedAt, ImageSizeInBytes, ImageManifestMediaType, ArtifactMediaType, ImageScanStatus, ImageScanFindingsSummary, LastRecordedPullTime. |
| A.15.3 | I press Escape on the detail view. | I return to the ECR images list. The cursor position is preserved on the same image I had selected. |

### A.16 YAML Key (y)

| ID | Story | Expected |
|----|-------|----------|
| A.16.1 | I select an image and press y. | The YAML view opens. The frame title includes the image identifier and "yaml". The full image resource is rendered as syntax-highlighted YAML. |
| A.16.2 | The YAML content is longer than the visible area. | I can scroll with j/k/g/G. Scroll indicators appear when content extends beyond the visible area. |
| A.16.3 | I press Escape on the YAML view. | I return to the ECR images list. |

### A.17 Refresh (ctrl+r)

| ID | Story | Expected |
|----|-------|----------|
| A.17.1 | I press ctrl+r on the images list. | The loading spinner appears. A fresh `ecr:DescribeImages` call is made. When it completes, the table repopulates with current data. |
| A.17.2 | A new image was pushed since the last load. I press ctrl+r. | The new image appears in the refreshed list. The count in the frame title increments. |
| A.17.3 | I had a filter active and press ctrl+r. | The data refreshes. The filter remains applied to the new data. The frame title count updates accordingly. |

### A.18 Escape (Back)

| ID | Story | Expected |
|----|-------|----------|
| A.18.1 | I press Escape on the ECR images list. | I return to the ECR Repositories list. The cursor is on the same repository I had entered. |

### A.19 Help (?)

| ID | Story | Expected |
|----|-------|----------|
| A.19.1 | I press ? on the images list. | The help screen replaces the table content inside the frame. It displays a four-column layout with categories: ECR IMAGES, GENERAL, NAVIGATION, HOTKEYS. |
| A.19.2 | The ECR IMAGES column lists: `<esc>` Back, `<d>` Detail, `<y>` YAML, `<c>` Copy URI. | These entries match the help screen design from the child view spec. |
| A.19.3 | I press any key on the help screen. | The help screen closes and the images list table reappears. |

### A.20 Command Mode (:)

| ID | Story | Expected |
|----|-------|----------|
| A.20.1 | I press : on the images list. | The header right side changes to ":|" (amber/bold). Command mode is active. |
| A.20.2 | I type "ec2" and press Enter. | The view navigates to the EC2 instances list. The ECR context is left behind. |
| A.20.3 | I press Escape in command mode. | Command mode is cancelled. The header reverts to "? for help". The images list remains. |

### A.21 Alternating Row Colors

| ID | Story | Expected |
|----|-------|----------|
| A.21.1 | The images list has more than 2 rows. | Alternating rows have a subtle background color difference (#1e2030) for readability. Selected row always has blue background regardless. Row-level severity coloring (RED for CRITICAL, YELLOW for HIGH) takes precedence over alternating background for non-selected rows. |

### A.22 View Stack

| ID | Story | Expected |
|----|-------|----------|
| A.22.1 | Main Menu -> ECR Repos -> ECR Images -> Image Detail -> YAML; then Escape four times. | Each Escape pops one level: YAML -> Detail -> Images -> ECR Repos -> Main Menu. No state is lost at any intermediate level. |
| A.22.2 | ECR Repos -> ECR Images -> Detail (d); then Escape twice. | Detail -> Images -> ECR Repos. The cursor is still on the same repository. |

---

## B. RDS Events View

### B.1 Loading State

| ID | Story | Expected |
|----|-------|----------|
| B.1.1 | I select an RDS instance in the RDS Instances list and press Enter. | A spinner (animated dot) is displayed centered inside the frame. The text reads "Fetching RDS events..." (or similar). The frame title shows the instance name with no count. The header shows "? for help" on the right. |
| B.1.2 | I press keys (j, k, /, N) while the spinner is visible. | No navigation or sort occurs. Keypresses are ignored or queued until data loads. The spinner continues uninterrupted. |
| B.1.3 | The API responds successfully with event data. | The spinner disappears. The table renders with column headers and rows. The frame title updates to "rds-events(N) -- db-instance-name" where N is the total event count and db-instance-name is the RDS instance I entered from. |
| B.1.4 | The API responds with an error (e.g., DBInstanceNotFoundFault, no credentials). | The spinner disappears. A red error flash message appears in the header right side. The frame content area shows an appropriate empty or error state. |

**AWS comparison:**
```
aws rds describe-events --source-identifier prod-payments-db \
  --source-type db-instance --duration 10080
```
Expected fields visible: Timestamp, Category, Message

### B.2 Empty State

| ID | Story | Expected |
|----|-------|----------|
| B.2.1 | The API returns zero events for the RDS instance in the 7-day window. | The frame title reads "rds-events(0) -- db-instance-name". The content area shows a centered message (e.g., "No events found") with a hint to refresh. |
| B.2.2 | I press ctrl+r on the empty state. | The loading spinner appears again while the refresh request is in flight. |
| B.2.3 | A newly created RDS instance has had no events yet. | Same behavior as B.2.1 — zero events displayed, no crash or error. |

**AWS comparison:**
```
aws rds describe-events --source-identifier brand-new-db \
  --source-type db-instance --duration 10080
# Returns {"Events": []}
```

### B.3 Column Layout

| ID | Story | Expected |
|----|-------|----------|
| B.3.1 | Events load and the table renders. | Three columns are displayed: "Timestamp" (width 22), "Category" (width 18), "Message" (width 60). Column headers are bold, colored blue (#7aa2f7), with no separator line below them. |
| B.3.2 | I verify column data against `aws rds describe-events`. | "Timestamp" maps to `.Events[].Date`. "Category" maps to `.Events[].EventCategories` (joined string). "Message" maps to `.Events[].Message`. Every event returned by the CLI for the 7-day window appears as a row in the table. |
| B.3.3 | An event message is longer than 60 characters. | The message is truncated to fit the 60-character column width. No row wrapping occurs. The full message is visible in the detail view (d) or YAML view (y). |
| B.3.4 | The terminal is narrower than the combined column widths (22+18+60=100 plus borders). | The rightmost column(s) are hidden (not truncated mid-value). Horizontal scroll with h/l is available to reveal the Message column. |

**AWS comparison:**
```
aws rds describe-events --source-identifier prod-payments-db \
  --source-type db-instance --duration 10080 --output table
```
Expected fields visible: Timestamp, Category, Message

### B.4 Frame Title

| ID | Story | Expected |
|----|-------|----------|
| B.4.1 | 18 events are loaded for RDS instance "prod-payments-db". | The frame top border shows the title centered: "rds-events(18) -- prod-payments-db" with equal-length dashes on both sides. |
| B.4.2 | A filter is active and matches 3 of 18 events. | The frame title reads "rds-events(3/18) -- prod-payments-db". |
| B.4.3 | A filter is active and matches 0 events. | The frame title reads "rds-events(0/18) -- prod-payments-db". The content area is empty (no rows). |

### B.5 Navigation

| ID | Story | Expected |
|----|-------|----------|
| B.5.1 | I press j (or down-arrow) with the first event selected. | The selection cursor moves to the second event. The previously selected row loses the blue highlight. The new row gains the full-width blue background (#7aa2f7) with dark foreground (#1a1b26), bold. |
| B.5.2 | I press k (or up-arrow) with the second event selected. | The selection cursor moves back to the first event. |
| B.5.3 | I press g. | The selection jumps to the very first event in the list. |
| B.5.4 | I press G. | The selection jumps to the very last event in the list. |
| B.5.5 | I press PageDown (or ctrl+d). | The selection moves down by one page of visible rows. |
| B.5.6 | I press PageUp (or ctrl+u). | The selection moves up by one page of visible rows. |
| B.5.7 | There are more events than fit on screen. I scroll past the visible area. | The table scrolls to keep the selected row visible. The column headers remain in place. |

### B.6 Horizontal Scroll

| ID | Story | Expected |
|----|-------|----------|
| B.6.1 | Terminal width is 120 columns and all three columns fit. | All columns are visible. h/l does nothing visible or is a no-op. |
| B.6.2 | Terminal width is 80 columns and the Message column (width 60) does not fully fit. | The Message column is partially hidden or hidden entirely. Pressing l reveals it while hiding leftmost columns. Pressing h reverses. Column headers scroll in sync with data. |

### B.7 Sorting

| ID | Story | Expected |
|----|-------|----------|
| B.7.1 | I press N on the events list. | Rows are sorted by category name in ascending order. The "Category" column header shows a sort indicator. |
| B.7.2 | I press N again. | Sort order toggles to descending. |
| B.7.3 | I press A on the events list. | Rows are sorted by timestamp in ascending order (oldest first). The "Timestamp" column header shows the sort indicator. |
| B.7.4 | I press A again. | Sort order toggles to descending (newest first). |

### B.8 Filter

| ID | Story | Expected |
|----|-------|----------|
| B.8.1 | I press / and type "failover". | Only rows whose category or message contains "failover" (case-insensitive) are displayed. The frame title updates to "rds-events(M/N) -- db-instance-name". |
| B.8.2 | I press / and type "maintenance". | Only maintenance-related events are shown. |
| B.8.3 | I press Escape in filter mode. | The filter is cleared. All events reappear. |
| B.8.4 | I type a filter string that matches no events. | Zero rows are displayed. The frame title shows "rds-events(0/N) -- db-instance-name". |

### B.9 Row Coloring — Event Category

| ID | Story | Expected |
|----|-------|----------|
| B.9.1 | An event has category "failover" (e.g., "Multi-AZ instance failover started."). | The entire row is rendered in RED (#f7768e). |
| B.9.2 | An event has category "failure". | The entire row is rendered in RED (#f7768e). |
| B.9.3 | An event has category "maintenance" (e.g., "Applying modification to database instance class."). | The entire row is rendered in YELLOW (#e0af68). |
| B.9.4 | An event has category "recovery". | The entire row is rendered in YELLOW (#e0af68). |
| B.9.5 | An event has category "availability" with a message indicating failover is complete or recovery. | The entire row is rendered in GREEN (#9ece6a). |
| B.9.6 | An event has category "notification" (e.g., "Automated backup completed."). | The entire row is rendered in PLAIN text color (#c0caf5). |
| B.9.7 | An event has category "configuration change". | The entire row is rendered in DIM (#565f89). |
| B.9.8 | I select a failover event row (RED). | The selected row has full-width blue background (#7aa2f7) overriding the red coloring. |
| B.9.9 | I move selection away from the failover event row. | The row reverts to RED (#f7768e) coloring. |

**AWS comparison:**
```
aws rds describe-events --source-identifier prod-payments-db \
  --source-type db-instance --duration 10080 \
  --query 'Events[].{Date:Date,Categories:EventCategories,Message:Message}'
```

### B.10 Specific Event Types

| ID | Story | Expected |
|----|-------|----------|
| B.10.1 | An RDS instance experienced a Multi-AZ failover. | Multiple events appear: "Multi-AZ instance failover started" (RED, failover category) and "Multi-AZ instance failover completed" (GREEN, availability category). Both are visible and distinguishable by color. |
| B.10.2 | An RDS instance underwent scheduled maintenance (e.g., patching). | Events appear with category "maintenance" such as "Applying modification..." and "Finished applying modification...". These rows are YELLOW. |
| B.10.3 | RDS storage autoscaling triggered for the instance. | An event with category "notification" or "configuration change" describes the storage increase. The event message mentions the new storage size. |
| B.10.4 | An RDS instance was rebooted. | An event with message like "DB instance is being rebooted" appears. The category determines the row color. |
| B.10.5 | An automated backup completed. | An event with category "notification" and message "Automated backup completed." appears. The row is PLAIN colored. |

### B.11 Copy Key (c) — Event Message

| ID | Story | Expected |
|----|-------|----------|
| B.11.1 | I select an event and press c. | The full event message text is copied to the system clipboard. A green flash message "Copied!" appears in the header right side. |
| B.11.2 | I select a maintenance event with a long message (truncated in the table). I press c. | The FULL message text is copied, not the truncated display text. |
| B.11.3 | After ~2 seconds. | The "Copied!" flash message auto-clears and the header right reverts to "? for help". |
| B.11.4 | I paste from clipboard into another application (e.g., an incident report). | The pasted text matches the complete event message exactly. |

**AWS comparison:**
```
aws rds describe-events --source-identifier prod-payments-db \
  --source-type db-instance --duration 10080 \
  --query 'Events[0].Message' --output text
```

### B.12 Detail Key (d)

| ID | Story | Expected |
|----|-------|----------|
| B.12.1 | I select an event and press d. | The detail view opens for the selected event. The frame title shows the event identifier or timestamp. |
| B.12.2 | I verify the detail fields match views.yaml dbi_events detail config. | The detail view shows key-value pairs for: Date, SourceIdentifier, SourceType, EventCategories, SourceArn, Message. |
| B.12.3 | I press Escape on the detail view. | I return to the RDS events list. The cursor position is preserved on the same event I had selected. |

**AWS comparison:**
```
aws rds describe-events --source-identifier prod-payments-db \
  --source-type db-instance --duration 10080 \
  --query 'Events[0]'
```

### B.13 YAML Key (y)

| ID | Story | Expected |
|----|-------|----------|
| B.13.1 | I select an event and press y. | The YAML view opens. The full event resource is rendered as syntax-highlighted YAML. |
| B.13.2 | I press Escape on the YAML view. | I return to the RDS events list. |

### B.14 7-Day Window

| ID | Story | Expected |
|----|-------|----------|
| B.14.1 | An RDS instance has events from the last 7 days. | All events within the 7-day window (Duration=10080 minutes) are shown. Events older than 7 days are not included. |
| B.14.2 | An RDS instance had a failover 8 days ago but has routine backups in the last 7 days. | The failover event is NOT shown (outside the 7-day window). Only the recent backup events are displayed. |

**AWS comparison:**
```
aws rds describe-events --source-identifier prod-payments-db \
  --source-type db-instance --duration 10080
# Only events from the last 10080 minutes (7 days) are returned
```

### B.15 Refresh (ctrl+r)

| ID | Story | Expected |
|----|-------|----------|
| B.15.1 | I press ctrl+r on the events list. | The loading spinner appears. A fresh `rds:DescribeEvents` call is made for the current instance with the 7-day window. The table updates with new results. |
| B.15.2 | A new event occurred since last load (e.g., a backup completed). I press ctrl+r. | The new event appears in the refreshed list. The count in the frame title increments. |
| B.15.3 | I had a filter active and press ctrl+r. | The data refreshes. The filter remains applied to the new data. |

### B.16 Escape (Back)

| ID | Story | Expected |
|----|-------|----------|
| B.16.1 | I press Escape on the RDS events list. | I return to the RDS Instances list. The cursor is on the same instance I had entered. |

### B.17 Help (?)

| ID | Story | Expected |
|----|-------|----------|
| B.17.1 | I press ? on the events list. | The help screen replaces the table content inside the frame. It displays a four-column layout with categories: RDS EVENTS, GENERAL, NAVIGATION, HOTKEYS. |
| B.17.2 | The RDS EVENTS column lists: `<esc>` Back, `<d>` Detail, `<y>` YAML, `<c>` Copy Message. | These entries match the help screen design from the child view spec. |
| B.17.3 | I press any key on the help screen. | The help screen closes and the events list table reappears. |

### B.18 Command Mode (:)

| ID | Story | Expected |
|----|-------|----------|
| B.18.1 | I press : on the events list. | The header right side changes to ":|" (amber/bold). Command mode is active. |
| B.18.2 | I type "s3" and press Enter. | The view navigates to the S3 bucket list. The RDS events context is left behind. |
| B.18.3 | I press Escape in command mode. | Command mode is cancelled. The header reverts to "? for help". The events list remains. |

### B.19 View Stack

| ID | Story | Expected |
|----|-------|----------|
| B.19.1 | Main Menu -> RDS Instances -> RDS Events -> Event Detail -> YAML; then Escape four times. | Each Escape pops one level: YAML -> Detail -> Events -> RDS Instances -> Main Menu. No state is lost at any intermediate level. |
| B.19.2 | RDS Instances -> RDS Events -> Detail (d); then Escape twice. | Detail -> Events -> RDS Instances. The cursor is still on the same RDS instance. |

### B.20 Category Column (Computed Field)

| ID | Story | Expected |
|----|-------|----------|
| B.20.1 | An event has a single category in EventCategories: ["availability"]. | The Category column shows "availability". |
| B.20.2 | An event has multiple categories in EventCategories: ["availability", "failover"]. | The Category column shows "availability, failover" (comma-separated join). |
| B.20.3 | An event has an empty EventCategories array. | The Category column shows a dash or empty string. The row is PLAIN colored. |

### B.21 Alternating Row Colors

| ID | Story | Expected |
|----|-------|----------|
| B.21.1 | The events list has more than 2 rows. | Alternating rows have a subtle background color difference (#1e2030) for readability. Selected row always has blue background regardless. Row-level category coloring (RED for failover, YELLOW for maintenance, etc.) takes precedence over alternating background for non-selected rows. |

---

## C. IAM Role Policies View

### C.1 Loading State

| ID | Story | Expected |
|----|-------|----------|
| C.1.1 | I select an IAM role in the IAM Roles list and press Enter. | A spinner (animated dot) is displayed centered inside the frame. The text reads "Fetching role policies..." (or similar). The frame title shows the role name with no count. The header shows "? for help" on the right. |
| C.1.2 | I press keys (j, k, /, N) while the spinner is visible. | No navigation or sort occurs. Keypresses are ignored or queued until data loads. The spinner continues uninterrupted. |
| C.1.3 | Both API calls (ListAttachedRolePolicies and ListRolePolicies) respond successfully. | The spinner disappears. The table renders with merged results from both API calls. The frame title updates to "role-policies(N) -- role-name" where N is the total combined count of managed and inline policies. |
| C.1.4 | One or both API calls respond with an error. | The spinner disappears. A red error flash message appears in the header right side. |

**AWS comparison:**
```
aws iam list-attached-role-policies --role-name payment-service-execution-role
aws iam list-role-policies --role-name payment-service-execution-role
```
Expected fields visible: Policy Name, Policy ARN, Type

### C.2 Empty State

| ID | Story | Expected |
|----|-------|----------|
| C.2.1 | Both API calls return zero policies (no managed, no inline). | The frame title reads "role-policies(0) -- role-name". The content area shows a centered message (e.g., "No policies found") with a hint to refresh. |
| C.2.2 | I press ctrl+r on the empty state. | The loading spinner appears again while the refresh request is in flight. |

**AWS comparison:**
```
aws iam list-attached-role-policies --role-name empty-role
# Returns {"AttachedPolicies": []}
aws iam list-role-policies --role-name empty-role
# Returns {"PolicyNames": []}
```

### C.3 Column Layout

| ID | Story | Expected |
|----|-------|----------|
| C.3.1 | Policies load and the table renders. | Three columns are displayed: "Policy Name" (width 40), "Policy ARN" (width 56), "Type" (width 10). Column headers are bold, colored blue (#7aa2f7), with no separator line below them. |
| C.3.2 | I verify managed policy column data against `aws iam list-attached-role-policies`. | "Policy Name" maps to `.AttachedPolicies[].PolicyName`. "Policy ARN" maps to `.AttachedPolicies[].PolicyArn`. "Type" shows "Managed" for these entries. |
| C.3.3 | I verify inline policy column data against `aws iam list-role-policies`. | "Policy Name" maps to `.PolicyNames[]`. "Policy ARN" shows a dash (inline policies have no ARN). "Type" shows "Inline" for these entries. |
| C.3.4 | A policy name is longer than 40 characters. | The name is truncated to fit the 40-character column width. No row wrapping occurs. |
| C.3.5 | A policy ARN is longer than 56 characters. | The ARN is truncated to fit the 56-character column width. The full ARN is visible in the detail view (d) or YAML view (y). |
| C.3.6 | The terminal is narrower than the combined column widths (40+56+10=106 plus borders). | The rightmost column(s) are hidden (not truncated mid-value). Horizontal scroll with h/l is available to reveal hidden columns. |

**AWS comparison:**
```
aws iam list-attached-role-policies --role-name payment-service-execution-role --output table
aws iam list-role-policies --role-name payment-service-execution-role --output table
```
Expected fields visible: Policy Name, Policy ARN, Type

### C.4 Frame Title

| ID | Story | Expected |
|----|-------|----------|
| C.4.1 | 7 total policies (5 managed + 2 inline) are loaded for role "payment-service-execution-role". | The frame top border shows the title centered: "role-policies(7) -- payment-service-execution-role" with equal-length dashes on both sides. |
| C.4.2 | A filter is active and matches 2 of 7 policies. | The frame title reads "role-policies(2/7) -- payment-service-execution-role". |
| C.4.3 | A filter is active and matches 0 policies. | The frame title reads "role-policies(0/7) -- payment-service-execution-role". The content area is empty (no rows). |

### C.5 Navigation

| ID | Story | Expected |
|----|-------|----------|
| C.5.1 | I press j (or down-arrow) with the first policy selected. | The selection cursor moves to the second policy. The previously selected row loses the blue highlight. The new row gains the full-width blue background (#7aa2f7) with dark foreground (#1a1b26), bold. |
| C.5.2 | I press k (or up-arrow) with the second policy selected. | The selection cursor moves back to the first policy. |
| C.5.3 | I press g. | The selection jumps to the very first policy in the list. |
| C.5.4 | I press G. | The selection jumps to the very last policy in the list. |
| C.5.5 | I press PageDown (or ctrl+d). | The selection moves down by one page of visible rows. |
| C.5.6 | I press PageUp (or ctrl+u). | The selection moves up by one page of visible rows. |
| C.5.7 | There are more policies than fit on screen. I scroll past the visible area. | The table scrolls to keep the selected row visible. The column headers remain in place. |

### C.6 Horizontal Scroll

| ID | Story | Expected |
|----|-------|----------|
| C.6.1 | Terminal width is 120 columns and all three columns fit. | All columns are visible. h/l does nothing visible or is a no-op. |
| C.6.2 | Terminal width is 80 columns and the Policy ARN column does not fit. | The rightmost columns are hidden. Pressing l reveals them. Pressing h reverses. Column headers scroll in sync with data. |

### C.7 Sorting

| ID | Story | Expected |
|----|-------|----------|
| C.7.1 | I press N on the policies list. | Rows are sorted by policy name in ascending order. The "Policy Name" column header shows a sort indicator. |
| C.7.2 | I press N again. | Sort order toggles to descending. |
| C.7.3 | I press S on the policies list. | Rows are sorted by type ("Inline" before "Managed" or vice versa). The "Type" column header shows the sort indicator. |
| C.7.4 | I sort by name, then refresh with ctrl+r. | After data reloads, the sort order and direction are preserved. |

### C.8 Filter

| ID | Story | Expected |
|----|-------|----------|
| C.8.1 | I press / and type "S3". | Only policies whose name contains "S3" (case-insensitive) are displayed. The frame title updates to "role-policies(M/N) -- role-name". |
| C.8.2 | I press / and type "Inline". | Only inline policies are displayed (matching the Type column or policy name containing "inline"). |
| C.8.3 | I press Escape in filter mode. | The filter is cleared. All policies reappear. |
| C.8.4 | I type a filter string that matches no policies. | Zero rows are displayed. The frame title shows "role-policies(0/N) -- role-name". |

### C.9 Row Coloring — Security Risk Policies

| ID | Story | Expected |
|----|-------|----------|
| C.9.1 | A policy named "AdministratorAccess" is in the list. | The entire row is rendered in RED (#f7768e). This is a deliberate security risk highlight. |
| C.9.2 | A policy named "PowerUserAccess" is in the list. | The entire row is rendered in RED (#f7768e). |
| C.9.3 | A regular managed policy (e.g., "AmazonS3ReadOnlyAccess") is in the list. | The entire row is rendered in PLAIN text color (#c0caf5). |
| C.9.4 | A customer-managed policy (not AWS-managed) is in the list. | The entire row is rendered in PLAIN text color (#c0caf5). |
| C.9.5 | An inline policy is in the list. | The entire row is rendered in DIM (#565f89) to visually distinguish it from managed policies. |
| C.9.6 | I select the AdministratorAccess row (RED). | The selected row has full-width blue background (#7aa2f7) with dark foreground (#1a1b26), bold. The red coloring is overridden by the selection highlight. |
| C.9.7 | I move selection away from the AdministratorAccess row. | The row reverts to RED (#f7768e) coloring. |

**AWS comparison:**
```
aws iam list-attached-role-policies --role-name admin-role \
  --query 'AttachedPolicies[?PolicyName==`AdministratorAccess`]'
```

### C.10 Merged API Results

| ID | Story | Expected |
|----|-------|----------|
| C.10.1 | A role has 5 managed policies and 2 inline policies. | All 7 policies appear in a single unified list. The Type column distinguishes them: "Managed" for the 5 managed policies and "Inline" for the 2 inline policies. The frame title shows "role-policies(7) -- role-name". |
| C.10.2 | A role has only managed policies (no inline). | All rows show "Managed" in the Type column. No "Inline" rows are present. The frame title count reflects only managed policy count. |
| C.10.3 | A role has only inline policies (no managed). | All rows show "Inline" in the Type column. All Policy ARN values show a dash. The frame title count reflects only inline policy count. |
| C.10.4 | A role has both managed and inline policies with the same name (unlikely but possible). | Both entries appear as separate rows. The Type column distinguishes them ("Managed" vs "Inline"). |

**AWS comparison:**
```
# Two separate API calls merged into one view:
aws iam list-attached-role-policies --role-name my-role
# Returns: {"AttachedPolicies": [{"PolicyName": "...", "PolicyArn": "..."}]}
aws iam list-role-policies --role-name my-role
# Returns: {"PolicyNames": ["custom-inline-policy"]}
```

### C.11 Inline Policy — No ARN

| ID | Story | Expected |
|----|-------|----------|
| C.11.1 | An inline policy named "payment-service-custom-policy" is displayed. | The Policy Name column shows "payment-service-custom-policy". The Policy ARN column shows a dash or empty value. The Type column shows "Inline". |
| C.11.2 | I press d on an inline policy. | The detail view opens showing PolicyName with the inline policy name and PolicyArn as null/dash/empty. |

### C.12 Copy Key (c) — Policy ARN or Name

| ID | Story | Expected |
|----|-------|----------|
| C.12.1 | I select a managed policy (e.g., "AmazonS3ReadOnlyAccess") and press c. | The Policy ARN is copied to the system clipboard (e.g., "arn:aws:iam::aws:policy/AmazonS3ReadOnlyAccess"). A green flash message "Copied!" appears in the header right side. |
| C.12.2 | I select an inline policy (no ARN) and press c. | The policy name is copied to the system clipboard (e.g., "payment-service-custom-policy"). A green flash message "Copied!" appears. |
| C.12.3 | After ~2 seconds. | The "Copied!" flash message auto-clears and the header right reverts to "? for help". |
| C.12.4 | I paste from clipboard after copying a managed policy ARN. | The pasted text matches the full policy ARN exactly. This ARN can be used to look up the policy document via `aws iam get-policy`. |

**AWS comparison:**
```
aws iam list-attached-role-policies --role-name my-role \
  --query 'AttachedPolicies[0].PolicyArn' --output text
```

### C.13 Detail Key (d)

| ID | Story | Expected |
|----|-------|----------|
| C.13.1 | I select a managed policy and press d. | The detail view opens for the selected policy. The frame title shows the policy name. |
| C.13.2 | I verify the detail fields match views.yaml role_policies detail config. | The detail view shows key-value pairs for: PolicyName, PolicyArn. |
| C.13.3 | I press d on an inline policy. | The detail view opens. PolicyName shows the inline policy name. PolicyArn shows null, dash, or empty. |
| C.13.4 | I press Escape on the detail view. | I return to the role policies list. The cursor position is preserved on the same policy I had selected. |

### C.14 YAML Key (y)

| ID | Story | Expected |
|----|-------|----------|
| C.14.1 | I select a managed policy and press y. | The YAML view opens. The full policy resource is rendered as syntax-highlighted YAML, including PolicyName and PolicyArn. |
| C.14.2 | I select an inline policy and press y. | The YAML view opens. The resource shows the policy name and Type as "Inline". PolicyArn is null or absent. |
| C.14.3 | I press Escape on the YAML view. | I return to the role policies list. |

### C.15 Refresh (ctrl+r)

| ID | Story | Expected |
|----|-------|----------|
| C.15.1 | I press ctrl+r on the policies list. | The loading spinner appears. Fresh calls to both `iam:ListAttachedRolePolicies` and `iam:ListRolePolicies` are made. When both complete, the table repopulates with merged results. |
| C.15.2 | A new policy was attached to the role since the last load. I press ctrl+r. | The new policy appears in the refreshed list. The count in the frame title increments. |
| C.15.3 | An inline policy was removed since the last load. I press ctrl+r. | The removed policy no longer appears. The count decrements. |
| C.15.4 | I had a filter active and press ctrl+r. | The data refreshes. The filter remains applied to the new data. The frame title count updates accordingly. |

### C.16 Escape (Back)

| ID | Story | Expected |
|----|-------|----------|
| C.16.1 | I press Escape on the role policies list. | I return to the IAM Roles list. The cursor is on the same role I had entered. |

### C.17 Help (?)

| ID | Story | Expected |
|----|-------|----------|
| C.17.1 | I press ? on the policies list. | The help screen replaces the table content inside the frame. It displays a four-column layout with categories: ROLE POLICIES, GENERAL, NAVIGATION, HOTKEYS. |
| C.17.2 | The ROLE POLICIES column lists: `<esc>` Back, `<d>` Detail, `<y>` YAML, `<c>` Copy ARN. | These entries match the help screen design from the child view spec. |
| C.17.3 | I press any key on the help screen. | The help screen closes and the policies list table reappears. |

### C.18 Command Mode (:)

| ID | Story | Expected |
|----|-------|----------|
| C.18.1 | I press : on the policies list. | The header right side changes to ":|" (amber/bold). Command mode is active. |
| C.18.2 | I type "ec2" and press Enter. | The view navigates to the EC2 instances list. The role policies context is left behind. |
| C.18.3 | I press Escape in command mode. | Command mode is cancelled. The header reverts to "? for help". The policies list remains. |

### C.19 View Stack

| ID | Story | Expected |
|----|-------|----------|
| C.19.1 | Main Menu -> IAM Roles -> Role Policies -> Policy Detail -> YAML; then Escape four times. | Each Escape pops one level: YAML -> Detail -> Policies -> IAM Roles -> Main Menu. No state is lost at any intermediate level. |
| C.19.2 | IAM Roles -> Role Policies -> Detail (d); then Escape twice. | Detail -> Policies -> IAM Roles. The cursor is still on the same IAM role. |

### C.20 Alternating Row Colors

| ID | Story | Expected |
|----|-------|----------|
| C.20.1 | The policies list has more than 2 rows. | Alternating rows have a subtle background color difference (#1e2030) for readability. Selected row always has blue background regardless. Row-level coloring (RED for AdministratorAccess/PowerUserAccess, DIM for inline) takes precedence over alternating background for non-selected rows. |

---

## D. Cross-Cutting Concerns

### D.1 Header Consistency

| ID | Story | Expected |
|----|-------|----------|
| D.1.1 | In every child view (ECR Images, RDS Events, Role Policies), the header displays: "a9s" (accent bold), version (dim), profile:region (bold). | Visual inspection confirms across all three child views. |
| D.1.2 | The header right side shows "? for help" in normal mode across all child views. | Confirmed in ECR Images, RDS Events, and Role Policies views. |
| D.1.3 | Flash messages ("Copied!", error messages) appear and auto-clear in all child views. | Confirmed: flash appears in header right side, auto-clears after ~2 seconds. |

### D.2 Terminal Resize

| ID | Story | Expected |
|----|-------|----------|
| D.2.1 | I resize the terminal while viewing ECR Images. | The layout reflows. Column visibility adjusts to the new width. The frame border redraws correctly. |
| D.2.2 | I resize the terminal while viewing RDS Events. | Same reflow behavior. The Message column may become hidden or visible based on new width. |
| D.2.3 | I resize the terminal while viewing Role Policies. | Same reflow behavior. The Policy ARN column may become hidden or visible based on new width. |
| D.2.4 | I resize the terminal to below 60 columns while in any child view. | An error message appears: "Terminal too narrow. Please resize." |
| D.2.5 | I resize the terminal to below 7 lines while in any child view. | An error message appears: "Terminal too short. Please resize." |

### D.3 Profile and Region Switch

| ID | Story | Expected |
|----|-------|----------|
| D.3.1 | I switch AWS profile while viewing ECR Images (via : then ctx or profile command). | The profile changes. The view returns to the main menu or reloads with the new profile's data. The header updates to show the new profile. |
| D.3.2 | I switch AWS region while viewing RDS Events (via : then region command). | The region changes. The view returns to the main menu or reloads with the new region's data. The header updates to show the new region. |

### D.4 Enter Key on Child View Rows

| ID | Story | Expected |
|----|-------|----------|
| D.4.1 | I press Enter on a row in ECR Images. | The detail view opens for that image (same behavior as pressing d). There is no further child view to drill into from an image. |
| D.4.2 | I press Enter on a row in RDS Events. | The detail view opens for that event (same behavior as pressing d). There is no further child view to drill into from an event. |
| D.4.3 | I press Enter on a row in Role Policies. | The detail view opens for that policy (same behavior as pressing d). There is no further child view to drill into from a policy. |

### D.5 Full Navigation Flow

| ID | Story | Expected |
|----|-------|----------|
| D.5.1 | Main Menu -> ECR Repos -> select repo -> Enter -> ECR Images -> select image -> d -> Detail -> y -> YAML -> Esc -> Detail -> Esc -> Images -> Esc -> ECR Repos -> Esc -> Main Menu. | The full round trip completes without errors. Each Escape returns to the correct parent view with preserved cursor positions. |
| D.5.2 | Main Menu -> RDS Instances -> select instance -> Enter -> RDS Events -> select event -> d -> Detail -> Esc -> Events -> c (copy message) -> Esc -> RDS Instances -> Esc -> Main Menu. | The full round trip completes without errors. The copy operation works within the child view. |
| D.5.3 | Main Menu -> IAM Roles -> select role -> Enter -> Role Policies -> / -> type "Admin" -> verify filter -> Esc (clear filter) -> Esc (back to roles) -> Esc -> Main Menu. | Filter works within the child view. Escape in filter mode clears the filter first, then the next Escape navigates back. |
