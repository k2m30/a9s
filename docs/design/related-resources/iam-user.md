# IAM Users (iam-user) — Related Resources

## Real-World Use Cases

**1. "What can this user do?"** A user's effective permissions come from three sources: directly attached policies, group memberships (and group policies), and permission boundaries. You need to check all three.

**2. "Is this user's access key compromised?"** During a security incident, find the user's access keys, check their age and last-used date, and search CloudTrail for actions performed with those keys.

**3. "Does this user have MFA enabled?"** Security audit: users with console access but no MFA are a high-risk finding.

## Reverse Relationships

| Related Resource | How to Find | Scenario | Priority |
|-----------------|-------------|----------|----------|
| CloudTrail Events (ct-event) | `cloudtrail:LookupEvents` with `AttributeKey=Username, AttributeValue={username}`. Returns all API actions performed by this user. | "What has this user done?" Security investigation — full activity audit trail. | P0 |

## Algorithmic Relationships

| Related Resource | Algorithm | Scenario | Priority |
|-----------------|-----------|----------|----------|
| IAM Groups (iam-group) | `iam:ListGroupsForUser` with `UserName`. Returns all groups this user belongs to. Navigate to groups to see inherited permissions. | "What groups is this user in?" Groups provide the bulk of a user's permissions in well-designed IAM. | P0 |
| Attached Policies (policy) | `iam:ListAttachedUserPolicies` (managed) + `iam:ListUserPolicies` (inline). Navigate to each policy to see permissions. | "What policies are directly attached to this user?" Direct attachment is often a sign of ad-hoc permission grants. | P0 |
| Access Keys | `iam:ListAccessKeys` with `UserName`. Returns key IDs, status (Active/Inactive), and creation date. Critical for security: old or unused keys should be rotated or deleted. | "Are there active access keys? How old are they?" Keys older than 90 days are a common audit finding. | P0 |
| MFA Devices | `iam:ListMFADevices` with `UserName`. Returns MFA device serial numbers and enable date. | "Is MFA enabled?" No-MFA with console access is a critical security finding. | P1 |
| Login Profile | `iam:GetLoginProfile` with `UserName`. Returns whether console access is enabled and when the password was last set. Returns error if no console access. | "Does this user have AWS Console access?" Users with only programmatic access don't need console passwords. | P1 |
| Last Activity | `iam:GenerateServiceLastAccessedDetails` → `iam:GetServiceLastAccessedDetails`. | "When was this user last active? Which services did they use?" For cleanup and least-privilege analysis. | P1 |

## CloudTrail Events (T key)

| Event Name | Why Engineers Search For It |
|-----------|---------------------------|
| DeleteUser | "Who deleted this user?" |
| CreateAccessKey / DeleteAccessKey | "Who created or rotated access keys for this user?" New access keys could indicate compromised credentials being refreshed by an attacker. |
| CreateLoginProfile / UpdateLoginProfile | "Who enabled or changed console access?" Creating console access for a programmatic-only user is suspicious. |
