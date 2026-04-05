# EC2 Related-Resource Navigation: QA User Stories

Issue: #140 (EC2 related views)
Feature spec: `specs/008-fix-ec2-detail-nav/spec.md`
Design spec: `docs/design/related-resources.md` v4.3
Depends on: `docs/qa/related-resources-stories.md` (Tier 1 common stories)

---

**Scope:** EC2 resource type only. These stories cover the concrete EC2
interactions that exercise the related-resource navigation infrastructure.
They do NOT duplicate the Tier 1 common stories (two-column layout, focus
switching, search/filter, responsive layout, background checking) which
apply to all 66 resource types including EC2.

**Demo fixture IDs used throughout:**

| Resource | ID | Name |
|----------|-----|------|
| EC2 Instance | `i-0a1b2c3d4e5f60001` | web-prod-01 |
| VPC | `vpc-0abc123def456789a` | production-vpc |
| Subnet | `subnet-0aaa111111111111a` | public-a |
| Security Group 1 | `sg-0aaa111111111111a` | acme-web-alb-sg |
| Security Group 2 | `sg-0bbb222222222222b` | acme-web-app-sg |
| AMI | `ami-0abc123def456789a` | amazon-linux-2023 |
| Target Group | `tg-web-prod` | web-prod-tg |
| Auto Scaling Group | `web-prod-asg` | web-prod-asg |
| CloudWatch Alarm 1 | `web-prod-cpu-high` | web-prod-cpu-high |
| CloudWatch Alarm 2 | `web-prod-status-check` | web-prod-status-check |
| Elastic IP | `eipalloc-0abc123def456789a` | web-prod-eip |
| EBS Snapshot | `snap-0abc123def456789a` | web-prod-vol-snap |

---

## Section 1 -- Left Column Navigation (j/k Cursor)

### Story EC2-001: Cursor starts on first field row
**Priority**: P1
**Depends on**: none

**Given** the user opens the detail view for EC2 instance `i-0a1b2c3d4e5f60001` (web-prod-01)
**When** the detail view renders
**Then** the cursor highlight is on the first field row (`InstanceId: i-0a1b2c3d4e5f60001`)
**And** the frame title reads `detail -- i-0a1b2c3d4e5f60001 (web-prod-01)`
**And** the left column shows all EC2 detail fields: InstanceId, State, InstanceType, InstanceLifecycle, ImageId, KeyName, Placement, VpcId, SubnetId, PrivateIpAddress, PrivateDnsName, PublicIpAddress, IamInstanceProfile, SecurityGroups, EbsOptimized, MetadataOptions, LaunchTime, Architecture, Platform, Tags

**Note**: The cursor row uses the standard selection highlight: `#7aa2f7` background, `#1a1b26` foreground, bold.

---

### Story EC2-002: j moves cursor down one row
**Priority**: P1
**Depends on**: EC2-001

**Given** the cursor is on the first row (InstanceId)
**When** the user presses `j`
**Then** the cursor moves to the second row (State)
**And** the first row (InstanceId) loses its highlight
**And** the viewport scroll offset has not changed

---

### Story EC2-003: k moves cursor up one row
**Priority**: P1
**Depends on**: EC2-002

**Given** the cursor is on the second row (State)
**When** the user presses `k`
**Then** the cursor moves back to the first row (InstanceId)

---

### Story EC2-004: k at first field is a no-op
**Priority**: P1
**Depends on**: EC2-001

**Given** the cursor is on the first row (InstanceId)
**When** the user presses `k`
**Then** the cursor remains on the first row (no movement, no wrapping)

---

### Story EC2-005: j at last field is a no-op
**Priority**: P1
**Depends on**: EC2-001

**Given** the cursor is on the last field row (the last sub-field under Tags)
**When** the user presses `j`
**Then** the cursor remains on the last row (no movement, no wrapping)

---

### Story EC2-006: Cursor scrolls viewport at edge of visible area
**Priority**: P1
**Depends on**: EC2-001

**Given** the EC2 detail view has more field rows than fit in the visible height (e.g., 25+ rows in a 20-row terminal)
**And** the cursor is on the last visible row at the bottom of the viewport
**When** the user presses `j`
**Then** the viewport scrolls down so the cursor remains visible on the next row
**And** the rows above shift up by one

**Given** the cursor is at the top of the viewport after scrolling down
**When** the user presses `k`
**Then** the viewport scrolls up so the cursor remains visible

---

### Story EC2-007: g jumps to first row, G jumps to last row
**Priority**: P2
**Depends on**: EC2-001

**Given** the cursor is somewhere in the middle of the EC2 detail field list
**When** the user presses `g`
**Then** the cursor jumps to the first field row (InstanceId)

**Given** the cursor is on the first field row
**When** the user presses `G`
**Then** the cursor jumps to the last field row (last sub-field under Tags)

---

### Story EC2-008: Cursor traverses section headers and sub-fields uniformly
**Priority**: P1
**Depends on**: EC2-001

**Given** the cursor is on the row just before the `Placement:` section header
**When** the user presses `j`
**Then** the cursor moves to the `Placement:` section header row
**When** the user presses `j` again
**Then** the cursor moves to `AvailabilityZone:` (the first sub-field of Placement)

**Note**: The cursor moves one row at a time through ALL rows: plain fields, section headers, navigable fields, and sub-fields alike. It does NOT skip non-navigable rows.

