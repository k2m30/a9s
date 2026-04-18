package fakes

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/eventbridge"

	"github.com/k2m30/a9s/v3/internal/demo/fixtures"
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
	return &eventbridge.ListTargetsByRuleOutput{Targets: f.fix.TargetsByRule[ruleName]}, nil
}

// ListRuleNamesByTarget is a no-op stub satisfying EventBridgeListRuleNamesByTargetAPI.
// Demo mode does not model EventBridge rules-by-target lookups.
func (f *EventBridgeFake) ListRuleNamesByTarget(_ context.Context, _ *eventbridge.ListRuleNamesByTargetInput, _ ...func(*eventbridge.Options)) (*eventbridge.ListRuleNamesByTargetOutput, error) {
	return &eventbridge.ListRuleNamesByTargetOutput{}, nil
}
