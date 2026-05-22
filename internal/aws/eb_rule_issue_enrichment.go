// eb_rule_issue_enrichment.go — Wave 2 issue enrichment for the eb-rule resource type.
package aws

import (
	"context"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/eventbridge"
	eventbridgetypes "github.com/aws/aws-sdk-go-v2/service/eventbridge/types"

	"github.com/k2m30/a9s/v3/internal/resource"
)

// EnrichEventBridgeRuleTargets is a Wave 2 enricher for EventBridge rules.
// Per rule (cap 50) it calls ListTargetsByRule and raises findings for:
//   - Rule state == ENABLED AND len(Targets) == 0 → "!" finding (rule matches but goes nowhere)
//   - Rule state == DISABLED AND len(Targets) > 0 → "~" finding (disabled rule still has targets — drift)
//   - Any target without DeadLetterConfig → "~" finding (no DLQ on target)
func EnrichEventBridgeRuleTargets(ctx context.Context, clients *ServiceClients, resources []resource.Resource, _ resource.ResourceCache) (IssueEnricherResult, error) {
	findings := make(map[string]resource.EnrichmentFinding)
	fieldUpdates := make(map[string]map[string]string)
	truncatedIDs := make(map[string]bool)
	if clients.EventBridge == nil || len(resources) == 0 {
		return IssueEnricherResult{Findings: findings, TruncatedIDs: truncatedIDs}, nil
	}

	truncated := len(resources) > EnrichmentCap
	checked := 0

	for _, r := range resources {
		if checked >= EnrichmentCap {
			truncated = true
			break
		}

		ruleName := r.Fields["name"]
		if ruleName == "" {
			ruleName = r.ID
		}
		if ruleName == "" {
			continue
		}

		eventBus := r.Fields["event_bus"]
		state := strings.ToUpper(r.Fields["state"])

		var targets []eventbridgetypes.Target
		targetsTruncated := false
		var targetsNextToken *string
		targetPages := 0
		for {
			if targetPages >= PerParentPageCap {
				targetsTruncated = true
				truncated = true
				truncatedIDs[r.ID] = true
				break
			}
			pageInput := &eventbridge.ListTargetsByRuleInput{
				Rule:      aws.String(ruleName),
				NextToken: targetsNextToken,
			}
			if eventBus != "" {
				pageInput.EventBusName = aws.String(eventBus)
			}
			out, err := clients.EventBridge.ListTargetsByRule(ctx, pageInput)
			targetPages++
			if err != nil {
				truncated = true
				truncatedIDs[r.ID] = true
				break
			}
			targets = append(targets, out.Targets...)
			if out.NextToken == nil {
				break
			}
			targetsNextToken = out.NextToken
		}
		checked++

		targetCountStr := resource.FormatExact(len(targets))
		if targetsTruncated {
			targetCountStr = resource.FormatApproximate(len(targets))
		}
		fieldUpdates[ruleName] = map[string]string{
			"target_count": targetCountStr,
		}
		var rows []resource.FindingRow

		// ENABLED rule with no targets → rule fires but goes nowhere.
		if state == "ENABLED" && len(targets) == 0 && !targetsTruncated {
			rows = append(rows, resource.FindingRow{
				Label: "Targets",
				Value: "enabled rule has no targets (rule matches but goes nowhere)",
				Tier:  "!",
			})
		}

		// DISABLED rule still has targets → probable drift/oversight.
		if state == "DISABLED" && len(targets) > 0 {
			rows = append(rows, resource.FindingRow{
				Label: "Targets",
				Value: fmt.Sprintf("disabled rule still has %d target(s) (drift)", len(targets)),
				Tier:  "~",
			})
		}

		// Targets without DeadLetterConfig → missing DLQ.
		for _, target := range targets {
			if target.DeadLetterConfig == nil {
				targetID := ""
				if target.Id != nil {
					targetID = *target.Id
				}
				rows = append(rows, resource.FindingRow{
					Label: "Target",
					Value: fmt.Sprintf("%s: no dead-letter config", targetID),
					Tier:  "~",
				})
			}
		}

		if len(rows) == 0 {
			continue
		}

		// Determine severity: "!" if any row is "!", otherwise "~".
		severity := "~"
		for _, row := range rows {
			if row.Tier == "!" {
				severity = "!"
				break
			}
		}

		findings[ruleName] = resource.EnrichmentFinding{
			Severity: severity,
			Summary:  rows[0].Value,
			Rows:     rows,
		}
	}

	issueCount := 0
	for _, f := range findings {
		if f.Severity == "!" {
			issueCount++
		}
	}

	return IssueEnricherResult{IssueCount: issueCount, Truncated: truncated, TruncatedIDs: truncatedIDs, Findings: findings, FieldUpdates: fieldUpdates}, nil
}
