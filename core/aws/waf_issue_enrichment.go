// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// waf_issue_enrichment.go — Wave 2 issue enrichment for the waf resource type.
package aws

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
	wafv2svc "github.com/aws/aws-sdk-go-v2/service/wafv2"
	wafv2types "github.com/aws/aws-sdk-go-v2/service/wafv2/types"

	"github.com/k2m30/a9s/v3/core/catalog"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// waf canonical FindingCodes.
const (
	wafCodeNoLogging domain.FindingCode = "waf.no-logging"
	wafCodeNoRules   domain.FindingCode = "waf.no-rules"
)

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
	result := IssueEnricherResult{
		Findings:     make(map[string][]domain.Finding),
		TruncatedIDs: make(map[string]bool),
		FieldUpdates: make(map[string]map[string]string),
	}
	if clients.WAFv2 == nil {
		return result, nil
	}
	var failures []string
	total := 0
	resources = capAtEnrichmentCap(&result, resources, resourceIDsOf)
	n := len(resources)
	var mu sync.Mutex
	_ = ForEachParallel(ctx, n, EnrichmentParallelism, func(i int) {
		r := resources[i]
		arn := r.Fields["arn"]
		if arn == "" {
			arn = r.ID
		}
		if arn == "" {
			return
		}
		mu.Lock()
		total++
		mu.Unlock()
		var rows []domain.DetailRow

		// Check logging configuration.
		_, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*wafv2svc.GetLoggingConfigurationOutput, error) {
			return clients.WAFv2.GetLoggingConfiguration(ctx, &wafv2svc.GetLoggingConfigurationInput{
				ResourceArn: aws.String(arn),
			})
		})
		if err != nil {
			if _, ok := errors.AsType[*wafv2types.WAFNonexistentItemException](err); ok {
				rows = append(rows, domain.DetailRow{
					Label: "Logging",
					Value: "off",
					Tier:  "~",
				})
			} else {
				// Unexpected error — skip this ACL.
				mu.Lock()
				failures = append(failures, fmt.Sprintf("%s: %v", r.ID, err))
				result.TruncatedIDs[r.ID] = true
				mu.Unlock()
				return
			}
		}

		// Check resource associations.
		assocOut, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*wafv2svc.ListResourcesForWebACLOutput, error) {
			return clients.WAFv2.ListResourcesForWebACL(ctx, &wafv2svc.ListResourcesForWebACLInput{
				WebACLArn: aws.String(arn),
			})
		})
		if err != nil {
			mu.Lock()
			failures = append(failures, fmt.Sprintf("%s: %v", r.ID, err))
			result.TruncatedIDs[r.ID] = true
			mu.Unlock()
			return
		}
		if len(assocOut.ResourceArns) == 0 {
			rows = append(rows, domain.DetailRow{
				Label: "Associations",
				Value: "WebACL not associated with any resources (orphan)",
				Tier:  "~",
			})
		}

		// Compute rules_summary by fetching the full WebACL (optional — only if the
		// client implements WAFv2GetWebACLAPI, which production clients do but test
		// fakes focused on logging may not).
		rulesSummary := "0 rules"
		noRules := false
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
					noRules = true
				} else {
					rulesSummary = fmt.Sprintf("%d/%d BLOCK", blockCount, ruleCount)
				}
			}
		}

		mu.Lock()
		defer mu.Unlock()
		result.FieldUpdates[r.ID] = map[string]string{
			"rules_summary": rulesSummary,
		}

		if noRules {
			setWave2Finding(&result, r.ID, wafCodeNoRules, "web ACL has no rules", "~", "waf",
				[]domain.DetailRow{{Label: "Rules", Value: "0", Tier: "~"}})

		}

		if len(rows) == 0 {
			return
		}
		setWave2Finding(&result, r.ID, wafCodeNoLogging, catalog.Phrase(wafCodeNoLogging), "~", "waf", rows)
	})
	sort.Strings(failures)
	// All WAF logging findings are severity "~" (informational).
	MarkInformationalOnly(&result)
	return result,
		AggregateFailures("waf-enrich", failures, total)
}
