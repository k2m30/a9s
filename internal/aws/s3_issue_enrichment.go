// s3_issue_enrichment.go — Wave 2 issue enrichment for the s3 resource type.
package aws

import (
	"context"
	"errors"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	smithy "github.com/aws/smithy-go"

	"github.com/k2m30/a9s/v3/internal/resource"
)

func init() {
	registerIssueEnricher("s3", EnrichS3PublicAccessBlock, 100)
	resource.RegisterIssueEnricherFieldKeys("s3", []string{"status"})
}

// EnrichS3PublicAccessBlock calls GetPublicAccessBlock per bucket (cap EnrichmentCap)
// and emits a finding when the bucket has no PAB configuration or when any of the
// four PAB flags is false.
//
// Contract (EnrichmentFinding):
//   - Severity is always "!" (important background concern on a Healthy row).
//   - Summary is always "public access block incomplete" — stable across all instances.
//   - Rows carry the per-case detail (never duplicated in Summary).
//
// On NoSuchPublicAccessBlockConfiguration:
//   Rows: {Label:"Status", Value:"no public access block configuration"},
//         {Label:"Account-level PAB", Value:"may still apply"}
//
// On out.PublicAccessBlockConfiguration == nil (no API error):
//   Same Rows as above.
//
// On partial PAB (one or more flags false):
//   Rows: one entry per false flag: {Label:"<FlagName>", Value:"false"},
//         plus {Label:"Account-level PAB", Value:"may still apply"}
//
// On any other API error: no finding emitted; TruncatedIDs[id] = true.
// IssueCount stays 0 (framework counts "!" findings directly).
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
			var apiErr smithy.APIError
			if errors.As(err, &apiErr) && apiErr.ErrorCode() == "NoSuchPublicAccessBlockConfiguration" {
				findings[name] = resource.EnrichmentFinding{
					Severity: "!",
					Summary:  "public access block incomplete",
					Rows: []resource.FindingRow{
						{Label: "Status", Value: "no public access block configuration"},
						{Label: "Account-level PAB", Value: "may still apply"},
					},
				}
				fieldUpdates[name] = map[string]string{"status": "public access block incomplete"}
				continue
			}
			// Other errors: data incomplete — do not emit a finding.
			truncated = true
			truncatedIDs[r.ID] = true
			continue
		}
		if out.PublicAccessBlockConfiguration == nil {
			findings[name] = resource.EnrichmentFinding{
				Severity: "!",
				Summary:  "public access block incomplete",
				Rows: []resource.FindingRow{
					{Label: "Status", Value: "no public access block configuration"},
					{Label: "Account-level PAB", Value: "may still apply"},
				},
			}
			fieldUpdates[name] = map[string]string{"status": "public access block incomplete"}
			continue
		}
		cfg := out.PublicAccessBlockConfiguration
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
		var falseFlags []resource.FindingRow
		for _, fc := range flags {
			if fc.value == nil || !*fc.value {
				falseFlags = append(falseFlags, resource.FindingRow{Label: fc.name, Value: "false"})
			}
		}
		if len(falseFlags) == 0 {
			// All flags true — healthy bucket. No finding.
			continue
		}
		falseFlags = append(falseFlags, resource.FindingRow{Label: "Account-level PAB", Value: "may still apply"})
		findings[name] = resource.EnrichmentFinding{
			Severity: "!",
			Summary:  "public access block incomplete",
			Rows:     falseFlags,
		}
		fieldUpdates[name] = map[string]string{"status": "public access block incomplete"}
	}
	return IssueEnricherResult{IssueCount: 0, Truncated: truncated, TruncatedIDs: truncatedIDs, Findings: findings, FieldUpdates: fieldUpdates}, nil
}
