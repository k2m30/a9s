// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// waf_issue_enrichment.go — Wave 2 issue enrichment for the waf resource type.
package aws

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
	wafv2svc "github.com/aws/aws-sdk-go-v2/service/wafv2"
	wafv2types "github.com/aws/aws-sdk-go-v2/service/wafv2/types"

	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// waf canonical FindingCodes.
const (
	wafCodeNoLogging domain.FindingCode = "waf.no-logging"
	wafCodeNoRules   domain.FindingCode = "waf.no-rules"
	wafCodeOrphan    domain.FindingCode = "waf.orphan"
)

// wafScopeErr is a GetLoggingConfiguration read that failed for one log scope
// with anything but "not found".
type wafScopeErr struct {
	scope wafv2types.LogScope
	err   error
}

// wafLoggingConfigurations is the one reader of a web ACL's logging
// configurations. WAF keeps one per log scope — the customer's own, and the
// ones Security Lake and a CloudWatch telemetry rule manage — and
// GetLoggingConfiguration answers for one scope at a time
// (https://docs.aws.amazon.com/waf/latest/APIReference/API_GetLoggingConfiguration.html),
// so every scope the SDK knows is asked. A scope answering "not found" has
// none; any other failure is returned with its scope.
func wafLoggingConfigurations(ctx context.Context, api WAFv2API, arn string) ([]wafv2types.LoggingConfiguration, []wafScopeErr) {
	var configs []wafv2types.LoggingConfiguration
	var errs []wafScopeErr
	for _, scope := range wafv2types.LogScope("").Values() {
		out, err := api.GetLoggingConfiguration(ctx, &wafv2svc.GetLoggingConfigurationInput{ResourceArn: aws.String(arn), LogScope: scope})
		if _, none := errors.AsType[*wafv2types.WAFNonexistentItemException](err); none {
			continue
		}
		if err != nil {
			errs = append(errs, wafScopeErr{scope, err})
			continue
		}
		if out.LoggingConfiguration != nil {
			configs = append(configs, *out.LoggingConfiguration)
		}
	}
	return configs, errs
}

// EnrichWAFLogging calls GetLoggingConfiguration, ListResourcesForWebACL, and GetWebACL per WebACL
// (cap EnrichmentCap) and raises findings for:
//   - GetLoggingConfiguration returns WAFNonexistentItemException under every
//     log scope → "~" finding "no logging configuration"
//   - ListResourcesForWebACL returns empty ResourceArns → "~" finding
//     "not associated with any resource" (waf.orphan — a separate condition
//     from the logging one, and separately worded)
//
// Also writes FieldUpdates["rules_summary"] = "<N> rules BLOCK" or "0 rules ALLOW".
// Skip if clients.WAFv2 == nil. Per-WebACL errors (other than WAFNonexistentItemException) are
// aggregated and returned as a composite error alongside partial findings.
func EnrichWAFLogging(ctx context.Context, clients *ServiceClients, resources []resource.Resource, _ resource.ResourceCache) (IssueEnricherResult, error) {
	result := IssueEnricherResult{
		Findings:     make(map[string][]domain.Finding),
		TruncatedIDs: make(map[string]string),
		FieldUpdates: make(map[string]map[string]string),
	}
	if clients.WAFv2 == nil {
		return result, nil
	}
	var failures []Failure
	total := 0
	resources = capAtEnrichmentCap(&result, resources, nil, resourceIDsOf)
	var mu sync.Mutex
	loopErr := ForEachRow(ctx, &result, resourceIDs(resources), EnrichmentParallelism, func(i int) {
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
		// The logging, association and rule reads answer separate questions:
		// one that fails leaves its own row unread and the others evaluated.
		var readErrs []error
		var loggingRows, orphanRows []domain.DetailRow
		scope := cmp.Or(r.Fields["scope"], string(wafv2types.ScopeRegional))
		api := clients.wafIn(scope)

		// One configuration under any scope is logging, whatever the other
		// scopes answered. No configuration while a scope failed is unknown,
		// not off.
		configs, scopeErrs := wafLoggingConfigurations(ctx, api, arn)
		switch {
		case len(configs) > 0:
		case len(scopeErrs) > 0:
			readErrs = append(readErrs, scopeErrs[0].err)
		default:
			loggingRows = append(loggingRows, domain.DetailRow{
				Label: "Logging",
				Value: "off",
				Tier:  "~",
			})
		}

		associations, err := wafAssociations(ctx, clients, scope, arn)
		if err != nil {
			readErrs = append(readErrs, err)
		} else if len(associations) == 0 {
			orphanRows = append(orphanRows, domain.DetailRow{
				Label: "Associations",
				Value: "none",
				Tier:  "~",
			})
		}

		// Compute rules_summary by fetching the full WebACL when the client
		// implements WAFv2GetWebACLAPI.
		rulesSummary := "0 rules"
		unprotected := false
		var gerr error
		if getACLAPI, ok := api.(WAFv2GetWebACLAPI); ok && r.Fields["name"] != "" && r.Fields["id"] != "" {
			var getOut *wafv2svc.GetWebACLOutput
			getOut, gerr = getACLAPI.GetWebACL(ctx, &wafv2svc.GetWebACLInput{
				Name:  aws.String(r.Fields["name"]),
				Id:    aws.String(r.Fields["id"]),
				Scope: wafv2types.Scope(scope),
			})
			if gerr != nil {
				readErrs = append(readErrs, gerr)
			} else if getOut.WebACL != nil {
				blockCount := 0
				for _, rule := range getOut.WebACL.Rules {
					if rule.Action != nil && rule.Action.Block != nil {
						blockCount++
					}
				}
				ruleCount := len(getOut.WebACL.Rules)
				if ruleCount == 0 {
					rulesSummary = "0 rules"
					// WAF applies the default action to every request no rule
					// matches: with no rules, Block stops everything and only
					// Allow leaves the resource unprotected.
					unprotected = getOut.WebACL.DefaultAction != nil && getOut.WebACL.DefaultAction.Allow != nil
				} else {
					rulesSummary = fmt.Sprintf("%d/%d BLOCK", blockCount, ruleCount)
				}
			}
		}

		mu.Lock()
		defer mu.Unlock()
		for _, err := range readErrs {
			MarkSkipped(&result, r.ID, &failures, err)
		}
		if gerr == nil {
			result.FieldUpdates[r.ID] = map[string]string{
				"rules_summary": rulesSummary,
			}
		}

		if unprotected {
			setWave2Finding(&result, r.ID, wafCodeNoRules, []domain.DetailRow{{Label: "Rules", Value: "0", Tier: tierOf(wafCodeNoRules)}})

		}

		if len(loggingRows) > 0 {
			setWave2Finding(&result, r.ID, wafCodeNoLogging, loggingRows)
		}
		if len(orphanRows) > 0 {
			setWave2Finding(&result, r.ID, wafCodeOrphan, orphanRows)
		}
	})

	return result, errors.Join(loopErr, AggregateFailures("web ACL logging and associations", failures, total))
}