---

## Section 2 -- Left Column Enter on Navigable Field

### Story EC2-009: Enter on VpcId opens VPC detail view
**Priority**: P1
**Depends on**: EC2-001

**Given** the cursor is on the `VpcId: vpc-0abc123def456789a` row
**And** the value `vpc-0abc123def456789a` is underlined in accent color (`#7aa2f7`), indicating it is navigable
**When** the user presses `Enter`
**Then** a new detail view is pushed showing the VPC resource
**And** the frame title reads `detail -- vpc-0abc123def456789a (production-vpc)`
**And** the left column shows VPC detail fields: VpcId, CidrBlock, State, IsDefault, InstanceTenancy, DhcpOptionsId, OwnerId, CidrBlockAssociationSet, Ipv6CidrBlockAssociationSet, Tags
**And** the right column shows VPC's own related resource types (EC2 Instances, Subnets, Security Groups, Route Tables, NAT Gateways, Internet Gateways, etc.)

**AWS comparison:**
```
aws ec2 describe-vpcs --vpc-ids vpc-0abc123def456789a
```

---

### Story EC2-010: Enter on SubnetId opens Subnet detail view
**Priority**: P1
**Depends on**: EC2-001

**Given** the cursor is on the `SubnetId: subnet-0aaa111111111111a` row
**And** the value is underlined in accent color (navigable)
**When** the user presses `Enter`
**Then** a new detail view is pushed showing the Subnet resource
**And** the frame title reads `detail -- subnet-0aaa111111111111a (public-a)`
**And** the left column shows Subnet detail fields: SubnetId, VpcId, CidrBlock, AvailabilityZone, AvailabilityZoneId, State, AvailableIpAddressCount, MapPublicIpOnLaunch, DefaultForAz, SubnetArn, OwnerId, Tags

**AWS comparison:**
```
aws ec2 describe-subnets --subnet-ids subnet-0aaa111111111111a
```

---

### Story EC2-011: Enter on SecurityGroups sub-field GroupId opens SG detail
**Priority**: P1
**Depends on**: EC2-001

**Given** the EC2 detail shows multiple SecurityGroups, each with a GroupId sub-field
**And** the cursor is on `GroupId: sg-0aaa111111111111a` (the first security group)
**And** the value is underlined in accent color (navigable)
**When** the user presses `Enter`
**Then** a new detail view is pushed showing the Security Group resource
**And** the frame title reads `detail -- sg-0aaa111111111111a (acme-web-alb-sg)`
**And** the left column shows SG detail fields: GroupId, GroupName, VpcId, Description, OwnerId, SecurityGroupArn, IpPermissions, IpPermissionsEgress, Tags

**AWS comparison:**
```
aws ec2 describe-security-groups --group-ids sg-0aaa111111111111a
```

---

### Story EC2-012: Each SecurityGroup sub-field is independently navigable
**Priority**: P1
**Depends on**: EC2-011

**Given** the EC2 instance has two SecurityGroups: `sg-0aaa111111111111a` and `sg-0bbb222222222222b`
**When** the user moves the cursor to the first GroupId (`sg-0aaa111111111111a`)
**Then** that value is underlined; pressing Enter opens the acme-web-alb-sg detail

**When** the user returns (Esc) and moves the cursor to the second GroupId (`sg-0bbb222222222222b`)
**Then** that value is also underlined; pressing Enter opens the acme-web-app-sg detail

**Note**: Each array item is a separate row with its own navigation target.

---

### Story EC2-013: Enter on ImageId opens AMI detail view
**Priority**: P1
**Depends on**: EC2-001

**Given** the cursor is on the `ImageId: ami-0abc123def456789a` row
**And** the value is underlined in accent color (navigable)
**When** the user presses `Enter`
**Then** a new detail view is pushed showing the AMI resource
**And** the frame title reads `detail -- ami-0abc123def456789a (amazon-linux-2023)`
**And** the left column shows AMI detail fields: ImageId, Name, State, Description, Architecture, PlatformDetails, RootDeviceType, VirtualizationType, EnaSupport, BootMode, CreationDate, DeprecationTime, Public, OwnerId, ImageLocation, BlockDeviceMappings, Tags

**AWS comparison:**
```
aws ec2 describe-images --image-ids ami-0abc123def456789a
```

---

### Story EC2-014: Enter on a non-navigable field is a no-op
**Priority**: P1
**Depends on**: EC2-001

**Given** the cursor is on the `InstanceType: t3.large` row
**And** the value `t3.large` is NOT underlined (plain text in `#c0caf5`)
**When** the user presses `Enter`
**Then** nothing happens -- no navigation, no flash message, no error

**Given** the cursor is on the `State: running` row
**When** the user presses `Enter`
**Then** nothing happens

**Given** the cursor is on `PrivateIpAddress: 10.0.48.175`
**When** the user presses `Enter`
**Then** nothing happens

**Note**: Non-navigable fields are visually distinguishable by the absence of underline styling on the value.

---

### Story EC2-015: Enter on a section header is a no-op
**Priority**: P1
**Depends on**: EC2-008

**Given** the cursor is on the `Placement:` section header row
**And** the header is rendered in amber bold (`#e0af68`)
**When** the user presses `Enter`
**Then** nothing happens

**Given** the cursor is on the `MetadataOptions:` section header row
**When** the user presses `Enter`
**Then** nothing happens

