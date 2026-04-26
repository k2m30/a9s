# QA User Stories: Audit and Expand Status Color-Coding Across All Views

Covers GitHub issue #61: expanding RowColorStyle coverage to all resource types with status/state fields. All stories are written from a black-box perspective against the design spec and `views.yaml`.

Row coloring in a9s follows the k9s pattern: the **entire row** is colored based on the resource's status value, not just the status cell. The selected row always overrides status coloring with the blue highlight.

---

## Baseline: Currently Covered Statuses (20 Entries)

The following statuses already have row coloring. These stories verify the existing baseline is intact after the expansion.

### A.1 Green Row (Running / Available / Active / In-Use / Succeeded)

| ID | Story | Expected |
|----|-------|----------|
| A.1.1 | An EC2 instance has State `running`. | The entire row renders in GREEN (`#9ece6a`). |
| A.1.2 | An RDS instance has Status `available`. | The entire row renders in GREEN (`#9ece6a`). |
| A.1.3 | An ECS cluster has Status `ACTIVE`. | The entire row renders in GREEN (`#9ece6a`). Status comparison is case-insensitive. |
| A.1.4 | An ENI has Status `in-use`. | The entire row renders in GREEN (`#9ece6a`). |
| A.1.5 | A Step Functions execution has Status `SUCCEEDED`. | The entire row renders in GREEN (`#9ece6a`). |

**AWS comparison:**

```
aws ec2 describe-instances --query 'Reservations[].Instances[].{ID:InstanceId,State:State.Name}'
aws rds describe-db-instances --query 'DBInstances[].{ID:DBInstanceIdentifier,Status:DBInstanceStatus}'
```

### A.2 Red Row (Stopped / Failed / Error / Deleting / Deleted / Timed_Out)

| ID | Story | Expected |
|----|-------|----------|
| A.2.1 | An EC2 instance has State `stopped`. | The entire row renders in RED (`#f7768e`). |
| A.2.2 | A Step Functions execution has Status `FAILED`. | The entire row renders in RED (`#f7768e`). |
| A.2.3 | A resource has status `error`. | The entire row renders in RED (`#f7768e`). |
| A.2.4 | A resource has status `deleting`. | The entire row renders in RED (`#f7768e`). |
| A.2.5 | A resource has status `deleted`. | The entire row renders in RED (`#f7768e`). |
| A.2.6 | A Step Functions execution has Status `TIMED_OUT`. | The entire row renders in RED (`#f7768e`). |

### A.3 Yellow Row (Pending / Creating / Modifying / Updating / Pending_Redrive)

| ID | Story | Expected |
|----|-------|----------|
| A.3.1 | An EC2 instance has State `pending`. | The entire row renders in YELLOW (`#e0af68`). |
| A.3.2 | A DynamoDB table has Status `CREATING`. | The entire row renders in YELLOW (`#e0af68`). |
| A.3.3 | An RDS instance has Status `modifying`. | The entire row renders in YELLOW (`#e0af68`). |
| A.3.4 | A resource has status `updating`. | The entire row renders in YELLOW (`#e0af68`). |
| A.3.5 | A Step Functions execution with redrive has Status `PENDING_REDRIVE`. | The entire row renders in YELLOW (`#e0af68`). |

### A.4 Gray/Dim Row (Terminated / Shutting-Down / Aborted)

| ID | Story | Expected |
|----|-------|----------|
| A.4.1 | An EC2 instance has State `terminated`. | The entire row renders in DIM (`#565f89`). |
| A.4.2 | An EC2 instance has State `shutting-down`. | The entire row renders in DIM (`#565f89`). |
| A.4.3 | A Step Functions execution has Status `ABORTED`. | The entire row renders in DIM (`#565f89`). |

---

## B. Target Group Health (Child View) -- New Color Mappings

All stories below verify the tg_health child view row coloring.

**AWS comparison:**

```
aws elbv2 describe-target-health --target-group-arn arn:aws:elasticloadbalancing:us-east-1:123456789012:targetgroup/my-tg/abc123
```

Expected fields visible: Target ID, Port, AZ, Health, Reason, Description

### B.1 Health State Colors

