# Child View: CodePipeline --> Pipeline Stage State

**Status:** Planned
**Tier:** SHOULD-HAVE

---

## Navigation

- **Entry:** Press Enter on a pipeline in the CodePipeline list
- **Frame title:** `pipeline-stages(4) — payment-service-deploy`
- **View stack:** CodePipeline --> Pipeline Stages --> (detail/YAML via d/y)
- **Esc** returns to CodePipeline list
- **No new key bindings** beyond the standard set

## views.yaml

```yaml
pipeline_stages:
  list:
    Stage:
      key: stage_name
      width: 20
    Stage Status:
      key: stage_status
      width: 14
    Action:
      key: action_name
      width: 24
    Action Status:
      key: action_status
      width: 14
    Last Changed:
      key: last_change_time
      width: 22
    External URL:
      key: external_url
      width: 40
  detail:
    - stage_name
    - stage_status
    - action_name
    - action_status
    - last_change_time
    - external_url
    - action_token
    - action_error_details
    - revision_id
    - revision_summary
```

Note: This view is unique because `GetPipelineState` returns a hierarchical structure (stages containing actions) that must be flattened into rows. Each row represents a **stage-action pair**. Stages with multiple actions produce multiple rows. The Stage column value is shown only on the first row for each stage; subsequent action rows for the same stage leave it blank (visual grouping).

Computed fields:
- `stage_name`: from `StageStates[].StageName`
- `stage_status`: from `StageStates[].LatestExecution.Status` (InProgress, Succeeded, Failed, Stopped)
- `action_name`: from `StageStates[].ActionStates[].ActionName`
- `action_status`: from `StageStates[].ActionStates[].LatestExecution.Status`
- `last_change_time`: from `StageStates[].ActionStates[].LatestExecution.LastStatusChange`
- `external_url`: from `StageStates[].ActionStates[].LatestExecution.ExternalExecutionUrl` (link to CodeBuild, CodeDeploy, etc.)

Source: `codepipeline:GetPipelineState` response (not a single SDK struct — it is a nested response)

## AWS API

- `codepipeline:GetPipelineState` with `name` (pipeline name)
- **Not paginated** — returns the full current state of all stages and actions in one call
- **Latency:** Fast (<1 second). Single call, immediate response.

## ASCII Wireframe

```
 a9s v0.5.0  prod:us-east-1                                              ? for help
┌────────── pipeline-stages(4) — payment-service-deploy ─────────────────────────┐
│ STAGE                STAGE STATUS   ACTION                   ACTION STATUS   L…  │
│ Source               Succeeded      SourceAction             Succeeded       2…  │
│ Build                Succeeded      BuildAction              Succeeded       2…  │
│ Staging              Succeeded      DeployToStaging          Succeeded       2…  │
│                                     IntegrationTests         Succeeded       2…  │
│ Production           InProgress     ApprovalGate             InProgress      —   │
│                                     DeployToProduction       —               —   │
└─────────────────────────────────────────────────────────────────────────────────┘
```

Real-world deployment stuck at approval:
```
│ STAGE                STAGE STATUS   ACTION                   ACTION STATUS       │
│ Source               Succeeded      GitHub                   Succeeded           │
│ Build                Succeeded      CodeBuild                Succeeded           │
│ DeployStaging        Succeeded      ECS-Deploy-Staging       Succeeded           │
│                                     RunE2ETests              Succeeded           │
│ Approval             InProgress     ManualApproval           InProgress          │
│ DeployProd           —              ECS-Deploy-Prod          —                   │
```

Failed pipeline:
```
│ STAGE                STAGE STATUS   ACTION                   ACTION STATUS       │
│ Source               Succeeded      GitHub                   Succeeded           │
│ Build                Failed         CodeBuild                Failed              │
│ DeployStaging        —              ECS-Deploy-Staging       —                   │
│ DeployProd           —              ECS-Deploy-Prod          —                   │
```

Row coloring by action status (entire row):
- `Succeeded`: GREEN `#9ece6a`
- `Failed`: RED `#f7768e`
- `InProgress`: YELLOW `#e0af68`
- `Stopped` / `Abandoned`: DIM `#565f89`
- No status (not yet reached): DIM `#565f89`

Selected row: full-width blue background overrides status coloring.

## Copy Behavior

`c` copies the external execution URL if present (the deep link to the CodeBuild build, CodeDeploy deployment, etc.), otherwise the action name. The external URL is what you paste into a browser to get more detail.

## Help Screen

```
┌──────────────────────────────── Help ───────────────────────────────────────────┐
│ PIPELINE STAGES       GENERAL              NAVIGATION           HOTKEYS         │
│                                                                                 │
│ <esc>   Back          <ctrl-r> Refresh     <j>       Down       <?>   Help      │
│ <d>     Detail        </>      Filter      <k>       Up         <:>   Command   │
│ <y>     YAML          <:>      Command     <g>       Top                        │
│ <c>     Copy URL                           <G>       Bottom                     │
│                                            <h/l>     Cols                       │
│                                            <pgup/dn> Page                       │
│                                                                                 │
│                       Press any key to close                                    │
└─────────────────────────────────────────────────────────────────────────────────┘
```