**Given** the cursor is on the `SecurityGroups:` section header row
**When** the user presses `Enter`
**Then** nothing happens -- only the individual GroupId sub-fields within SecurityGroups are navigable, not the section header itself

---

### Story EC2-016: IamInstanceProfile.Arn is NOT navigable
**Priority**: P2
**Depends on**: EC2-001

**Given** the cursor is on the `Arn:` sub-field under `IamInstanceProfile:`
**And** the value shows an instance profile ARN
**When** the user looks at the value styling
**Then** it is NOT underlined (plain text in `#c0caf5`)
**When** the user presses `Enter`
**Then** nothing happens

**Note**: Instance profile ARNs are not IAM Role ARNs. Navigating to a role via an instance profile ARN would route to the wrong resource type. The EC2-to-Role relationship is algorithmic (requires a multi-hop lookup), not a direct forward field.

---

### Story EC2-017: Navigable field underline disappears under cursor
**Priority**: P2
**Depends on**: EC2-009

**Given** the cursor is NOT on the VpcId row
**When** the user looks at the VpcId value
**Then** it is underlined in accent color (`#7aa2f7`)

**Given** the cursor moves to the VpcId row
**When** the full-row selection highlight takes over
**Then** the underline disappears on the VpcId value

**Given** the cursor moves off the VpcId row
**When** the user looks at the VpcId value again
**Then** the underline reappears

---

## Section 3 -- Right Column (RELATED Panel)

### Story EC2-018: Right column visible by default showing EC2 related types
**Priority**: P1
**Depends on**: EC2-001

**Given** the terminal is at least 100 columns wide
**When** the user opens the detail view for EC2 instance `i-0a1b2c3d4e5f60001`
**Then** the right column is visible with the `RELATED` header at the top (dim `#565f89`)
**And** the right column lists EC2's reverse/algorithmic relationships: Target Groups, Auto Scaling Groups, CloudWatch Alarms, EKS Node Groups, CloudFormation Stacks, Elastic Beanstalk, EBS Snapshots, Elastic IPs, CloudTrail Events
**And** the right column does NOT list VPC, Subnet, Security Groups, or AMIs (those are forward fields in the left column)
**And** the column separator `|` is drawn in dim color (`#414868`) because the left column is focused

**AWS comparison:**
There is no single AWS CLI command that shows all reverse relationships for an EC2 instance. Each would require a separate call:
```
aws elbv2 describe-target-health ...
aws autoscaling describe-auto-scaling-instances --instance-ids i-0a1b2c3d4e5f60001
aws cloudwatch describe-alarms --dimensions Name=InstanceId,Value=i-0a1b2c3d4e5f60001
aws ec2 describe-addresses --filters Name=instance-id,Values=i-0a1b2c3d4e5f60001
```

---

### Story EC2-019: Right column rows start dim during initial load
**Priority**: P1
**Depends on**: EC2-018

**Given** the user opens the EC2 detail view for the first time for this instance
**When** the detail view first renders
**Then** all right-column rows appear in dim text (`#565f89`)
**And** the left column is immediately usable with navigable field underlines visible
**And** no spinners or "Loading..." text appear anywhere

---

### Story EC2-020: Right column rows light up as counts arrive
**Priority**: P1
**Depends on**: EC2-019

**Given** background availability checks are running
**When** the Auto Scaling Groups check completes and finds 1 ASG
**Then** the "Auto Scaling Groups" row changes from dim to normal text (`#c0caf5`) and displays "Auto Scaling Groups (1)"

**When** the CloudWatch Alarms check completes and finds 2 alarms
**Then** the "CloudWatch Alarms" row changes to normal text and displays "CloudWatch Alarms (2)"

**When** the EKS Node Groups check completes and finds 0 matches
**Then** the "EKS Node Groups" row stays dim

**Note**: Rows light up silently -- no transition animation, no flash message.

---

### Story EC2-021: Tab moves focus to right column
**Priority**: P1
**Depends on**: EC2-018

**Given** the left column is focused with the cursor on a field row
**When** the user presses `Tab`
**Then** the cursor disappears from the left column
**And** a cursor appears on the first available (non-dim) row in the right column
**And** the column separator changes from dim (`#414868`) to accent color (`#7aa2f7`)
**And** navigable field underlines in the left column remain visible

---

### Story EC2-022: Tab returns focus to left column
**Priority**: P1
**Depends on**: EC2-021

**Given** the right column is focused with the cursor on a related type row
**When** the user presses `Tab`
**Then** the cursor disappears from the right column
**And** the cursor reappears in the left column at the previously selected field row
**And** the column separator returns to dim color (`#414868`)

---

### Story EC2-023: r key toggles right column off and on
**Priority**: P1
**Depends on**: EC2-018

**Given** the right column is visible
**When** the user presses `r`
**Then** the right column disappears
**And** the left column expands to fill the full frame width
**And** the column separator disappears
**And** navigable field underlines remain visible on the left column

**Given** the right column is hidden
**When** the user presses `r` again
**Then** the two-column layout is restored with the right column at 32 characters wide
**And** the column separator reappears

---

### Story EC2-024: h/l switches focus between columns
**Priority**: P2
**Depends on**: EC2-021

**Given** the left column is focused
**When** the user presses `l`
**Then** focus moves to the right column (same as Tab)

