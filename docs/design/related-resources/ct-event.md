# CloudTrail Events (ct-event) — Related Resources

## Real-World Use Cases

**1. "What resource was affected by this event?"** The event has `Resources[]` with ARNs and types. Navigate to the actual resource in a9s to see its current state — was it successfully modified, deleted, or is it in an error state?

**2. "Who performed this action?"** The `userIdentity` section reveals the actor — an IAM user, an assumed role, an AWS service, or a federated identity. Navigate to the IAM role or user to understand their permissions.

**3. "What else did this principal do?"** After finding a suspicious event, filter CloudTrail for other events by the same `userIdentity.arn` to see the full scope of their activity.

## Reverse Relationships

| Related Resource | How to Find | Scenario | Priority |
|-----------------|-------------|----------|----------|
| (None) | CloudTrail events are immutable audit records. Nothing references them. They ARE the reference. | — | — |

## Algorithmic Relationships

| Related Resource | Algorithm | Scenario | Priority |
|-----------------|-----------|----------|----------|
| Affected Resource | Parse `Resources[].ResourceType` and `Resources[].ResourceName` from the event. Map AWS resource types (e.g., `AWS::EC2::Instance`, `AWS::S3::Bucket`) to a9s resource types and navigate to the specific resource by ID/ARN. Also parse `requestParameters` for resource identifiers when `Resources[]` is empty (many events don't populate it). | "Show me the resource this event affected." THE core navigation — from audit event to live resource state. | P0 |
| IAM Role (role) | Parse `userIdentity.sessionContext.sessionIssuer.arn` for assumed role events. This is the role that was assumed to perform the action. Navigate to the role to check its permissions and trust policy. | "Which role did the actor use?" Answers "who can perform this action?" at the IAM level. | P0 |
| IAM User (iam-user) | Parse `userIdentity.arn` for IAM user events (where `userIdentity.type` is `IAMUser`). Navigate to the user to see their access keys, MFA status, and group memberships. | "Which IAM user performed this action?" | P1 |
| Source IP Context | Parse `sourceIPAddress` — if it's an AWS service (e.g., `lambda.amazonaws.com`, `ecs-tasks.amazonaws.com`), that identifies the service that made the call. If it's a public IP, it may map to a VPN endpoint, bastion host, or suspicious origin. | "Where did this action come from?" Distinguish between human actions (VPN IP) and service actions (AWS service endpoint). | P1 |

## CloudTrail Events (T key)

N/A — CloudTrail events ARE the audit data. There are no CloudTrail events about CloudTrail events (with the exception of LookupEvents API calls, which are management events themselves).
