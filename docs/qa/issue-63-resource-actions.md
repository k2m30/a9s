# QA User Stories: Resource-Specific Actions (Issue #63)

Scope: resource-specific actions that move a9s beyond read-only browsing into
operational tasks -- view, download, invoke, start/stop, copy, and other
actions engineers perform after finding a resource.

All stories are written from a black-box perspective against the design spec and
`views.yaml` configuration files. AWS CLI equivalents are cited so testers can
verify data parity.

---

## A. Clipboard Actions (v4.0)

### A.1 Copy S3 Object URI

| ID | Story | Expected |
|----|-------|----------|
| A.1.1 | I am viewing the S3 Objects list inside bucket "static-assets-prod". I select object "images/logo.png" and press `c`. | The S3 URI "s3://static-assets-prod/images/logo.png" is copied to the system clipboard. The header right side flashes "Copied!" in green bold (#9ece6a) for approximately 2 seconds, then reverts to "? for help". |
| A.1.2 | I select a deeply nested object "data/2026/03/27/report.csv" and press `c`. | The full S3 URI "s3://static-assets-prod/data/2026/03/27/report.csv" is copied. The flash message confirms "Copied!". |
| A.1.3 | I select a folder prefix "data/2026/" (displayed as a folder row) and press `c`. | The S3 URI for the prefix "s3://static-assets-prod/data/2026/" is copied. |

**AWS comparison:**

```
# No direct CLI equivalent -- the user would manually construct:
# s3://static-assets-prod/images/logo.png
# a9s copies this automatically
```

Expected S3 object fields: Key (width 36), Size (width 12), Storage Class (width 16), Last Modified (width 22)

### A.2 Copy Secret Value

| ID | Story | Expected |
|----|-------|----------|
| A.2.1 | I am viewing the Secrets Manager list. I select secret "prod/api/database-password" and press `c`. | The plaintext secret value is fetched from AWS and copied to the system clipboard. The header flashes "Copied!" in green bold (#9ece6a). The secret value is NOT displayed on screen. |
| A.2.2 | The secret "prod/api/database-password" is a JSON-formatted secret (e.g., `{"username":"admin","password":"s3cr3t"}`). I press `c`. | The entire JSON string is copied to the clipboard as-is. |
| A.2.3 | The secret has rotation enabled and I press `c`. | The current version (AWSCURRENT) of the secret is copied. |
| A.2.4 | I do not have permission to retrieve the secret value (missing `secretsmanager:GetSecretValue`). I press `c`. | A red error flash appears in the header: "Error: AccessDenied" (or similar). Nothing is copied to the clipboard. The application does not crash. |

**AWS comparison:**

```
aws secretsmanager get-secret-value --secret-id prod/api/database-password --query 'SecretString' --output text
```

Expected Secrets Manager list fields: Secret Name (width 36), Description (width 30), Last Accessed (width 18), Last Changed (width 18), Rotation (width 10)

### A.3 Copy SSM Parameter Value

| ID | Story | Expected |
|----|-------|----------|
| A.3.1 | I am viewing the SSM Parameters list. I select parameter "/app/config/api-key" and press `c`. | The parameter value is fetched (with decryption if SecureString) and copied to the clipboard. The header flashes "Copied!". |
| A.3.2 | The parameter is of type "SecureString". I press `c`. | The decrypted value is copied. If the user lacks `kms:Decrypt` permission, a red error flash appears instead. |
| A.3.3 | The parameter is of type "StringList" (e.g., "us-east-1,us-west-2,eu-west-1"). I press `c`. | The full comma-separated string is copied to the clipboard. |

**AWS comparison:**

```
aws ssm get-parameter --name /app/config/api-key --with-decryption --query 'Parameter.Value' --output text
```

Expected SSM list fields: Name (width 40), Type (width 14), Version (width 8), Last Modified (width 22), Description (width 30)

### A.4 Copy Resource ARN (General)

| ID | Story | Expected |
|----|-------|----------|
| A.4.1 | I am viewing the EC2 instance list. I select instance "api-prod-01" and press `c`. | The instance ID (e.g., "i-0abc123def456789a") or the full ARN is copied to the clipboard. The header flashes "Copied!". |
| A.4.2 | I am viewing the Lambda list. I select function "data-processor" and press `c`. | The function ARN is copied to the clipboard. |
| A.4.3 | I am viewing the RDS instances list. I select "prod-database" and press `c`. | The DB instance ARN is copied to the clipboard. |
| A.4.4 | I am viewing the EKS clusters list. I select "prod-cluster" and press `c`. | The cluster ARN is copied to the clipboard. |
| A.4.5 | I am viewing the CloudFormation stacks list. I select a stack and press `c`. | The stack ID (ARN) is copied to the clipboard. |

**AWS comparison:**

```
aws ec2 describe-instances --instance-ids i-0abc123 --query 'Reservations[0].Instances[0].InstanceId'
aws lambda get-function --function-name data-processor --query 'Configuration.FunctionArn'
```

---

## B. S3 Object Download (v4.0)

### B.1 Download S3 Object

| ID | Story | Expected |
|----|-------|----------|
| B.1.1 | I am viewing the S3 Objects list inside bucket "static-assets-prod". I select object "images/logo.png" (Size: 45KB) and press `d`. | A download begins. The header or status area shows a progress indicator (e.g., "Downloading images/logo.png..."). When complete, the header flashes a success message indicating the download path (e.g., "Downloaded to ~/Downloads/logo.png" or a configurable path). |
| B.1.2 | I select a large object "backups/db-dump.sql.gz" (Size: 2.1GB) and press `d`. | A progress indicator is shown during the download. The download completes or can be cancelled with Esc. |
| B.1.3 | I select a folder prefix row (not an actual object) and press `d`. | Nothing happens or a flash message indicates "Cannot download a folder prefix". |
| B.1.4 | I do not have `s3:GetObject` permission. I press `d` on an object. | A red error flash appears: "Error: AccessDenied". No file is downloaded. |
| B.1.5 | The download destination disk is full. | An error flash appears indicating the disk space issue. Partial downloads are cleaned up. |

**AWS comparison:**

```
aws s3 cp s3://static-assets-prod/images/logo.png ~/Downloads/logo.png
```

Expected S3 object fields: Key (width 36), Size (width 12), Storage Class (width 16), Last Modified (width 22)

---

## C. EC2 Instance Actions (v4.1)

### C.1 Start Instance

| ID | Story | Expected |
|----|-------|----------|
| C.1.1 | I am viewing the EC2 list. Instance "dev-server" is in state "stopped" (row is red #f7768e). I press `S` (shift+s). | A confirmation dialog appears: "Start instance dev-server (i-0abc123)? [y/N]" or similar. The dialog requires explicit confirmation before executing. |
| C.1.2 | I confirm the start action by pressing `y`. | The API call is made. The header flashes "Starting instance dev-server..." in green bold. After the API responds, the flash updates to "Instance dev-server started" or similar. The instance state in the list changes from "stopped" to "pending" (yellow row). |
| C.1.3 | I press `N` or `Esc` on the confirmation dialog. | The action is cancelled. No API call is made. The flash shows "Cancelled" or the dialog simply closes. The instance remains "stopped". |
| C.1.4 | The instance is already in state "running". I press `S`. | Either: (a) nothing happens, or (b) a flash message indicates "Instance is already running". No confirmation dialog appears for an invalid action. |
| C.1.5 | I do not have `ec2:StartInstances` permission. I confirm the start action. | A red error flash appears: "Error: UnauthorizedOperation" or "Error: insufficient permissions". The instance state does not change. |

**AWS comparison:**

```
aws ec2 start-instances --instance-ids i-0abc123
```

Expected EC2 list fields: Name (width 24), State (width 12), Lifecycle (width 12), Type (width 14), Private IP (width 16), Public IP (width 16), Instance ID (width 20), Launch Time (width 22)

### C.2 Stop Instance

| ID | Story | Expected |
|----|-------|----------|
| C.2.1 | Instance "api-prod-01" is in state "running" (row is green #9ece6a). I press `s` (lowercase). | A confirmation dialog appears: "Stop instance api-prod-01 (i-0abc123)? [y/N]" -- this is a DESTRUCTIVE action requiring confirmation. |
| C.2.2 | I confirm with `y`. | The API call is made. The header flashes the action status. The instance state transitions from "running" to "stopping" (yellow). |
| C.2.3 | I press `N` or `Esc`. | The action is cancelled. Instance remains "running". |
| C.2.4 | The instance is already "stopped". I press `s`. | Nothing happens or a flash indicates the instance is already stopped. |

**AWS comparison:**

```
aws ec2 stop-instances --instance-ids i-0abc123
```

### C.3 Reboot Instance

| ID | Story | Expected |
|----|-------|----------|
| C.3.1 | Instance "api-prod-01" is "running". I press `R` (if not already taken by Related -- the issue specifies this key). | A confirmation dialog appears: "Reboot instance api-prod-01 (i-0abc123)? [y/N]" -- this is a DESTRUCTIVE action requiring confirmation. |
| C.3.2 | I confirm with `y`. | The API call is made. The header flashes the reboot status. The instance remains "running" (reboot does not change state to stopped). |
| C.3.3 | I press `N` or `Esc`. | The action is cancelled. No API call is made. |

**AWS comparison:**

```
aws ec2 reboot-instances --instance-ids i-0abc123
```

> **Note:** The `R` key may conflict with Issue #64 (Related Resources). The final
> key assignment will need to resolve this -- the issue suggests `R` for reboot
> but Issue #64 also uses `R` for Related. One may need to be changed.

---

## D. ECS Actions (v4.1)

### D.1 Force New Deployment

| ID | Story | Expected |
|----|-------|----------|
| D.1.1 | I am viewing the ECS Services list. I select service "api-service" (Status: "ACTIVE", Running: 3). I press `R` (or the assigned action key). | A confirmation dialog appears: "Force new deployment for api-service? [y/N]". |
| D.1.2 | I confirm with `y`. | The API call is made (UpdateService with forceNewDeployment=true). The header flashes "Deployment initiated for api-service". |
| D.1.3 | I cancel with `N` or `Esc`. | No API call is made. Service remains unchanged. |

**AWS comparison:**

```
aws ecs update-service --cluster CLUSTER --service api-service --force-new-deployment
```

Expected ECS service list fields: Service Name (width 32), Cluster (width 24), Status (width 12), Desired (width 9), Running (width 9), Launch Type (width 12)

### D.2 Stop Task

| ID | Story | Expected |
|----|-------|----------|
| D.2.1 | I am viewing the ECS Tasks list (child view of ECS service). I select a task in state "RUNNING". I press `x`. | A confirmation dialog appears: "Stop task TASK_ID? [y/N]" -- this is a DESTRUCTIVE action. |
| D.2.2 | I confirm with `y`. | The API call is made (StopTask). The header flashes the status. The task state transitions from "RUNNING" to "STOPPING". |
| D.2.3 | I cancel. | No action taken. |

**AWS comparison:**

```
aws ecs stop-task --cluster CLUSTER --task TASK_ARN
```

---

## E. Lambda Actions (v4.2)

### E.1 Invoke Function

| ID | Story | Expected |
|----|-------|----------|
| E.1.1 | I am viewing the Lambda list. I select function "health-check" and press `i`. | An input dialog or prompt appears asking for the test event payload. A default empty event `{}` is pre-filled. |
| E.1.2 | I accept the default empty event and press Enter to invoke. | The API call is made (lambda:Invoke). A spinner or progress indicator appears: "Invoking health-check...". When the invocation completes, the result is displayed: status code (e.g., 200), execution duration, and the response payload (truncated if large). |
| E.1.3 | The invocation succeeds with StatusCode 200. | The header flashes a success message. The response payload is shown in the frame content area or a detail view. |
| E.1.4 | The invocation fails (function error, timeout). | The header shows a red error flash with the error type (e.g., "Error: function timed out" or "Error: Runtime.HandlerNotFound"). The error details from the response are visible. |
| E.1.5 | I do not have `lambda:InvokeFunction` permission. | A red error flash: "Error: AccessDenied". No invocation occurs. |
| E.1.6 | I press `Esc` on the event payload prompt. | The invocation is cancelled. No API call is made. |

**AWS comparison:**

```
aws lambda invoke --function-name health-check --payload '{}' /dev/stdout
```

Expected Lambda list fields: Function Name (width 36), Runtime (width 16), Memory (width 8), Timeout (width 8), State (width 10), Last Modified (width 22)

### E.2 Download Lambda Deployment Package

| ID | Story | Expected |
|----|-------|----------|
| E.2.1 | I am viewing the Lambda list. I select function "data-processor" and press `d`. | The function code location is fetched. A download begins for the deployment package (zip). The header shows progress: "Downloading data-processor package...". |
| E.2.2 | The download completes. | The header flashes "Downloaded to ~/Downloads/data-processor.zip" (or configurable path). |
| E.2.3 | The function uses a container image (PackageType: Image). I press `d`. | A flash message indicates "Cannot download container image function" or the download is for the image URI metadata. |
| E.2.4 | I lack `lambda:GetFunction` permission. | A red error flash: "Error: AccessDenied". |

**AWS comparison:**

```
aws lambda get-function --function-name data-processor --query 'Code.Location'
# Then download from the presigned URL
```

---

## F. ASG Scaling Action (v4.2)

### F.1 Set Desired Capacity

| ID | Story | Expected |
|----|-------|----------|
| F.1.1 | I am viewing the ASG list. I select "api-prod-asg" (Min: 2, Max: 10, Desired: 4). I press `s` (or the assigned action key). | An input prompt appears showing the current desired capacity (4) and asking for the new value. Min (2) and Max (10) are displayed as constraints. |
| F.1.2 | I type "6" and press Enter. | A confirmation dialog: "Set desired capacity for api-prod-asg from 4 to 6? [y/N]". |
| F.1.3 | I confirm with `y`. | The API call is made. The header flashes "Desired capacity set to 6 for api-prod-asg". |
| F.1.4 | I type "15" (exceeds Max of 10) and press Enter. | An error flash: "Value 15 exceeds maximum capacity of 10". No API call is made. |
| F.1.5 | I type "0" (below Min of 2) and press Enter. | An error flash: "Value 0 is below minimum capacity of 2". No API call is made. |
| F.1.6 | I press Esc on the input prompt. | The action is cancelled. No change is made. |

**AWS comparison:**

```
aws autoscaling set-desired-capacity --auto-scaling-group-name api-prod-asg --desired-capacity 6
```

Expected ASG list fields: ASG Name (width 36), Min (width 6), Max (width 6), Desired (width 8), Instances (width 10), Status (width 12)

---

## G. RDS Reboot (v4.2)

### G.1 Reboot DB Instance

| ID | Story | Expected |
|----|-------|----------|
| G.1.1 | I am viewing the RDS instances list. I select "prod-database" (Status: "available"). I press `R` (or the assigned action key). | A confirmation dialog: "Reboot DB instance prod-database? [y/N]" -- DESTRUCTIVE action. |
| G.1.2 | I confirm with `y`. | The API call is made (rds:RebootDBInstance). The header flashes "Rebooting prod-database...". The status transitions from "available" to "rebooting" (yellow row). |
| G.1.3 | I cancel. | No action taken. |
| G.1.4 | The DB instance is not in "available" state (e.g., "modifying"). | The action is rejected with a flash: "Cannot reboot: instance is not available" or the API returns an error. |

**AWS comparison:**

```
aws rds reboot-db-instance --db-instance-identifier prod-database
```

Expected RDS list fields: DB Identifier (width 28), Engine (width 12), Version (width 10), Status (width 14), Class (width 16), Endpoint (width 40), Multi-AZ (width 10)

---

## H. CloudWatch Alarm Action (v4.2)

### H.1 Disable/Enable Alarm Actions

| ID | Story | Expected |
|----|-------|----------|
| H.1.1 | I am viewing the CloudWatch Alarms list. I select alarm "high-cpu-prod" with ActionsEnabled: true. I press `D`. | A confirmation dialog: "Disable actions for alarm high-cpu-prod? [y/N]". |
| H.1.2 | I confirm with `y`. | The API call is made (cloudwatch:DisableAlarmActions). The header flashes "Actions disabled for high-cpu-prod". |
| H.1.3 | The alarm has ActionsEnabled: false. I press `D`. | The dialog reads "Enable actions for alarm high-cpu-prod? [y/N]" -- toggling behavior. |
| H.1.4 | I confirm. | The API call is made (cloudwatch:EnableAlarmActions). The header flashes "Actions enabled for high-cpu-prod". |

**AWS comparison:**

```
aws cloudwatch disable-alarm-actions --alarm-names high-cpu-prod
aws cloudwatch enable-alarm-actions --alarm-names high-cpu-prod
```

Expected Alarm list fields: Alarm Name (width 36), State (width 12), Metric (width 24), Namespace (width 24), Threshold (width 12)

---

## I. Confirmation Dialogs for Destructive Actions

### I.1 Confirmation Dialog Behavior

| ID | Story | Expected |
|----|-------|----------|
| I.1.1 | A confirmation dialog is displayed for any destructive action (stop, reboot, force deploy, stop task). | The dialog clearly names the resource being acted upon (name and ID), the action to be performed, and asks for explicit confirmation. |
| I.1.2 | I press `y` (lowercase) on a confirmation dialog. | The action is confirmed and executed. |
| I.1.3 | I press `Y` (uppercase) on a confirmation dialog. | The action is confirmed and executed (case-insensitive confirmation). |
| I.1.4 | I press `n` on a confirmation dialog. | The action is cancelled. No API call is made. |
| I.1.5 | I press `N` on a confirmation dialog. | The action is cancelled. |
| I.1.6 | I press `Esc` on a confirmation dialog. | The action is cancelled. The dialog closes. |
| I.1.7 | I press any other key (e.g., `j`, `k`, `space`) on a confirmation dialog. | The dialog does not execute the action. Only `y`/`Y` confirms; all other keys cancel or are ignored. |
| I.1.8 | The dialog is visible. I press `ctrl+c`. | The application force-quits as usual, regardless of the dialog state. |

### I.2 Non-Destructive Actions Skip Confirmation

| ID | Story | Expected |
|----|-------|----------|
| I.2.1 | I press `c` to copy a resource ARN or secret value. | No confirmation dialog appears. The copy executes immediately. |
| I.2.2 | I press `d` to download an S3 object. | No confirmation dialog appears. The download begins immediately. |
| I.2.3 | I press `c` to copy an S3 URI. | No confirmation dialog appears. The copy executes immediately. |

---

## J. Read-Only Mode

### J.1 Read-Only Flag Disables Actions

| ID | Story | Expected |
|----|-------|----------|
| J.1.1 | I launch a9s with a `--read-only` flag (or similar). I navigate to the EC2 list, select a running instance, and press `s` to stop. | Nothing happens. The `s` key is ignored. No confirmation dialog appears. A flash message may indicate "Read-only mode: actions disabled". |
| J.1.2 | In read-only mode, I press `S` to start an instance. | Nothing happens. Action keys are disabled. |
| J.1.3 | In read-only mode, I press `c` to copy a resource ID. | The copy action WORKS. Copying to clipboard is a read-only operation and is not disabled by read-only mode. |
| J.1.4 | In read-only mode, I press `d` to view/download an S3 object. | Downloading (reading data) may be allowed in read-only mode. This is a policy decision -- the story verifies the behavior is consistent and documented. |
| J.1.5 | In read-only mode, I press `i` to invoke a Lambda function. | Nothing happens. Lambda invocation is a write/execute action and is blocked. |
| J.1.6 | In read-only mode, I navigate, filter, sort, view details, view YAML, use all read-only features. | All read-only features work normally. Only mutating actions are disabled. |

---

## K. IAM Permission Error Handling

### K.1 Graceful Permission Failures

| ID | Story | Expected |
|----|-------|----------|
| K.1.1 | I attempt to stop an EC2 instance but lack `ec2:StopInstances`. | After confirming the action, a red error flash appears: "Error: UnauthorizedOperation" or "Error: You are not authorized to perform ec2:StopInstances". The application does not crash. I remain on the EC2 list. |
| K.1.2 | I attempt to invoke a Lambda but lack `lambda:InvokeFunction`. | After the invoke attempt, a red error flash: "Error: AccessDenied". |
| K.1.3 | I attempt to force a new ECS deployment but lack `ecs:UpdateService`. | After confirming, a red error flash with the permission error. |
| K.1.4 | I attempt to set ASG desired capacity but lack `autoscaling:SetDesiredCapacity`. | After confirming, a red error flash. |
| K.1.5 | After any permission error, I press Esc or navigate elsewhere. | The error flash clears. Normal navigation continues. The application state is not corrupted. |

---

## L. Success and Failure Feedback

### L.1 Status Bar / Header Feedback

| ID | Story | Expected |
|----|-------|----------|
| L.1.1 | A non-destructive action succeeds (copy, download). | The header right side flashes a green (#9ece6a) bold success message (e.g., "Copied!"). The flash auto-clears after approximately 2 seconds. |
| L.1.2 | A destructive action succeeds (start, stop, reboot). | The header right side flashes a green bold success message (e.g., "Instance started"). The flash auto-clears after approximately 2 seconds. |
| L.1.3 | An action fails due to API error. | The header right side shows a red (#f7768e) bold error message with the error type. The error persists until navigation or another action. |
| L.1.4 | An action is in progress (download, invocation). | The header or frame content shows a progress indicator until the action completes. |

---

## M. Help Screen Shows Action Key Bindings

### M.1 Help Screen Integration

| ID | Story | Expected |
|----|-------|----------|
| M.1.1 | I am on the EC2 instance list. I press `?`. | The help screen shows action keys: `<S>` Start, `<s>` Stop, and possibly `<R>` Reboot under the RESOURCE or HOTKEYS column. Keys in green bold (#9ece6a), descriptions in white (#c0caf5). |
| M.1.2 | I am on the S3 Objects list. I press `?`. | The help screen shows `<c>` Copy URI and `<d>` Download. |
| M.1.3 | I am on the Secrets Manager list. I press `?`. | The help screen shows `<c>` Copy Secret. |
| M.1.4 | I am on the Lambda list. I press `?`. | The help screen shows `<i>` Invoke and `<d>` Download. |
| M.1.5 | I am on the ECS Services list. I press `?`. | The help screen shows the force deploy key binding. |
| M.1.6 | I am on the ASG list. I press `?`. | The help screen shows the set desired capacity key binding. |
| M.1.7 | I am on the CloudWatch Alarms list. I press `?`. | The help screen shows `<D>` Disable/Enable. |
| M.1.8 | I am on the RDS list. I press `?`. | The help screen shows the reboot key binding. |
| M.1.9 | Action key bindings are only shown for resource types that support them. | The EC2 help screen does not show Lambda-specific keys. The Lambda help screen does not show EC2-specific keys. |

---

## N. Action Keys Do Not Interfere with Existing Bindings

### N.1 Key Binding Conflict Avoidance

| ID | Story | Expected |
|----|-------|----------|
| N.1.1 | I am in filter mode (`/` active) on the EC2 list. I type `s`. | The letter "s" is appended to the filter text. No stop-instance action occurs. |
| N.1.2 | I am in command mode (`:` active) on the Lambda list. I type `i`. | The letter "i" is appended to the command text. No Lambda invocation occurs. |
| N.1.3 | I am viewing the detail view for an EC2 instance. I press `s`. | Nothing happens or scrolls down (if `s` is not mapped in detail view). No stop-instance action occurs from the detail view -- actions are only available from the list view. |
| N.1.4 | I am on the main menu. I press `S`, `s`, `i`, `d`, `D`, `x`. | None of these keys trigger any action. They are either ignored or treated as filter/command input if those modes are active. |

---

## O. Concurrent Actions and Race Conditions

### O.1 Action During Loading

| ID | Story | Expected |
|----|-------|----------|
| O.1.1 | The EC2 list is showing a loading spinner (data is being fetched). I press `s`. | Nothing happens. Action keys are ignored while data is loading. |
| O.1.2 | I initiate a stop-instance action. While waiting for the API response, I press `Esc`. | The navigation back occurs. The API call may still complete in the background but the result is not displayed (or is shown as a flash when I return). |
| O.1.3 | I initiate two rapid actions on the same resource (e.g., start then stop). | The second action is ignored or blocked while the first is in progress. The application does not make conflicting API calls. |

---

## P. Terminal Resize During Action

### P.1 Resize Interactions

| ID | Story | Expected |
|----|-------|----------|
| P.1.1 | A confirmation dialog is displayed. I resize the terminal. | The dialog re-renders at the new size. The confirmation prompt remains readable. |
| P.1.2 | A download progress indicator is showing. I resize the terminal. | The progress indicator re-renders correctly. The download continues uninterrupted. |
| P.1.3 | I resize below minimum dimensions while a confirmation dialog is open. | The "Terminal too narrow/short" error replaces the dialog. Resizing back restores the dialog. |

---

## Q. Key Binding Coverage Summary

| Key | Resource Type | Action | Stories |
|-----|--------------|--------|---------|
| `c` | S3 Objects | Copy S3 URI | A.1.1-A.1.3 |
| `c` | Secrets Manager | Copy secret value | A.2.1-A.2.4 |
| `c` | SSM Parameters | Copy parameter value | A.3.1-A.3.3 |
| `c` | EC2, Lambda, RDS, EKS, etc. | Copy resource ID/ARN | A.4.1-A.4.5 |
| `d` | S3 Objects | Download object | B.1.1-B.1.5 |
| `d` | Lambda | Download deployment package | E.2.1-E.2.4 |
| `S` | EC2 | Start instance | C.1.1-C.1.5 |
| `s` | EC2 | Stop instance | C.2.1-C.2.4 |
| `R` | EC2 | Reboot instance | C.3.1-C.3.3 |
| `R` | ECS Services | Force new deployment | D.1.1-D.1.3 |
| `x` | ECS Tasks | Stop task | D.2.1-D.2.3 |
| `i` | Lambda | Invoke function | E.1.1-E.1.6 |
| `s` | ASG | Set desired capacity | F.1.1-F.1.6 |
| `R` | RDS | Reboot DB instance | G.1.1-G.1.4 |
| `D` | CloudWatch Alarms | Disable/Enable actions | H.1.1-H.1.4 |
| `y`/`Y` | (all) | Confirm destructive action | I.1.2-I.1.3 |
| `n`/`N`/`Esc` | (all) | Cancel destructive action | I.1.4-I.1.6 |
| `?` | (all with actions) | Help shows action keys | M.1.1-M.1.9 |
