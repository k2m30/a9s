# IAM Groups (iam-group) — Related Resources

## Real-World Use Cases

**1. "Who is in this group?"** Before modifying group permissions, know the blast radius — how many users inherit these permissions? The group listing doesn't show members; that requires a separate call.

**2. "What permissions does group membership grant?"** Navigate to the group's attached policies to see the effective permissions for all members.

**3. "Should this user be in this group?"** During access reviews, check each group's members against expected team membership. Former team members or role changes often leave stale group memberships.

## Reverse Relationships

| Related Resource | How to Find | Scenario | Priority |
|-----------------|-------------|----------|----------|
| IAM Users (iam-user) | `iam:GetGroup` with `GroupName`. Returns `Users[]` — all members of this group. | "Who is in this group?" THE primary question. This is also a child view. | P0 |

## Algorithmic Relationships

| Related Resource | Algorithm | Scenario | Priority |
|-----------------|-----------|----------|----------|
| Attached Policies (policy) | `iam:ListAttachedGroupPolicies` (managed) + `iam:ListGroupPolicies` (inline). Navigate to each policy for permissions detail. | "What permissions does this group grant?" All group members inherit these permissions. | P0 |
| Members' Activity | For each user returned by `iam:GetGroup`, check last activity and console login date. Identify stale members. | "Are all members still active?" Inactive users with group permissions are an unnecessary risk. | P1 |

## CloudTrail Events (T key)

| Event Name | Why Engineers Search For It |
|-----------|---------------------------|
| AddUserToGroup | "Who added this user to the group?" Group membership grants permissions — adding a user to an admin group is a privilege escalation. |
| RemoveUserFromGroup | "Who removed this user?" Could be intentional offboarding or accidental permission revocation. |
| AttachGroupPolicy / DetachGroupPolicy | "Who changed group permissions?" Affects all group members simultaneously. |
