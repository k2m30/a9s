// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package aws

import (
	"cmp"
	"context"
	"slices"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/eventbridge"
	eventbridgetypes "github.com/aws/aws-sdk-go-v2/service/eventbridge/types"

	"github.com/k2m30/a9s/v3/core/resource"
)

// checkEbRuleRole reads RoleArn from the Rule RawStruct.
// Pattern F — no cache needed.
func checkEbRuleRole(_ context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	rule, ok := assertStruct[eventbridgetypes.Rule](res.RawStruct)
	if !ok {
		return NotRead("role")
	}
	if rule.RoleArn == nil || *rule.RoleArn == "" {
		return foundNone("role", "rule.RoleArn")
	}
	return relatedRefs("role", []string{*rule.RoleArn}, refContext(clients, cache, "role"))
}

// ebRuleTargets calls events:ListTargetsByRule(rule) and returns, as target,
// the targets whose ARN is one of service's (e.g. "states" for sfn). A rule
// name is unique on its own bus, so the bus goes with it.
func ebRuleTargets(ctx context.Context, clients any, cache resource.ResourceCache, res resource.Resource, target, service string) resource.RelatedCheckResult {
	ruleName := res.Fields["name"]
	if ruleName == "" {
		return NotRead(target)
	}
	c, ok := clients.(*ServiceClients)
	if !ok || c == nil || c.EventBridge == nil {
		return NotRead(target)
	}
	targets, complete, err := ebListTargets(ctx, c.EventBridge, res.Fields["event_bus"], ruleName)
	// no finding: the pivot shows "?" rather than a target count nobody read.
	if err != nil {
		return NotRead(target)
	}
	var arns []string
	for _, t := range targets {
		if t.Arn == nil {
			continue
		}
		if _, ok := ARNForService(*t.Arn, service); ok {
			arns = append(arns, *t.Arn)
		}
	}
	ids, dropped := resolveRefs(target, arns, refContext(clients, cache, target))
	return relatedResultTrunc(target, ids, dropped || !complete)
}

func checkEbRuleKinesis(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	return ebRuleTargets(ctx, clients, cache, res, "kinesis", "kinesis")
}

func checkEbRuleLambda(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	return ebRuleTargets(ctx, clients, cache, res, "lambda", "lambda")
}

func checkEbRuleLogs(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	return ebRuleTargets(ctx, clients, cache, res, "logs", "logs")
}

func checkEbRuleSFN(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	return ebRuleTargets(ctx, clients, cache, res, "sfn", "states")
}

func checkEbRuleSNS(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	return ebRuleTargets(ctx, clients, cache, res, "sns", "sns")
}

func checkEbRuleSQS(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	return ebRuleTargets(ctx, clients, cache, res, "sqs", "sqs")
}

// ebListTargets is events:ListTargetsByRule for the rule name on bus ("" for
// the default bus).
func ebListTargets(ctx context.Context, api EventBridgeListTargetsByRuleAPI, bus, name string) ([]eventbridgetypes.Target, bool, error) {
	return PageAll(ctx, PerParentPageCap, func(ctx context.Context, token *string) ([]eventbridgetypes.Target, *string, error) {
		in := &eventbridge.ListTargetsByRuleInput{Rule: aws.String(name), NextToken: token}
		if bus != "" {
			in.EventBusName = aws.String(bus)
		}
		out, err := api.ListTargetsByRule(ctx, in)
		if err != nil {
			return nil, nil, err
		}
		return out.Targets, out.NextToken, nil
	})
}

// ebRulesTargeting is the eb-rule pivot of a resource EventBridge rules can
// target: the rules events:ListRuleNamesByTarget names for targetARN.
func ebRulesTargeting(ctx context.Context, clients any, _ resource.ResourceCache, targetARN string) resource.RelatedCheckResult {
	read, err := ebTargetRead(ctx, clients, targetARN, nil)
	if err != nil {
		return ReadFailed("eb-rule", err)
	}
	return relatedAnswer("eb-rule", read)
}

// ebTargetRead reads the rules events:ListRuleNamesByTarget names for
// targetARN. The call answers for one bus and hands back bare names, and a
// rule row is keyed by the bus it sits on, so it is asked once per bus of the
// account and each name it returns belongs to the bus it was asked about.
// keep, when set, decides from a named rule's targets whether it counts: a
// cluster is the target of every task a rule runs on it. Without the target's
// ARN nothing can be asked.
func ebTargetRead(ctx context.Context, clients any, targetARN string, keep func([]eventbridgetypes.Target) bool) (relatedRead, error) {
	if targetARN == "" {
		return relatedRead{unread: true}, nil
	}
	c, ok := clients.(*ServiceClients)
	if !ok || c == nil || c.EventBridge == nil {
		return relatedRead{}, errClientMissing
	}
	buses, complete, err := ebRuleBuses(ctx, c.EventBridge)
	if err != nil {
		return relatedRead{}, err
	}
	var ids []string
	var failures []Failure
	asked := 0
	for _, bus := range buses {
		asked++
		names, busComplete, err := PageAll(ctx, PerParentPageCap, func(ctx context.Context, token *string) ([]string, *string, error) {
			in := &eventbridge.ListRuleNamesByTargetInput{TargetArn: &targetARN, NextToken: token}
			if bus != "" {
				in.EventBusName = aws.String(bus)
			}
			out, err := c.EventBridge.ListRuleNamesByTarget(ctx, in)
			if err != nil {
				return nil, nil, err
			}
			return out.RuleNames, out.NextToken, nil
		})
		if err != nil {
			failures = append(failures, FailedCall(cmp.Or(bus, "default"), err))
		}
		complete = complete && busComplete && err == nil
		for _, name := range names {
			if keep != nil {
				asked++
				targets, targetsComplete, err := ebListTargets(ctx, c.EventBridge, bus, name)
				if err != nil {
					failures = append(failures, FailedCall(ebRuleID(bus, name), err))
				}
				complete = complete && targetsComplete && err == nil
				if !keep(targets) {
					continue
				}
			}
			if id := ebRuleID(bus, name); !slices.Contains(ids, id) {
				ids = append(ids, id)
			}
		}
	}
	return relatedRead{
		ids:     ids,
		partial: !complete,
		unread:  len(failures) == asked && asked > 0,
		failure: AggregateFailures("eb-rule: ListRuleNamesByTarget", failures, asked),
	}, nil
}
