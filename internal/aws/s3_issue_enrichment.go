// s3_issue_enrichment.go — Wave 2 issue enrichment for the s3 resource type.
package aws

import (
	"context"
	"errors"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	smithy "github.com/aws/smithy-go"

	"github.com/k2m30/a9s/v3/internal/domain"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// s3 canonical FindingCodes.
const (
	s3CodePublicAccessBlockIncomplete domain.FindingCode = "s3.public-access-block-incomplete"
)

// EnrichS3PublicAccessBlock calls GetPublicAccessBlock per bucket (cap EnrichmentCap)
// and emits a finding when the bucket has no PAB configuration or when any of the
// four PAB flags is false.
//
// Contract (Finding):
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
// On PermanentRedirect (301) / IllegalLocationConstraintException (400):
//   The bucket lives in a different region than the configured S3 client.
//   ListBuckets returns ALL buckets globally regardless of region, but
//   per-bucket calls require the bucket's regional endpoint. Mark
//   TruncatedIDs[id]=true (data incomplete → row "?" marker) but do NOT
//   add to the failure-aggregate error: cross-region buckets are
//   operational, not bugs, and surfacing them in the `!` log produces
//   noise on multi-region accounts.
//
// On any other API error: no finding emitted; TruncatedIDs[id] = true and
// the failure aggregates into the returned composite error.
// IssueCount stays 0 (framework counts "!" findings directly).
func EnrichS3PublicAccessBlock(ctx context.Context, clients *ServiceClients, resources []resource.Resource, _ resource.ResourceCache) (IssueEnricherResult, error) {
	result := IssueEnricherResult{
		Findings:     make(map[string]domain.Finding),
		TruncatedIDs: make(map[string]bool),
		FieldUpdates: make(map[string]map[string]string),
	}
	if clients.S3 == nil {
		return result, nil
	}
	truncated := len(resources) > EnrichmentCap
	var failures []string
	total := min(len(resources), EnrichmentCap)
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
		bucketName := name
		out, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*s3.GetPublicAccessBlockOutput, error) {
			return clients.S3.GetPublicAccessBlock(ctx, &s3.GetPublicAccessBlockInput{
				Bucket: aws.String(bucketName),
			})
		})
		if err != nil {
			var apiErr smithy.APIError
			if errors.As(err, &apiErr) && apiErr.ErrorCode() == "NoSuchPublicAccessBlockConfiguration" {
				setWave2Finding(&result, name, s3CodePublicAccessBlockIncomplete, "public access block incomplete", "!", "s3", []domain.DetailRow{
					{Label: "Status", Value: "no public access block configuration"},
					{Label: "Account-level PAB", Value: "may still apply"},
				})
				result.FieldUpdates[name] = map[string]string{"status": "public access block incomplete"}
				continue
			}
			// Cross-region buckets: ListBuckets returns ALL buckets globally, but
			// per-bucket calls require the bucket's regional endpoint. AWS rejects
			// with PermanentRedirect (301) or IllegalLocationConstraintException (400)
			// when the configured client region differs from the bucket's region.
			// This is a legitimate environmental condition (multi-region account),
			// not a bug — mark data incomplete (TruncatedIDs → "?" row marker)
			// but do NOT spam the failure log. Classification shared with the
			// related-def checkers in s3_related.go via isS3CrossRegionErr.
			if isS3CrossRegionErr(err) {
				truncated = true
				result.TruncatedIDs[r.ID] = true
				continue
			}
			// Other errors: data incomplete — do not emit a finding.
			truncated = true
			result.TruncatedIDs[r.ID] = true
			failures = append(failures, fmt.Sprintf("%s: %v", name, err))
			continue
		}
		if out.PublicAccessBlockConfiguration == nil {
			setWave2Finding(&result, name, s3CodePublicAccessBlockIncomplete, "public access block incomplete", "!", "s3", []domain.DetailRow{
				{Label: "Status", Value: "no public access block configuration"},
				{Label: "Account-level PAB", Value: "may still apply"},
			})
			result.FieldUpdates[name] = map[string]string{"status": "public access block incomplete"}
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
		var falseFlags []domain.DetailRow
		for _, fc := range flags {
			if fc.value == nil || !*fc.value {
				falseFlags = append(falseFlags, domain.DetailRow{Label: fc.name, Value: "false"})
			}
		}
		if len(falseFlags) == 0 {
			// All flags true — healthy bucket. No finding.
			continue
		}
		falseFlags = append(falseFlags, domain.DetailRow{Label: "Account-level PAB", Value: "may still apply"})
		setWave2Finding(&result, name, s3CodePublicAccessBlockIncomplete, "public access block incomplete", "!", "s3", falseFlags)
		result.FieldUpdates[name] = map[string]string{"status": "public access block incomplete"}
	}
	result.IssueCount = 0
	result.Truncated = truncated
	return result,
		AggregateFailures("s3-enrich: GetPublicAccessBlock", failures, total)
}
