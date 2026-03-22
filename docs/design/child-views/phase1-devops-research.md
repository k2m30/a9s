# Phase 1: DevOps Research — Child View Candidates

**Agent:** a9s-devops
**Constraint:** Do NOT read any *.go files. Only read files under `docs/design/` and `docs/design/child-views/`. Use external knowledge and web search for AWS API research.

## Context

a9s is a read-only AWS TUI tool (like k9s for Kubernetes). It currently supports 62 resource types. Two already have custom "child views" — drill-down screens accessed via Enter on a parent resource that show related sub-resources:

| Parent Resource | Child View | What it shows | Design doc |
|---|---|---|---|
| S3 Buckets | S3 Objects | Objects/prefixes inside a bucket (list-objects-v2) | `docs/design/child-views/s3-objects.md` |
| Route 53 Hosted Zones | R53 Records | DNS records in the zone (list-resource-record-sets) | `docs/design/child-views/r53-records.md` |

Read both design docs to understand what a child view looks like in practice.

## Your Task

For each of the 62 implemented resource types listed below, evaluate whether a custom child view would deliver meaningful value to a DevOps/SRE/Platform engineer during **real daily operational work**.

### The 62 implemented resource types

COMPUTE: ec2, lambda, asg, eb (Elastic Beanstalk), ecs (clusters), ecs-svc, ecs-task
CONTAINERS: eks, ng (node groups)
NETWORKING: elb (load balancers), tg (target groups), sg (security groups), vpc, subnet, rtb (route tables), nat, igw, vpce (VPC endpoints), tgw (transit gateways), eni, eip
DATABASES & STORAGE: s3 *(has child)*, dbi (RDS instances), redis, dbc (DocumentDB), ddb (DynamoDB), opensearch, redshift, efs, rds-snap, docdb-snap
MONITORING: alarm (CloudWatch alarms), logs (CW Log Groups), trail (CloudTrail)
MESSAGING: sqs, sns, sns-sub, eb-rule (EventBridge rules), kinesis, msk, sfn (Step Functions)
SECRETS & CONFIG: secrets (Secrets Manager), ssm (SSM Parameters), kms
DNS & CDN: r53 *(has child)*, cf (CloudFront), acm, apigw (API Gateway)
SECURITY & IAM: role (IAM Roles), policy (IAM Policies), iam-user, iam-group, waf
CI/CD: cfn (CloudFormation stacks), pipeline (CodePipeline), cb (CodeBuild), ecr, codeartifact
DATA & ANALYTICS: glue, athena
BACKUP: backup, ses

### Evaluation criteria

Think through real DevOps scenarios — not theoretical ones. Ask yourself:

1. **Frequency**: How often does a DevOps engineer need to drill into this resource? Daily? Weekly? Rarely?
2. **Pain point**: Does the AWS Console make this drill-down annoying (too many clicks, slow, context-switching)? Would having it in a TUI save real time?
3. **Actionable data**: Does the child data help make immediate decisions (e.g., "this Lambda is failing" → check recent invocations → see error logs)?
4. **Read-only API availability**: Is there a read-only AWS API call that returns the child data? (No write/mutate operations allowed.)
5. **Data volume**: Is the child data small enough to display meaningfully in a terminal? (Thousands of log lines need filtering; a list of 5 ECS tasks is perfect.)

### Scenarios to consider

For each candidate, ground your recommendation in at least one concrete scenario:
- Incident response / debugging production issues
- Daily health checks / morning standup prep
- Security audits / compliance reviews
- Cost investigation
- Deployment verification ("did my deploy land?")
- Capacity planning

### Output format

Categorize every resource into exactly one tier:

#### MUST-HAVE
Resources where a child view is essential — engineers would use it daily/weekly, and the AWS Console makes this painful. For each, provide:
- **Resource**: name and short_name
- **Child view name**: what the drill-down screen shows
- **Scenario(s)**: 1-3 concrete DevOps scenarios (not abstract)
- **AWS API(s)**: exact read-only API call(s) needed
- **Columns/fields**: what data to show in the child table
- **Nesting potential**: can this child itself have children? If yes, describe briefly

#### SHOULD-HAVE
Resources where a child view adds clear value but is not critical. Same format as MUST-HAVE.

#### OPTIONAL
Resources where a child view is nice but rarely needed. Brief justification only — one line per resource.

#### NOT NEEDED
Resources where the existing detail/YAML view is sufficient. Brief justification — one line per resource. This should be the largest category.

### Important guidelines

- **Be ruthless.** Most resources do NOT need child views. The detail/YAML view already shows the full AWS response. Only recommend a child view when drill-down reveals a *different entity type* (like S3 bucket → objects, not just "more fields").
- **Nested children are fine.** If Lambda → Invocations → Log Events makes sense as 3 levels, say so. Each screen will be implemented as a separate GitHub issue.
- **Think like a DevOps engineer at 2 AM during an incident.** What drill-downs would save you from opening the AWS Console?
- **Use web search** to verify AWS API calls exist and are read-only. Don't guess API names.

## Reference Data

`.a9s/views_reference.yaml` lists every available AWS SDK struct field path for each resource type. These are the fields that already exist in the parent resource list/detail views. Use this to understand what data is already visible without a child view — a child view is only justified when it reveals a **different entity type** (e.g., Lambda → invocations) not just more fields of the same struct.

## Deliverable

Write your analysis to `docs/design/child-views/devops-research.md`. This will be reviewed by the user before Phase 2 begins.