| ID | Story | Expected |
|----|-------|----------|
| B.1.1 | A target has Health state `healthy`. | The entire row renders in GREEN (`#9ece6a`). |
| B.1.2 | A target has Health state `unhealthy`. | The entire row renders in RED (`#f7768e`). |
| B.1.3 | A target has Health state `draining`. | The entire row renders in YELLOW (`#e0af68`). |
| B.1.4 | A target has Health state `initial`. | The entire row renders in YELLOW (`#e0af68`) or a transitional color (e.g., Cyan) to distinguish from draining. |
| B.1.5 | A target has Health state `unused`. | The entire row renders in DIM (`#565f89`). |
| B.1.6 | A target has Health state `unavailable`. | The entire row renders in RED (`#f7768e`). |
| B.1.7 | A target group has targets in all six health states. | Each row is colored according to its health state. Running down the list, the colors vary per row, forming a visual health dashboard. |
| B.1.8 | I select a `healthy` (green) target row. | The selected row shows full-width blue background (`#7aa2f7`) with dark foreground (`#1a1b26`), overriding the green color. |
| B.1.9 | I select an `unhealthy` (red) target row. | The selected row shows full-width blue background, overriding the red color. |

---

## C. CloudFormation Stacks -- Pattern-Based Color Mapping

CloudFormation is the only resource requiring pattern-based matching because its status values contain suffixes like `_COMPLETE`, `_IN_PROGRESS`, and `_FAILED`.

**AWS comparison:**

```
aws cloudformation describe-stacks --query 'Stacks[].{Name:StackName,Status:StackStatus}'
```

Expected fields visible: Stack Name, Status, Created, Updated, Description

### C.1 Completion Statuses (Green)

| ID | Story | Expected |
|----|-------|----------|
| C.1.1 | A stack has Status `CREATE_COMPLETE`. | The entire row renders in GREEN (`#9ece6a`). |
| C.1.2 | A stack has Status `UPDATE_COMPLETE`. | The entire row renders in GREEN (`#9ece6a`). |
| C.1.3 | A stack has Status `DELETE_COMPLETE`. | The entire row renders in GREEN (`#9ece6a`). |
| C.1.4 | A stack has Status `IMPORT_COMPLETE`. | The entire row renders in GREEN (`#9ece6a`). |
| C.1.5 | A stack has Status `UPDATE_ROLLBACK_COMPLETE`. | The entire row renders in GREEN (`#9ece6a`) because the rollback itself completed successfully. |

### C.2 In-Progress Statuses (Yellow)

| ID | Story | Expected |
|----|-------|----------|
| C.2.1 | A stack has Status `CREATE_IN_PROGRESS`. | The entire row renders in YELLOW (`#e0af68`). |
| C.2.2 | A stack has Status `UPDATE_IN_PROGRESS`. | The entire row renders in YELLOW (`#e0af68`). |
| C.2.3 | A stack has Status `DELETE_IN_PROGRESS`. | The entire row renders in YELLOW (`#e0af68`). |
| C.2.4 | A stack has Status `ROLLBACK_IN_PROGRESS`. | The entire row renders in YELLOW (`#e0af68`). |
| C.2.5 | A stack has Status `UPDATE_ROLLBACK_IN_PROGRESS`. | The entire row renders in YELLOW (`#e0af68`). |
| C.2.6 | A stack has Status `IMPORT_IN_PROGRESS`. | The entire row renders in YELLOW (`#e0af68`). |
| C.2.7 | A stack has Status `UPDATE_COMPLETE_CLEANUP_IN_PROGRESS`. | The entire row renders in YELLOW (`#e0af68`). |
| C.2.8 | A stack has Status `UPDATE_ROLLBACK_COMPLETE_CLEANUP_IN_PROGRESS`. | The entire row renders in YELLOW (`#e0af68`). |

### C.3 Failed Statuses (Red)

