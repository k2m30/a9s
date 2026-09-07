// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package aws

import (
	"context"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/eventbridge"
	eventbridgetypes "github.com/aws/aws-sdk-go-v2/service/eventbridge/types"

	"github.com/k2m30/a9s/v3/core/resource"
)

// checkEbRuleRole reads RoleArn from the Rule RawStruct and extracts the role name.
// Pattern F — no cache needed.
func checkEbRuleRole(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	rule, ok := assertStruct[eventbridgetypes.Rule](res.RawStruct)
	if !ok {
		return resource.UnknownRelated("role")
	}
	if rule.RoleArn == nil || *rule.RoleArn == "" {
		return resource.KnownRelated("role", nil, false)
	}
	arn := *rule.RoleArn
	idx := strings.LastIndex(arn, "/")
	if idx < 0 || idx == len(arn)-1 {
		return resource.KnownRelated("role", nil, false)
	}
	roleName := arn[idx+1:]
	return relatedResult("role", []string{roleName})
}

// ebRuleTargetsByService calls events:ListTargetsByRule(rule) and returns the
// target resource IDs whose ARN carries the given service prefix (e.g.
// "kinesis"). Name extraction:
//   - kinesis: after ":stream/"
//   - lambda:  after ":function:"
//   - logs:    after ":log-group:"  (trim trailing ":*")
//   - states:  after ":stateMachine:" (SFN)
//   - sns:     after last ":"
//   - sqs:     after last ":"
func ebRuleTargetsByService(ctx context.Context, clients any, ruleName string, service string) ([]string, bool) {
	if ruleName == "" {
		return nil, true // genuinely empty → Count: 0, not -1
	}
	c, ok := clients.(*ServiceClients)
	if !ok || c == nil || c.EventBridge == nil {
		return nil, false
	}
	out, err := c.EventBridge.ListTargetsByRule(ctx, &eventbridge.ListTargetsByRuleInput{
		Rule: aws.String(ruleName),
	})
	if err != nil || out == nil {
		return nil, false
	}
	prefix := "arn:" + PartitionForRegion(sessionRegion(c)) + ":" + service + ":"
	var ids []string
	for _, t := range out.Targets {
		if t.Arn == nil || !strings.HasPrefix(*t.Arn, prefix) {
			continue
		}
		arn := *t.Arn
		name := ""
		switch service {
		case "kinesis":
			if _, after, ok := strings.Cut(arn, ":stream/"); ok {
				name = after
			}
		case "lambda":
			if _, after, ok := strings.Cut(arn, ":function:"); ok {
				if before, _, hasSep := strings.Cut(after, ":"); hasSep {
					name = before // strip :version
				} else {
					name = after
				}
			}
		case "logs":
			if _, after, ok := strings.Cut(arn, ":log-group:"); ok {
				name = strings.TrimSuffix(after, ":*")
			}
		case "states":
			if _, after, ok := strings.Cut(arn, ":stateMachine:"); ok {
				name = after
			}
		case "sns", "sqs":
			if i := strings.LastIndex(arn, ":"); i >= 0 {
				name = arn[i+1:]
			}
		}
		if name != "" {
			ids = append(ids, name)
		}
	}
	return ids, true
}

func checkEbRuleKinesis(ctx context.Context, clients any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	ids, ok := ebRuleTargetsByService(ctx, clients, res.ID, "kinesis")
	if !ok {
		return resource.UnknownRelated("kinesis")
	}
	return relatedResult("kinesis", ids)
}

func checkEbRuleLambda(ctx context.Context, clients any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	ids, ok := ebRuleTargetsByService(ctx, clients, res.ID, "lambda")
	if !ok {
		return resource.UnknownRelated("lambda")
	}
	return relatedResult("lambda", ids)
}

func checkEbRuleLogs(ctx context.Context, clients any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	ids, ok := ebRuleTargetsByService(ctx, clients, res.ID, "logs")
	if !ok {
		return resource.UnknownRelated("logs")
	}
	return relatedResult("logs", ids)
}

func checkEbRuleSFN(ctx context.Context, clients any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	ids, ok := ebRuleTargetsByService(ctx, clients, res.ID, "states")
	if !ok {
		return resource.UnknownRelated("sfn")
	}
	return relatedResult("sfn", ids)
}

func checkEbRuleSNS(ctx context.Context, clients any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	ids, ok := ebRuleTargetsByService(ctx, clients, res.ID, "sns")
	if !ok {
		return resource.UnknownRelated("sns")
	}
	return relatedResult("sns", ids)
}

func checkEbRuleSQS(ctx context.Context, clients any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	ids, ok := ebRuleTargetsByService(ctx, clients, res.ID, "sqs")
	if !ok {
		return resource.UnknownRelated("sqs")
	}
	return relatedResult("sqs", ids)
}
