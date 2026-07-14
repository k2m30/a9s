# QA User Stories: Resource Price Column in List Views (Issue #73)

Regenerated 2026-07-14 against the refreshed issue #73 scope. This fully replaces the earlier draft, whose scope (a `Cost/mo` column limited to a handful of compute types, several types deliberately showing no column, and "EBS pending #66") is obsolete.

Scope: a single **price column** in resource list views. Every cell is a pure lookup from static, embedded, region-aware rate tables generated at dev time by `cmd/pricegen`. No runtime API calls are ever made — the column works offline and in `--demo`. This is unrelated to the `:costs` Cost Explorer screen, which shows billed actuals; this column shows deterministic list-price estimates for every row.

Three cell kinds:

1. **Computed estimate** — carries a `~` prefix (`~$70/mo`). Rate × instance/node type, rate × provisioned size, or a flat fee.
2. **Bare unit rate** — no `~`, no calculation (`$0.023/GB`, `$0.20/1M req`) for usage-priced types.
3. **`—`** — only when no rate applies (unknown/newer instance type, unpriced region, DynamoDB on-demand table). Never a wrong number.

All estimates reflect the currently selected region. On-demand list price only — reserved instances, savings plans, and free tier are out of scope. All stories treat a9s as a black box.

---

## A. EC2 Instance Prices (computed — rate × instance type)

### A.1 Computed Estimate with `~` Prefix

| # | Story | Expected |
|---|-------|----------|
| A.1.1 | I open the EC2 instance list. | A price column is visible alongside the existing columns (Name, State, Lifecycle, Type, Private IP, Public IP, Instance ID, Launch Time). |
| A.1.2 | I observe the price column header. | The header is a concise label (e.g. "Price") styled bold blue (`#7aa2f7`) like all other column headers; individual cells carry their own unit (`/mo`), so the header is unit-neutral. |
| A.1.3 | I observe an on-demand instance. | The cell shows a computed monthly estimate with a leading `~` (e.g. `~$30/mo`, `~$138/mo`) derived from instance type × region rate × 730 hours. |
| A.1.4 | I compare a `t3.medium` against an `m5.xlarge`. | The `t3.medium` shows a lower `~$/mo` estimate than the `m5.xlarge`; relative ordering matches public on-demand pricing. |
| A.1.5 | I observe a spot instance (Lifecycle = "spot"). | The cell reflects the spot rate, distinguished from on-demand pricing for the same type — the lifecycle drives which rate is looked up. |
| A.1.6 | I observe an instance whose type is newer than the binary / absent from the rate table. | The cell shows `—`, never `~$0/mo` and never a wrong number. |
| A.1.7 | I observe a stopped instance. | The cell shows the instance-type reference rate or `—` per the display rules — it never fabricates a fee the stopped instance isn't accruing. |

**AWS comparison:**

```
aws ec2 describe-instances --query 'Reservations[].Instances[].{ID:InstanceId,Type:InstanceType,State:State.Name,Lifecycle:InstanceLifecycle}'
aws pricing get-products --service-code AmazonEC2 --filters Type=TERM_MATCH,Field=instanceType,Value=t3.medium Type=TERM_MATCH,Field=location,Value="US East (N. Virginia)"
```

Expected fields visible: Name, State, Lifecycle, Type, Private IP, Public IP, Instance ID, Launch Time, Price (per views.yaml ec2 list + new price column).

---

## B. Computed by Instance / Node Type

Types priced as `rate(type, region) × count × 730h`. Each shows a `~$/mo` estimate.

### B.1 Per-Type Estimates

