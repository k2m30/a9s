# SES Identities (ses) — Related Resources

## Real-World Use Cases

**1. "Is this domain properly configured for email sending?"** SES domain verification requires DNS records (DKIM, SPF, DMARC) in Route 53. Navigate to the hosted zone to verify the records exist.

**2. "Where do bounce and complaint notifications go?"** SES can publish delivery feedback to SNS topics. You need to find which topics receive bounces, complaints, and delivery notifications.

**3. "Is this identity actually sending email?"** CloudWatch metrics show send volume, bounce rate, and complaint rate. High bounce/complaint rates can trigger SES sending restrictions.

## Reverse Relationships

| Related Resource | How to Find | Scenario | Priority |
|-----------------|-------------|----------|----------|
| (Minimal) | SES identities are not referenced by other AWS resources via ARN. They are used by application code for sending email. | — | — |

## Algorithmic Relationships

| Related Resource | Algorithm | Scenario | Priority |
|-----------------|-----------|----------|----------|
| Route 53 Records (r53) | For domain identities: DKIM requires 3 CNAME records (`{token}._domainkey.{domain}` → `{token}.dkim.amazonses.com`). SPF requires a TXT record (`v=spf1 include:amazonses.com ~all`). DMARC is optional but recommended. Search the domain's R53 hosted zone for these records. | "Are the DNS records correct for email delivery?" Missing DKIM records cause email to land in spam. | P0 |
| SNS Topics (sns) | `ses:GetIdentityNotificationAttributes` returns SNS topic ARNs for `Bounce`, `Complaint`, and `Delivery` notification types. Navigate to each topic to see who receives these notifications. | "Where do bounce/complaint notifications go?" Unmonitored bounces lead to SES reputation damage and eventual sending suspension. | P1 |
| Configuration Sets (not in a9s) | `ses:ListConfigurationSets` → `ses:DescribeConfigurationSet` for event destinations. Configuration sets route sending events to SNS, Kinesis Data Firehose, or CloudWatch. | "How is email sending tracked?" | P2 |

## CloudTrail Events (T key)

| Event Name | Why Engineers Search For It |
|-----------|---------------------------|
| DeleteIdentity | "Who deleted this SES identity?" The domain or email address can no longer be used for sending. |
| SetIdentityDkimEnabled | "Who enabled or disabled DKIM?" Disabling DKIM degrades email deliverability. |
| PutIdentityPolicy | "Who changed the sending authorization policy?" Policies control who can send on behalf of this identity. |