**Given** the right column is focused
**When** the user presses `h`
**Then** focus moves to the left column (same as Tab back)

**Given** the left column is already focused
**When** the user presses `h`
**Then** nothing happens (already on the leftmost column)

---

## Section 4 -- Right Column Enter (count=1)

### Story EC2-025: Enter on Target Groups (count=1) opens TG detail directly
**Priority**: P1
**Depends on**: EC2-021

**Given** the right column is focused and the cursor is on "Target Groups (1)"
**When** the user presses `Enter`
**Then** the Target Group detail view is pushed onto the view stack (no intermediate list)
**And** the frame title reads `detail -- tg-web-prod (web-prod-tg)`
**And** the left column shows TG detail fields: TargetGroupName, TargetGroupArn, Port, Protocol, ProtocolVersion, VpcId, TargetType, HealthCheckPath, HealthCheckPort, HealthCheckProtocol, HealthCheckEnabled, HealthCheckIntervalSeconds, HealthCheckTimeoutSeconds, HealthyThresholdCount, UnhealthyThresholdCount, Matcher, LoadBalancerArns
**And** the right column shows the TG's own related types

**AWS comparison:**
```
aws elbv2 describe-target-groups --names web-prod-tg
```

---

### Story EC2-026: Esc from TG detail returns to EC2 detail at same cursor
**Priority**: P1
**Depends on**: EC2-025

**Given** the user navigated from EC2 detail (right column) to the TG detail
**When** the user presses `Esc`
**Then** the TG detail view is popped from the stack
**And** the EC2 detail view reappears
**And** the right column is focused with the cursor on the same "Target Groups (1)" row where Enter was pressed

---

### Story EC2-027: Enter on Auto Scaling Groups (count=1) opens ASG detail
**Priority**: P1
**Depends on**: EC2-021

**Given** the right column is focused and the cursor is on "Auto Scaling Groups (1)"
**When** the user presses `Enter`
**Then** the ASG detail view is pushed directly (no intermediate list)
**And** the frame title identifies the ASG (e.g., `detail -- web-prod-asg (web-prod-asg)`)
**And** the left column shows ASG detail fields: AutoScalingGroupName, AutoScalingGroupARN, MinSize, MaxSize, DesiredCapacity, AvailabilityZones, LaunchConfigurationName, HealthCheckType, HealthCheckGracePeriod, TargetGroupARNs, LoadBalancerNames, SuspendedProcesses, TerminationPolicies, VPCZoneIdentifier, CreatedTime, Tags

**AWS comparison:**
```
aws autoscaling describe-auto-scaling-groups --auto-scaling-group-names web-prod-asg
```

---

### Story EC2-028: Enter on Elastic IPs (count=1) opens EIP detail
**Priority**: P1
**Depends on**: EC2-021

**Given** the right column is focused and the cursor is on "Elastic IPs (1)"
**When** the user presses `Enter`
**Then** the EIP detail view is pushed directly
**And** the frame title identifies the Elastic IP (e.g., `detail -- eipalloc-0abc123def456789a (web-prod-eip)`)
**And** the left column shows EIP detail fields: AllocationId, PublicIp, AssociationId, InstanceId, Domain, NetworkBorderGroup, SubnetId, PrivateIpAddress, NetworkInterfaceId, Tags

**AWS comparison:**
```
aws ec2 describe-addresses --allocation-ids eipalloc-0abc123def456789a
```

---

## Section 5 -- Right Column Enter (count>1)

### Story EC2-029: Enter on CloudWatch Alarms (count=2) opens filtered alarm list
**Priority**: P1
**Depends on**: EC2-021

**Given** the right column is focused and the cursor is on "CloudWatch Alarms (2)"
**When** the user presses `Enter`
**Then** a resource list view is pushed showing exactly 2 alarms
**And** the frame title reads `alarms(2) -- i-0a1b2c3d4e5f60001 (web-prod-01)`
**And** the list shows ONLY the 2 alarms related to this EC2 instance (web-prod-cpu-high, web-prod-status-check), not all alarms in the account
**And** the list columns show: Alarm Name, State, Metric, Namespace, Threshold

**AWS comparison:**
```
aws cloudwatch describe-alarms --dimensions Name=InstanceId,Value=i-0a1b2c3d4e5f60001
```
Expected: exactly 2 alarms in the filtered result.

---

### Story EC2-030: Enter on alarm in filtered list opens alarm detail
**Priority**: P1
**Depends on**: EC2-029

**Given** the filtered alarm list is showing 2 alarms
**And** the cursor is on "web-prod-cpu-high"
**When** the user presses `Enter`
**Then** the alarm detail view is pushed
**And** the frame title reads `detail -- web-prod-cpu-high (web-prod-cpu-high)`
**And** the left column shows alarm detail fields: AlarmName, AlarmArn, StateValue, StateReason, StateUpdatedTimestamp, StateTransitionedTimestamp, MetricName, Namespace, Statistic, Period, EvaluationPeriods, DatapointsToAlarm, Threshold, ComparisonOperator, TreatMissingData, Dimensions, AlarmDescription, AlarmActions, OKActions, InsufficientDataActions, ActionsEnabled

**AWS comparison:**
```
aws cloudwatch describe-alarms --alarm-names web-prod-cpu-high
```

---

