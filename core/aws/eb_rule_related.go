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
// the targets whose ARN is one of service's (e.g. "states" for sfn), and the
// dead-letter queues the targets name: DeadLetterConfig.Arn is "the ARN of
// the SQS queue specified as the target for the dead-letter queue"
// (https://docs.aws.amazon.com/eventbridge/latest/APIReference/API_DeadLetterConfig.html).
// A rule name is unique on its own bus, so the bus goes with it.
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
	if err != nil {
		return ReadFailed(target, err)
	}
	var arns []string
	for _, t := range targets {
		named := []string{aws.ToString(t.Arn)}
		if t.DeadLetterConfig != nil {
			named = append(named, aws.ToString(t.DeadLetterConfig.Arn))
		}
		for _, a := range named {
			if _, ok := ARNForService(a, service); ok {
				arns = append(arns, a)
			}
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

// ebRulesWithDLQ reads the rules of the eb-rule list one of whose targets
// names queueARN as its dead-letter queue, with one ListTargetsByRule per rule.
func ebRulesWithDLQ(ctx context.Context, clients any, cache resource.ResourceCache, queueARN string) relatedRead {
	rules, truncated, err := FetchRelatedTarget(ctx, clients, cache, "eb-rule")
	if rules == nil {
		return unreadBy(err)
	}
	c, ok := clients.(*ServiceClients)
	if !ok || c == nil || c.EventBridge == nil {
		return relatedRead{unread: true}
	}
	rules, capped := fanOut(rules)
	read := relatedRead{partial: truncated || capped}
	var failures []Failure
	for _, rule := range rules {
		targets, complete, err := ebListTargets(ctx, c.EventBridge, rule.Fields["event_bus"], rule.Fields["name"])
		if err != nil {
			failures = append(failures, FailedCall(rule.ID, err))
			continue
		}
		read.partial = read.partial || !complete
		if slices.ContainsFunc(targets, func(t eventbridgetypes.Target) bool {
			return t.DeadLetterConfig != nil && aws.ToString(t.DeadLetterConfig.Arn) == queueARN
		}) {
			read.ids = append(read.ids, rule.ID)
		}
	}
	read.partial = read.partial || len(failures) > 0
	read.unread = len(rules) > 0 && len(failures) == len(rules)
	read.failure = AggregateFailures("eb-rule: ListTargetsByRule", failures, len(rules))
	return read
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
