// cf_issue_enrichment.go — Wave 2 issue enrichment for the cf resource type.
package aws

import (
	"context"
	"strings"
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudfront"
	cftypes "github.com/aws/aws-sdk-go-v2/service/cloudfront/types"

	"github.com/k2m30/a9s/v3/internal/domain"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// cf canonical FindingCodes.
const (
	cfCodeInsecureProtocol domain.FindingCode = "cf.insecure-protocol"

	// cfCodeInProgress — Status==InProgress. Distribution config change is
	// still propagating to edge locations.
	cfCodeInProgress domain.FindingCode = "cf.status.in-progress"
	// cfCodeDisabled — Enabled==false. Distribution is administratively
	// disabled and not serving traffic.
	cfCodeDisabled domain.FindingCode = "cf.disabled"
)

// EnrichCloudFrontDistribution calls GetDistributionConfig per distribution (cap EnrichmentCap)
// and returns a Finding for any distribution with insecure viewer or origin protocol settings.
//
// Findings (severity "~" — informational):
//   - DefaultCacheBehavior.ViewerProtocolPolicy == "allow-all" → "no HTTPS redirect (insecure)"
//   - Any Origin with CustomOriginConfig.OriginProtocolPolicy == "http-only" → "origin without TLS"
//
// Skip if clients.CloudFront == nil. Per-distribution errors → truncated.
func EnrichCloudFrontDistribution(ctx context.Context, clients *ServiceClients, resources []resource.Resource, _ resource.ResourceCache) (IssueEnricherResult, error) {
	result := IssueEnricherResult{
		Findings:     make(map[string]domain.Finding),
		TruncatedIDs: make(map[string]bool),
	}
	if clients.CloudFront == nil {
		return result, nil
	}
	truncated := len(resources) > EnrichmentCap
	n := min(len(resources), EnrichmentCap)
	var mu sync.Mutex
	_ = ForEachParallel(ctx, n, EnrichmentParallelism, func(i int) {
		r := resources[i]
		distID := r.ID
		if distID == "" {
			return
		}
		out, err := clients.CloudFront.GetDistributionConfig(ctx, &cloudfront.GetDistributionConfigInput{
			Id: aws.String(distID),
		})
		mu.Lock()
		defer mu.Unlock()
		if err != nil {
			truncated = true
			result.TruncatedIDs[r.ID] = true
			return
		}
		if out.DistributionConfig == nil {
			return
		}
		cfg := out.DistributionConfig
		var rows []domain.DetailRow
		var summaries []string

		// Check viewer protocol policy on default cache behavior.
		if cfg.DefaultCacheBehavior != nil &&
			cfg.DefaultCacheBehavior.ViewerProtocolPolicy == cftypes.ViewerProtocolPolicyAllowAll {
			summaries = append(summaries, "no HTTPS redirect (insecure)")
			rows = append(rows, domain.DetailRow{
				Label: "ViewerProtocolPolicy",
				Value: "allow-all",
				Tier:  "~",
			})
		}

		// Check origin protocol policies.
		if cfg.Origins != nil {
			for _, origin := range cfg.Origins.Items {
				if origin.CustomOriginConfig != nil &&
					origin.CustomOriginConfig.OriginProtocolPolicy == cftypes.OriginProtocolPolicyHttpOnly {
					originID := ""
					if origin.Id != nil {
						originID = *origin.Id
					}
					summaries = append(summaries, "origin without TLS")
					rows = append(rows, domain.DetailRow{
						Label: "Origin",
						Value: originID,
						Tier:  "~",
					})
					rows = append(rows, domain.DetailRow{
						Label: "OriginProtocolPolicy",
						Value: "http-only",
						Tier:  "~",
					})
				}
			}
		}

		if len(summaries) == 0 {
			return
		}
		summary := strings.Join(summaries, "; ")
		setWave2Finding(&result, distID, cfCodeInsecureProtocol, summary, "~", "cf", rows, "")
	})
	// All CloudFront findings are severity "~" (informational).
	// IssueCount counts only "!" severity findings; "~" do not contribute.
	result.IssueCount = 0
	result.Truncated = truncated
	return result, nil
}