### Story EC2-031: Esc from alarm detail returns to filtered alarm list
**Priority**: P1
**Depends on**: EC2-030

**Given** the user is viewing the alarm detail for web-prod-cpu-high
**When** the user presses `Esc`
**Then** the alarm detail is popped
**And** the filtered alarm list reappears showing the same 2 alarms
**And** the cursor is on the same alarm row where Enter was pressed

---

### Story EC2-032: Esc from filtered alarm list returns to EC2 detail
**Priority**: P1
**Depends on**: EC2-031

**Given** the user is back on the filtered alarm list
**When** the user presses `Esc`
**Then** the filtered alarm list is popped
**And** the EC2 detail view reappears
**And** the right column is focused with the cursor on "CloudWatch Alarms (2)"

---

### Story EC2-033: CloudFormation Stacks with count=0 -- Enter is a no-op
**Priority**: P1
**Depends on**: EC2-021

**Given** the right column shows "CloudFormation Stacks" in dim text (`#565f89`) because the count is 0 (no stack manages this instance)
**When** the user presses j/k in the right column
**Then** the cursor skips over the dim "CloudFormation Stacks" row

**Note**: The cursor cannot land on dim rows, so pressing Enter on a count=0 row is structurally impossible.

---

### Story EC2-034: EBS Snapshots via multi-hop (count>1)
**Priority**: P2
**Depends on**: EC2-021

**Given** the right column is focused and shows "EBS Snapshots (2)" because the instance has attached volumes with existing snapshots
**When** the user presses `Enter`
**Then** a filtered snapshot list is pushed showing exactly 2 EBS snapshots
**And** the frame title reads `ebs-snap(2) -- i-0a1b2c3d4e5f60001 (web-prod-01)`
**And** each snapshot in the list shows: Name, Snapshot ID, State, Volume ID, Size (GiB), Encrypted, Description, Started, Progress

**AWS comparison:**
```
aws ec2 describe-instances --instance-ids i-0a1b2c3d4e5f60001 --query 'Reservations[].Instances[].BlockDeviceMappings[].Ebs.VolumeId'
aws ec2 describe-snapshots --filters Name=volume-id,Values=vol-xxx,vol-yyy
```
Multi-hop: instance -> volume IDs -> snapshots of those volumes.

---

## Section 6 -- Full Navigation Chain (Back and Forth)

### Story EC2-035: Chain A -- EC2 to VPC detail and back
**Priority**: P1
**Depends on**: EC2-009

**Screen 1 -- EC2 detail:**
- Frame title: `detail -- i-0a1b2c3d4e5f60001 (web-prod-01)`
- Left column focused, cursor on VpcId row
- VpcId value `vpc-0abc123def456789a` is underlined in accent color
- Right column visible with EC2 related types

**Action:** User presses `Enter`

**Screen 2 -- VPC detail:**
- Frame title: `detail -- vpc-0abc123def456789a (production-vpc)`
- Left column shows VPC fields: VpcId, CidrBlock, State, IsDefault, InstanceTenancy, DhcpOptionsId, OwnerId, CidrBlockAssociationSet, Ipv6CidrBlockAssociationSet, Tags
- Cursor on first row (VpcId)
- Right column shows VPC's related types (EC2 Instances, Subnets, Security Groups, Route Tables, NAT Gateways, Internet Gateways, etc.)
- Header: `a9s v3.28.0  prod:us-east-1`

**Action:** User presses `Esc`

**Screen 3 -- back to EC2 detail:**
- Frame title: `detail -- i-0a1b2c3d4e5f60001 (web-prod-01)`
- Cursor is restored to the VpcId row (the row where Enter was pressed)
- Left column focus is active
- Right column is in the same state as before (visible, same counts)

---

### Story EC2-036: Chain B -- EC2 to Subnet detail and back
**Priority**: P1
**Depends on**: EC2-010

**Screen 1 -- EC2 detail:**
- Cursor on SubnetId row, value `subnet-0aaa111111111111a` underlined

**Action:** User presses `Enter`

**Screen 2 -- Subnet detail:**
- Frame title: `detail -- subnet-0aaa111111111111a (public-a)`
- Left column shows: SubnetId, VpcId (navigable, underlined), CidrBlock, AvailabilityZone, AvailabilityZoneId, State, AvailableIpAddressCount, MapPublicIpOnLaunch, DefaultForAz, SubnetArn, OwnerId, Tags
- Right column shows Subnet's related types (EC2 Instances, NAT Gateways, Network Interfaces, Route Tables, CloudTrail Events, etc.)

**Action:** User presses `Esc`

**Screen 3 -- back to EC2 detail:**
- Cursor restored to SubnetId row
- All state preserved

---

### Story EC2-037: Chain C -- EC2 to SecurityGroup detail and back
**Priority**: P1
**Depends on**: EC2-011

**Screen 1 -- EC2 detail:**
- Cursor on first GroupId sub-field under SecurityGroups: `sg-0aaa111111111111a`

**Action:** User presses `Enter`

**Screen 2 -- Security Group detail:**
- Frame title: `detail -- sg-0aaa111111111111a (acme-web-alb-sg)`
- Left column shows: GroupId, GroupName, VpcId (navigable, underlined), Description, OwnerId, SecurityGroupArn, IpPermissions, IpPermissionsEgress, Tags
- Right column shows SG's related types (EC2 Instances, RDS Instances, ELBs, etc.)

