# QA User Stories: EKS Clusters and Secrets Manager Views

Covers list, detail, YAML, and (Secrets only) reveal views for EKS and Secrets Manager resource types.

---

## EKS Clusters

### EKS-LIST-01: Navigate to EKS cluster list

**Given** the main menu is displayed
**When** I select "EKS Clusters" and press Enter (or type `:eks` Enter)
**Then** the frame title shows `eks-clusters(<count>)` with the total number of clusters
**And** a loading spinner with "Fetching EKS clusters..." appears while data loads
**And** once loaded, the table displays these columns left-to-right:

| Column header    | Data path       | Width |
|------------------|-----------------|-------|
| Cluster Name     | Name            | 28    |
| Version          | Version         | 10    |
| Status           | Status          | 14    |
| Endpoint         | Endpoint        | 48    |
| Platform Version | PlatformVersion | 18    |

**AWS comparison:** `aws eks list-clusters` returns cluster names, then `aws eks describe-cluster --name <name>` returns each cluster's fields.

---

### EKS-LIST-02: Row coloring by cluster status

**Given** the EKS cluster list is displayed
**When** clusters have various statuses
**Then** each entire row is colored according to status:
- ACTIVE clusters: entire row in green (#9ece6a)
- CREATING clusters: entire row in yellow (#e0af68)
- DELETING clusters: entire row in red (#f7768e)
- FAILED clusters: entire row in red (#f7768e)
- Any other status: entire row in plain (#c0caf5)

**And** the currently selected row always overrides the status color with a full-width blue background (#7aa2f7) and dark foreground (#1a1b26), bold.

---

### EKS-LIST-03: Sort EKS clusters

**Given** the EKS cluster list is displayed
**When** I press `N`
**Then** the list sorts by Cluster Name ascending and the column header shows `Cluster Name` with an up-arrow indicator
**When** I press `N` again
**Then** the sort toggles to descending and the indicator becomes a down-arrow
**When** I press `S`
**Then** the list sorts by the Status column

---

### EKS-LIST-04: Filter EKS clusters

**Given** the EKS cluster list shows 5 clusters
**When** I press `/` and type `prod`
**Then** the header right side shows `/prod` with a cursor in amber bold
**And** only rows matching "prod" (case-insensitive substring) are displayed
**And** the frame title updates to `eks-clusters(<matched>/<total>)` (e.g. `eks-clusters(2/5)`)
**When** I press Esc
**Then** the filter clears and all 5 clusters reappear
**And** the frame title returns to `eks-clusters(5)`

---

### EKS-LIST-05: Horizontal column scrolling

**Given** the terminal width is narrow enough that not all 5 EKS columns fit
**When** I press `l` (or right arrow)
**Then** the visible column window shifts right, revealing hidden rightmost columns
**And** column headers scroll in sync with the data rows
**When** I press `h` (or left arrow)
**Then** the visible column window shifts back left

---

### EKS-LIST-06: Copy EKS cluster identifier

**Given** the EKS cluster list is displayed and the cursor is on a cluster row
**When** I press `c`
**Then** the cluster name (or ARN) is copied to the system clipboard
**And** the header right side briefly shows "Copied!" in green bold
**And** the flash message auto-clears after approximately 2 seconds

---

### EKS-DETAIL-01: Open EKS detail view

**Given** the EKS cluster list is displayed and the cursor is on a cluster named "my-cluster"
**When** I press Enter or `d`
**Then** the table is replaced by a detail view inside the same frame
**And** the frame title shows the cluster name (e.g. `my-cluster`)
**And** the following fields are displayed as key-value pairs:

| Field                     |
|---------------------------|
| Name                      |
| Version                   |
| Status                    |
| Endpoint                  |
| PlatformVersion           |
| Arn                       |
| RoleArn                   |
| KubernetesNetworkConfig   |

**And** keys are rendered in blue (#7aa2f7), values in plain (#c0caf5)
**And** section headers (if any) are rendered in yellow/orange (#e0af68) bold
**And** the Status value uses the same color rules as list rows (ACTIVE=green, CREATING=yellow, DELETING=red)

**AWS comparison:** `aws eks describe-cluster --name my-cluster` returns these fields.

---

### EKS-DETAIL-02: KubernetesNetworkConfig nested fields

**Given** the EKS detail view is displayed for a cluster
**When** I look at the KubernetesNetworkConfig section
**Then** its nested sub-fields (such as IpFamily, ServiceIpv4Cidr, ServiceIpv6Cidr, ElasticLoadBalancing.Enabled) are rendered indented under the parent heading
**And** each sub-field follows the standard key: value formatting

---

### EKS-DETAIL-03: Scroll long EKS detail content

**Given** the EKS detail view content is longer than the visible frame area
**When** I press `j` or down-arrow
**Then** the detail content scrolls down one line
**When** I press `k` or up-arrow
**Then** the detail content scrolls up one line
**When** I press `g`
**Then** the view jumps to the top of the detail content
**When** I press `G`
**Then** the view jumps to the bottom

---

### EKS-DETAIL-04: Copy detail content

**Given** the EKS detail view is displayed
**When** I press `c`
**Then** the full detail text is copied to the system clipboard
**And** the header shows "Copied!" flash message

---

### EKS-DETAIL-05: Navigate back from EKS detail

**Given** the EKS detail view is displayed
**When** I press Esc
**Then** the view returns to the EKS cluster list
**And** the previously selected row is still highlighted

---

### EKS-YAML-01: Open EKS YAML view from list

**Given** the EKS cluster list is displayed and the cursor is on "my-cluster"
**When** I press `y`
**Then** the table is replaced by a YAML-formatted view of the full cluster resource
**And** the frame title shows `my-cluster yaml`
**And** YAML keys are blue (#7aa2f7), string values green (#9ece6a), numeric values orange (#ff9e64), booleans purple (#bb9af7), null values dim (#565f89)
**And** nested structures use indentation with dim tree connector lines

---

### EKS-YAML-02: Open EKS YAML view from detail

**Given** the EKS detail view is displayed
**When** I press `y`
**Then** the view switches to YAML view showing the same cluster data
**And** the frame title updates to include "yaml"

---

### EKS-YAML-03: Scroll and copy in EKS YAML view

**Given** the EKS YAML view is displayed
**When** I press `j`/`k`/`g`/`G`
**Then** the YAML content scrolls accordingly
**When** I press `c`
**Then** the full YAML text is copied to the clipboard
**And** the header shows "Copied!" flash message

---

### EKS-YAML-04: Navigate back from EKS YAML view

**Given** the EKS YAML view was opened from the list
**When** I press Esc
**Then** the view returns to the EKS cluster list

**Given** the EKS YAML view was opened from the detail view
**When** I press Esc
**Then** the view returns to the EKS detail view (view stack is popped one level)

---

### EKS-HELP-01: Help screen shows EKS-relevant keys

**Given** the EKS cluster list is displayed
**When** I press `?`
**Then** the help screen appears inside the frame
**And** the RESOURCE column shows `<esc> Back` and available resource-level keys
**And** the NAVIGATION column lists `j`/`k`/`g`/`G`/`h`/`l`/Enter/`d`/`y`/`c`/`N`/`S`/`A`
**And** pressing any key closes the help overlay and returns to the EKS list

---

## Secrets Manager

### SEC-LIST-01: Navigate to Secrets Manager list

**Given** the main menu is displayed
**When** I select "Secrets Manager" and press Enter (or type `:secrets` Enter)
**Then** the frame title shows `secrets(<count>)` with the total number of secrets
**And** a loading spinner with "Fetching..." appears while data loads
**And** once loaded, the table displays these columns left-to-right:

| Column header  | Data path        | Width |
|----------------|------------------|-------|
| Secret Name    | Name             | 36    |
| Description    | Description      | 30    |
| Last Accessed  | LastAccessedDate | 18    |
| Last Changed   | LastChangedDate  | 18    |
| Rotation       | RotationEnabled  | 10    |

**AWS comparison:** `aws secretsmanager list-secrets` returns these fields.

---

### SEC-LIST-02: Row display for secrets

**Given** the Secrets Manager list is displayed
**Then** each row shows the secret name, description (truncated to column width if needed), formatted dates for Last Accessed and Last Changed, and a boolean value for Rotation (true/false)
**And** rows use plain text color (#c0caf5) since secrets do not have a running/stopped status concept
**And** the selected row has the standard full-width blue background treatment

---

### SEC-LIST-03: Sort secrets

**Given** the Secrets Manager list is displayed
**When** I press `N`
**Then** the list sorts by Secret Name ascending with an up-arrow indicator on the column header
**When** I press `N` again
**Then** the sort toggles to descending
**When** I press `A`
**Then** the list sorts by age (date-based column)

---

### SEC-LIST-04: Filter secrets

**Given** the Secrets Manager list shows 10 secrets
**When** I press `/` and type `api`
**Then** only secrets whose rows contain "api" (case-insensitive) are displayed
**And** the frame title updates to `secrets(<matched>/<total>)` (e.g. `secrets(3/10)`)
**When** I press Esc
**Then** the filter clears and all 10 secrets reappear

---

### SEC-LIST-05: Horizontal column scrolling

**Given** the terminal is narrow enough that not all 5 Secrets columns fit
**When** I press `l`/`h` (or arrow keys)
**Then** the visible column window scrolls right/left
**And** column headers and data rows stay in sync

---

### SEC-LIST-06: Copy secret identifier

**Given** the Secrets Manager list is displayed and the cursor is on a secret
**When** I press `c`
**Then** the secret name (or ARN) is copied to the system clipboard
**And** the header briefly shows "Copied!" in green bold, auto-clearing after ~2 seconds

---

### SEC-DETAIL-01: Open Secrets Manager detail view

**Given** the Secrets Manager list is displayed and the cursor is on "prod/api/db-password"
**When** I press Enter or `d`
**Then** the table is replaced by a detail view inside the frame
**And** the frame title shows the secret name (e.g. `prod/api/db-password`)
**And** the following fields are displayed as key-value pairs:

| Field            |
|------------------|
| Name             |
| Description      |
| LastAccessedDate |
| LastChangedDate  |
| RotationEnabled  |
| ARN              |
| KmsKeyId         |
| Tags             |

**And** keys are blue (#7aa2f7), values plain (#c0caf5)
**And** Tags are rendered with their nested Key/Value pairs indented

**AWS comparison:** `aws secretsmanager list-secrets` provides these metadata fields (the detail view does NOT reveal the secret value).

---

### SEC-DETAIL-02: Scroll long secret detail content

**Given** the Secrets Manager detail view has content longer than the frame
**When** I press `j`/`k`/`g`/`G`
**Then** the content scrolls accordingly (same behavior as EKS-DETAIL-03)

---

### SEC-DETAIL-03: Copy detail content

**Given** the Secrets Manager detail view is displayed
**When** I press `c`
**Then** the full detail text (metadata, NOT the secret value) is copied to the clipboard
**And** the header shows "Copied!" flash message

---

### SEC-DETAIL-04: Navigate back from secret detail

**Given** the Secrets Manager detail view is displayed
**When** I press Esc
**Then** the view returns to the Secrets Manager list with the same row selected

---

### SEC-YAML-01: Open Secrets Manager YAML view

**Given** the Secrets Manager list is displayed and the cursor is on "prod/api/db-password"
**When** I press `y`
**Then** the table is replaced by a YAML-formatted view of the secret metadata
**And** the frame title shows `prod/api/db-password yaml`
**And** YAML syntax coloring applies (keys blue, strings green, booleans purple, etc.)
**And** the YAML does NOT contain the secret plaintext value

---

### SEC-YAML-02: Open Secrets Manager YAML from detail

**Given** the Secrets Manager detail view is displayed
**When** I press `y`
**Then** the view switches to YAML view for the same secret
**And** the frame title updates to include "yaml"

---

### SEC-YAML-03: Scroll and copy in Secrets Manager YAML view

**Given** the Secrets Manager YAML view is displayed
**When** I press `j`/`k`/`g`/`G`
**Then** the content scrolls accordingly
**When** I press `c`
**Then** the full YAML text (metadata only, not the secret value) is copied to the clipboard

---

### SEC-YAML-04: Navigate back from Secrets Manager YAML view

**Given** the YAML view was opened from the list
**When** I press Esc
**Then** the view returns to the Secrets Manager list

**Given** the YAML view was opened from the detail view
**When** I press Esc
**Then** the view returns to the detail view (view stack pops one level)

---

### SEC-REVEAL-01: Reveal secret value from list

**Given** the Secrets Manager list is displayed and the cursor is on "prod/api/db-password"
**When** I press `x`
**Then** the view changes to the Reveal view
**And** the frame title shows the secret name (e.g. `prod/api/database-password`)
**And** the header right side changes from "? for help" to a persistent red warning: `Secret visible -- press esc to close`
**And** the reveal view displays:
  1. The secret name in bold
  2. A horizontal divider line (dim)
  3. The plaintext secret value in green (#9ece6a)
  4. Metadata in dim text: type, last rotated date, rotation enabled status

**AWS comparison:** `aws secretsmanager get-secret-value --secret-id prod/api/db-password` returns the `.SecretString` field, which matches the plaintext shown in the reveal view.

---

### SEC-REVEAL-02: Red warning header persists during reveal

**Given** the Reveal view is displayed
**Then** the header right side shows `Secret visible -- press esc to close` in red (#f7768e) bold
**And** this warning does NOT auto-clear (it is persistent, not a transient flash)
**And** it replaces the normal "? for help" hint for the entire duration the reveal view is open

---

### SEC-REVEAL-03: Copy secret value from reveal view

**Given** the Reveal view is displayed showing the plaintext secret
**When** I press `c`
**Then** the plaintext secret value is copied to the system clipboard
**And** the header continues to show the red warning (the "Copied!" flash may briefly overlay it or appear alongside)

---

### SEC-REVEAL-04: Close reveal view with Esc

**Given** the Reveal view is displayed
**When** I press Esc
**Then** the reveal view closes
**And** the view returns to the Secrets Manager list
**And** the header right side returns to the normal "? for help" hint
**And** the previously selected row is still highlighted

---

### SEC-REVEAL-05: x key does nothing on non-secret resource types

**Given** the EC2 instance list (or any non-Secrets Manager resource list) is displayed
**When** I press `x`
**Then** nothing happens -- no view change, no error, no flash message
**And** the `x` key binding is exclusive to the Secrets Manager resource type

---

### SEC-REVEAL-06: Reveal handles JSON secret values

**Given** a secret stores a JSON string (e.g. `{"username":"admin","password":"s3cret"}`)
**When** I press `x` to reveal it
**Then** the full JSON string is displayed in the reveal view in green
**And** pressing `c` copies the entire JSON string to the clipboard

---

### SEC-REVEAL-07: Reveal handles empty or deleted secrets gracefully

**Given** a secret exists in the list but has been scheduled for deletion or has no current value
**When** I press `x`
**Then** the application handles the error gracefully (e.g. shows an error flash in the header)
**And** the application does not crash or hang

---

## Cross-Cutting: Refresh Behavior

### CROSS-01: Refresh EKS cluster list

**Given** the EKS cluster list is displayed
**When** I press Ctrl+r
**Then** the loading spinner appears and the cluster data is re-fetched from AWS
**And** once loaded, the list updates with current data
**And** the cursor position resets to the first row (or is preserved if the previously selected cluster still exists)

---

### CROSS-02: Refresh Secrets Manager list

**Given** the Secrets Manager list is displayed
**When** I press Ctrl+r
**Then** the loading spinner appears and the secrets data is re-fetched from AWS
**And** the list updates with current data

---

## Cross-Cutting: Command Navigation

### CROSS-03: Switch to EKS via command mode

**Given** any view is displayed
**When** I press `:` and type `eks` and press Enter
**Then** the application navigates to the EKS cluster list view

---

### CROSS-04: Switch to Secrets via command mode

**Given** any view is displayed
**When** I press `:` and type `secrets` and press Enter
**Then** the application navigates to the Secrets Manager list view

---

## Cross-Cutting: Empty State

### CROSS-05: EKS list with no clusters

**Given** the AWS account has no EKS clusters in the current region
**When** I navigate to the EKS cluster list
**Then** the frame shows a centered empty-state message (e.g. "No EKS clusters found")
**And** the frame title shows `eks-clusters(0)`

---

### CROSS-06: Secrets list with no secrets

**Given** the AWS account has no secrets in the current region
**When** I navigate to the Secrets Manager list
**Then** the frame shows a centered empty-state message
**And** the frame title shows `secrets(0)`

---

## Cross-Cutting: Error Handling

### CROSS-07: AWS credentials missing or expired

**Given** the AWS credentials are missing or expired
**When** I navigate to the EKS or Secrets Manager list
**Then** the header right side shows an error flash in red (e.g. "Error: no credentials")
**And** the error is persistent until the user navigates away or refreshes

---

### CROSS-08: Insufficient IAM permissions

**Given** the AWS credentials lack `eks:ListClusters` or `secretsmanager:ListSecrets` permissions
**When** I navigate to the respective resource list
**Then** an error message appears (not a crash)
**And** the header shows an appropriate error flash
