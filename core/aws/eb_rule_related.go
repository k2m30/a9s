// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package aws

import (
	"context"

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
		return resource.UnknownRelated("role")
	}
	if rule.RoleArn == nil || *rule.RoleArn == "" {
		return resource.ProvenZero("role", "rule.RoleArn")
	}
	return relatedRefs("role", []string{*rule.RoleArn}, refContext(clients, cache, "role"))
}

// ebRuleTargets calls events:ListTargetsByRule(rule) and returns, as target,
// the targets whose ARN is one of service's (e.g. "states" for sfn).
func ebRuleTargets(ctx context.Context, clients any, cache resource.ResourceCache, ruleName, target, service string) resource.RelatedCheckResult {
	if ruleName == "" {
		return resource.KnownRelated(target, nil, false)
	}
	c, ok := clients.(*ServiceClients)
	if !ok || c == nil || c.EventBridge == nil {
		return resource.UnknownRelated(target)
	}
	targets, complete, err := PageAll(ctx, PerParentPageCap, func(ctx context.Context, token *string) ([]eventbridgetypes.Target, *string, error) {
		out, err := c.EventBridge.ListTargetsByRule(ctx, &eventbridge.ListTargetsByRuleInput{
			Rule:      aws.String(ruleName),
			NextToken: token,
		})
		if err != nil {
			return nil, nil, err
		}
		return out.Targets, out.NextToken, nil
	})
	// no finding: the pivot shows "?" rather than a target count nobody read.
	if err != nil {
		return resource.UnknownRelated(target)
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
	return ebRuleTargets(ctx, clients, cache, res.ID, "kinesis", "kinesis")
}

func checkEbRuleLambda(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	return ebRuleTargets(ctx, clients, cache, res.ID, "lambda", "lambda")
}

func checkEbRuleLogs(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	return ebRuleTargets(ctx, clients, cache, res.ID, "logs", "logs")
}

func checkEbRuleSFN(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	return ebRuleTargets(ctx, clients, cache, res.ID, "sfn", "states")
}

func checkEbRuleSNS(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	return ebRuleTargets(ctx, clients, cache, res.ID, "sns", "sns")
}

func checkEbRuleSQS(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	return ebRuleTargets(ctx, clients, cache, res.ID, "sqs", "sqs")
}

// ebRulesTargeting is the eb-rule pivot of a resource EventBridge rules can
// target: the rules events:ListRuleNamesByTarget names for targetARN.
func ebRulesTargeting(ctx context.Context, clients any, targetARN string) resource.RelatedCheckResult {
	c, ok := clients.(*ServiceClients)
	if !ok || c == nil || c.EventBridge == nil {
		return resource.UnknownRelated("eb-rule")
	}
	api, ok := c.EventBridge.(EventBridgeListRuleNamesByTargetAPI)
	if !ok {
		return resource.UnknownRelated("eb-rule")
	}
	names, complete, err := PageAll(ctx, PerParentPageCap, func(ctx context.Context, token *string) ([]string, *string, error) {
		out, err := api.ListRuleNamesByTarget(ctx, &eventbridge.ListRuleNamesByTargetInput{TargetArn: &targetARN, NextToken: token})
		if err != nil {
			return nil, nil, err
		}
		return out.RuleNames, out.NextToken, nil
	})
	if err != nil {
		return resource.ErrorRelated("eb-rule", err)
	}
	return relatedResultTrunc("eb-rule", names, !complete)
}
