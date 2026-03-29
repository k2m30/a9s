# WAF Web ACLs (waf) — Related Resources

## Real-World Use Cases

**1. "What resources does this WAF protect?"** WAF Web ACLs are associated with ALBs, API Gateways, CloudFront distributions, and AppSync APIs. The Web ACL itself doesn't list its associations in the main describe response — you need a separate API call.

**2. "Is this WAF actually blocking anything?"** CloudWatch metrics show AllowedRequests, BlockedRequests, and CountedRequests per rule. Navigate to metrics to understand if the WAF is earning its keep.

**3. "Why is the WAF blocking legitimate traffic?"** Check the rules and their actions. Rate-based rules, geographic restrictions, or overly aggressive SQL injection patterns can cause false positives. The WAF logs show which rule blocked which request.

## Reverse Relationships

| Related Resource | How to Find | Scenario | Priority |
|-----------------|-------------|----------|----------|
| (Minimal) | Resources don't reference WAF by ARN in their own API responses. The association is managed by WAF, not the resource. Use the forward algorithmic lookup below. | — | — |

## Algorithmic Relationships

| Related Resource | Algorithm | Scenario | Priority |
|-----------------|-----------|----------|----------|
| Associated Resources (elb, apigw, cf) | `wafv2:ListResourcesForWebACL` with `WebACLArn` — returns ARNs of all resources (ALBs, API GW stages) this WAF protects. For CloudFront: `wafv2:ListResourcesForWebACL` with scope `CLOUDFRONT`. Single API call — purpose-built. Parse ARNs to identify resource types and navigate. | "What does this WAF protect?" THE primary question. Must know before modifying rules. | P0 |
| CloudWatch Log Group / S3 / Kinesis (logs, s3) | `wafv2:GetLoggingConfiguration` with `ResourceArn` (the Web ACL ARN). Returns the logging destination — can be CloudWatch Logs (`arn:aws:logs:...`), S3 (`arn:aws:s3:::...`), or Kinesis Data Firehose (`arn:aws:firehose:...`). | "Where are WAF logs?" Needed for debugging false positives — logs show which rule matched, the request details, and the action taken. | P1 |
| CloudWatch Metrics | Search alarms or check metrics with `WebACL` and `Rule` dimensions. Metrics: AllowedRequests, BlockedRequests, CountedRequests per rule. | "Is this WAF blocking traffic?" Operational awareness of WAF behavior. | P1 |

## CloudTrail Events (T key)

| Event Name | Why Engineers Search For It |
|-----------|---------------------------|
| UpdateWebACL | "Who changed the WAF rules?" Rule changes can block legitimate traffic (false positives) or stop blocking attacks (false negatives). |
| DeleteWebACL | "Who removed WAF protection?" Resources become unprotected against web attacks. |
| AssociateWebACL / DisassociateWebACL | "Who attached or detached this WAF from a resource?" Detaching WAF from an ALB removes protection. |
