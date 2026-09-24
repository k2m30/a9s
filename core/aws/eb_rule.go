// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package aws

import (
	"cmp"
	"context"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/eventbridge"
	eventbridgetypes "github.com/aws/aws-sdk-go-v2/service/eventbridge/types"

	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// FetchEventBridgeRulesPage fetches the account's EventBridge rules: every
// bus of the account, then every rule on each of them. ListRules answers for
// one bus and defaults to the default one, so an account's rules are one list
// only once the buses have been enumerated — and a rule's row is keyed by its
// bus, so a bus nobody listed is a pivot pointing at a row that cannot exist.
//
// The whole list is read here rather than handed back a page at a time: a
// NextToken belongs to one bus's rule list and cannot carry a walk across
// several. An api that cannot enumerate buses is asked for rules without
// naming one, which is the default bus.
func FetchEventBridgeRulesPage(ctx context.Context, api EventBridgeListRulesAPI, _ string) (resource.FetchResult, error) {
	buses, busesComplete, busErr := ebRuleBuses(ctx, api)
	if busErr != nil && len(buses) == 0 {
		return resource.FetchResult{}, busErr
	}

	var resources []resource.Resource
	rules, complete, err := ebRulesOnBuses(ctx, api, buses)
	if err != nil {
		return resource.FetchResult{}, err
	}
	complete = complete && busesComplete && busErr == nil

	for _, rule := range rules {
		name := ""
		if rule.Name != nil {
			name = *rule.Name
		}

		state := string(rule.State)

		description := ""
		if rule.Description != nil {
			description = *rule.Description
		}

		eventBus := ""
		if rule.EventBusName != nil {
			eventBus = *rule.EventBusName
		}

		schedule := ""
		if rule.ScheduleExpression != nil {
			schedule = *rule.ScheduleExpression
		}

		eventPattern := ""
		if rule.EventPattern != nil {
			eventPattern = *rule.EventPattern
		}

		findings := ebRuleStateFindings(state)

		r := resource.Resource{
			ID:   ebRuleID(eventBus, name),
			Name: name,
			Fields: map[string]string{
				"name":          name,
				"state":         state,
				"description":   description,
				"event_bus":     eventBus,
				"schedule":      schedule,
				"event_pattern": eventPattern,
			},
			Findings:  findings,
			RawStruct: rule,
		}

		resources = append(resources, r)
	}

	totalHint := len(resources)
	if !complete {
		totalHint = -1
	}

	return resource.FetchResult{
		Resources: resources,
		Pagination: &resource.PaginationMeta{
			// A walk that stopped at its page cap has no cursor to hand back:
			// what it read is a lower bound on the account's rules, and
			// LowerBoundOnly is what says so without a cursor behind it.
			IsTruncated:    !complete,
			LowerBoundOnly: !complete,
			PageSize:       len(resources),
			TotalHint:      totalHint,
		},
	}, busErr
}

// ebRuleBuses returns the names of the account's event buses. Both ListRules
// and ListRuleNamesByTarget answer for one bus, so every read of the account's
// rules is a walk over these. An api that cannot enumerate them answers for
// the bus those calls default to, named by the empty string; complete is
// false when the walk stopped at its page cap.
func ebRuleBuses(ctx context.Context, api any) ([]string, bool, error) {
	busAPI, ok := api.(EventBridgeListEventBusesAPI)
	if !ok {
		return []string{""}, true, nil
	}
	buses, complete, err := PageAll(ctx, PerParentPageCap, func(ctx context.Context, token *string) ([]eventbridgetypes.EventBus, *string, error) {
		out, err := busAPI.ListEventBuses(ctx, &eventbridge.ListEventBusesInput{
			Limit:     aws.Int32(DefaultPageSize),
			NextToken: token,
		})
		if err != nil {
			return nil, nil, err
		}
		return out.EventBuses, out.NextToken, nil
	})
	if err != nil && len(buses) == 0 {
		return nil, false, fmt.Errorf("listing EventBridge event buses: %w", err)
	}
	var names []string
	for _, bus := range buses {
		if name := aws.ToString(bus.Name); name != "" {
			names = append(names, name)
		}
	}
	if err != nil {
		return names, false, fmt.Errorf("listing EventBridge event buses: %w", err)
	}
	if len(names) == 0 {
		// Every account has a default bus, so an answer naming none of them
		// has answered for nothing; the bus the calls default to is still
		// there to be read.
		return []string{""}, complete, nil
	}
	return names, complete, nil
}

// ebRulesOnBuses returns every rule on each of buses, in bus order. An empty
// bus name asks for the rules ListRules answers with when no bus is named.
func ebRulesOnBuses(ctx context.Context, api EventBridgeListRulesAPI, buses []string) (rules []eventbridgetypes.Rule, complete bool, err error) {
	complete = true
	for _, bus := range buses {
		onBus, busComplete, err := PageAll(ctx, PerParentPageCap, func(ctx context.Context, token *string) ([]eventbridgetypes.Rule, *string, error) {
			input := &eventbridge.ListRulesInput{
				Limit:     aws.Int32(DefaultPageSize),
				NextToken: token,
			}
			if bus != "" {
				input.EventBusName = aws.String(bus)
			}
			out, err := api.ListRules(ctx, input)
			if err != nil {
				return nil, nil, err
			}
			return out.Rules, out.NextToken, nil
		})
		if err != nil {
			return nil, false, fmt.Errorf("fetching EventBridge rules: %w", err)
		}
		rules = append(rules, onBus...)
		complete = complete && busComplete
	}
	return rules, complete, nil
}

// ebRuleID is the row identity of an EventBridge rule: a rule name is unique
// on its event bus only, and an account may carry buses beyond the default
// one. A reference that names no bus belongs to the default bus, which is
// where EventBridge puts a rule created without one.
func ebRuleID(eventBus, name string) string {
	return cmp.Or(eventBus, "default") + "/" + name
}

// ebRuleRefToID reads a rule reference as the rule row's ID: a rule ARN
// ("rule/<name>" on the default bus, "rule/<bus>/<name>" on any other),
// CloudFormation's "<bus>|<name>" physical ID, a bare name, or the ID
// itself.
func ebRuleRefToID(ref string, rc domain.RefContext) (string, bool) {
	res, isARN, ok := localARN(ref, rc, "events")
	if !ok {
		return "", false
	}
	if isARN {
		if res, ok = afterPrefix(res, "rule/"); !ok {
			return "", false
		}
	} else if bus, name, onBus := strings.Cut(res, "|"); onBus {
		return ebRuleID(bus, name), bus != "" && name != ""
	}
	if bus, name, qualified := strings.Cut(res, "/"); qualified {
		return ebRuleID(bus, name), bus != "" && name != ""
	}
	return ebRuleID("", res), res != ""
}

// ebRuleStateFindings is the one predicate for a rule's lifecycle state.
// colorEBRule runs it over Fields for rows built outside the fetcher.
func ebRuleStateFindings(state string) []domain.Finding {
	if strings.EqualFold(state, "DISABLED") {
		return []domain.Finding{wave1Finding(CodeEBRuleDisabled)}
	}
	return nil
}