| # | Story | Expected |
|---|-------|----------|
| B.1.1 | I open the RDS (DB Instances) list. | The price column shows a `~$/mo` estimate per instance derived from DB instance class × multi-AZ × engine (alongside DB Identifier, Engine, Version, Status, Class, Endpoint, Multi-AZ). |
| B.1.2 | I compare a Multi-AZ RDS instance to a single-AZ instance of the same class. | The Multi-AZ instance shows roughly double the `~$/mo` estimate. |
| B.1.3 | I open the DocumentDB / Aurora members list. | Each member shows a `~$/mo` estimate derived from its instance class. |
| B.1.4 | I open the ElastiCache list and view a 3-node cluster vs a 1-node cluster of the same node type. | The 3-node cluster shows roughly 3× the `~$/mo` estimate (node type × node count). |
| B.1.5 | I open the Redshift list and compare a 2-node cluster to a 4-node cluster of the same node type. | The 4-node cluster shows roughly double the `~$/mo` estimate. |
| B.1.6 | I open the OpenSearch list and compare a 3-instance domain to a 1-instance domain of the same instance type. | The 3-instance domain shows roughly 3× the `~$/mo` estimate (instance type × count; EBS storage folded in where present). |
| B.1.7 | I open the MSK list. | Each cluster shows a `~$/mo` estimate derived from broker type × broker count (storage GB folded in). |
| B.1.8 | I open the ECS services list (Fargate). | Each service shows a `~$/mo` estimate from vCPU + GB rates × task size × desired count. |

**AWS comparison:**

```
aws rds describe-db-instances --query 'DBInstances[].{ID:DBInstanceIdentifier,Class:DBInstanceClass,Engine:Engine,MultiAZ:MultiAZ}'
aws elasticache describe-cache-clusters --query 'CacheClusters[].{ID:CacheClusterId,Type:CacheNodeType,Nodes:NumCacheNodes}'
aws redshift describe-clusters --query 'Clusters[].{ID:ClusterIdentifier,NodeType:NodeType,Nodes:NumberOfNodes}'
aws opensearch describe-domain --domain-name my-domain --query 'DomainStatus.ClusterConfig.{Type:InstanceType,Count:InstanceCount}'
```

Expected fields visible per type (per views.yaml) plus the new Price column.

---

## C. Computed by Provisioned Size

Types priced as `rate × provisioned quantity`. Each shows a `~$/mo` estimate — except unpriceable rows.

### C.1 Storage-Sized Estimates

| # | Story | Expected |
|---|-------|----------|
| C.1.1 | I open the EBS volumes list. | Each volume shows a `~$/mo` estimate from volume type × size; gp3/io1/io2 volumes fold in provisioned IOPS/throughput. EBS is a first-class resource type. |
| C.1.2 | I compare a small gp3 volume to a large gp3 volume. | The larger volume shows a proportionally higher `~$/mo` estimate. |
| C.1.3 | I open the EFS file systems list. | Each file system shows a `~$/mo` estimate from storage bytes × storage class. |
| C.1.4 | I open the ECR repositories list. | Each repository shows a `~$/mo` estimate from stored image bytes × `$/GB-mo`. |
| C.1.5 | I open the CloudWatch Logs groups list. | Each log group shows a `~$/mo` estimate for its storage share only (`storedBytes × $/GB-mo`); ingestion is usage and excluded. |
| C.1.6 | I open the Backup vaults list. | Each vault shows a `~$/mo` estimate from stored bytes × vault storage rate. |
| C.1.7 | I open the DynamoDB tables list and view a provisioned-mode table. | The table shows a `~$/mo` estimate derived from its RCU/WCU rates. |
| C.1.8 | I view a DynamoDB on-demand (pay-per-request) table. | The cell shows `—` — cost depends on table class and traffic, so no honest estimate exists. |

**AWS comparison:**

```
aws ec2 describe-volumes --query 'Volumes[].{ID:VolumeId,Type:VolumeType,Size:Size,Iops:Iops,Throughput:Throughput}'
aws efs describe-file-systems --query 'FileSystems[].{ID:FileSystemId,Size:SizeInBytes.Value}'
aws dynamodb describe-table --table-name my-table --query 'Table.{Billing:BillingModeSummary.BillingMode,RCU:ProvisionedThroughput.ReadCapacityUnits}'
```

