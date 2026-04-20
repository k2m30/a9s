// s3_issue_enrichment.go — Wave 2 issue enrichment for the s3 resource type.
package aws

import (
	"context"
	"errors"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	smithy "github.com/aws/smithy-go"

	"github.com/k2m30/a9s/v3/internal/resource"
)

func init() {
	registerIssueEnricher("s3", EnrichS3PublicAccessBlock, 100)
	resource.RegisterIssueEnricherFieldKeys("s3", []string{"public_access"})
}

// EnrichS3PublicAccessBlock calls GetPublicAccessBlock per bucket (cap EnrichmentCap)
// and returns a Finding when the bucket has no PAB configuration, or when any of the
// four PAB flags is false.
//
// Severity is "~" (informational).
// Summaries:
//   - "no public access block (account-level may apply)" — NoSuchPublicAccessBlockConfiguration
//   - "public-access block partial: <flag>=false" — one or more flags false
//
// IssueCount stays 0.
// Skip when clients.S3 == nil.
func EnrichS3PublicAccessBlock(ctx context.Context, clients *ServiceClients, resources []resource.Resource) (IssueEnricherResult, error) {
	findings := make(map[string]resource.EnrichmentFinding)
	fieldUpdates := make(map[string]map[string]string)
	truncatedIDs := make(map[string]bool)
	if clients.S3 == nil {
		return IssueEnricherResult{Findings: findings, TruncatedIDs: truncatedIDs}, nil
	}
	truncated := len(resources) > EnrichmentCap
	for i, r := range resources {
		if i >= EnrichmentCap {
			break
		}
		name := r.Name
		if name == "" {
			name = r.ID
		}
		if name == "" {
			continue
		}
		out, err := clients.S3.GetPublicAccessBlock(ctx, &s3.GetPublicAccessBlockInput{
			Bucket: aws.String(name),
		})
		if err != nil {
			// Check for NoSuchPublicAccessBlockConfiguration (bucket has no PAB config set).
			var apiErr smithy.APIError
			if errors.As(err, &apiErr) && apiErr.ErrorCode() == "NoSuchPublicAccessBlockConfiguration" {
				findings[name] = resource.EnrichmentFinding{
					Severity: "~",
					Summary:  "no public access block (account-level may apply)",
				}
				fieldUpdates[name] = map[string]string{"public_access": "?"}
				continue
			}
			// Other errors: skip but signal incomplete data.
			truncated = true
			truncatedIDs[r.ID] = true
			continue
		}
		if out.PublicAccessBlockConfiguration == nil {
			findings[name] = resource.EnrichmentFinding{
				Severity: "~",
				Summary:  "no public access block (account-level may apply)",
			}
			fieldUpdates[name] = map[string]string{"public_access": "?"}
			continue
		}
		cfg := out.PublicAccessBlockConfiguration
		// Check each of the four PAB flags; report the first false one.
		type flagCheck struct {
			name  string
			value *bool
		}
		flags := []flagCheck{
			{"BlockPublicAcls", cfg.BlockPublicAcls},
			{"IgnorePublicAcls", cfg.IgnorePublicAcls},
			{"BlockPublicPolicy", cfg.BlockPublicPolicy},
			{"RestrictPublicBuckets", cfg.RestrictPublicBuckets},
		}
		allBlocked := true
		for _, fc := range flags {
			if fc.value == nil || !*fc.value {
				allBlocked = false
				findings[name] = resource.EnrichmentFinding{
					Severity: "~",
					Summary:  fmt.Sprintf("public-access block partial: %s=false", fc.name),
				}
				break
			}
		}
		if allBlocked {
			fieldUpdates[name] = map[string]string{"public_access": "BLOCKED"}
		} else {
			fieldUpdates[name] = map[string]string{"public_access": "RISK"}
		}
	}
	return IssueEnricherResult{IssueCount: 0, Truncated: truncated, TruncatedIDs: truncatedIDs, Findings: findings, FieldUpdates: fieldUpdates}, nil
}
