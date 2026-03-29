# IAM Policies (policy) — Related Resources

## Real-World Use Cases

**1. "What roles, users, and groups use this policy?"** Before modifying a policy, you need the blast radius. IAM provides a purpose-built API for this: `ListEntitiesForPolicy` returns ALL attached entities in a single call.

**2. "What does this policy actually allow?"** Navigate to the policy document to see Actions, Resources, and Conditions. For customer-managed policies, check version history to understand how permissions evolved.

**3. "Is this policy over-permissioned?"** Security audit: does the policy grant `*` on `*`? Which services does it allow access to? Compare with actual usage via Access Analyzer.

## Reverse Relationships

| Related Resource | How to Find | Scenario | Priority |
|-----------------|-------------|----------|----------|
| IAM Roles (role) | `iam:ListEntitiesForPolicy` with `PolicyArn` — returns `PolicyRoles[]` with all roles this policy is attached to. | "Which roles use this policy?" The primary question. Modifying the policy changes permissions for ALL attached roles. | P0 |
| IAM Users (iam-user) | Same `iam:ListEntitiesForPolicy` — returns `PolicyUsers[]`. | "Which users have this policy directly attached?" Direct user attachment is generally a bad practice (should be via groups). | P1 |
| IAM Groups (iam-group) | Same `iam:ListEntitiesForPolicy` — returns `PolicyGroups[]`. | "Which groups use this policy?" Group attachment means all group members inherit these permissions. | P1 |

## Algorithmic Relationships

| Related Resource | Algorithm | Scenario | Priority |
|-----------------|-----------|----------|----------|
| Policy Versions | `iam:ListPolicyVersions` → `iam:GetPolicyVersion` for each. Shows the history of permission changes. The `IsDefaultVersion` flag indicates the active version. | "How have permissions changed over time?" Compare versions to understand when and how permissions were modified. | P1 |
| Referenced Resources (from policy document) | Parse the policy document's `Resource` fields for specific ARNs. E.g., `arn:aws:s3:::my-bucket/*` → S3 bucket, `arn:aws:dynamodb:*:*:table/my-table` → DynamoDB table. Map to a9s resources. | "What specific resources does this policy grant access to?" Concrete resource ARNs in the policy show exactly what's in scope. | P1 |

## CloudTrail Events (T key)

| Event Name | Why Engineers Search For It |
|-----------|---------------------------|
| CreatePolicyVersion | "Who changed the policy permissions?" Each permission change creates a new version. Shows the actor and the new policy document. |
| DeletePolicy | "Who deleted this policy?" All attached entities lose these permissions immediately. |
| AttachRolePolicy / AttachUserPolicy / AttachGroupPolicy | "Who attached this policy to new entities?" Expanding the policy's reach — more roles/users/groups now have these permissions. |
