// acm_issue_enrichment.go — Wave 2 issue enrichment for the acm resource type.
package aws

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	acmsvc "github.com/aws/aws-sdk-go-v2/service/acm"
	acmtypes "github.com/aws/aws-sdk-go-v2/service/acm/types"

	"github.com/k2m30/a9s/v3/internal/domain"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// acm canonical FindingCodes.
const (
	acmCodeExpiresSoon domain.FindingCode = "acm.expires-soon"
	acmCodeOrphan      domain.FindingCode = "acm.orphan"
)

// EnrichACMCertificate calls DescribeCertificate per ACM certificate (cap EnrichmentCap)
// and raises findings for:
//   - NotAfter within 30 days → "!" finding "expires in <N> days" (or "expired" if past)
//   - ISSUED certificate with no InUseBy entries → "~" finding "certificate not in use (orphan)"
//
// IssueCount counts only "!" severity findings — "~" (informational) are excluded from the badge.
// Skip if clients.ACM == nil. Per-cert errors → Truncated.
func EnrichACMCertificate(ctx context.Context, clients *ServiceClients, resources []resource.Resource, _ resource.ResourceCache) (IssueEnricherResult, error) {
	result := IssueEnricherResult{
		Findings:     make(map[string]domain.Finding),
		TruncatedIDs: make(map[string]bool),
	}
	if clients.ACM == nil {
		return result, nil
	}
	truncated := len(resources) > EnrichmentCap
	now := time.Now()
	bangCount := 0
	for i, r := range resources {
		if i >= EnrichmentCap {
			break
		}
		// DescribeCertificate requires the certificate ARN. The acm fetcher
		// (acm.go) sets ID = domain name and stores the ARN in
		// Fields["certificate_arn"]. Passing r.ID errors with ValidationError.
		certARN := r.Fields["certificate_arn"]
		if certARN == "" {
			continue
		}
		out, err := clients.ACM.DescribeCertificate(ctx, &acmsvc.DescribeCertificateInput{
			CertificateArn: aws.String(certARN),
		})
		if err != nil {
			truncated = true
			result.TruncatedIDs[r.ID] = true
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
				setWave2Finding(&result, r.ID, acmCodeExpiresSoon, summary, "!", "acm", nil)
				bangCount++
				continue
			}
		}
		// Orphan check — only for ISSUED certs not already flagged.
		if cert.Status == acmtypes.CertificateStatusIssued && len(cert.InUseBy) == 0 {
			setWave2Finding(&result, r.ID, acmCodeOrphan, "certificate not in use (orphan)", "~", "acm", nil)
			// "~" is informational — not counted in IssueCount.
		}
	}
	result.IssueCount = bangCount
	result.Truncated = truncated
	return result, nil
}