Expected fields visible per type (per views.yaml) plus the new Price column.

---

## D. Flat-Fee Types

Types with a fixed `$/hr` base or flat monthly charge. Each shows a `~$/mo` estimate; usage-dependent add-ons (data processing, LCU) are excluded.

### D.1 Flat-Fee Estimates

| # | Story | Expected |
|---|-------|----------|
| D.1.1 | I open the NAT Gateways list. | Each active gateway shows a `~$/mo` estimate (hourly base × 730h); data-processing charges are excluded. |
| D.1.2 | I open the Load Balancers list and view an ALB. | The ALB shows a `~$/mo` base estimate; LCU charges are excluded. |
| D.1.3 | I view an NLB in the same list. | The NLB shows a `~$/mo` base estimate that may differ from the ALB base, reflecting its load-balancer type. |
| D.1.4 | I open the Elastic IPs list. | An idle (unassociated) EIP shows `~$3.65/mo`; an EIP attached to a running instance shows the appropriate free/`—` treatment, making idle EIPs visible as cost sinks. |
| D.1.5 | I open the EKS clusters list. | Each cluster shows `~$73/mo` for its control plane. |
| D.1.6 | I open the KMS keys list. | Each customer-managed key shows `~$1/mo`; AWS-managed keys with no charge render `—`. |
| D.1.7 | I open the Secrets Manager secrets list. | Each secret shows `~$0.40/mo`. |
| D.1.8 | I open the Route53 hosted zones list. | Each hosted zone shows `~$0.50/mo`. |
| D.1.9 | I open the CloudWatch alarms list. | A standard alarm shows `~$0.10/mo`; a high-resolution alarm shows `~$0.30/mo`. |
| D.1.10 | I open the VPC interface endpoints list. | Each endpoint shows a `~$/mo` estimate of hourly rate × AZ count. |
| D.1.11 | I open the Transit Gateway attachments list. | Each attachment shows a `~$/mo` estimate from its hourly rate. |
| D.1.12 | I open the WAF web ACLs list. | Each web ACL shows a `~$/mo` estimate of `$5/mo + $1/mo per rule`. |

**AWS comparison:**

```
aws ec2 describe-nat-gateways --query 'NatGateways[].{ID:NatGatewayId,State:State}'
aws ec2 describe-addresses --query 'Addresses[].{PublicIp:PublicIp,AssociationId:AssociationId}'
aws eks list-clusters
aws wafv2 list-web-acls --scope REGIONAL
```

Expected fields visible per type (per views.yaml) plus the new Price column.

---

## E. Usage-Priced Types — Bare Unit Rates (no `~`, no calculation)

These types don't get a computed monthly figure. They show a single dominant unit rate from the same rate table, verbatim, with no `~` prefix.

### E.1 Unit-Rate Cells

| # | Story | Expected |
|---|-------|----------|
| E.1.1 | I open the S3 Buckets list. | Each bucket shows `$0.023/GB` (Standard headline rate) with no `~` prefix — the class mix isn't knowable without extra API calls, so the bucket cell shows the headline rate only. |
| E.1.2 | I drill into a bucket and view the objects child list. | Each object shows its own class-accurate `$/GB` rate from its `StorageClass` — Standard, IA, Glacier, and Deep Archive objects each show their true rate. |
| E.1.3 | I open the Lambda Functions list. | Each function shows `$0.20/1M req`. |
| E.1.4 | I open the SQS Queues list. | Each queue shows `$0.40/1M req`. |
| E.1.5 | I open the SNS Topics list. | Each topic shows `$0.50/1M pub`. |
| E.1.6 | I open the API Gateways list. | Each HTTP API shows `$1.00/1M req`. |
| E.1.7 | I open the CloudFront Distributions list. | Each distribution shows `$0.085/GB`. |
| E.1.8 | I open the Athena list. | Each entry shows `$5.00/TB scanned`. |
| E.1.9 | I open the Glue list. | Each entry shows `$0.44/DPU-hr`. |
| E.1.10 | I open the CodeBuild projects list. | Each project shows a `$/build-min` rate that reflects its compute type. |
| E.1.11 | I open the Step Functions list. | Each Standard state machine shows `$25/1M transitions`. |
| E.1.12 | I open the EventBridge list. | Each entry shows `$1.00/1M events`. |
| E.1.13 | I open the SES list. | Each entry shows `$0.10/1k emails`. |
| E.1.14 | I compare a unit-rate cell to a computed cell. | Unit-rate cells carry NO `~` prefix, distinguishing "a rate you multiply by usage" from a `~$/mo` estimate. |

