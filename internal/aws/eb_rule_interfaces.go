package aws

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/eventbridge"
)

// EventBridgeListRulesAPI defines the interface for the EventBridge ListRules operation.
type EventBridgeListRulesAPI interface {
	ListRules(ctx context.Context, params *eventbridge.ListRulesInput, optFns ...func(*eventbridge.Options)) (*eventbridge.ListRulesOutput, error)
}

// EventBridgeListTargetsByRuleAPI defines the interface for the EventBridge ListTargetsByRule operation.
type EventBridgeListTargetsByRuleAPI interface {
	ListTargetsByRule(ctx context.Context, params *eventbridge.ListTargetsByRuleInput, optFns ...func(*eventbridge.Options)) (*eventbridge.ListTargetsByRuleOutput, error)
}

// EventBridgeListRuleNamesByTargetAPI defines the interface for the EventBridge ListRuleNamesByTarget operation.
type EventBridgeListRuleNamesByTargetAPI interface {
	ListRuleNamesByTarget(ctx context.Context, params *eventbridge.ListRuleNamesByTargetInput, optFns ...func(*eventbridge.Options)) (*eventbridge.ListRuleNamesByTargetOutput, error)
}

// EventBridgeAPI is the aggregate interface covering all EventBridge operations used by a9s fetchers.
// *eventbridge.Client structurally satisfies this interface.
type EventBridgeAPI interface {
	EventBridgeListRulesAPI
	EventBridgeListTargetsByRuleAPI
	EventBridgeListRuleNamesByTargetAPI
}
