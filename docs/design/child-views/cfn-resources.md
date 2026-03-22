# Child View: CloudFormation Stacks --> Stack Resources

**Status:** Planned
**Tier:** MUST-HAVE

---

## Navigation

- **Entry:** Press `r` on a CloudFormation stack in the CFN Stacks list
- **Frame title:** `cfn-resources(23) — payment-service-prod`
- **View stack:** CFN Stacks --> Stack Resources --> (detail/YAML via d/y)
- **Esc** returns to CFN Stacks list
- **New key bindings:**
  - `r` on CFN Stacks list opens Stack Resources (the "what exists?" view)
  - `Enter` on CFN Stacks list opens Stack Events (the "what happened?" view — see cfn-events.md)

## views.yaml

```yaml
cfn_resources:
  list:
    Logical ID:
      path: LogicalResourceId
      width: 28
    Physical ID:
      path: PhysicalResourceId
      width: 28
    Type:
      path: ResourceType
      width: 28
    Status:
      path: ResourceStatus
      width: 24
    Drift:
      path: DriftInformation.StackResourceDriftStatus
      width: 12
    Updated:
      path: LastUpdatedTimestamp
      width: 22
  detail:
    - LogicalResourceId
    - PhysicalResourceId
    - ResourceType
    - ResourceStatus
    - ResourceStatusReason
    - DriftInformation
    - LastUpdatedTimestamp
    - ModuleInfo
    - Description
```

Source struct: `cftypes.StackResourceSummary`

## AWS API

- `cloudformation:ListStackResources` with `StackName`
- Paginated via `NextToken`
- **Latency:** Fast (<1 second). Most stacks have fewer than 200 resources.
- **Note:** For nested stacks, a resource with `ResourceType=AWS::CloudFormation::Stack` could theoretically be drilled into, but this is an edge case for future consideration.

## ASCII Wireframe

```
 a9s v0.5.0  prod:us-east-1                                              ? for help
┌────────────── cfn-resources(23) — payment-service-prod ────────────────────────┐
│ LOGICAL ID                   PHYSICAL ID                  TYPE                  … │
│ ApiLoadBalancer              arn:aws:elb:us-east-1:12…    AWS::ELB::LoadBalan…   │
│ ApiTargetGroup               arn:aws:elb:us-east-1:12…    AWS::ELB::TargetGro…   │
│ ApiListenerHTTPS             arn:aws:elb:us-east-1:12…    AWS::ELB::Listener     │
│ EcsCluster                   arn:aws:ecs:us-east-1:12…    AWS::ECS::Cluster      │
│ EcsService                   arn:aws:ecs:us-east-1:12…    AWS::ECS::Service      │
│ TaskDefinition               arn:aws:ecs:us-east-1:12…    AWS::ECS::TaskDefin…   │
│ LogGroup                     /ecs/payment-api              AWS::Logs::LogGroup    │
│   · · · (16 more)                                                                │
└──────────────────────────────────────────────────────────────────────────────────┘
```

Scrolled right to reveal Status and Drift columns:
```
│ TYPE                          STATUS                   DRIFT        UPDATED       │
│ AWS::ELB::LoadBalancer        CREATE_COMPLETE          NOT_CHECKED  2026-03-20…   │
│ AWS::ELB::TargetGroup         UPDATE_COMPLETE          NOT_CHECKED  2026-03-22…   │
│ AWS::ELB::Listener            CREATE_COMPLETE          IN_SYNC      2026-03-15…   │
│ AWS::ECS::Cluster             CREATE_COMPLETE          IN_SYNC      2026-02-10…   │
│ AWS::ECS::Service             UPDATE_COMPLETE          MODIFIED     2026-03-22…   │
│ AWS::ECS::TaskDefinition      CREATE_COMPLETE          NOT_CHECKED  2026-03-22…   │
│ AWS::Logs::LogGroup           CREATE_COMPLETE          IN_SYNC      2026-02-10…   │
```

Row coloring by resource status (entire row):
- `*_COMPLETE` (not delete/rollback): GREEN `#9ece6a`
- `*_IN_PROGRESS`: YELLOW `#e0af68`
- `*_FAILED`: RED `#f7768e`
- `DELETE_COMPLETE`: DIM `#565f89`

Additionally, drift status modifies coloring:
- `MODIFIED` (drifted): YELLOW `#e0af68` (overrides green if status is COMPLETE)

Selected row: full-width blue background overrides all status coloring.

## Copy Behavior

`c` copies the PhysicalResourceId — the actual AWS resource ID you can paste into the Console search bar or another a9s command.

## Help Screen

```
┌──────────────────────────────── Help ───────────────────────────────────────────┐
│ STACK RESOURCES       GENERAL              NAVIGATION           HOTKEYS         │
│                                                                                 │
│ <esc>   Back          <ctrl-r> Refresh     <j>       Down       <?>   Help      │
│ <d>     Detail        </>      Filter      <k>       Up         <:>   Command   │
│ <y>     YAML          <:>      Command     <g>       Top                        │
│ <c>     Copy Phys ID                       <G>       Bottom                     │
│                                            <h/l>     Cols                       │
│                                            <pgup/dn> Page                       │
│                                                                                 │
│                       Press any key to close                                    │
└─────────────────────────────────────────────────────────────────────────────────┘
```
