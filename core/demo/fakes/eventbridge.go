// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package fakes

import (
	"context"
	"slices"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/eventbridge"
	eventbridgetypes "github.com/aws/aws-sdk-go-v2/service/eventbridge/types"

	"github.com/k2m30/a9s/v3/core/demo/fixtures"
)

// EventBridgeFake implements aws.EventBridgeAPI against fixture data loaded at construction time.
type EventBridgeFake struct {
	fix *fixtures.EventBridgeFixtures
}

// NewEventBridge constructs an EventBridgeFake backed by fixture data from the fixtures package.
func NewEventBridge() *EventBridgeFake {
	return &EventBridgeFake{fix: fixtures.NewEventBridgeFixtures()}
}

func (f *EventBridgeFake) ListRules(_ context.Context, _ *eventbridge.ListRulesInput, _ ...func(*eventbridge.Options)) (*eventbridge.ListRulesOutput, error) {
	return &eventbridge.ListRulesOutput{Rules: f.fix.Rules}, nil
}

func (f *EventBridgeFake) ListTargetsByRule(_ context.Context, input *eventbridge.ListTargetsByRuleInput, _ ...func(*eventbridge.Options)) (*eventbridge.ListTargetsByRuleOutput, error) {
	var ruleName string
	if input != nil && input.Rule != nil {
		ruleName = *input.Rule
	}
	if !f.hasRule(ruleName) {
		return nil, &eventbridgetypes.ResourceNotFoundException{
			Message: notFoundMessage("Rule", ruleName),
		}
	}
	return &eventbridge.ListTargetsByRuleOutput{Targets: f.fix.TargetsByRule[ruleName]}, nil
}

// ListRuleNamesByTarget scans the fixture-registered TargetsByRule map for
// any rule with a target whose Arn matches the requested TargetArn, deriving
// the reverse (target -> rule names) lookup generically rather than
// requiring a second fixture map. Required for the pipeline:eb-rule and
// sfn:eb-rule related-panel pivot witnesses (both call this API directly
// with the pipeline/state-machine ARN as TargetArn).
func (f *EventBridgeFake) ListRuleNamesByTarget(_ context.Context, input *eventbridge.ListRuleNamesByTargetInput, _ ...func(*eventbridge.Options)) (*eventbridge.ListRuleNamesByTargetOutput, error) {
	if input == nil || input.TargetArn == nil || *input.TargetArn == "" {
		return &eventbridge.ListRuleNamesByTargetOutput{}, nil
	}
	targetArn := *input.TargetArn
	var names []string
	for ruleName, targets := range f.fix.TargetsByRule {
		for _, t := range targets {
			if t.Arn != nil && *t.Arn == targetArn {
				names = append(names, ruleName)
				break
			}
		}
	}
	return &eventbridge.ListRuleNamesByTargetOutput{RuleNames: names}, nil
}

// hasRule reports whether the fixtures register this rule. A rule with no
// targets still answers an empty target list — that is the shape the
// no-targets finding is built on.
func (f *EventBridgeFake) hasRule(name string) bool {
	return slices.ContainsFunc(f.fix.Rules, func(r eventbridgetypes.Rule) bool {
		return aws.ToString(r.Name) == name
	})
}
