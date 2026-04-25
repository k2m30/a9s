# QA Stories — Issue #247: CloudTrail Events related view for all resource types (T key)

Scope: extend EC2's existing CloudTrail Events related-panel entry to every registered resource type, and add a `T` shortcut that opens it from list, detail, and yaml views. Behaviour must mirror EC2 exactly — no new defaults, frame titles, empty-state copy, or delivery-delay hints.

Conventions used below:
- "any resource type" means each resource type registered in a9s (S3, EC2, RDS, Redis, DocumentDB, EKS, Node Groups, Secrets Manager, SSM, KMS, VPC, SG, Subnet, NAT, IGW, EIP, ENI, Route Tables, TGW, VPCE, ELB, TG, R53, ACM, CloudFront, API GW, Lambda, ECS Cluster/Service/Task, ECR, ASG, EB, CFN, CodeBuild, CodePipeline, CodeArtifact, Glue, Athena, OpenSearch, Redshift, DynamoDB, MSK, SQS, SNS, EventBridge, Kinesis, SFN, IAM Roles, IAM Users, IAM Groups, IAM Policies, WAF, Backup, SES, EFS, CloudWatch Alarms, Log Groups, CloudTrail Trails, etc.).
- "Originator view" means the view from which the user opened CloudTrail Events (list, detail, or yaml).

---

## Related-panel entry: presence and parity with EC2

### Story: CloudTrail Events appears in the right-column related panel for every resource type

**Given** the user has navigated into any resource list and selected a row
**When** the right-column related panel renders
**Then** an entry labelled `CloudTrail Events` is present in the related panel
**And** it appears for every registered resource type, not only EC2

**AWS comparison:**

```
aws cloudtrail lookup-events --lookup-attributes AttributeKey=ResourceName,AttributeValue=<arn-or-id>
```

Expected fields visible (in the related panel row): the same `CloudTrail Events` label EC2 currently shows.

### Story: CloudTrail Events entry behaves identically to EC2's existing entry

**Given** the user is on a non-EC2 resource (for example an RDS instance) with the related panel showing `CloudTrail Events`
**When** the user selects that entry and presses Enter
**Then** the CloudTrail Events view opens for that resource
**And** the layout, columns, frame chrome, scroll, and key bindings match the EC2 → CloudTrail Events flow

**AWS comparison:**

```
aws cloudtrail lookup-events --lookup-attributes AttributeKey=ResourceName,AttributeValue=<rds-arn>
```

### Story: Detail view also exposes CloudTrail Events in the related panel

**Given** the user has opened the detail view of any resource type
**When** the right-column related panel is shown
**Then** `CloudTrail Events` is listed there
**And** selecting it and pressing Enter opens the CloudTrail Events view for the current resource

**AWS comparison:**

```
aws cloudtrail lookup-events --lookup-attributes AttributeKey=ResourceName,AttributeValue=<arn>
```

---

## `T` shortcut: list, detail, yaml

### Story: T opens CloudTrail Events from a resource list

**Given** the user is viewing the list of any resource type with one row highlighted
**When** the user presses `T`
**Then** the CloudTrail Events view opens for the highlighted resource
**And** the result is identical to selecting `CloudTrail Events` in the related panel and pressing Enter

**AWS comparison:**

```
aws cloudtrail lookup-events --lookup-attributes AttributeKey=ResourceName,AttributeValue=<highlighted-resource-arn>
```

### Story: T opens CloudTrail Events from a detail view

**Given** the user is viewing the detail (two-column) view of any resource
**When** the user presses `T`
**Then** the CloudTrail Events view opens for that resource

**AWS comparison:**

```
aws cloudtrail lookup-events --lookup-attributes AttributeKey=ResourceName,AttributeValue=<arn>
```

### Story: T opens CloudTrail Events from a yaml view

**Given** the user is viewing the yaml view of any resource
**When** the user presses `T`
**Then** the CloudTrail Events view opens for that resource without first returning to detail or list