**Action:** User presses `Esc`

**Screen 3 -- back to EC2 detail:**
- Cursor restored to the first GroupId sub-field row

---

### Story EC2-038: Chain D -- EC2 to right-col TG (count=1) and back
**Priority**: P1
**Depends on**: EC2-025, EC2-026

**Screen 1 -- EC2 detail, left column focused:**
- Cursor on InstanceId row

**Action:** User presses `Tab`

**Screen 2 -- EC2 detail, right column focused:**
- Cursor moves to first available row in right column (e.g., "Target Groups (1)")
- Column separator changes to accent color (`#7aa2f7`)
- Left column cursor disappears, navigable underlines remain

**Action:** User presses `j`/`k` to position cursor on "Target Groups (1)", then presses `Enter`

**Screen 3 -- TG detail:**
- Frame title: `detail -- tg-web-prod (web-prod-tg)`
- Left column shows TG detail fields
- Right column shows TG's related types

**Action:** User presses `Esc`

**Screen 4 -- back to EC2 detail, right column focused:**
- Cursor is on "Target Groups (1)" in the right column
- Column separator is accent color
- Left column shows EC2 fields with navigable underlines

---

### Story EC2-039: Chain E -- EC2 to filtered alarm list to alarm detail and back
**Priority**: P1
**Depends on**: EC2-029, EC2-030, EC2-031, EC2-032

**Screen 1 -- EC2 detail, right column focused:**
- Cursor on "CloudWatch Alarms (2)"

**Action:** User presses `Enter`

**Screen 2 -- Filtered alarm list:**
- Frame title: `alarms(2) -- i-0a1b2c3d4e5f60001 (web-prod-01)`
- List shows exactly 2 alarms: web-prod-cpu-high, web-prod-status-check
- Cursor on first alarm row

**Action:** User presses `Enter` on web-prod-cpu-high

**Screen 3 -- Alarm detail:**
- Frame title: `detail -- web-prod-cpu-high (web-prod-cpu-high)`
- Left column shows alarm fields (AlarmName, StateValue, MetricName, etc.)
- Right column shows alarm's related types

**Action:** User presses `Esc`

**Screen 4 -- Filtered alarm list (same as Screen 2):**
- Cursor on web-prod-cpu-high
- Still shows only 2 alarms

**Action:** User presses `Esc`

**Screen 5 -- EC2 detail, right column focused:**
- Cursor on "CloudWatch Alarms (2)"
- All EC2 detail state preserved

---

### Story EC2-040: Chain F -- EC2 to VPC to Subnet (depth=4) and unwind
**Priority**: P1
**Depends on**: EC2-009

**Screen 1 -- EC2 detail (depth 3: menu -> ec2-list -> ec2-detail):**
- Header: `a9s v3.28.0  prod:us-east-1`
- Cursor on VpcId row

**Action:** User presses `Enter`

**Screen 2 -- VPC detail (depth 4: menu -> ec2-list -> ec2-detail -> vpc-detail):**
- Header: `a9s v3.28.0  prod:us-east-1` (depth <= 4, version still shown)
- Frame title: `detail -- vpc-0abc123def456789a (production-vpc)`
- Left column shows VPC fields
- VpcId value in the Subnet detail's left column will be navigable

**Action:** User presses `Tab` to focus right column, moves cursor to "Subnets (6)", presses `Enter`

**Screen 3 -- Filtered subnet list (depth 5):**
- Header changes to: `a9s [5]  prod:us-east-1` (depth > 4, depth indicator replaces version)
- Frame title: `subnets(6) -- vpc-0abc123def456789a (production-vpc)`
- List shows exactly 6 subnets

**Action:** User selects a subnet and presses `Enter`

**Screen 4 -- Subnet detail (depth 6):**
- Header: `a9s [6]  prod:us-east-1`
- Frame title: `detail -- subnet-0aaa111111111111a (public-a)`
- Left column shows Subnet fields; VpcId is navigable (underlined)
- Right column shows Subnet's related types

**Action:** User presses `Esc`

**Screen 5 -- Filtered subnet list (depth 5):**
- Header: `a9s [5]  prod:us-east-1`
- Cursor on the subnet that was opened

**Action:** User presses `Esc`

**Screen 6 -- VPC detail (depth 4):**
- Header: `a9s v3.28.0  prod:us-east-1` (back to version, depth <= 4)
- Right column focused, cursor on "Subnets (6)"

**Action:** User presses `Esc`

**Screen 7 -- EC2 detail (depth 3):**
- Header: `a9s v3.28.0  prod:us-east-1`
- Cursor on VpcId row in left column

---

### Story EC2-041: Chain G -- Mixed left-and-right navigation
**Priority**: P1
**Depends on**: EC2-011, EC2-021

**Screen 1 -- EC2 detail:**
- Left column focused, cursor on first GroupId: `sg-0aaa111111111111a`

**Action:** User presses `Enter` (left column, navigable field)

**Screen 2 -- SG detail for acme-web-alb-sg:**
- Frame title: `detail -- sg-0aaa111111111111a (acme-web-alb-sg)`
- Right column shows SG's reverse relationships (EC2 Instances, RDS Instances, Load Balancers, etc.)

**Action:** User presses `Tab` to focus right column, moves cursor to "EC2 Instances", presses `Enter`

