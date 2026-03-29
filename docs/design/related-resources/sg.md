# Security Groups (sg) — Related Resources

## Real-World Use Cases

**1. "What resources use this security group?"** THE most common question about SGs. Before modifying a rule, you need to know the blast radius — will changing port 5432 access affect 3 instances or 300? The SG itself has zero information about who uses it.

**2. "Which other SGs reference this one as a source?"** SG rules can reference other SGs as source/destination. If you delete or modify this SG, you may break rules in OTHER SGs that point to it. Circular references are common (e.g., app SG allows inbound from ALB SG, ALB SG allows outbound to app SG).

**3. "Why can't service A talk to service B?"** The classic connectivity debugging workflow. You need to check: does service A's SG allow outbound to service B's port? Does service B's SG allow inbound from service A's SG or IP? This requires seeing both SGs and cross-referencing their rules.

**4. "Is this SG unused?"** During security cleanup, you need to find SGs that are no longer attached to any resource. The SG API has no "in use" flag — you must check all possible consumers.

## Reverse Relationships

Security groups are the most heavily reverse-referenced resource in AWS. Almost every VPC resource references SGs.

| Related Resource | How to Find | Scenario | Priority |
|-----------------|-------------|----------|----------|
| ENIs (eni) — UNIVERSAL | `ec2:DescribeNetworkInterfaces` with `Filters=[{Name=group-id, Values=[sg-xxx]}]`. This is the BEST universal reverse lookup because every VPC resource that uses a SG creates an ENI with that SG attached. Returns ENIs for EC2, RDS, ELB, Lambda, ECS, EKS, VPC Endpoints, etc. — all in one call. Parse the ENI `Description` and `InterfaceType` to identify the owning service. | "What uses this SG?" The single API call that answers this across ALL services. | P0 |
| EC2 Instances (ec2) | `ec2:DescribeInstances` with `Filters=[{Name=instance.group-id, Values=[sg-xxx]}]`. More specific than ENI lookup — returns full instance details. | "Which EC2 instances use this SG?" The most common specific query. | P0 |
| Other Security Groups (sg) | `ec2:DescribeSecurityGroupRules` with `Filters=[{Name=referenced-group-info.group-id, Values=[sg-xxx]}]`. Returns rules in OTHER SGs that reference this SG as a source/destination. | "Which SGs reference this one?" Must check before deletion — removing a referenced SG breaks those rules. Also important for understanding network trust boundaries. | P0 |
| RDS Instances (dbi) | `rds:DescribeDBInstances` and filter by `VpcSecurityGroups[].VpcSecurityGroupId`. If a9s has RDS data cached, search in-memory. | "Which databases use this SG?" Modifying the SG affects database connectivity. | P1 |
| ELB (elb) | `elbv2:DescribeLoadBalancers` and filter by `SecurityGroups[]`. ALBs and CLBs use SGs (NLBs don't). If a9s has ELB data cached, search in-memory. | "Which load balancers use this SG?" | P1 |
| Lambda Functions (lambda) | `lambda:ListFunctions` and filter by `VpcConfig.SecurityGroupIds`. Expensive — must iterate all functions. If a9s has Lambda data cached, search in-memory. | "Which VPC Lambdas use this SG?" | P1 |
| ElastiCache Redis (redis) | If a9s has Redis data cached, filter by security group IDs in their configs. | "Which Redis clusters use this SG?" | P1 |
| VPC Endpoints (vpce) | `ec2:DescribeVpcEndpoints` and filter by `Groups[].GroupId`. | "Which VPC endpoints use this SG?" | P2 |
| CloudFormation Stacks (cfn) | Check for `aws:cloudformation:stack-name` tag. | "Which stack manages this SG?" | P2 |

## Algorithmic Relationships

| Related Resource | Algorithm | Scenario | Priority |
|-----------------|-----------|----------|----------|
| VPC (vpc) | SG response has `VpcId` — FORWARD. Navigate to the VPC for network context. | "Which VPC is this SG in?" Prevents confusion when multiple VPCs have similarly named SGs. | P1 |

## CloudTrail Events (T key)

| Event Name | Why Engineers Search For It |
|-----------|---------------------------|
| AuthorizeSecurityGroupIngress | "Who opened a port?" THE most security-sensitive SG event. Every new inbound rule is a potential attack surface expansion. Shows the actor, the port/protocol, and the source CIDR or SG. |
| RevokeSecurityGroupIngress | "Who closed a port?" Understanding why connectivity broke — someone may have tightened rules without realizing the impact. |
| DeleteSecurityGroup | "Who deleted this SG?" Deletion fails if the SG is still referenced by any resource or other SG rule — but if it succeeds, it means the SG was truly unused. |
