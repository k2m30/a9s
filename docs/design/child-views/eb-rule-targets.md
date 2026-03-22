# Child View: EventBridge Rules --> Targets

**Status:** Planned
**Tier:** SHOULD-HAVE

---

## Navigation

- **Entry:** Press Enter on an EventBridge rule in the EventBridge Rules list
- **Frame title:** `eb-targets(2) — daily-etl-trigger`
- **View stack:** EventBridge Rules --> Targets --> (detail/YAML via d/y)
- **Esc** returns to EventBridge Rules list
- **No new key bindings** beyond the standard set

## views.yaml

```yaml
eb_rule_targets:
  list:
    Target ID:
      path: Id
      width: 20
    Target ARN:
      path: Arn
      width: 48
    Resource:
      key: resource_type_name
      width: 28
    Input:
      key: input_summary
      width: 36
  detail:
    - Id
    - Arn
    - RoleArn
    - Input
    - InputPath
    - InputTransformer
    - DeadLetterConfig
    - RetryPolicy
    - SqsParameters
    - EcsParameters
    - KinesisParameters
    - BatchParameters
    - HttpParameters
    - SageMakerPipelineParameters
    - RedshiftDataParameters
    - AppSyncParameters
```

Note on computed fields:
- `resource_type_name`: extracted from the ARN (e.g., "Lambda: data-pipeline-daily" from `arn:aws:lambda:...:function:data-pipeline-daily`, "SQS: processing-queue" from `arn:aws:sqs:...:processing-queue`, "SFN: order-workflow" from `arn:aws:states:...:stateMachine:order-workflow`)
- `input_summary`: shows `Input` (constant JSON, truncated), or `InputPath` (JSONPath), or "InputTransformer" if `InputTransformer` is set, or "— " if the event is passed through unmodified

Source struct: `ebtypes.Target` (from `eventbridge` or `cloudwatchevents` package)

## AWS API

- `events:ListTargetsByRule` with `Rule` name and `EventBusName`
- **No pagination** — AWS supports up to 5 targets per rule (soft limit, can be increased to 100). Returns all targets in one call.
- **Latency:** Fast (<1 second)

## ASCII Wireframe

```
 a9s v0.5.0  prod:us-east-1                                              ? for help
┌──────────────── eb-targets(2) — daily-etl-trigger ─────────────────────────────┐
│ TARGET ID            TARGET ARN                                       RESOURC…  │
│ etl-lambda           arn:aws:lambda:us-east-1:123456:function:data-…  Lambda:…  │
│ dlq-notifications    arn:aws:sqs:us-east-1:123456:etl-failure-dlq     SQS: et…  │
└─────────────────────────────────────────────────────────────────────────────────┘
```

Another example — rule with ECS target:
```
 a9s v0.5.0  prod:us-east-1                                              ? for help
┌──────────── eb-targets(1) — scheduled-report-generator ────────────────────────┐
│ TARGET ID            TARGET ARN                                       RESOURC…  │
│ ecs-task             arn:aws:ecs:us-east-1:123456:cluster/prod        ECS: pr…  │
└─────────────────────────────────────────────────────────────────────────────────┘
```

Scrolled right to show Input column:
```
│ RESOURCE                       INPUT                                             │
│ Lambda: data-pipeline-daily    {"mode": "full", "date": "<aws.scheduler.exec…   │
│ SQS: etl-failure-dlq           —                                                │
```

No status-based row coloring — targets have no health/status. All rows PLAIN `#c0caf5`.

Selected row: full-width blue background.

## Copy Behavior

`c` copies the Target ARN — the resource identifier for the target. This is what you need to cross-reference ("is this Lambda still deployed? Is this SQS queue still receiving?").

## Help Screen

```
┌──────────────────────────────── Help ───────────────────────────────────────────┐
│ EB TARGETS            GENERAL              NAVIGATION           HOTKEYS         │
│                                                                                 │
│ <esc>   Back          <ctrl-r> Refresh     <j>       Down       <?>   Help      │
│ <d>     Detail        </>      Filter      <k>       Up         <:>   Command   │
│ <y>     YAML          <:>      Command     <g>       Top                        │
│ <c>     Copy ARN                           <G>       Bottom                     │
│                                            <h/l>     Cols                       │
│                                            <pgup/dn> Page                       │
│                                                                                 │
│                       Press any key to close                                    │
└─────────────────────────────────────────────────────────────────────────────────┘
```
