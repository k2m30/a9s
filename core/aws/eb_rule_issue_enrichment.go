// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// eb_rule_issue_enrichment.go — Wave 2 issue enrichment for the eb-rule resource type.
package aws

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/eventbridge"
	eventbridgetypes "github.com/aws/aws-sdk-go-v2/service/eventbridge/types"

	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// eb-rule canonical FindingCodes.
const (
	ebRuleCodeNoTargets   domain.FindingCode = "eb-rule.no-targets"
	ebRuleCodeTargetIssue domain.FindingCode = "eb-rule.target-issue"
)

// EnrichEventBridgeRuleTargets is a Wave 2 enricher for EventBridge rules.
// Per rule (cap 50) it calls ListTargetsByRule and raises findings for:
//   - Rule state == ENABLED AND len(Targets) == 0 → "!" finding (rule matches but goes nowhere)
//   - Rule state == DISABLED AND len(Targets) > 0 → "~" finding (disabled rule still has targets — drift)
//   - Any target without DeadLetterConfig → "~" finding (no DLQ on target)
func EnrichEventBridgeRuleTargets(ctx context.Context, clients *ServiceClients, resources []resource.Resource, _ resource.ResourceCache) (IssueEnricherResult, error) {
	result := IssueEnricherResult{
		Findings:     make(map[string][]domain.Finding),
		TruncatedIDs: make(map[string]string),
		FieldUpdates: make(map[string]map[string]string),
	}
	if clients.EventBridge == nil || len(resources) == 0 {
		return result, nil
	}

	truncated := false
	resources = capAtEnrichmentCap(&result, resources, nil, resourceIDsOf)
	n := len(resources)
	var failures []Failure
	var mu sync.Mutex

	loopErr := ForEachRow(ctx, &result, resourceIDs(resources), EnrichmentParallelism, func(i int) {
		r := resources[i]

		ruleName := r.Fields["name"]
		if ruleName == "" {
			return
		}

		eventBus := r.Fields["event_bus"]
		state := strings.ToUpper(r.Fields["state"])

		targets, complete, fetchErr := PageAll(ctx, PerParentPageCap, func(ctx context.Context, token *string) ([]eventbridgetypes.Target, *string, error) {
			pageInput := &eventbridge.ListTargetsByRuleInput{
				Rule:      aws.String(ruleName),
				NextToken: token,
			}
			if eventBus != "" {
				pageInput.EventBusName = aws.String(eventBus)
			}
			out, err := clients.EventBridge.ListTargetsByRule(ctx, pageInput)
			if err != nil {
				return nil, nil, err
			}
			return out.Targets, out.NextToken, nil
		})
		targetsTruncated := !complete && fetchErr == nil

		targetCountStr := resource.FormatExact(len(targets))
		if targetsTruncated {
			targetCountStr = resource.FormatTruncated(len(targets))
		}
		var rows []domain.DetailRow

		// A rule is either DISABLED or firing: ENABLED_WITH_ALL_CLOUDTRAIL_MANAGEMENT_EVENTS
		// is a second enabled state, and a rule in it delivers like any other.
		noTargets := fetchErr == nil && state != "DISABLED" && state != "" && len(targets) == 0 && !targetsTruncated

		if state == "DISABLED" && len(targets) > 0 {
			rows = append(rows, domain.DetailRow{
				Label: "Targets",
				Value: fmt.Sprintf("disabled rule still has %d target(s) (drift)", len(targets)),
				Tier:  "~",
			})
		}

		for _, target := range targets {
			if target.DeadLetterConfig == nil {
				targetID := ""
				if target.Id != nil {
					targetID = *target.Id
				}
				rows = append(rows, domain.DetailRow{
					Label: "Target",
					Value: fmt.Sprintf("%s: no dead-letter config", targetID),
					Tier:  "~",
				})
			}
		}

		mu.Lock()
		defer mu.Unlock()

		switch {
		case fetchErr != nil:
			// A refused call counted nothing: the column is blanked, since
			// "0" is the answer of a call that succeeded with no targets.
			targetCountStr = ""
			truncated = true
			MarkSkipped(&result, r.ID, &failures, fetchErr)
		case targetsTruncated:
			truncated = true
			// A page cap, not a failed call, and not a coverage gap on the
			// row: the "+" on target_count is where it is reported. The rows
			// below name targets the walk really did see — a target without a
			// dead-letter config is a fact whatever lies on page 11. Only
			// noTargets, which needs a complete walk, is suppressed, by its
			// own !targetsTruncated guard above.
		}
		result.FieldUpdates[r.ID] = map[string]string{
			"target_count": targetCountStr,
		}

		if noTargets {
			setWave2Finding(&result, r.ID, ebRuleCodeNoTargets, []domain.DetailRow{{Label: "Targets", Value: "none", Tier: tierOf(ebRuleCodeNoTargets)}})

		}

		if len(rows) == 0 {
			return
		}
		setWave2Finding(&result, r.ID, ebRuleCodeTargetIssue, rows)
	})

	SetTruncated(&result, truncated)
	return result, errors.Join(loopErr, AggregateFailures("ListTargetsByRule", failures, n))
}