**AWS comparison:**

```
aws cloudtrail lookup-events --lookup-attributes AttributeKey=ResourceName,AttributeValue=<arn>
```

### Story: T is a no-op on the main menu

**Given** the user is on the main menu (resource type list)
**When** the user presses `T`
**Then** nothing happens — the menu does not navigate, no error is shown, focus stays on the menu

### Story: T is a no-op inside CloudTrail Events views

**Given** the user is already inside a CloudTrail Events list or a CloudTrail Event detail view
**When** the user presses `T`
**Then** nothing happens — there is no recursive navigation and no error

---

## Actor-mode vs ARN-mode lookup

### Story: IAM Roles use Username for the lookup

**Given** the user is viewing the IAM Roles list or the detail of an IAM Role
**When** the user opens CloudTrail Events (via related panel Enter or `T`)
**Then** the events shown are those where the role's username is the actor, not where the role ARN is a resource

**AWS comparison:**

```
aws cloudtrail lookup-events --lookup-attributes AttributeKey=Username,AttributeValue=<role-name>
```

### Story: IAM Users use Username for the lookup

**Given** the user is viewing the IAM Users list or the detail of an IAM User
**When** the user opens CloudTrail Events (via related panel Enter or `T`)
**Then** the events shown are those where the user's username is the actor

**AWS comparison:**

```
aws cloudtrail lookup-events --lookup-attributes AttributeKey=Username,AttributeValue=<user-name>
```

### Story: Non-IAM-actor resource types use Resource ARN

**Given** the user is viewing any resource type that is not an IAM Role or IAM User (for example RDS, S3, Lambda, VPC, KMS, Secrets Manager)
**When** the user opens CloudTrail Events
**Then** the lookup uses the resource's ARN (or an ARN constructed for resource types without one), not Username

**AWS comparison:**

```
aws cloudtrail lookup-events --lookup-attributes AttributeKey=ResourceName,AttributeValue=<arn>
```

---

## Status bar (bottom-line key helper) legend

### Story: Status bar legend shows T in any resource list

**Given** the user is viewing any resource list
**When** the user looks at the bottom-line key helper
**Then** `T` is shown as one of the available shortcuts with a label indicating CloudTrail Events

### Story: Status bar legend shows T in any detail view

**Given** the user is viewing any resource detail view
**When** the user looks at the bottom-line key helper
**Then** `T` is shown in the legend

### Story: Status bar legend shows T in any yaml view

**Given** the user is viewing any resource yaml view
**When** the user looks at the bottom-line key helper
**Then** `T` is shown in the legend

### Story: Status bar legend does not show T on the main menu

**Given** the user is on the main menu
**When** the user looks at the bottom-line key helper
**Then** `T` is not advertised there

### Story: Status bar legend does not show T inside CloudTrail Events views

**Given** the user is inside a CloudTrail Events list or event detail
**When** the user looks at the bottom-line key helper
**Then** `T` is not advertised there

---

## `?` help overlay

### Story: Help overlay lists T

**Given** the user is in any view that supports `T` (resource list, detail, or yaml)
**When** the user presses `?`
**Then** the help overlay opens
**And** `T` is listed alongside the other navigation keys with a CloudTrail Events label

### Story: Help overlay closes on any key (regression)

**Given** the help overlay is open and lists `T`
**When** the user presses any key
**Then** the overlay closes and focus returns to the originator view

---

## Demo mode

### Story: Demo mode returns contextually appropriate events for each resource type

**Given** a9s is running with `--demo`
**And** the user opens CloudTrail Events for any resource type via related panel or `T`
**Then** the events shown are contextually appropriate for that resource type (for example, RDS shows RDS-related event names, Lambda shows Lambda-related event names)
**And** no real AWS calls are made

**AWS comparison:** N/A (demo mode); equivalent live call would be:

```
aws cloudtrail lookup-events --lookup-attributes AttributeKey=ResourceName,AttributeValue=<arn>
```

