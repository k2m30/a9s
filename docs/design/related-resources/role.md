# IAM Roles (role) — Related Resources

## Real-World Use Cases

**1. "Why is this Lambda getting AccessDenied?"** Navigate from the Lambda's execution role to its attached policies. The role has AmazonS3ReadOnlyAccess but the Lambda needs `sqs:SendMessage`. Three hops to the answer: Lambda → Role ARN → attached policies → policy document.

**2. "What can assume this role?"** The trust policy (`AssumeRolePolicyDocument`) lists which principals — other accounts, AWS services, OIDC providers — can assume the role. This is THE security boundary for IAM roles.

**3. "Is this role actually used?"** Before deleting a role during cleanup, check: when was it last used? Which services has it accessed? IAM's `GetServiceLastAccessedDetails` answers this definitively.

**4. "What resources use this role?"** IAM roles are referenced by EC2 (via instance profiles), Lambda (execution role), ECS (task role, execution role), Step Functions, CodeBuild, CodePipeline, Glue, and many more. Finding all consumers requires cross-service search.

## Reverse Relationships

IAM Roles are the most heavily referenced resource in AWS — almost every service uses them.

| Related Resource | How to Find | Scenario | Priority |
|-----------------|-------------|----------|----------|
| Lambda Functions (lambda) | `lambda:ListFunctions` and match `Role` field against this role's ARN. If a9s has Lambda data cached, search in-memory. | "Which Lambdas use this role?" The most common role consumer. | P0 |
| EC2 Instances (ec2) | `ec2:DescribeInstances` and match `IamInstanceProfile.Arn` — instance profiles wrap roles. The instance profile name often matches the role name, but not always. To be precise: `iam:ListInstanceProfilesForRole` → get instance profile ARN → match against instances. | "Which instances use this role?" EC2 roles via instance profiles. | P0 |
| ECS Task Definitions (not in a9s) | Search task definitions for `taskRoleArn` or `executionRoleArn` matching this role's ARN. | "Which ECS tasks use this role?" ECS has two role types: task role (what the container can access) and execution role (what ECS itself needs). | P0 |
| EKS Service Accounts (not in a9s) | EKS pods use roles via IRSA (IAM Roles for Service Accounts). Check this role's trust policy for `Federated` principal with EKS OIDC provider URL and `Condition` matching a Kubernetes service account name. | "Which Kubernetes pods use this role?" The trust policy reveals the pod identity. | P1 |
| Step Functions (sfn) | `states:ListStateMachines` → `states:DescribeStateMachine` → match `roleArn`. | "Which workflows use this role?" | P1 |
| CodeBuild Projects (cb) | Search CodeBuild projects for `serviceRole` matching this role's ARN. | "Which build projects use this role?" | P1 |
| CodePipeline (pipeline) | Search pipelines for `roleArn` matching. | "Which pipelines use this role?" | P1 |
| Glue Jobs (glue) | Search Glue jobs for `Role` matching. | "Which ETL jobs use this role?" | P1 |
| CloudFormation Stacks (cfn) | Search stacks for `RoleARN` matching (CFN service role). | "Which stacks deploy using this role?" | P2 |

## Algorithmic Relationships

| Related Resource | Algorithm | Scenario | Priority |
|-----------------|-----------|----------|----------|
| Trust Policy → Trusted Principals | Parse `AssumeRolePolicyDocument` for `Principal` entries: `Service` (which AWS services), `AWS` (which accounts/roles/users), `Federated` (OIDC/SAML providers). Condition keys narrow the scope (e.g., `sts:ExternalId` for cross-account, `accounts.google.com:sub` for web identity). | "Who can assume this role?" THE security question for roles. Overly permissive trust policies are a top IAM risk. | P0 |
| Attached Policies → Permissions | `iam:ListAttachedRolePolicies` (managed policies) + `iam:ListRolePolicies` (inline policy names). Navigate to each policy to see what actions are allowed/denied. This is also a child view. | "What can this role do?" Understanding the role's effective permissions. | P0 |
| Last Activity | `iam:GenerateServiceLastAccessedDetails` → `iam:GetServiceLastAccessedDetails` with `JobId`. Returns which AWS services this role accessed and when. | "Is this role still in use?" For cleanup: if last accessed date is months ago, the role may be orphaned. For least-privilege: if it has S3 access but never used S3, remove that permission. | P1 |

## CloudTrail Events (T key)

| Event Name | Why Engineers Search For It |
|-----------|---------------------------|
| DeleteRole | "Who deleted this role?" Any service using this role will start getting AccessDenied errors immediately. |
| AttachRolePolicy / DetachRolePolicy | "Who changed the role's permissions?" Adding AdministratorAccess or removing required policies. The most common IAM audit event. |
| UpdateAssumeRolePolicy | "Who changed who can assume this role?" Trust policy changes are the highest-risk IAM modification — they control which principals can obtain the role's permissions. |