// wafRegionalResourceTypes are the resource types a REGIONAL web ACL can
// protect: every wafv2 ResourceType but AMPLIFY, whose applications take a
// CLOUDFRONT-scope ACL.
var wafRegionalResourceTypes = []wafv2types.ResourceType{
	wafv2types.ResourceTypeApplicationLoadBalancer,
	wafv2types.ResourceTypeApiGateway,
	wafv2types.ResourceTypeAppsync,
	wafv2types.ResourceTypeCognitioUserPool,
	wafv2types.ResourceTypeAppRunnerService,
	wafv2types.ResourceTypeVerifiedAccessInstance,
	wafv2types.ResourceTypeAgentcoreGateway,
}

// wafAssociations returns the resources this web ACL protects, from the
// service that reports them for its scope. ListResourcesForWebACL answers
// one resource type per call, so a REGIONAL ACL's answer is the union over
// wafRegionalResourceTypes; a type that fails fails the whole answer.
func wafAssociations(ctx context.Context, clients *ServiceClients, scope, arn string) ([]string, error) {
	if scope == wafScopeCloudFront {
		ids, _, err := wafDistributionIDs(ctx, clients, arn)
		return ids, err
	}
	var all []string
	for _, rt := range wafRegionalResourceTypes {
		arns, err := wafResourcesOfType(ctx, clients.WAFv2, arn, rt)
		if err != nil {
			return nil, err
		}
		all = append(all, arns...)
	}
	return all, nil
}

// wafResourcesOfType returns the resources of one type a REGIONAL web ACL
// protects. ResourceType is always named: omitted, it means
// APPLICATION_LOAD_BALANCER.
func wafResourcesOfType(ctx context.Context, api WAFv2ListResourcesForWebACLAPI, arn string, rt wafv2types.ResourceType) ([]string, error) {
	out, err := api.ListResourcesForWebACL(ctx, &wafv2svc.ListResourcesForWebACLInput{
		WebACLArn:    aws.String(arn),
		ResourceType: rt,
	})
	if err != nil {
		return nil, err
	}
	return out.ResourceArns, nil
}
