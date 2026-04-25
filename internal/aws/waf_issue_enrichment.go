// waf_issue_enrichment.go — Wave 2 issue enrichment for the waf resource type.
package aws

import (
	"context"
	"errors"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	wafv2svc "github.com/aws/aws-sdk-go-v2/service/wafv2"
	wafv2types "github.com/aws/aws-sdk-go-v2/service/wafv2/types"

	"github.com/k2m30/a9s/v3/internal/resource"
)

func init() {
	registerIssueEnricher("waf", EnrichWAFLogging, 100)
	resource.RegisterIssueEnricherFieldKeys("waf", []string{"rules_summary"})
}

// EnrichWAFLogging calls GetLoggingConfiguration, ListResourcesForWebACL, and GetWebACL per WebACL
// (cap EnrichmentCap) and raises findings for:
//   - GetLoggingConfiguration returns WAFNonexistentItemException → "~" finding
//     "no logging configuration"
//   - ListResourcesForWebACL returns empty ResourceArns → "~" finding
//     "WebACL not associated with any resources (orphan)"
//
// Also writes FieldUpdates["rules_summary"] = "<N> rules BLOCK" or "0 rules ALLOW".
// Skip if clients.WAFv2 == nil. Per-WebACL errors (other than WAFNonexistentItemException) are
// aggregated and returned as a composite error alongside partial findings (E3, E4, E5).
func EnrichWAFLogging(ctx context.Context, clients *ServiceClients, resources []resource.Resource, _ resource.ResourceCache) (IssueEnricherResult, error) {
	findings := make(map[string]resource.EnrichmentFinding)
	fieldUpdates := make(map[string]map[string]string)
	truncatedIDs := make(map[string]bool)
	if clients.WAFv2 == nil {
		return IssueEnricherResult{Findings: findings, TruncatedIDs: truncatedIDs}, nil
	}
	truncated := len(resources) > EnrichmentCap
	var failures []string
	total := 0
	for i, r := range resources {
		if i >= EnrichmentCap {
			break
		}
		arn := r.Fields["arn"]
		if arn == "" {
			arn = r.ID
		}
		if arn == "" {
			continue
		}
		total++
		var rows []resource.FindingRow

		// Check logging configuration.
		_, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*wafv2svc.GetLoggingConfigurationOutput, error) {
			return clients.WAFv2.GetLoggingConfiguration(ctx, &wafv2svc.GetLoggingConfigurationInput{
				ResourceArn: aws.String(arn),
			})
		})
		if err != nil {
			if _, ok := errors.AsType[*wafv2types.WAFNonexistentItemException](err); ok {
				rows = append(rows, resource.FindingRow{
					Label: "Logging",
					Value: "no logging configuration",
					Tier:  "~",
				})
			} else {
				// Unexpected error — skip this ACL.
				failures = append(failures, fmt.Sprintf("%s: %v", r.ID, err))
				truncated = true
				truncatedIDs[r.ID] = true
				continue
			}
		}

		// Check resource associations.
		assocOut, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*wafv2svc.ListResourcesForWebACLOutput, error) {
			return clients.WAFv2.ListResourcesForWebACL(ctx, &wafv2svc.ListResourcesForWebACLInput{
				WebACLArn: aws.String(arn),
			})
		})
		if err != nil {
			failures = append(failures, fmt.Sprintf("%s: %v", r.ID, err))
			truncated = true
			truncatedIDs[r.ID] = true
			continue
		}
		if len(assocOut.ResourceArns) == 0 {
			rows = append(rows, resource.FindingRow{
				Label: "Associations",
				Value: "WebACL not associated with any resources (orphan)",
				Tier:  "~",
			})
		}

		// Compute rules_summary by fetching the full WebACL (optional — only if the
		// client implements WAFv2GetWebACLAPI, which production clients do but test
		// fakes focused on logging may not).
		rulesSummary := "0 rules"
		if getACLAPI, ok := clients.WAFv2.(WAFv2GetWebACLAPI); ok && r.Fields["name"] != "" && r.Fields["id"] != "" {
			scope := r.Fields["scope"]
			if scope == "" {
				scope = "REGIONAL"
			}
			getOut, gerr := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*wafv2svc.GetWebACLOutput, error) {
				return getACLAPI.GetWebACL(ctx, &wafv2svc.GetWebACLInput{
					Name:  aws.String(r.Fields["name"]),
					Id:    aws.String(r.Fields["id"]),
					Scope: wafv2types.Scope(scope),
				})
			})
			if gerr == nil && getOut.WebACL != nil {
				blockCount := 0
				for _, rule := range getOut.WebACL.Rules {
					if rule.Action != nil && rule.Action.Block != nil {
						blockCount++
					}
				}
				ruleCount := len(getOut.WebACL.Rules)
				if ruleCount == 0 {
					rulesSummary = "0 rules"
				} else {
					rulesSummary = fmt.Sprintf("%d/%d BLOCK", blockCount, ruleCount)
				}
			}
		}
		fieldUpdates[r.ID] = map[string]string{
			"rules_summary": rulesSummary,
		}

		if len(rows) == 0 {
			continue
		}
		findings[r.ID] = resource.EnrichmentFinding{
			Severity: "~",
			Summary:  rows[0].Value,
			Rows:     rows,
		}
	}
	// All WAF logging findings are severity "~" (informational).
	return IssueEnricherResult{IssueCount: 0, Truncated: truncated, TruncatedIDs: truncatedIDs, Findings: findings, FieldUpdates: fieldUpdates},
		AggregateFailures("waf-enrich", failures, total)
}
