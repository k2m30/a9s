# Load Balancers (elb) — Related Resources

## Real-World Use Cases

**1. "Why is this ALB returning 502s?"** The ELB itself is healthy — the problem is downstream. You need to navigate to its target groups, then to target health for each TG. The ELB knows its TGs (via listeners), but seeing target health requires two additional hops.

**2. "What DNS names point to this load balancer?"** Before decommissioning, find all Route 53 records (alias and CNAME) that resolve to this ELB's DNS name. Missing one means an outage for that domain.

**3. "Is this ALB protected by WAF?"** During a security review, check if the ALB has a WAF Web ACL associated. The ELB's own API response doesn't include WAF info — it's stored on the WAF side.

**4. "Where are the access logs for this ELB?"** Access logs go to an S3 bucket, but the bucket name is in the ELB attributes (a separate API call), not the main describe response.

## Reverse Relationships

| Related Resource | How to Find | Scenario | Priority |
|-----------------|-------------|----------|----------|
| Route 53 Records (r53) | Search Route 53 hosted zones for alias records where `AliasTarget.DNSName` matches this ELB's DNS name and `AliasTarget.HostedZoneId` matches the ELB's canonical hosted zone ID. If a9s has R53 data cached, search in-memory. | "What domains point to this ELB?" Must find all DNS entries before decommission or migration. | P0 |
| CloudFront Distributions (cf) | Search CF distributions for origins where `DomainName` matches this ELB's DNS name. | "Is this ELB behind a CDN?" CloudFront → ALB is a common pattern for web apps. | P1 |
| WAF Web ACLs (waf) | `wafv2:GetWebACLForResource` with this ALB's ARN. Returns the associated WAF Web ACL, if any. Only works for ALBs (not NLBs or CLBs). | "Is this ALB protected by WAF rules?" Security audit — unprotected internet-facing ALBs are a finding. | P1 |
| CloudWatch Alarms (alarm) | Search alarms with `LoadBalancer` dimension matching the ELB's ARN suffix (format: `app/{name}/{id}` for ALB, `net/{name}/{id}` for NLB). | "What monitoring watches this ELB?" Alarms on 5XX count, target response time, unhealthy host count. | P1 |
| CloudFormation Stacks (cfn) | Check for `aws:cloudformation:stack-name` tag. | "Which stack manages this ELB?" | P2 |

## Algorithmic Relationships

| Related Resource | Algorithm | Scenario | Priority |
|-----------------|-----------|----------|----------|
| Target Groups (tg) | `elbv2:DescribeTargetGroups` with `LoadBalancerArn` — returns all TGs associated with this ELB. Alternatively, ELB → Listeners → each listener's default action and rules reference TG ARNs. | "Where is traffic going?" Navigate to TGs to see registered targets and their health. The core debugging path for 502/503 errors. | P0 |
| S3 Bucket (s3) — Access Logs | `elbv2:DescribeLoadBalancerAttributes` — check `access_logs.s3.enabled`, `access_logs.s3.bucket`, `access_logs.s3.prefix`. | "Where are the access logs?" Needed for detailed request-level analysis during incidents or forensics. | P1 |
| ACM Certificate (acm) | Multi-hop: ELB → Listeners → HTTPS listeners have `Certificates[].CertificateArn`. `elbv2:DescribeListenerCertificates` for additional SNI certs. | "Which SSL certificates does this ELB use? Are any expiring?" Certificate expiration causes hard-to-debug HTTPS failures. | P1 |
| Security Groups (sg) | ELB response has `SecurityGroups[]` — FORWARD (ALB and CLB only, not NLB). Navigate to SGs to verify inbound rules allow traffic on listener ports. | "Why can't clients reach this ELB?" The SG must allow inbound traffic on the listener ports (80, 443, etc.). | P1 |

## CloudTrail Events (T key)

| Event Name | Why Engineers Search For It |
|-----------|---------------------------|
| DeleteLoadBalancer | "Who deleted this load balancer?" Immediate outage for all traffic routed through it. |
| ModifyLoadBalancerAttributes | "Who changed deletion protection, idle timeout, or access log settings?" |
| CreateLoadBalancer | "Who created this ELB, and with what configuration?" Cost and security audit. |