| ID | Story | Expected |
|----|-------|----------|
| C.3.1 | A stack has Status `CREATE_FAILED`. | The entire row renders in RED (`#f7768e`). |
| C.3.2 | A stack has Status `UPDATE_FAILED`. | The entire row renders in RED (`#f7768e`). |
| C.3.3 | A stack has Status `DELETE_FAILED`. | The entire row renders in RED (`#f7768e`). |
| C.3.4 | A stack has Status `ROLLBACK_FAILED`. | The entire row renders in RED (`#f7768e`). |
| C.3.5 | A stack has Status `UPDATE_ROLLBACK_FAILED`. | The entire row renders in RED (`#f7768e`). |
| C.3.6 | A stack has Status `IMPORT_ROLLBACK_FAILED`. | The entire row renders in RED (`#f7768e`). |

### C.4 Rollback Statuses (Red)

| ID | Story | Expected |
|----|-------|----------|
| C.4.1 | A stack has Status `ROLLBACK_COMPLETE`. | The entire row renders in RED (`#f7768e`) because a rollback means the original operation failed. |
| C.4.2 | A stack has Status `IMPORT_ROLLBACK_COMPLETE`. | The entire row renders in RED (`#f7768e`). |

### C.5 CloudFormation Events Child View

| ID | Story | Expected |
|----|-------|----------|
| C.5.1 | A CFN event has Status `CREATE_COMPLETE`. | The event row renders in GREEN. |
| C.5.2 | A CFN event has Status `CREATE_IN_PROGRESS`. | The event row renders in YELLOW. |
| C.5.3 | A CFN event has Status `CREATE_FAILED`. | The event row renders in RED. |
| C.5.4 | A CFN event has Status `DELETE_COMPLETE`. | The event row renders in GREEN. |

**AWS comparison:**

```
aws cloudformation describe-stack-events --stack-name my-stack --query 'StackEvents[].{LogicalId:LogicalResourceId,Status:ResourceStatus}'
```

Expected fields visible: Timestamp, Logical ID, Type, Status, Reason

### C.6 CloudFormation Resources Child View

| ID | Story | Expected |
|----|-------|----------|
| C.6.1 | A CFN resource has Status `CREATE_COMPLETE`. | The resource row renders in GREEN. |
| C.6.2 | A CFN resource has Status `UPDATE_IN_PROGRESS`. | The resource row renders in YELLOW. |
| C.6.3 | A CFN resource has Status `DELETE_FAILED`. | The resource row renders in RED. |

**AWS comparison:**

```
aws cloudformation describe-stack-resources --stack-name my-stack --query 'StackResources[].{LogicalId:LogicalResourceId,Status:ResourceStatus}'
```

Expected fields visible: Logical ID, Physical ID, Type, Status, Drift, Updated

### C.7 Mixed CloudFormation Stack List

| ID | Story | Expected |
|----|-------|----------|
| C.7.1 | An account has stacks in CREATE_COMPLETE, UPDATE_IN_PROGRESS, and CREATE_FAILED states. | Looking at the stack list, each row is visually distinct: the completed stack is green, the in-progress stack is yellow, and the failed stack is red. The Status column text is the same color as the rest of each row (entire-row coloring). |
| C.7.2 | I select a `CREATE_FAILED` (red) stack row. | The selected row shows full-width blue background, overriding the red color. |
| C.7.3 | I filter the CFN list by typing `/FAILED`. | Only stacks with `FAILED` in their status appear. All visible rows are RED. The frame title shows a filtered count (e.g., `cfn(2/15)`). |

---

## D. CloudWatch Alarms -- New Color Mappings

**AWS comparison:**

```
aws cloudwatch describe-alarms --query 'MetricAlarms[].{Name:AlarmName,State:StateValue}'
```

Expected fields visible: Alarm Name, State, Metric, Namespace, Threshold

### D.1 Alarm State Colors

| ID | Story | Expected |
|----|-------|----------|
| D.1.1 | An alarm has State `OK`. | The entire row renders in GREEN (`#9ece6a`). |
| D.1.2 | An alarm has State `ALARM`. | The entire row renders in RED (`#f7768e`). |
| D.1.3 | An alarm has State `INSUFFICIENT_DATA`. | The entire row renders in YELLOW (`#e0af68`). |
| D.1.4 | An account has alarms in all three states. | The alarm list displays a mix of green, red, and yellow rows, forming a visual alarm dashboard. |
| D.1.5 | I filter the alarm list by typing `/ALARM`. | Only alarms in the ALARM state appear. All visible rows are RED. |
| D.1.6 | I select an `ALARM` (red) row. | The selected row shows full-width blue background, overriding the red color. |

