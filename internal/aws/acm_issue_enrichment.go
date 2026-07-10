// acm_issue_enrichment.go — Wave 2 issue enrichment for the acm resource type.
package aws

import (
	"context"
	"fmt"
	"sync"
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

	// acmCodeStatusPendingValidation — Status==PENDING_VALIDATION. The
	// certificate is awaiting DNS/email validation before ACM can issue it.
	acmCodeStatusPendingValidation domain.FindingCode = "acm.status.pending-validation"
	// acmCodeStatusFailed — Status is one of EXPIRED, REVOKED, FAILED, or
	// VALIDATION_TIMED_OUT. The certificate cannot be used for TLS.
	acmCodeStatusFailed domain.FindingCode = "acm.status.failed"
	// acmCodeStatusInactive — Status==INACTIVE. An imported certificate no
	// longer attached to any resource for TLS termination.
	acmCodeStatusInactive domain.FindingCode = "acm.status.inactive"
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
		Findings:     make(map[string][]domain.Finding),
		TruncatedIDs: make(map[string]bool),
	}
	if clients.ACM == nil {
		return result, nil
	}
	truncated := len(resources) > EnrichmentCap
	now := time.Now()
	bangCount := 0
	n := min(len(resources), EnrichmentCap)
	var mu sync.Mutex
	_ = ForEachParallel(ctx, n, EnrichmentParallelism, func(i int) {
		r := resources[i]
		// DescribeCertificate requires the certificate ARN, read from
		// Fields["certificate_arn"] — the acm fetcher's stable source. (r.ID is
		// also the ARN since the domain-name-ID collision fix, but Fields is what
		// this checker has always used.)
		certARN := r.Fields["certificate_arn"]
		if certARN == "" {
			return
		}
		out, err := clients.ACM.DescribeCertificate(ctx, &acmsvc.DescribeCertificateInput{
			CertificateArn: aws.String(certARN),
		})
		mu.Lock()
		defer mu.Unlock()
		if err != nil {
			truncated = true
			result.TruncatedIDs[r.ID] = true
			return
		}
		if out.Certificate == nil {
			return
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
				setWave2Finding(&result, r.ID, acmCodeExpiresSoon, summary, "!", "acm", nil, "")
				bangCount++
				return
			}
		}
		// Orphan check — only for ISSUED certs not already flagged.
		if cert.Status == acmtypes.CertificateStatusIssued && len(cert.InUseBy) == 0 {
			setWave2Finding(&result, r.ID, acmCodeOrphan, "certificate not in use (orphan)", "~", "acm", nil, "")
			// "~" is informational — not counted in IssueCount.
		}
	})
	result.IssueCount = bangCount
	result.Truncated = truncated
	return result, nil
}
