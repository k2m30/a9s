# ACM Certificates (acm) — Related Resources

## Real-World Use Cases

**1. "What resources use this certificate?"** Before deleting or allowing a certificate to expire, you need to know every ELB listener, CloudFront distribution, and API Gateway custom domain that references it. ACM provides this with a single field.

**2. "Why isn't this certificate renewing?"** ACM auto-renewal requires DNS validation records to be in place (for DNS-validated certs) or email access (for email-validated certs). Check if the validation records still exist in Route 53.

**3. "Is this certificate covering all the right domains?"** Check SANs (Subject Alternative Names) to verify all expected domains are included.

## Reverse Relationships

ACM has the best reverse lookup in AWS — the `InUseBy` field.

| Related Resource | How to Find | Scenario | Priority |
|-----------------|-------------|----------|----------|
| Resources Using This Cert | `acm:DescribeCertificate` returns `InUseBy[]` — a list of ARNs for ALL AWS resources using this certificate (ELB listeners, CloudFront distributions, API GW custom domains, etc.). This is purpose-built and definitive. Parse ARNs to identify resource types and navigate. | "What breaks if this certificate expires?" THE critical question. A single expired cert can take down multiple services simultaneously. | P0 |

## Algorithmic Relationships

| Related Resource | Algorithm | Scenario | Priority |
|-----------------|-----------|----------|----------|
| Route 53 Records (r53) | For DNS-validated certs: `DomainValidationOptions[].ResourceRecord` contains the CNAME record that must exist in Route 53 for validation and renewal. Check if the record still exists. | "Will this certificate auto-renew?" If the validation CNAME was removed from Route 53, auto-renewal will fail. | P0 |
| Load Balancers (elb) | Parse `InUseBy[]` ARNs for ELB listener ARNs (format: `arn:aws:elasticloadbalancing:...listener/...`). Navigate to the ELB. | "Which ELBs use this certificate?" | P0 |
| CloudFront (cf) | Parse `InUseBy[]` ARNs for CloudFront distribution ARNs. Navigate to the distribution. | "Which CF distributions use this certificate?" | P0 |
| API Gateway (apigw) | Parse `InUseBy[]` ARNs for API GW domain ARNs. Navigate to the API. | "Which APIs use this certificate?" | P1 |

## CloudTrail Events (T key)

| Event Name | Why Engineers Search For It |
|-----------|---------------------------|
| DeleteCertificate | "Who deleted this certificate?" If it was in use, deletion fails — but if it succeeds, something was already detached. |
| RequestCertificate | "Who requested this certificate and for which domains?" Audit trail for certificate provisioning. |
| RenewCertificate | "Did auto-renewal succeed?" ACM triggers this automatically. If it fails, the certificate will expire. |