---

## E. ECS Tasks (Child View) -- New Color Mappings

**AWS comparison:**

```
aws ecs describe-tasks --cluster my-cluster --tasks TASK_ARN --query 'tasks[].{ID:taskArn,Status:lastStatus}'
```

Expected fields visible: Task ID, Status, Health, Task Definition, Started At, Stopped Reason

### E.1 ECS Task Status Colors

| ID | Story | Expected |
|----|-------|----------|
| E.1.1 | An ECS task has Status `RUNNING`. | The entire row renders in GREEN (`#9ece6a`). |
| E.1.2 | An ECS task has Status `PENDING`. | The entire row renders in YELLOW (`#e0af68`). |
| E.1.3 | An ECS task has Status `STOPPED`. | The entire row renders in RED (`#f7768e`). |
| E.1.4 | An ECS service has tasks in all three states (during a deployment). | Looking at the tasks child view, each row is colored by its status: running tasks are green, pending tasks are yellow, stopped tasks are red. |
| E.1.5 | I select a `STOPPED` (red) task row. | The selected row shows full-width blue background, overriding the red color. |

---

## F. Elastic Beanstalk -- New Color Mappings

Elastic Beanstalk has both a `Status` field and a `Health` field. The `Health` field directly maps to color names.

**AWS comparison:**

```
aws elasticbeanstalk describe-environments --query 'Environments[].{Name:EnvironmentName,Status:Status,Health:Health}'
```

Expected fields visible: Environment, Application, Status, Health, Version

### F.1 Beanstalk Health Colors

| ID | Story | Expected |
|----|-------|----------|
| F.1.1 | An environment has Health `Green`. | The entire row renders in GREEN (`#9ece6a`). |
| F.1.2 | An environment has Health `Yellow`. | The entire row renders in YELLOW (`#e0af68`). |
| F.1.3 | An environment has Health `Red`. | The entire row renders in RED (`#f7768e`). |
| F.1.4 | An environment has Health `Grey`. | The entire row renders in DIM (`#565f89`). |
| F.1.5 | An account has environments in all four health states. | The list shows a mix of green, yellow, red, and dim rows matching each environment's health. |

---

## G. ACM Certificates -- New Color Mappings

**AWS comparison:**

```
aws acm list-certificates --query 'CertificateSummaryList[].{Domain:DomainName,Status:Status}'
```

Expected fields visible: Domain Name, Status, Type, Expires, In Use

### G.1 Certificate Status Colors

| ID | Story | Expected |
|----|-------|----------|
| G.1.1 | A certificate has Status `ISSUED`. | The entire row renders in GREEN (`#9ece6a`). |
| G.1.2 | A certificate has Status `PENDING_VALIDATION`. | The entire row renders in YELLOW (`#e0af68`). |
| G.1.3 | A certificate has Status `EXPIRED`. | The entire row renders in RED (`#f7768e`). |
| G.1.4 | A certificate has Status `REVOKED`. | The entire row renders in RED (`#f7768e`). |
| G.1.5 | A certificate has Status `FAILED`. | The entire row renders in RED (`#f7768e`). |
| G.1.6 | A certificate has Status `INACTIVE`. | The entire row renders in DIM (`#565f89`). |
| G.1.7 | I view the ACM list with a mix of issued and pending certificates. | Issued certificates are green; pending validation certificates are yellow. The visual contrast makes it easy to identify certificates that still need DNS validation. |

---

## H. CloudFront Distributions -- New Color Mappings

**AWS comparison:**

```
aws cloudfront list-distributions --query 'DistributionList.Items[].{Domain:DomainName,Status:Status,Enabled:Enabled}'
```

Expected fields visible: Domain Name, Distribution ID, Status, Enabled, Aliases, Price Class

### H.1 CloudFront Status Colors