**AWS comparison:**

```
aws s3api list-buckets
aws s3api list-objects-v2 --bucket my-bucket --query 'Contents[].{Key:Key,Class:StorageClass,Size:Size}'
aws lambda list-functions --query 'Functions[].FunctionName'
```

Expected fields visible per type (per views.yaml) plus the new Price column showing a bare unit rate.

---

## F. In-Flight Resource Types

Four types being added; the price column covers them as they land.

### F.1 New-Type Price Cells

| # | Story | Expected |
|---|-------|----------|
| F.1.1 | I open the `mwaa` (Managed Airflow) environments list. | Each environment shows a computed `~$/mo` estimate from its environment class base rate (mw1.small/medium/large × `$/hr`); autoscaled workers/schedulers are usage and excluded. |
| F.1.2 | I open `mwaa` against a live account whose role denies `mwaa:ListEnvironments`. | a9s degrades honestly (error/empty as designed) and never fabricates a zero price or a fake row. |
| F.1.3 | I open the `transfer` (Transfer Family) servers list. | Each server shows a computed flat `~$/mo` estimate of `$/hr × enabled protocol count` (~$216/mo per protocol). |
| F.1.4 | I open the `lt` (Launch Templates) list. | Each template shows the defined `InstanceType`'s on-demand rate as a reference price — the template itself is free; the cell is a reference, not a bill. |
| F.1.5 | I open the `vpc-peer` (VPC peering connections) list. | Each connection shows the `$0.01/GB` cross-AZ unit rate (no `~`) — there is no hourly charge. |

**AWS comparison:**

```
aws mwaa list-environments
aws transfer list-servers --query 'Servers[].{Id:ServerId,Protocols:Protocols}'
aws ec2 describe-launch-templates --query 'LaunchTemplates[].{Id:LaunchTemplateId,Name:LaunchTemplateName}'
aws ec2 describe-vpc-peering-connections --query 'VpcPeeringConnections[].VpcPeeringConnectionId'
```

Expected fields visible per type (per views.yaml) plus the new Price column.

---

## G. Price Column Visual Integration

### G.1 Alignment, Prefix, and Styling

| # | Story | Expected |
|---|-------|----------|
| G.1.1 | I observe the price column values. | All values are right-aligned within the column for easy vertical scanning of amounts. |
| G.1.2 | I scan a list mixing computed and unit-rate rows. | Computed cells carry a `~` prefix; unit-rate cells do not; `—` marks unpriceable rows. The three kinds are visually distinguishable at a glance. |
| G.1.3 | I move the selection onto a priced row. | The price cell uses the standard selection styling (blue background, dark foreground, bold) like every other cell. |
| G.1.4 | I observe the price cell on a status-colored row (e.g. a running EC2 instance in green). | The price cell renders in the same row color as the rest of the row, preserving the status-colored-row design. |
| G.1.5 | I scroll columns horizontally with `h`/`l` on a narrow terminal. | The price column is reachable via horizontal scroll if it overflows, scrolling in sync with headers and data. |
| G.1.6 | I observe the price column across several resource types. | The column occupies a consistent position and uses a consistent header label everywhere it appears. |

