# CloudFront Distributions (cf) — Related Resources

## Real-World Use Cases

**1. "Where does this distribution pull content from?"** CloudFront origins can be S3 buckets, ALBs, API Gateways, or custom HTTP endpoints. The distribution configuration has the origin details, but you need to navigate to the actual origin resource to check its health.

**2. "Is this distribution protected by WAF?"** Security audit: internet-facing distributions should have WAF rules. The WebACLId is in the distribution config but you need to navigate to the WAF resource to see the actual rules.

**3. "Which SSL certificate does this distribution use?"** The distribution references an ACM certificate. Navigate to the cert to check expiration, validation status, and SANs.

**4. "Which DNS records point to this distribution?"** Route 53 alias records with CloudFront's hosted zone ID. Find them before modifying or disabling the distribution.

## Reverse Relationships

| Related Resource | How to Find | Scenario | Priority |
|-----------------|-------------|----------|----------|
| Route 53 Records (r53) | Search R53 hosted zones for alias records with `HostedZoneId=Z2FDTNDATAQYW2` and `DNSName` matching this distribution's domain. If a9s has R53 data cached, search in-memory. | "What domains point to this distribution?" Must update DNS before disabling. | P0 |

## Algorithmic Relationships

| Related Resource | Algorithm | Scenario | Priority |
|-----------------|-----------|----------|----------|
| S3 Bucket (s3) — Origin | Distribution `Origins[].DomainName` — FORWARD. Parse for S3 bucket names: `{bucket}.s3.amazonaws.com`, `{bucket}.s3.{region}.amazonaws.com`, or S3 website endpoints. Navigate to the bucket to check its content and policy. | "Which S3 bucket serves content?" The most common CF origin type. | P0 |
| Load Balancer (elb) — Origin | Distribution `Origins[].DomainName` may be an ALB/NLB DNS name. Match against ELB DNS names in a9s. | "Which ALB serves dynamic content?" CF → ALB is the standard web application pattern. | P0 |
| API Gateway (apigw) — Origin | `Origins[].DomainName` containing `{api-id}.execute-api.{region}.amazonaws.com`. | "Which API is behind CloudFront?" | P1 |
| ACM Certificate (acm) | Distribution `ViewerCertificate.ACMCertificateArn` — FORWARD. Navigate to the cert for expiration and validation status. | "Is the SSL certificate valid and current?" Certificate problems cause browser warnings for all users. | P1 |
| WAF Web ACL (waf) | Distribution `WebACLId` — FORWARD. Navigate to the WAF to see rules protecting this distribution. | "What WAF rules protect this distribution?" | P1 |
| Lambda@Edge (lambda) | Distribution `DefaultCacheBehavior.LambdaFunctionAssociations[]` and `CacheBehaviors[].LambdaFunctionAssociations[]` — FORWARD. These are Lambda functions executing at CloudFront edge locations. | "What Lambda functions run at the edge?" Lambda@Edge functions transform requests/responses and can cause subtle issues. | P2 |
| Access Logs S3 Bucket (s3) | Distribution `Logging.Bucket` — FORWARD (if enabled). | "Where are the access logs?" Needed for traffic analysis and debugging. | P2 |

## CloudTrail Events (T key)

| Event Name | Why Engineers Search For It |
|-----------|---------------------------|
| UpdateDistribution | "Who changed origins, cache behavior, or certificates?" Distribution changes propagate globally and take 5-20 minutes — a bad change affects all users everywhere. |
| DeleteDistribution | "Who deleted this distribution?" Must be disabled first, so this is deliberate. |
| CreateInvalidation | "Who invalidated the cache?" Cache invalidations can cause origin overload if all content expires simultaneously. |
