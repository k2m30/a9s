# Child View: IAM Roles --> Attached Policies

**Status:** Planned
**Tier:** SHOULD-HAVE

---

## Navigation

- **Entry:** Press Enter on a role in the IAM Roles list
- **Frame title:** `role-policies(7) — payment-service-execution-role`
- **View stack:** IAM Roles --> Attached Policies --> (detail/YAML via d/y)
- **Esc** returns to IAM Roles list
- **No new key bindings** beyond the standard set

## views.yaml

```yaml
role_policies:
  list:
    Policy Name:
      path: PolicyName
      width: 40
    Policy ARN:
      path: PolicyArn
      width: 56
    Type:
      key: policy_type
      width: 10
  detail:
    - PolicyName
    - PolicyArn
```

Note on computed fields and data merging:
- `policy_type`: "Managed" for policies from `ListAttachedRolePolicies`, "Inline" for policies from `ListRolePolicies`
- This view merges results from two API calls into a single list. Managed policies have both name and ARN. Inline policies have only a name (PolicyArn shows "— " for inline policies).

Source structs:
- `iamtypes.AttachedPolicy` (for managed policies)
- String (inline policy names from `ListRolePolicies`)

## AWS API

- **Call 1:** `iam:ListAttachedRolePolicies` with `RoleName` — returns managed policies (name + ARN)
- **Call 2:** `iam:ListRolePolicies` with `RoleName` — returns inline policy names only
- Both calls paginated via `Marker` / `IsTruncated`
- **Latency:** Fast (<1 second each). Roles typically have 2-10 attached policies.
- **Note:** To show inline policy documents, a further `iam:GetRolePolicy` call would be needed per inline policy. This is deferred — the YAML view of the inline policy name is sufficient for now.

## ASCII Wireframe

```
 a9s v0.5.0  prod:us-east-1                                              ? for help
┌──────── role-policies(7) — payment-service-execution-role ─────────────────────┐
│ POLICY NAME                              POLICY ARN                        TY…  │
│ AmazonECSTaskExecutionRolePolicy         arn:aws:iam::aws:policy/service-…  Ma…  │
│ AmazonS3ReadOnlyAccess                   arn:aws:iam::aws:policy/AmazonS3… Ma…  │
│ CloudWatchLogsFullAccess                 arn:aws:iam::aws:policy/CloudWat… Ma…  │
│ AmazonSQSFullAccess                      arn:aws:iam::aws:policy/AmazonSQ… Ma…  │
│ SecretsManagerReadWrite                  arn:aws:iam::aws:policy/Secrets…   Ma…  │
│ payment-service-custom-policy            —                                  In…  │
│ xray-tracing-inline                      —                                  In…  │
└─────────────────────────────────────────────────────────────────────────────────┘
```

Row coloring:
- Managed policies with `aws:policy/` in ARN (AWS-managed): PLAIN `#c0caf5`
- Managed policies with customer-managed ARNs: PLAIN `#c0caf5`
- Inline policies: DIM `#565f89` (to visually distinguish them)
- Policies named `AdministratorAccess` or `PowerUserAccess`: RED `#f7768e` (security risk highlight)

The red highlighting for overprivileged policies is a deliberate design choice. During a security audit or incident investigation ("why does this role have admin access?"), instantly spotting the dangerous policy saves precious time. This is not a status per se, but a risk signal — consistent with the design philosophy of coloring rows by operational significance.

Selected row: full-width blue background overrides all coloring.

## Copy Behavior

`c` copies the Policy ARN (for managed policies) or the policy name (for inline policies). The ARN is what you need to look up the policy document or cross-reference in IAM.

## Help Screen

```
┌──────────────────────────────── Help ───────────────────────────────────────────┐
│ ROLE POLICIES         GENERAL              NAVIGATION           HOTKEYS         │
│                                                                                 │
│ <esc>   Back          <ctrl-r> Refresh     <j>       Down       <?>   Help      │
│ <d>     Detail        </>      Filter      <k>       Up         <:>   Command   │
│ <y>     YAML          <:>      Command     <g>       Top                        │
│ <c>     Copy ARN                           <G>       Bottom                     │
│                                            <h/l>     Cols                       │
│                                            <pgup/dn> Page                       │
│                                                                                 │
│                       Press any key to close                                    │
└─────────────────────────────────────────────────────────────────────────────────┘
```
