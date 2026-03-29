# EKS Clusters (eks) — Related Resources

## Real-World Use Cases

**1. "Is the EKS control plane logging everything it should?"** The cluster configuration shows which log types are enabled, but the actual logs go to a CloudWatch log group. You need to find that log group to see API audit events, authenticator logs, and scheduler decisions.

**2. "Which node groups provide compute for this cluster?"** The cluster API response has VPC config and endpoint settings but does NOT list its node groups. Those are a separate resource.

**3. "Why can't pods in this cluster pull images from ECR?"** You need to trace: cluster → node group → node IAM role → role's policies. The node role needs ECR pull permissions. Three hops through different resource types.

**4. "What IAM roles can authenticate to this cluster?"** EKS uses OIDC federation for pod identity. The cluster's OIDC issuer URL maps to an IAM OIDC provider, which trusts specific roles. This is a multi-hop path through IAM.

## Reverse Relationships

| Related Resource | How to Find | Scenario | Priority |
|-----------------|-------------|----------|----------|
| EKS Node Groups (ng) | `eks:ListNodegroups` with `clusterName`. If a9s has NG data loaded, filter in-memory by cluster name. | "What compute capacity does this cluster have?" The most common navigation from cluster to its child resources. | P0 |
| CloudFormation Stacks (cfn) | Check for `aws:cloudformation:stack-name` tag on the cluster, or search CFN stacks. EKS clusters created by CDK/CFN have this tag. Additionally, EKS creates a hidden CFN stack for the cluster VPC networking. | "Which IaC stack manages this cluster?" | P2 |
| CloudWatch Alarms (alarm) | Search alarms with `ClusterName` dimension. Container Insights alarms use EKS cluster name as a dimension. | "What monitoring watches this cluster?" | P1 |

## Algorithmic Relationships

| Related Resource | Algorithm | Scenario | Priority |
|-----------------|-----------|----------|----------|
| CloudWatch Log Group (logs) | Fixed naming: `/aws/eks/{cluster-name}/cluster`. Only exists if control plane logging is enabled (check `Logging.ClusterLogging[].Enabled` in the cluster response). | "Show me the API server audit logs." Critical during security investigations — who made kubectl calls? What was denied? | P0 |
| IAM OIDC Provider (not in a9s) | Cluster response has `identity.oidc.issuer` URL. Strip `https://` prefix and search `iam:ListOpenIDConnectProviders` → `iam:GetOpenIDConnectProvider` for matching URL. | "Which IAM OIDC provider federates this cluster?" Needed for setting up or debugging pod IAM roles (IRSA). | P1 |
| VPC / Subnets / Security Groups | Cluster response has `resourcesVpcConfig.vpcId`, `subnetIds[]`, `securityGroupIds[]`, and `clusterSecurityGroupId` — all FORWARD. But the cluster SG is special: it's automatically created and applied to both control plane and managed node ENIs. | "What network config does this cluster use?" Navigate to VPC/subnets to check IP availability, SGs to verify port access. | P1 |
| IAM Roles (role) | Cluster response has `roleArn` (cluster service role) — FORWARD. Additionally, each node group has its own IAM role. Multi-hop: cluster → node groups → each NG's `nodeRole`. | "What permissions does the cluster have? What can nodes do?" Debugging RBAC vs IAM permission issues. | P1 |
| EC2 Instances (ec2) | Multi-hop: cluster → node groups → ASGs → instances. `eks:ListNodegroups` → each NG's `resources.autoScalingGroups[].name` → `autoscaling:DescribeAutoScalingGroups` → `Instances[]`. | "Which EC2 instances are part of this cluster?" For capacity analysis, instance health, or to find a node where a specific pod is running. | P1 |

## CloudTrail Events (T key)

| Event Name | Why Engineers Search For It |
|-----------|---------------------------|
| DeleteCluster | "Who deleted this EKS cluster?" Catastrophic action — find the actor. |
| UpdateClusterConfig | "Who changed endpoint access, logging, or encryption settings?" Changes to public/private endpoint access affect who can reach the API server. |
| AssociateAccessPolicy / CreateAccessEntry | "Who granted IAM principal access to this cluster?" EKS access entry API events show RBAC-level changes. |
