// fakes_eventbridge_test.go is the single source for EventBridge
// SDK-interface fakes shared across tests/unit.
//
// Convention (docs/go-codebase-checklist.md, DRY section): one configurable
// fake per SDK interface, in a service-named fakes_<service>_test.go file --
// never re-implement the same interface under a new name in another file,
// and never add another wave/batch-named fake file (fakes_us1_batchN_test.go,
// fakes_related_checkers_branch_coverage_test.go, fakes_related_checkers_misc_test.go are historical
// accretion naming, not a pattern to extend).
package unit_test

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/eventbridge"
	eventbridgetypes "github.com/aws/aws-sdk-go-v2/service/eventbridge/types"
)

// fakeEventBridgeAPI implements awsclient.EventBridgeAPI (ListRules,
// ListTargetsByRule, ListRuleNamesByTarget).
//
// Each method is configured independently:
//   - ListRules: RulesOutput/RulesErr (unconditional)
//   - ListTargetsByRule: Targets/TargetsErr (unconditional)
//   - ListRuleNamesByTarget: RuleNamesByTargetArn (keyed by the request's
//     TargetArn) if set, else the unconditional RuleNames/RuleNamesErr
type fakeEventBridgeAPI struct {
	RulesOutput *eventbridge.ListRulesOutput
	RulesErr    error

	Targets    []eventbridgetypes.Target
	TargetsErr error

	RuleNamesByTargetArn map[string][]string
	RuleNames            []string
	RuleNamesErr         error
}

func (f *fakeEventBridgeAPI) ListRules(
	_ context.Context,
	_ *eventbridge.ListRulesInput,
	_ ...func(*eventbridge.Options),
) (*eventbridge.ListRulesOutput, error) {
	if f.RulesErr != nil {
		return nil, f.RulesErr
	}
	if f.RulesOutput != nil {
		return f.RulesOutput, nil
	}
	return &eventbridge.ListRulesOutput{}, nil
}

func (f *fakeEventBridgeAPI) ListTargetsByRule(
	_ context.Context,
	_ *eventbridge.ListTargetsByRuleInput,
	_ ...func(*eventbridge.Options),
) (*eventbridge.ListTargetsByRuleOutput, error) {
	if f.TargetsErr != nil {
		return nil, f.TargetsErr
	}
	return &eventbridge.ListTargetsByRuleOutput{Targets: f.Targets}, nil
}

func (f *fakeEventBridgeAPI) ListRuleNamesByTarget(
	_ context.Context,
	params *eventbridge.ListRuleNamesByTargetInput,
	_ ...func(*eventbridge.Options),
) (*eventbridge.ListRuleNamesByTargetOutput, error) {
	if f.RuleNamesByTargetArn != nil {
		if params == nil || params.TargetArn == nil {
			return &eventbridge.ListRuleNamesByTargetOutput{}, nil
		}
		return &eventbridge.ListRuleNamesByTargetOutput{RuleNames: f.RuleNamesByTargetArn[*params.TargetArn]}, nil
	}
	if f.RuleNamesErr != nil {
		return nil, f.RuleNamesErr
	}
	return &eventbridge.ListRuleNamesByTargetOutput{RuleNames: f.RuleNames}, nil
}
