// acm_issue_enrichment.go — Wave 2 issue enrichment for the acm resource type.
package aws

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	acmsvc "github.com/aws/aws-sdk-go-v2/service/acm"
	acmtypes "github.com/aws/aws-sdk-go-v2/service/acm/types"

	"github.com/k2m30/a9s/v3/internal/resource"
)

func init() {
	registerIssueEnricher("acm", EnrichACMCertificate, 100)
}

// EnrichACMCertificate calls DescribeCertificate per ACM certificate (cap EnrichmentCap)
// and raises findings for:
//   - NotAfter within 30 days → "!" finding "expires in <N> days" (or "expired" if past)
//   - ISSUED certificate with no InUseBy entries → "~" finding "certificate not in use (orphan)"
//
// IssueCount counts only "!" severity findings — "~" (informational) are excluded from the badge.
// Skip if clients.ACM == nil. Per-cert errors → Truncated.
func EnrichACMCertificate(ctx context.Context, clients *ServiceClients, resources []resource.Resource) (IssueEnricherResult, error) {
	findings := make(map[string]resource.EnrichmentFinding)
	truncatedIDs := make(map[string]bool)
	if clients.ACM == nil {
		return IssueEnricherResult{Findings: findings, TruncatedIDs: truncatedIDs}, nil
	}
	truncated := len(resources) > EnrichmentCap
	now := time.Now()
	bangCount := 0
	for i, r := range resources {
		if i >= EnrichmentCap {
			break
		}
		certARN := r.ID
		if certARN == "" {
			continue
		}
		out, err := clients.ACM.DescribeCertificate(ctx, &acmsvc.DescribeCertificateInput{
			CertificateArn: aws.String(certARN),
		})
		if err != nil {
			truncated = true
			truncatedIDs[r.ID] = true
			continue
		}
		if out.Certificate == nil {
			continue
		}
		cert := out.Certificate
		// Expiry check — takes priority over orphan check.
		if cert.NotAfter != nil {
			remaining := cert.NotAfter.Sub(now)
			const expiryWindow = 30 * 24 * time.Hour
			if remaining < expiryWindow {
				var summary string
				if remaining < 0 {
					summary = "expired"
				} else {
					days := int(remaining.Hours() / 24)
					summary = fmt.Sprintf("expires in %d days", days)
				}
				findings[certARN] = resource.EnrichmentFinding{
					Severity: "!",
					Summary:  summary,
				}
				bangCount++
				continue
			}
		}
		// Orphan check — only for ISSUED certs not already flagged.
		if cert.Status == acmtypes.CertificateStatusIssued && len(cert.InUseBy) == 0 {
			findings[certARN] = resource.EnrichmentFinding{
				Severity: "~",
				Summary:  "certificate not in use (orphan)",
			}
			// "~" is informational — not counted in IssueCount.
		}
	}
	return IssueEnricherResult{IssueCount: bangCount, Truncated: truncated, TruncatedIDs: truncatedIDs, Findings: findings}, nil
}