| ID | Story | Expected |
|----|-------|----------|
| H.1.1 | A distribution has Status `Deployed`. | The entire row renders in GREEN (`#9ece6a`). |
| H.1.2 | A distribution has Status `InProgress`. | The entire row renders in YELLOW (`#e0af68`). |
| H.1.3 | A distribution has Enabled `false` (disabled). | The entire row renders in DIM (`#565f89`). |
| H.1.4 | I view the CloudFront list with deployed and in-progress distributions. | Deployed distributions are green; in-progress distributions are yellow. |

---

## I. EventBridge Rules -- New Color Mappings

**AWS comparison:**

```
aws events list-rules --query 'Rules[].{Name:Name,State:State}'
```

Expected fields visible: Rule Name, State, Event Bus, Schedule, Description

### I.1 EventBridge Rule State Colors

| ID | Story | Expected |
|----|-------|----------|
| I.1.1 | A rule has State `ENABLED`. | The entire row renders in GREEN (`#9ece6a`). |
| I.1.2 | A rule has State `DISABLED`. | The entire row renders in DIM (`#565f89`). |
| I.1.3 | I view the EventBridge rules list with a mix of enabled and disabled rules. | Enabled rules are green; disabled rules are dim. The visual contrast makes it easy to identify inactive rules. |

---

## J. KMS Keys -- New Color Mappings

**AWS comparison:**

```
aws kms describe-key --key-id KEY_ID --query 'KeyMetadata.{ID:KeyId,State:KeyState}'
```

Expected fields visible: Alias, Key ID, Status, Description

### J.1 KMS Key Status Colors

| ID | Story | Expected |
|----|-------|----------|
| J.1.1 | A key has Status `Enabled`. | The entire row renders in GREEN (`#9ece6a`). |
| J.1.2 | A key has Status `Disabled`. | The entire row renders in DIM (`#565f89`). |
| J.1.3 | A key has Status `PendingDeletion`. | The entire row renders in RED (`#f7768e`). |
| J.1.4 | A key has Status `PendingImport`. | The entire row renders in YELLOW (`#e0af68`). |

---

## K. MSK Clusters -- New Color Mappings

**AWS comparison:**

```
aws kafka list-clusters-v2 --query 'ClusterInfoList[].{Name:ClusterName,State:State}'
```

Expected fields visible: Cluster Name, Type, State, Version

### K.1 MSK Cluster State Colors

| ID | Story | Expected |
|----|-------|----------|
| K.1.1 | A cluster has State `ACTIVE`. | The entire row renders in GREEN (`#9ece6a`). (Already covered by baseline `active`.) |
| K.1.2 | A cluster has State `HEALING`. | The entire row renders in YELLOW (`#e0af68`). |
| K.1.3 | A cluster has State `REBOOTING_BROKER`. | The entire row renders in YELLOW (`#e0af68`). |
| K.1.4 | A cluster has State `CREATING`. | The entire row renders in YELLOW (`#e0af68`). (Already covered by baseline `creating`.) |
| K.1.5 | A cluster has State `DELETING`. | The entire row renders in RED (`#f7768e`). (Already covered by baseline `deleting`.) |
| K.1.6 | A cluster has State `MAINTENANCE`. | The entire row renders in YELLOW (`#e0af68`). |

---

## L. NAT Gateways -- New Color Mappings

**AWS comparison:**

```
aws ec2 describe-nat-gateways --query 'NatGateways[].{ID:NatGatewayId,State:State}'
```

Expected fields visible: Name, NAT Gateway ID, VPC ID, Subnet ID, State, Public IP

### L.1 NAT Gateway State Colors

| ID | Story | Expected |
|----|-------|----------|
| L.1.1 | A NAT gateway has State `available`. | The entire row renders in GREEN (`#9ece6a`). (Already covered by baseline.) |
| L.1.2 | A NAT gateway has State `pending`. | The entire row renders in YELLOW (`#e0af68`). (Already covered by baseline.) |
| L.1.3 | A NAT gateway has State `deleting`. | The entire row renders in RED (`#f7768e`). (Already covered by baseline.) |
| L.1.4 | A NAT gateway has State `deleted`. | The entire row renders in RED (`#f7768e`). (Already covered by baseline.) |
| L.1.5 | A NAT gateway has State `failed`. | The entire row renders in RED (`#f7768e`). (Already covered by baseline.) |

