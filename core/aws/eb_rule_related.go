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
		return resource.KnownRelated("role", nil, false)
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
	out, err := c.EventBridge.ListTargetsByRule(ctx, &eventbridge.ListTargetsByRuleInput{
		Rule: aws.String(ruleName),
	})
	// no finding: the pivot shows "?" rather than a target count nobody read.
	if err != nil || out == nil {
		return resource.UnknownRelated(target)
	}
	var arns []string
	for _, t := range out.Targets {
		if t.Arn == nil {
			continue
		}
		if _, ok := ARNForService(*t.Arn, service); ok {
			arns = append(arns, *t.Arn)
		}
	}
	return relatedRefs(target, arns, refContext(clients, cache, target))
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
