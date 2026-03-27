# QA User Stories: Resource Costs in List Views (Issue #73)

Scope: displaying estimated per-hour or per-month costs alongside resources in list views where pricing is reasonably deterministic. All stories treat a9s as a black box. Cost data is derived from instance type, node type, storage size, or provisioned capacity -- not from AWS Cost Explorer or billing APIs.

AWS pricing is region-specific. All cost estimates shown must reflect the currently selected region.

---

## A. EC2 Instance Costs

### A.1 Cost Column Presence

| # | Story | Expected |
|---|-------|----------|
| A.1.1 | I open the EC2 instance list. | A "Cost/mo" column is visible in the list view alongside the existing columns (Name, State, Lifecycle, Type, Private IP, Public IP, Instance ID, Launch Time). |
| A.1.2 | I observe the Cost/mo column header. | The header text reads "Cost/mo" (or similar concise label) and is styled in bold blue (`#7aa2f7`) like all other column headers. |
| A.1.3 | I observe cost values for running on-demand instances. | Each running on-demand instance shows a monthly dollar amount (e.g., `$30.37`, `$138.24`) derived from the instance type and region. |
| A.1.4 | I compare cost values across instance types. | A `t3.medium` instance shows a lower monthly cost than an `m5.xlarge` instance. Relative ordering matches public AWS pricing. |
| A.1.5 | I observe a spot instance (Lifecycle = "spot"). | The cost column shows the spot-appropriate cost or a visual indicator distinguishing it from on-demand pricing (e.g., lower value, "spot" annotation). |
| A.1.6 | I observe a stopped instance. | The cost column shows `$0.00` or a clear indicator that no compute cost is accruing (storage costs may still apply but are not shown here). |
| A.1.7 | I observe a terminated instance. | The cost column shows `$0.00` or `--`. |

**AWS comparison:**
```
aws ec2 describe-instances --query 'Reservations[].Instances[].{ID:InstanceId,Type:InstanceType,State:State.Name,Lifecycle:InstanceLifecycle}'
aws pricing get-products --service-code AmazonEC2 --filters Type=TERM_MATCH,Field=instanceType,Value=t3.medium Type=TERM_MATCH,Field=location,Value="US East (N. Virginia)"
```
Expected fields visible: Name, State, Lifecycle, Type, Private IP, Public IP, Instance ID, Launch Time, Cost/mo (per views.yaml ec2 list + new cost column).

### A.2 EC2 Cost Accuracy

| # | Story | Expected |
|---|-------|----------|
| A.2.1 | I switch region from us-east-1 to eu-west-1 using `:region` and re-open EC2 list. | The cost values update to reflect eu-west-1 pricing, which differs from us-east-1 for the same instance types. |
| A.2.2 | I compare the displayed cost for a `t3.micro` in us-east-1 against the AWS public pricing page. | The monthly cost is approximately correct (within a few percent of the published on-demand hourly rate multiplied by 730 hours). |
| A.2.3 | I view an instance type that does not exist in the pricing lookup table. | The cost column shows `--` or `N/A` rather than crashing or displaying $0.00 incorrectly. |

### A.3 EC2 Cost Sorting

| # | Story | Expected |
|---|-------|----------|
| A.3.1 | I sort the EC2 list by cost (if a cost sort key is available). | Rows reorder by cost value. The most expensive instances appear first (descending) or last (ascending). |
| A.3.2 | I sort by cost then by name. | Switching sort keys removes the sort indicator from the cost column and applies it to the name column. |

---

## B. RDS Instance Costs

### B.1 Cost Column Presence

| # | Story | Expected |
|---|-------|----------|
| B.1.1 | I open the RDS (DB Instances) list. | A "Cost/mo" column is visible alongside existing columns: DB Identifier, Engine, Version, Status, Class, Endpoint, Multi-AZ. |
| B.1.2 | I observe cost values for available instances. | Each available RDS instance shows a monthly dollar amount derived from the DB instance class, engine, and multi-AZ setting. |
| B.1.3 | I compare a Multi-AZ enabled instance to a single-AZ instance of the same class. | The Multi-AZ instance shows approximately double the cost of the single-AZ instance. |
| B.1.4 | I compare a `db.t3.micro` to a `db.r5.2xlarge`. | The `db.r5.2xlarge` shows a significantly higher monthly cost. |
| B.1.5 | I observe a stopped RDS instance. | The cost column shows `$0.00` or indicates no compute cost (storage costs are not computed here). |