---

## M. SES Identities -- New Color Mappings

**AWS comparison:**

```
aws sesv2 list-email-identities --query 'EmailIdentities[].{Name:IdentityName,Type:IdentityType}'
aws sesv2 get-email-identity --email-identity IDENTITY --query '{Status:VerificationStatus}'
```

Expected fields visible: Identity, Type, Verification, Sending

### M.1 SES Verification Status Colors

| ID | Story | Expected |
|----|-------|----------|
| M.1.1 | An identity has Verification `SUCCESS` (verified). | The entire row renders in GREEN (`#9ece6a`). |
| M.1.2 | An identity has Verification `PENDING`. | The entire row renders in YELLOW (`#e0af68`). |
| M.1.3 | An identity has Verification `FAILED`. | The entire row renders in RED (`#f7768e`). |
| M.1.4 | An identity has Verification `TEMPORARY_FAILURE`. | The entire row renders in YELLOW (`#e0af68`). |
| M.1.5 | An identity has Verification `NOT_STARTED`. | The entire row renders in DIM (`#565f89`). |

---

## N. Athena Workgroups -- New Color Mappings

**AWS comparison:**

```
aws athena list-work-groups --query 'WorkGroups[].{Name:Name,State:State}'
```

Expected fields visible: Workgroup, State, Description, Engine

### N.1 Athena Workgroup State Colors

| ID | Story | Expected |
|----|-------|----------|
| N.1.1 | A workgroup has State `ENABLED`. | The entire row renders in GREEN (`#9ece6a`). |
| N.1.2 | A workgroup has State `DISABLED`. | The entire row renders in DIM (`#565f89`). |

---

## O. Redshift Clusters -- New Color Mappings

**AWS comparison:**

```
aws redshift describe-clusters --query 'Clusters[].{ID:ClusterIdentifier,Status:ClusterStatus}'
```

Expected fields visible: Cluster ID, Status, Node Type, Nodes, Database, Endpoint

### O.1 Redshift Cluster Status Colors

| ID | Story | Expected |
|----|-------|----------|
| O.1.1 | A cluster has Status `available`. | The entire row renders in GREEN (`#9ece6a`). (Already covered by baseline.) |
| O.1.2 | A cluster has Status `rebooting`. | The entire row renders in YELLOW (`#e0af68`). |
| O.1.3 | A cluster has Status `resizing`. | The entire row renders in YELLOW (`#e0af68`). |
| O.1.4 | A cluster has Status `modifying`. | The entire row renders in YELLOW (`#e0af68`). (Already covered by baseline.) |
| O.1.5 | A cluster has Status `deleting`. | The entire row renders in RED (`#f7768e`). (Already covered by baseline.) |
| O.1.6 | A cluster has Status `paused`. | The entire row renders in DIM (`#565f89`). |

---

## P. Transit Gateways -- New Color Mappings

**AWS comparison:**

```
aws ec2 describe-transit-gateways --query 'TransitGateways[].{ID:TransitGatewayId,State:State}'
```

Expected fields visible: Name, TGW ID, State, Owner, Description

### P.1 Transit Gateway State Colors

| ID | Story | Expected |
|----|-------|----------|
| P.1.1 | A transit gateway has State `available`. | The entire row renders in GREEN (`#9ece6a`). (Already covered by baseline.) |
| P.1.2 | A transit gateway has State `pending`. | The entire row renders in YELLOW (`#e0af68`). (Already covered by baseline.) |
| P.1.3 | A transit gateway has State `deleting`. | The entire row renders in RED (`#f7768e`). (Already covered by baseline.) |
| P.1.4 | A transit gateway has State `deleted`. | The entire row renders in RED (`#f7768e`). (Already covered by baseline.) |

---

## Q. VPC Endpoints -- New Color Mappings

**AWS comparison:**

```
aws ec2 describe-vpc-endpoints --query 'VpcEndpoints[].{ID:VpcEndpointId,State:State}'
```