**Screen 3 -- Filtered EC2 list:**
- Frame title shows EC2 instances using this SG (e.g., `ec2(N) -- sg-0aaa111111111111a (acme-web-alb-sg)`)
- List shows only EC2 instances that reference this security group

**Action:** User presses `Esc`

**Screen 4 -- SG detail (right column focused):**
- Cursor on "EC2 Instances" row

**Action:** User presses `Esc`

**Screen 5 -- EC2 detail:**
- Cursor on first GroupId sub-field

**AWS comparison:**
```
aws ec2 describe-security-groups --group-ids sg-0aaa111111111111a
aws ec2 describe-instances --filters Name=instance.group-id,Values=sg-0aaa111111111111a
```

---

## Section 7 -- Edge Cases

### Story EC2-042: Navigable field target not in demo fixtures
**Priority**: P2
**Depends on**: EC2-009

**Given** the EC2 detail shows a navigable field with a resource ID that does not exist in the demo fixtures (e.g., the AMI was deregistered or belongs to another account)
**When** the user presses `Enter` on that field
**Then** a flash message "Resource not found" appears briefly in the header
**And** the user stays on the EC2 detail view at the same cursor position
**And** the flash message disappears after approximately 2 seconds

---

### Story EC2-043: Right column all count=0
**Priority**: P2
**Depends on**: EC2-019

**Given** all background checks have completed and every right-column row has count=0 (all dim)
**When** the user presses `Tab`
**Then** focus moves to the right column (separator changes to accent color)
**But** the cursor cannot land on any row (all rows are dim and skipped)
**And** pressing `Enter` has no effect
**And** pressing `Tab` again returns focus to the left column

---

### Story EC2-044: Terminal width below 60 -- too narrow for right column
**Priority**: P2
**Depends on**: none

**Given** the terminal is less than 60 columns wide
**When** the user opens the EC2 detail view
**Then** the application displays "Terminal too narrow"

**Given** the terminal is between 60 and 99 columns wide and the right column is in stacked mode
**When** the user presses `r` to toggle the right column
**Then** the related section below the detail fields toggles on/off normally

---

### Story EC2-045: Stacked layout at 80-99 columns
**Priority**: P2
**Depends on**: EC2-018

**Given** the terminal is 90 columns wide
**When** the user opens the EC2 detail view
**Then** the layout is stacked: detail fields appear on top, a dim separator line `-- Related ---` divides them from the related types list below
**And** there is no vertical separator character `|`
**And** Tab switches focus between the detail section (top) and the related section (bottom)
**And** j/k moves within whichever section has focus

---

### Story EC2-046: Depth indicator at depth 6+
**Priority**: P2
**Depends on**: EC2-040

**Given** the user has navigated from EC2 detail through 6 or more levels (e.g., EC2 -> VPC -> Subnets list -> Subnet detail -> VPC (via VpcId forward field) -> Subnets again)
**When** the user looks at the header
**Then** the version number is replaced by `[6]` (or the current depth number)
**And** the header reads: `a9s [6]  prod:us-east-1            ? for help`

**When** the user presses `Esc` to return to depth 4 or below
**Then** the header reverts to showing the version number: `a9s v3.28.0  prod:us-east-1`

---

### Story EC2-047: Esc at depth 1 returns to resource list
**Priority**: P1
**Depends on**: EC2-001

**Given** the user opened the EC2 detail view from the EC2 resource list (view stack: menu -> ec2-list -> ec2-detail)
**When** the user presses `Esc` with no search or filter active
**Then** the EC2 detail is popped and the EC2 resource list reappears
**And** the cursor is on the same EC2 instance row from which the detail was opened
**And** the user does NOT land on the main menu (Esc pops one level, not all the way back)

---

### Story EC2-048: Right column toggle state persists across navigation
**Priority**: P2
**Depends on**: EC2-023, EC2-009

**Given** the user presses `r` to hide the right column in the EC2 detail view
**When** the user presses Enter on VpcId to navigate to the VPC detail
**Then** the VPC detail view also has the right column hidden

**When** the user presses `Esc` to return to the EC2 detail
**Then** the EC2 detail view still has the right column hidden

**When** the user presses `r` to show the right column
**And** then navigates to SubnetId detail
**Then** the Subnet detail view has the right column visible

---

### Story EC2-049: Navigable fields work with right column hidden
**Priority**: P1
**Depends on**: EC2-023, EC2-009

**Given** the user has pressed `r` to hide the right column
**And** the left column fills the full frame width
**When** the user moves the cursor to VpcId and presses `Enter`
**Then** the VPC detail view opens at full width (right column still hidden)
**And** navigation works identically to when the right column is visible

---

### Story EC2-050: Copy field value from left column
**Priority**: P2
**Depends on**: EC2-001

**Given** the left column is focused and the cursor is on `InstanceId: i-0a1b2c3d4e5f60001`
**When** the user presses `c`
**Then** the value `i-0a1b2c3d4e5f60001` is copied to the clipboard
**And** the header shows "Copied!" flash in green (`#9ece6a`) for approximately 2 seconds

**Given** the cursor is on `VpcId: vpc-0abc123def456789a`
**When** the user presses `c`
**Then** the value `vpc-0abc123def456789a` is copied to the clipboard

---

### Story EC2-051: Copy from right column copies type name
**Priority**: P2
**Depends on**: EC2-021