**AWS comparison:**
```
aws rds describe-db-instances --query 'DBInstances[].{ID:DBInstanceIdentifier,Class:DBInstanceClass,Engine:Engine,MultiAZ:MultiAZ,Status:DBInstanceStatus}'
aws pricing get-products --service-code AmazonRDS --filters Type=TERM_MATCH,Field=instanceType,Value=db.t3.micro
```
Expected fields visible: DB Identifier, Engine, Version, Status, Class, Endpoint, Multi-AZ, Cost/mo (per views.yaml dbi list + new cost column).

---

## C. ElastiCache Redis Costs

### C.1 Cost Column Presence

| # | Story | Expected |
|---|-------|----------|
| C.1.1 | I open the ElastiCache Redis list. | A "Cost/mo" column is visible alongside existing columns: Cluster ID, Version, Node Type, Status, Nodes, Endpoint. |
| C.1.2 | I observe cost values for available clusters. | Each available Redis cluster shows a monthly dollar amount derived from the cache node type and number of nodes. |
| C.1.3 | I compare a `cache.t3.micro` to a `cache.r6g.xlarge`. | The `cache.r6g.xlarge` shows a significantly higher monthly cost. |
| C.1.4 | I observe a cluster with 3 nodes vs 1 node of the same type. | The 3-node cluster shows approximately 3x the cost of the single-node cluster. |

**AWS comparison:**
```
aws elasticache describe-cache-clusters --query 'CacheClusters[].{ID:CacheClusterId,Type:CacheNodeType,Status:CacheClusterStatus,Nodes:NumCacheNodes}'
aws pricing get-products --service-code AmazonElastiCache --filters Type=TERM_MATCH,Field=instanceType,Value=cache.t3.micro
```
Expected fields visible: Cluster ID, Version, Node Type, Status, Nodes, Endpoint, Cost/mo (per views.yaml redis list + new cost column).

---

## D. NAT Gateway Costs

### D.1 Cost Column Presence

| # | Story | Expected |
|---|-------|----------|
| D.1.1 | I open the NAT Gateways list. | A "Cost/mo" column is visible alongside existing columns: Name, NAT Gateway ID, VPC ID, Subnet ID, State, Public IP. |
| D.1.2 | I observe cost for an active NAT gateway. | The cost column shows the base hourly rate multiplied by 730 hours (approximately $32-45/mo depending on region). Data processing costs are not included (they are usage-dependent). |
| D.1.3 | I observe a NAT gateway in "deleted" or "failed" state. | The cost column shows `$0.00` or `--`. |

**AWS comparison:**
```
aws ec2 describe-nat-gateways --query 'NatGateways[].{ID:NatGatewayId,State:State,VpcId:VpcId}'
```
Expected fields visible: Name, NAT Gateway ID, VPC ID, Subnet ID, State, Public IP, Cost/mo (per views.yaml nat list + new cost column).

---

## E. Load Balancer Costs

### E.1 Cost Column Presence

| # | Story | Expected |
|---|-------|----------|
| E.1.1 | I open the Load Balancers list. | A "Cost/mo" column is visible alongside existing columns: Name, Type, Scheme, State, DNS Name, VPC ID. |
| E.1.2 | I observe cost for an active ALB. | The cost column shows the base hourly rate multiplied by 730 hours (approximately $16-22/mo depending on region). LCU costs are not included (they are usage-dependent). |
| E.1.3 | I observe cost for an active NLB. | The NLB base cost may differ from the ALB base cost. The displayed value reflects the correct load balancer type. |
| E.1.4 | I observe a load balancer in a non-active state. | The cost column shows `$0.00` or `--`. |

**AWS comparison:**
```
aws elbv2 describe-load-balancers --query 'LoadBalancers[].{Name:LoadBalancerName,Type:Type,State:State.Code}'
```
Expected fields visible: Name, Type, Scheme, State, DNS Name, VPC ID, Cost/mo (per views.yaml elb list + new cost column).

---

## F. Elastic IP Costs

### F.1 Cost Column Presence