Expected fields visible: Service Name, Endpoint ID, Type, State, VPC ID

### Q.1 VPC Endpoint State Colors

| ID | Story | Expected |
|----|-------|----------|
| Q.1.1 | An endpoint has State `available`. | The entire row renders in GREEN (`#9ece6a`). (Already covered by baseline.) |
| Q.1.2 | An endpoint has State `pending`. | The entire row renders in YELLOW (`#e0af68`). (Already covered by baseline.) |
| Q.1.3 | An endpoint has State `deleting`. | The entire row renders in RED (`#f7768e`). (Already covered by baseline.) |
| Q.1.4 | An endpoint has State `deleted`. | The entire row renders in RED (`#f7768e`). (Already covered by baseline.) |
| Q.1.5 | An endpoint has State `rejected`. | The entire row renders in RED (`#f7768e`). |
| Q.1.6 | An endpoint has State `expired`. | The entire row renders in RED (`#f7768e`). |
| Q.1.7 | An endpoint has State `pendingAcceptance`. | The entire row renders in YELLOW (`#e0af68`). |

---

## R. Cross-Cutting Color Behavior

### R.1 Case Insensitivity

| ID | Story | Expected |
|----|-------|----------|
| R.1.1 | An ECS cluster has Status `ACTIVE` (uppercase). | Row is GREEN. Status matching is case-insensitive. |
| R.1.2 | A CloudWatch alarm has State `Ok` (mixed case). | Row is GREEN. Status matching is case-insensitive. |
| R.1.3 | A CloudFront distribution has Status `Deployed` (title case). | Row is GREEN. Status matching is case-insensitive. |
| R.1.4 | A KMS key has Status `PendingDeletion` (camelCase). | Row is RED. Status matching is case-insensitive. |

### R.2 Selected Row Override

| ID | Story | Expected |
|----|-------|----------|
| R.2.1 | I select any colored row (green, red, yellow, dim) in any resource list. | The selected row always shows full-width blue background (`#7aa2f7`) with dark foreground (`#1a1b26`), bold text. Status coloring is completely overridden. |
| R.2.2 | I move the selection away from a colored row. | The previously selected row reverts to its status-based color (green, red, yellow, or dim). |

### R.3 Unknown/Missing Status Values

| ID | Story | Expected |
|----|-------|----------|
| R.3.1 | A resource has a status value not in any color mapping (e.g., an unknown future AWS status). | The row renders in the default unstyled color (`#c0caf5`, plain white). No crash or visual glitch. |
| R.3.2 | A resource has no status field at all (e.g., S3 buckets, Lambda functions, SQS queues, SNS topics, IAM resources). | Rows render in the default unstyled color (`#c0caf5`, plain white). These resources are unaffected by status coloring. |

### R.4 Interaction with Filter

| ID | Story | Expected |
|----|-------|----------|
| R.4.1 | I filter a resource list (e.g., `/running`). | Only matching rows appear. Matching rows retain their status color (e.g., running rows are still green). |
| R.4.2 | I filter the CloudFormation list by `/FAILED`. | Only stacks with FAILED in their status appear. All visible rows are RED. |
| R.4.3 | I filter the CloudWatch alarms list by `/ALARM`. | Only alarms in the ALARM state appear. All visible rows are RED. |

### R.5 Interaction with Sort

| ID | Story | Expected |
|----|-------|----------|
| R.5.1 | I sort a resource list by status (`S` key). | Rows reorder by status value. Row colors follow each row to its new position (e.g., a green `running` row is still green after sorting). |
| R.5.2 | I sort the CloudFormation list by status. | Rows group by status pattern. All `CREATE_COMPLETE` rows (green) group together, all `_IN_PROGRESS` rows (yellow) group together, etc. |

### R.6 Alternating Row Background

| ID | Story | Expected |
|----|-------|----------|
| R.6.1 | An unstyled resource list (e.g., S3 buckets) has alternating row backgrounds. | Odd rows use default background; even rows use subtle alt background (`#1e2030`). |
| R.6.2 | A status-colored resource list (e.g., EC2 instances with mixed states). | Status row coloring takes precedence over alternating row backgrounds. A green running row does not show the alternating background shift. |