---

## H. Sorting Semantics

### H.1 Sorting the Price Column

| # | Story | Expected |
|---|-------|----------|
| H.1.1 | I sort the EC2 list by the price column. | Rows reorder by computed value numerically — most/least expensive at one end, matching the `~$/mo` magnitudes. |
| H.1.2 | I sort a list that mixes computed and unit-rate cells. | Unit-rate cells sort as a group after (or before) the computed `~$/mo` values — the units aren't comparable, so the two kinds never interleave by raw number. |
| H.1.3 | I sort a list containing `—` cells. | Unpriceable (`—`) rows collect at the end of the sort order rather than sorting as `$0`. |
| H.1.4 | I sort by price, then switch the sort key to name. | The sort indicator moves off the price column and onto the name column. |

---

## I. Detail View

### I.1 Hourly Rate and Rate Breakdown

| # | Story | Expected |
|---|-------|----------|
| I.1.1 | I open the detail view of a computed-price resource (e.g. an EC2 instance). | The detail view additionally shows the hourly rate where one exists, alongside the `~$/mo` estimate seen in the list. |
| I.1.2 | I open the detail view of an S3 bucket. | The detail view shows the full per-class rate breakdown (Standard, IA, Glacier, Deep Archive), not just the `$0.023/GB` headline seen in the list. |
| I.1.3 | I open the detail view of an `mwaa` environment / a `transfer` server. | The `mwaa` detail notes that autoscaled workers are excluded from the estimate; the `transfer` detail shows the `$0.04/GB` data-transfer rate. |

---

## J. Offline, Region, and Edge Cases

### J.1 Region Sensitivity and Offline Behavior

| # | Story | Expected |
|---|-------|----------|
| J.1.1 | I switch region with `:region` and re-open a priced list. | Estimates update to the newly selected region's rates — the tables are region-keyed. |
| J.1.2 | I view a resource in a region the rate table doesn't cover. | The cell shows `—`, never a wrong number. |
| J.1.3 | I view an instance/node type newer than the binary. | The cell shows `—` until a future release regenerates the tables. |
| J.1.4 | I run a9s with `--demo`. | Every priced type shows non-`—` values — the demo fixtures are built to exercise real rates from the static tables. |
| J.1.5 | I run a9s offline / with no `pricing:GetProducts` permission on my role. | The price column still renders — all rates come from embedded tables, so no runtime call is made and none is required. |

### J.2 Filter, Refresh, and Empty States

| # | Story | Expected |
|---|-------|----------|
| J.2.1 | I filter a priced list with `/` and type a term. | The price column is preserved on the visible rows. |
| J.2.2 | I refresh the list with `ctrl+r`. | Prices recompute from the (unchanged) static tables alongside the resource refresh; the column persists. |
| J.2.3 | I open a priced list that returns zero resources. | No price values are shown; the normal empty-state indicator appears. |

---

## K. Display Format Rules

### K.1 Cell Content Rules

| # | Story | Expected |
|---|-------|----------|
| K.1.1 | I observe a computed estimate. | The cell is a `~`-prefixed dollar amount with a `/mo` (or hourly, in detail) suffix (e.g. `~$70/mo`, `~$3.65/mo`). |
| K.1.2 | I observe a unit-rate cell. | The cell is a bare dollar rate with its unit and no `~` (e.g. `$0.023/GB`, `$0.20/1M req`). |
| K.1.3 | I observe a resource with no applicable rate. | The cell is `—` only — never `$0.00`, never blank, never a fabricated number. |
| K.1.4 | I observe any price cell. | The cell shows one dominant rate (at most two terms); the full breakdown lives in the detail view. |
| K.1.5 | I observe a high-cost estimate (e.g. a large Redshift or MSK cluster). | The amount renders readably (thousands separated) so large `~$/mo` values remain scannable. |