| # | Story | Expected |
|---|-------|----------|
| F.1.1 | I open the Elastic IPs list. | A "Cost/mo" column is visible alongside existing columns: Name, Allocation ID, Public IP, Association, Instance, Domain. |
| F.1.2 | I observe cost for an EIP associated with a running instance. | The cost column shows `$0.00` (attached EIPs are free). |
| F.1.3 | I observe cost for an unassociated (idle) EIP. | The cost column shows approximately `$3.65/mo` (the idle EIP charge). |
| F.1.4 | I compare attached vs unattached EIPs side by side. | Attached EIPs show $0.00 while unattached EIPs show a non-zero cost, making idle EIPs immediately visible as cost sinks. |

**AWS comparison:**
```
aws ec2 describe-addresses --query 'Addresses[].{AllocationId:AllocationId,PublicIp:PublicIp,AssociationId:AssociationId,InstanceId:InstanceId}'
```
Expected fields visible: Name, Allocation ID, Public IP, Association, Instance, Domain, Cost/mo (per views.yaml eip list + new cost column).

---

## G. Redshift Costs

### G.1 Cost Column Presence

| # | Story | Expected |
|---|-------|----------|
| G.1.1 | I open the Redshift list. | A "Cost/mo" column is visible alongside existing columns: Cluster ID, Status, Node Type, Nodes, Database, Endpoint. |
| G.1.2 | I observe cost for an available Redshift cluster. | The cost column shows a monthly dollar amount derived from node type multiplied by number of nodes multiplied by 730 hours. |
| G.1.3 | I compare a 2-node cluster to a 4-node cluster of the same type. | The 4-node cluster shows approximately double the cost of the 2-node cluster. |

**AWS comparison:**
```
aws redshift describe-clusters --query 'Clusters[].{ID:ClusterIdentifier,NodeType:NodeType,Nodes:NumberOfNodes,Status:ClusterStatus}'
```
Expected fields visible: Cluster ID, Status, Node Type, Nodes, Database, Endpoint, Cost/mo (per views.yaml redshift list + new cost column).

---

## H. OpenSearch Costs

### H.1 Cost Column Presence

| # | Story | Expected |
|---|-------|----------|
| H.1.1 | I open the OpenSearch list. | A "Cost/mo" column is visible alongside existing columns: Domain Name, Engine Version, Instance Type, Instances, Endpoint. |
| H.1.2 | I observe cost for an active OpenSearch domain. | The cost column shows a monthly dollar amount derived from instance type multiplied by instance count multiplied by 730 hours. |
| H.1.3 | I compare a domain with 3 instances to one with 1 instance of the same type. | The 3-instance domain shows approximately 3x the cost. |

**AWS comparison:**
```
aws opensearch describe-domains --domain-names my-domain --query 'DomainStatusList[].{Name:DomainName,Type:ClusterConfig.InstanceType,Count:ClusterConfig.InstanceCount}'
```
Expected fields visible: Domain Name, Engine Version, Instance Type, Instances, Endpoint, Cost/mo (per views.yaml opensearch list + new cost column).

---

## I. Resources Without Cost Column

### I.1 Usage-Dependent Resources Show No Cost

| # | Story | Expected |
|---|-------|----------|
| I.1.1 | I open the S3 Buckets list. | No "Cost/mo" column appears. S3 costs depend on storage class, request patterns, and data transfer, making bucket-level estimates unreliable. |
| I.1.2 | I open the Lambda Functions list. | No "Cost/mo" column appears. Lambda costs depend on invocation count and duration. |
| I.1.3 | I open the DynamoDB Tables list. | No "Cost/mo" column appears. DynamoDB costs depend on read/write capacity mode and traffic patterns. |
| I.1.4 | I open the CloudFront Distributions list. | No "Cost/mo" column appears. CloudFront costs depend on traffic volume and edge locations. |
| I.1.5 | I open the API Gateways list. | No "Cost/mo" column appears. API Gateway costs depend on request volume. |
| I.1.6 | I open any other resource type not listed in sections A-H (e.g., VPCs, Security Groups, IAM Roles, SQS Queues). | No "Cost/mo" column appears. These resources either have no direct cost or have purely usage-dependent pricing. |

**AWS comparison:**
```
aws s3api list-buckets
aws lambda list-functions
aws dynamodb list-tables
```
Expected: Standard columns per views.yaml definitions. No cost column.