### R.7 Detail View Status Coloring

| ID | Story | Expected |
|----|-------|----------|
| R.7.1 | I open the detail view for an EC2 instance with State `running`. | In the detail view, the State value text renders in GREEN (`#9ece6a`). Other field values remain in the default color (`#c0caf5`). |
| R.7.2 | I open the detail view for a CFN stack with Status `CREATE_FAILED`. | In the detail view, the StackStatus value text renders in RED (`#f7768e`). |
| R.7.3 | I open the detail view for a CloudWatch alarm with State `ALARM`. | In the detail view, the StateValue text renders in RED (`#f7768e`). |

---

## S. Complete Status-to-Color Mapping Reference

This section summarizes all status values and their expected colors after the expansion. Testers can use this as a cross-reference.

### S.1 Green (`#9ece6a`) -- Healthy / Active / Complete

| Status Value | Resource Types |
|-------------|----------------|
| running | EC2 |
| available | RDS, Redis, Redshift, NAT GW, VPC Endpoints, ELB, TGW |
| active | ECS, EKS, DynamoDB |
| in-use | ENI, EBS Volumes (post #66) |
| succeeded | Step Functions executions |
| healthy | Target Group Health targets |
| ok | CloudWatch Alarms |
| *_COMPLETE | CloudFormation stacks (pattern match, excludes ROLLBACK_COMPLETE) |
| issued | ACM Certificates |
| deployed | CloudFront distributions |
| enabled | EventBridge Rules, KMS Keys, Athena Workgroups |
| green | Elastic Beanstalk Health |
| success / verified | SES Identities |
| completed | EBS Snapshots (post #66) |

### S.2 Red (`#f7768e`) -- Failed / Stopped / Unhealthy

| Status Value | Resource Types |
|-------------|----------------|
| stopped | EC2, ECS Tasks |
| failed | General, ACM, SES |
| error | General, EBS Volumes (post #66) |
| deleting | NAT GW, VPC Endpoints, TGW, KMS, EBS Volumes (post #66) |
| deleted | NAT GW, VPC Endpoints, TGW |
| timed_out | Step Functions executions |
| unhealthy | Target Group Health targets |
| unavailable | Target Group Health targets |
| alarm | CloudWatch Alarms |
| *_FAILED | CloudFormation stacks (pattern match) |
| ROLLBACK_COMPLETE | CloudFormation stacks |
| expired | ACM Certificates, VPC Endpoints |
| revoked | ACM Certificates |
| rejected | VPC Endpoints |
| PendingDeletion | KMS Keys |

### S.3 Yellow (`#e0af68`) -- Pending / Transitional

| Status Value | Resource Types |
|-------------|----------------|
| pending | EC2, NAT GW, VPC Endpoints, TGW, SES, EBS Snapshots (post #66) |
| creating | DynamoDB, MSK |
| modifying | RDS, Redshift |
| updating | General |
| pending_redrive | Step Functions |
| draining | Target Group Health targets |
| initial | Target Group Health targets |
| insufficient_data | CloudWatch Alarms |
| *_IN_PROGRESS | CloudFormation stacks (pattern match) |
| pending_validation | ACM Certificates |
| inprogress | CloudFront distributions |
| healing | MSK clusters |
| rebooting_broker | MSK clusters |
| rebooting | Redshift clusters |
| resizing | Redshift clusters |
| PendingImport | KMS Keys |
| pendingAcceptance | VPC Endpoints |
| yellow | Elastic Beanstalk Health |
| temporary_failure | SES Identities |
| maintenance | MSK clusters |

### S.4 Dim (`#565f89`) -- Terminated / Disabled / Inactive

| Status Value | Resource Types |
|-------------|----------------|
| terminated | EC2 |
| shutting-down | EC2 |
| aborted | Step Functions executions |
| unused | Target Group Health targets |
| disabled | EventBridge Rules, KMS Keys, Athena Workgroups, CloudFront (via Enabled: false) |
| inactive | ACM Certificates |
| grey | Elastic Beanstalk Health |
| not_started | SES Identities |
| paused | Redshift clusters |
