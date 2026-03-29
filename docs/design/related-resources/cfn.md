# CloudFormation Stacks (cfn) — Related Resources

## Real-World Use Cases

**1. "What resources did this stack create?"** CFN stacks manage collections of resources. The stack resources (a child view) show physical resource IDs that map to actual AWS resources in a9s. This is the most important navigation path for CFN.

**2. "Which stacks depend on this one?"** Stacks can export values that other stacks import. Deleting or modifying a stack with exports breaks dependent stacks. The `ListImports` API shows who depends on your exports.

**3. "Is this a nested stack?"** If `ParentId` is set, this stack is nested inside another. Navigate to the parent to understand the full deployment.

**4. "Why is this stack stuck in UPDATE_ROLLBACK_COMPLETE?"** You need stack events (a child view) and the specific resource that failed. Navigate from the failed resource to the actual AWS resource to understand the root cause.

## Reverse Relationships

| Related Resource | How to Find | Scenario | Priority |
|-----------------|-------------|----------|----------|
| Dependent Stacks (cfn) | For each stack export name: `cloudformation:ListImports` returns stacks that import this export. No export → no dependents. | "Which stacks break if I delete this one?" Cross-stack dependencies via exports/imports. Must check before deletion. | P0 |
| Nested Child Stacks (cfn) | `cloudformation:ListStackResources` and filter for `ResourceType=AWS::CloudFormation::Stack`. These are nested stacks. Navigate to each child stack. | "What nested stacks does this parent contain?" CDK and nested CloudFormation produce deep stack hierarchies. | P1 |

## Algorithmic Relationships

| Related Resource | Algorithm | Scenario | Priority |
|-----------------|-----------|----------|----------|
| Managed Resources (various) | `cloudformation:ListStackResources` returns all resources with `LogicalResourceId`, `PhysicalResourceId`, and `ResourceType`. Map `ResourceType` (e.g., `AWS::EC2::Instance`, `AWS::Lambda::Function`) to a9s resource types and navigate by `PhysicalResourceId`. This is also a child view. | "What resources does this stack manage?" THE core value of CFN in a9s — seeing the full inventory of managed resources and navigating to each one. | P0 |
| Parent Stack (cfn) | Stack response has `ParentId` — FORWARD (if nested). Navigate to the parent to understand the deployment hierarchy. | "Which parent stack contains this one?" | P1 |
| Root Stack (cfn) | Stack response has `RootId` — FORWARD (if deeply nested). Navigate to the root to see the top-level deployment. | "What's the top-level deployment?" In CDK projects, stacks can be 3+ levels deep. | P1 |
| IAM Role (role) | Stack has `RoleARN` — FORWARD (if a service role is configured). Navigate to the role to check what permissions CFN uses for this deployment. | "What permissions does CFN use for this stack?" If deployments are failing with permission errors, check this role. | P1 |
| S3 Bucket — Template | Stack template may be stored in S3. `cloudformation:GetTemplate` returns the template body. The S3 URL is sometimes in the `TemplateURL` parameter. | "Where is the template stored?" | P2 |

## CloudTrail Events (T key)

| Event Name | Why Engineers Search For It |
|-----------|---------------------------|
| DeleteStack | "Who deleted this stack?" All managed resources are deleted (unless `DeletionPolicy=Retain`). Catastrophic if unintentional. |
| UpdateStack | "Who triggered a stack update?" Shows the template change, parameter values, and actor. |
| CreateStack | "Who created this stack?" Initial deployment audit trail. |