### Story: Demo mode CloudTrail Events for IAM Roles uses the role's username context

**Given** a9s is running with `--demo`
**When** the user opens CloudTrail Events for an IAM Role
**Then** the demo events returned are those an actor-mode (Username) lookup would return, not resource-mode

---

## Navigation: returning to the originator

### Story: Esc returns from CloudTrail Events to the originating list

**Given** the user pressed `T` from a resource list
**When** they press Esc inside the CloudTrail Events view
**Then** they return to that resource list with the same row still highlighted

### Story: Esc returns from CloudTrail Events to the originating detail view

**Given** the user pressed `T` from a detail view
**When** they press Esc inside the CloudTrail Events view
**Then** they return to that detail view, not to the list

### Story: Esc returns from CloudTrail Events to the originating yaml view

**Given** the user pressed `T` from a yaml view
**When** they press Esc inside the CloudTrail Events view
**Then** they return to that yaml view, not to detail or list

### Story: Esc returns to originator when entered via the related panel

**Given** the user opened CloudTrail Events by selecting the related-panel entry and pressing Enter
**When** they press Esc inside CloudTrail Events
**Then** they return to the view they came from (list or detail)

---

## Edge cases

### Story: Resource with no ARN field still opens CloudTrail Events

**Given** the user is on a resource type whose underlying resource has no native ARN
**When** they press `T` or open the related-panel entry
**Then** CloudTrail Events opens using the same construction logic EC2 uses today
**And** the user is not shown an error

**AWS comparison:**

```
aws cloudtrail lookup-events --lookup-attributes AttributeKey=ResourceName,AttributeValue=<constructed-arn>
```

### Story: CloudTrail Events with zero results

**Given** the user opens CloudTrail Events for a resource that has no matching events in the lookup window
**When** the view loads
**Then** the empty state shown is identical to the one EC2 shows today — no new copy specific to this issue

### Story: T pressed during list loading

**Given** a resource list is still loading and no row is highlighted yet
**When** the user presses `T`
**Then** nothing happens — no crash, no navigation

### Story: T pressed while filter input is focused

**Given** the user has the list filter input focused and is typing
**When** they press the `T` character
**Then** `T` is treated as filter text, not as the CloudTrail Events shortcut

### Story: T pressed while command input is focused

**Given** the user has the command bar focused
**When** they press `T`
**Then** `T` is treated as command text, not as the shortcut

---

## Regression: EC2's existing flow is unchanged

### Story: EC2 related panel still lists CloudTrail Events exactly as before

**Given** the user is on the EC2 list or an EC2 detail view
**When** they look at the related panel
**Then** the `CloudTrail Events` entry is still present, in the same position, with the same label as before this change

**AWS comparison:**

```
aws cloudtrail lookup-events --lookup-attributes AttributeKey=ResourceName,AttributeValue=<ec2-instance-arn>
```

### Story: EC2 → CloudTrail Events via Enter still works

**Given** the user is on the EC2 related panel with `CloudTrail Events` highlighted
**When** they press Enter
**Then** the CloudTrail Events view opens exactly as it did before, with the same columns, frame title, and behaviour

### Story: T from the EC2 list opens CloudTrail Events

**Given** the user is on the EC2 list with an instance highlighted
**When** they press `T`
**Then** CloudTrail Events opens for that instance, identically to selecting the related-panel entry and pressing Enter

### Story: T from the EC2 detail view opens CloudTrail Events

**Given** the user is on an EC2 detail view
**When** they press `T`
**Then** CloudTrail Events opens for that instance

### Story: T from the EC2 yaml view opens CloudTrail Events

**Given** the user is on the EC2 yaml view
**When** they press `T`
**Then** CloudTrail Events opens for that instance

---

## Documentation surfacing (user-visible)

### Story: Keybindings documentation lists T

**Given** the user reads the published key bindings reference (README or website)
**When** they look at the navigation/related-view shortcuts
**Then** `T` is listed with a CloudTrail Events description, alongside the other navigation keys