---

## J. Cost Column Visual Integration

### J.1 Column Styling and Positioning

| # | Story | Expected |
|---|-------|----------|
| J.1.1 | I observe the cost column position in the EC2 list. | The Cost/mo column appears as the last (rightmost) column after all existing columns, or in a prominent position near Status/Type. |
| J.1.2 | I observe cost values alignment. | All cost values are right-aligned within the column (dollar amounts aligned on the decimal point) for easy visual scanning. |
| J.1.3 | I observe cost values for a running EC2 instance row (green). | The cost value renders in the same green row color as the rest of the row, maintaining the status-colored-row design. |
| J.1.4 | I observe cost values on the selected row. | The cost value uses the standard selection styling (blue background, dark foreground, bold) like all other cells. |
| J.1.5 | I observe cost values on a terminated/dimmed row. | The cost value is dimmed along with the rest of the row. |
| J.1.6 | I scroll columns horizontally with `h`/`l` on a narrow terminal. | The cost column is reachable via horizontal scroll if it overflows the visible area. It scrolls in sync with the column headers and data. |

### J.2 Cost Column in Detail View

| # | Story | Expected |
|---|-------|----------|
| J.2.1 | I press `d` on an EC2 instance to open the detail view. | The detail view may show a "Cost/mo" field showing the same estimated monthly cost as the list view column. |
| J.2.2 | I press `d` on an RDS instance. | The detail view may show a cost field reflecting DB instance class, multi-AZ, and region. |

---

## K. Cost Data Freshness and Edge Cases

### K.1 Region Sensitivity

| # | Story | Expected |
|---|-------|----------|
| K.1.1 | I switch region using `:region` and navigate to EC2. | Cost values reflect the newly selected region's pricing, not the previous region. |
| K.1.2 | I switch to a region with different pricing (e.g., us-east-1 to ap-southeast-1). | The same instance type shows different cost values in each region, matching the public pricing difference. |

### K.2 Edge Cases

| # | Story | Expected |
|---|-------|----------|
| K.2.1 | I view an instance type that is new or not in the pricing data. | The cost column shows `--` or `N/A` rather than $0.00 or causing an error. |
| K.2.2 | I view a resource list with zero resources. | No cost values are shown. The empty state message or "no resources" indicator appears as normal. |
| K.2.3 | I refresh the list with `ctrl+r`. | Cost values recalculate (in case underlying resource properties changed) alongside the resource data refresh. |
| K.2.4 | I filter the EC2 list with `/` and type a filter term. | Cost values are preserved on visible rows. Filtering by cost values may or may not be supported. |
| K.2.5 | I copy a resource ID with `c`. | The copied value is the resource ID (not the cost). Cost is display-only information. |

### K.3 Performance

| # | Story | Expected |
|---|-------|----------|
| K.3.1 | I open the EC2 list with 100+ instances. | Cost values appear within the same loading time as the resource list. Cost lookup does not cause a noticeable additional delay. |
| K.3.2 | I navigate rapidly between different resource lists. | Cost columns appear (or are absent) correctly for each resource type without lag or stale data from a previous view. |

---

## L. No Cost on EBS Volumes (Not Yet a Resource Type)

### L.1 EBS Not Available

| # | Story | Expected |
|---|-------|----------|
| L.1.1 | I look for EBS volumes in the main menu. | EBS volumes are not a listed resource type in the current main menu. Per issue #73, EBS is a good cost candidate -- once EBS volumes are added as a resource type, the Cost/mo column should show per-month cost based on volume type and size (e.g., gp3 at $0.08/GB/mo). |

---

## M. Cost Display Format

### M.1 Currency and Precision

| # | Story | Expected |
|---|-------|----------|
| M.1.1 | I observe a cost value. | The value includes a dollar sign and two decimal places (e.g., `$30.37`), using USD as the currency. |
| M.1.2 | I observe a zero-cost resource. | The value displays as `$0.00` or `--`, not as blank or missing. |
| M.1.3 | I observe a high-cost resource (e.g., a large Redshift cluster). | The value displays with comma separators for readability (e.g., `$1,234.56`) if the amount exceeds $999.99. |
| M.1.4 | I observe costs across multiple resource types. | All cost columns use the same format, currency symbol, and decimal precision for consistency. |