**Given** the right column is focused and the cursor is on "Auto Scaling Groups (1)"
**When** the user presses `c`
**Then** the text "Auto Scaling Groups" is copied to the clipboard
**And** the header shows "Copied!" flash

---

### Story EC2-052: Ctrl+R refreshes EC2 detail and re-checks all related
**Priority**: P2
**Depends on**: EC2-020

**Given** the user is in the EC2 detail view with all right-column checks completed
**When** the user presses `Ctrl+R`
**Then** the EC2 detail fields refresh from the data source
**And** all right-column rows reset to dim
**And** all background availability checks restart from scratch
**And** rows progressively light up as new results arrive

**AWS comparison:**
```
aws ec2 describe-instances --instance-ids i-0a1b2c3d4e5f60001
```
Plus all reverse-relationship checks re-executed.

---

### Story EC2-053: EC2 demo fixtures have fully populated fields
**Priority**: P1
**Depends on**: none

**Given** demo mode is active
**When** the user opens the EC2 detail view for `i-0a1b2c3d4e5f60001`
**Then** every field in the detail path list shows a real-looking value, not `--`:
- InstanceId: `i-0a1b2c3d4e5f60001`
- State: shows a state like `running`
- InstanceType: shows a type like `t3.large`
- ImageId: shows an AMI ID like `ami-0abc123def456789a`
- KeyName: shows a key name like `prod-keypair`
- Placement: section header with AvailabilityZone sub-field showing e.g., `us-east-1a`
- VpcId: shows `vpc-0abc123def456789a`
- SubnetId: shows `subnet-0aaa111111111111a`
- SecurityGroups: contains at least one GroupId
- PrivateIpAddress: shows an IP like `10.0.48.175`
- Architecture: shows e.g., `x86_64`
- LaunchTime: shows a timestamp

---

### Story EC2-054: Cross-reference IDs resolve to existing fixtures
**Priority**: P1
**Depends on**: EC2-053

**Given** the EC2 fixture has VpcId = `vpc-0abc123def456789a`
**When** the user navigates to that VPC via Enter
**Then** a VPC fixture with that ID exists and its detail renders with populated fields

**Given** the EC2 fixture has SubnetId = `subnet-0aaa111111111111a`
**When** the user navigates to that Subnet via Enter
**Then** a Subnet fixture with that ID exists and its detail renders

**Given** the EC2 fixture has SecurityGroups containing GroupId = `sg-0aaa111111111111a`
**When** the user navigates to that SG via Enter
**Then** an SG fixture with that ID exists and its detail renders

**Given** the EC2 fixture has ImageId = `ami-0abc123def456789a`
**When** the user navigates to that AMI via Enter
**Then** an AMI fixture with that ID exists and its detail renders

**Note**: This is the foundational precondition for all navigation stories in this document. Without valid cross-references, navigation stories produce "Resource not found" instead of the target detail.

---

### Story EC2-055: Help screen shows two-column key bindings
**Priority**: P2
**Depends on**: EC2-001

**Given** the user is in the EC2 two-column detail view
**When** the user presses `?`
**Then** the help screen appears with columns including DETAIL and RELATED bindings
**And** DETAIL column shows: Enter (Open link), Esc (Go back), h/l (Switch col), c (Copy value), y (YAML view), / (Search), n (Next match), N (Prev match)
**And** RELATED column shows: Tab (Switch col), r (Toggle col), / (Filter list)

**When** the user presses any key
**Then** the help screen closes and the EC2 detail view is restored

---

### Story EC2-056: YAML view hides right column
**Priority**: P2
**Depends on**: EC2-018

**Given** the user is in the EC2 two-column detail view
**When** the user presses `y`
**Then** the YAML view appears at full width
**And** the right column is not visible
**And** no separator line is drawn

**When** the user presses `y` again (or `Esc`)
**Then** the detail view returns with the right column state preserved (visible if it was visible before)

---

### Story EC2-057: Page up/down in left column
**Priority**: P2
**Depends on**: EC2-001

**Given** the EC2 detail view has many fields (40+ rows with sub-fields)
**And** the left column is focused with the cursor near the top
**When** the user presses `PgDn` or `Ctrl+D`
**Then** the cursor moves down by approximately one visible page

**When** the user presses `PgUp` or `Ctrl+U`
**Then** the cursor moves up by approximately one visible page

---

### Story EC2-058: CloudTrail Events opens pre-filtered search
**Priority**: P2
**Depends on**: EC2-021

**Given** the right column is focused and the cursor is on "CloudTrail Events"
**When** the user presses `Enter`
**Then** the CloudTrail search view opens pre-filtered for `i-0a1b2c3d4e5f60001`

**AWS comparison:**
```
aws cloudtrail lookup-events --lookup-attributes AttributeKey=ResourceName,AttributeValue=i-0a1b2c3d4e5f60001
```

---

### Story EC2-059: Session cache prevents re-checking on re-entry
**Priority**: P2
**Depends on**: EC2-020

**Given** the user viewed the EC2 detail for `i-0a1b2c3d4e5f60001` and all checks completed
**When** the user presses `Esc` to go back to the EC2 resource list, then re-enters the same instance's detail
**Then** right-column rows show their cached availability instantly (no dim flicker, no re-checking)

**When** the user presses `Ctrl+R`
**Then** the cache is cleared for this resource and all checks restart

---
