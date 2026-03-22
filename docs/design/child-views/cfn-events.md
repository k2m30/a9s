# Child View: CloudFormation Stacks --> Stack Events

**Status:** Planned
**Tier:** MUST-HAVE

---

## Navigation

- **Entry:** Press Enter on a CloudFormation stack in the CFN Stacks list
- **Frame title:** `cfn-events(156) — payment-service-prod`
- **View stack:** CFN Stacks --> Stack Events --> (detail/YAML via d/y)
- **Esc** returns to CFN Stacks list
- **New key bindings on CFN Stacks list:**
  - `Enter` opens Stack Events (the "what happened?" timeline — primary child)
  - `r` opens Stack Resources (the "what exists?" inventory — see cfn-resources.md)

## views.yaml

```yaml
cfn_events:
  list:
    Timestamp:
      path: Timestamp
      width: 22
    Logical ID:
      path: LogicalResourceId
      width: 28
    Type:
      path: ResourceType
      width: 28
    Status:
      path: ResourceStatus
      width: 24
    Reason:
      path: ResourceStatusReason
      width: 40
  detail:
    - Timestamp
    - LogicalResourceId
    - PhysicalResourceId
    - ResourceType
    - ResourceStatus
    - ResourceStatusReason
    - ClientRequestToken
    - HookType
    - HookStatus
    - HookStatusReason
    - HookInvocationPoint
    - HookFailureMode
    - EventId
```

Source struct: `cftypes.StackEvent`

## AWS API

- `cloudformation:DescribeStackEvents` with `StackName`
- Paginated via `NextToken` — events returned in reverse chronological order
- **Latency:** Fast for recent stacks (<1 second). Old stacks with thousands of events may have multiple pages.
- **Volume:** Long-lived stacks can have thousands of events. Default to showing most recent 200 (first page or two). Scroll down to load more.

## ASCII Wireframe

```
 a9s v0.5.0  prod:us-east-1                                              ? for help
┌────────────── cfn-events(156) — payment-service-prod ──────────────────────────┐
│ TIMESTAMP              LOGICAL ID                   TYPE                        … │
│ 2026-03-22 02:55       payment-service-prod         AWS::CloudFormation::Stack   │
│ 2026-03-22 02:54       ApiTargetGroup               AWS::ELB::TargetGroup        │
│ 2026-03-22 02:53       ApiListenerRule               AWS::ELB::ListenerRule       │
│ 2026-03-22 02:52       TaskDefinition               AWS::ECS::TaskDefinition     │
│ 2026-03-22 02:51       ApiSecurityGroup             AWS::EC2::SecurityGroup       │
│ 2026-03-22 02:50       LogGroup                     AWS::Logs::LogGroup          │
│ 2026-03-22 02:49       payment-service-prod         AWS::CloudFormation::Stack   │
│   · · · (149 more)                                                               │
└──────────────────────────────────────────────────────────────────────────────────┘
```

Scrolled right to reveal Status and Reason columns (via `l` key):
```
│ TYPE                          STATUS                   REASON                    │
│ AWS::CloudFormation::Stack    UPDATE_COMPLETE          —                          │
│ AWS::ELB::TargetGroup         UPDATE_COMPLETE          —                          │
│ AWS::ELB::ListenerRule        UPDATE_COMPLETE          —                          │
│ AWS::ECS::TaskDefinition     CREATE_COMPLETE          —                          │
│ AWS::EC2::SecurityGroup      UPDATE_COMPLETE          —                          │
│ AWS::Logs::LogGroup          CREATE_COMPLETE          —                          │
│ AWS::CloudFormation::Stack    UPDATE_IN_PROGRESS       User Initiated             │
```

Failed deployment example (the money view):
```
│ TIMESTAMP              LOGICAL ID                   TYPE                        … │
│ 2026-03-22 02:55       payment-service-prod         AWS::CloudFormation::Stack   │
│ 2026-03-22 02:54       ApiSecurityGroup             AWS::EC2::SecurityGroup       │
│ 2026-03-22 02:53       TaskDefinition               AWS::ECS::TaskDefinition     │
│ 2026-03-22 02:52       DatabaseSubnetGroup          AWS::RDS::DBSubnetGroup      │
│ 2026-03-22 02:51       payment-service-prod         AWS::CloudFormation::Stack   │
```

```
│ STATUS                   REASON                                                   │
│ ROLLBACK_COMPLETE        The following resource(s) failed to create: [Database…   │
│ CREATE_COMPLETE          —                                                        │
│ CREATE_COMPLETE          —                                                        │
│ CREATE_FAILED            The security group 'sg-0abc123' does not exist in VPC…   │
│ CREATE_IN_PROGRESS       User Initiated                                           │
```

Row coloring by resource status (entire row):
- `*_COMPLETE` (not rollback): GREEN `#9ece6a`
- `*_IN_PROGRESS`: YELLOW `#e0af68`
- `*_FAILED`: RED `#f7768e`
- `ROLLBACK_*`: RED `#f7768e`
- `DELETE_COMPLETE`: DIM `#565f89`
- `DELETE_IN_PROGRESS`: YELLOW `#e0af68`

Selected row: full-width blue background overrides status coloring.

## Copy Behavior

`c` copies the ResourceStatusReason (the error message) if present, otherwise the LogicalResourceId. During incident debugging, what you want to paste into Slack is the error reason.

## Help Screen

```
┌──────────────────────────────── Help ───────────────────────────────────────────┐
│ STACK EVENTS          GENERAL              NAVIGATION           HOTKEYS         │
│                                                                                 │
│ <esc>   Back          <ctrl-r> Refresh     <j>       Down       <?>   Help      │
│ <d>     Detail        </>      Filter      <k>       Up         <:>   Command   │
│ <y>     YAML          <:>      Command     <g>       Top                        │
│ <c>     Copy Reason                        <G>       Bottom                     │
│                                            <h/l>     Cols                       │
│                                            <pgup/dn> Page                       │
│                                                                                 │
│                       Press any key to close                                    │
└─────────────────────────────────────────────────────────────────────────────────┘
```
