package aws

import (
	"context"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/eventbridge"
	eventbridgetypes "github.com/aws/aws-sdk-go-v2/service/eventbridge/types"

	"github.com/k2m30/a9s/v3/internal/resource"
)

func init() {
	resource.RegisterRelated("eb-rule", []resource.RelatedDef{
		{TargetType: "role", DisplayName: "IAM Role", Checker: checkEbRuleRole, NeedsTargetCache: false},
		{TargetType: "kinesis", DisplayName: "Kinesis (targets)", Checker: checkEbRuleKinesis},
		{TargetType: "lambda", DisplayName: "Lambda (targets)", Checker: checkEbRuleLambda},
		{TargetType: "logs", DisplayName: "Log Groups (targets)", Checker: checkEbRuleLogs},
		{TargetType: "sfn", DisplayName: "Step Functions (targets)", Checker: checkEbRuleSFN},
		{TargetType: "sns", DisplayName: "SNS (targets)", Checker: checkEbRuleSNS},
		{TargetType: "sqs", DisplayName: "SQS (targets)", Checker: checkEbRuleSQS},
	})

	// eventbridgetypes.Rule: RoleArn (execution role for the rule target)
	resource.RegisterNavigableFields("eb-rule", []resource.NavigableField{
		{FieldPath: "RoleArn", TargetType: "role"},
	})
}

// checkEbRuleRole reads RoleArn from the Rule RawStruct and extracts the role name.
// Pattern F — no cache needed.
func checkEbRuleRole(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	rule, ok := assertStruct[eventbridgetypes.Rule](res.RawStruct)
	if !ok {
		return resource.RelatedCheckResult{TargetType: "role", Count: -1}
	}
	if rule.RoleArn == nil || *rule.RoleArn == "" {
		return resource.RelatedCheckResult{TargetType: "role", Count: 0}
	}
	arn := *rule.RoleArn
	idx := strings.LastIndex(arn, "/")
	if idx < 0 || idx == len(arn)-1 {
		return resource.RelatedCheckResult{TargetType: "role", Count: 0}
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
	prefix := "arn:aws:" + service + ":"
	var ids []string
	for _, t := range out.Targets {
		if t.Arn == nil || !strings.HasPrefix(*t.Arn, prefix) {
			continue
		}
		arn := *t.Arn
		name := ""
		switch service {
		case "kinesis":
			if idx := strings.Index(arn, ":stream/"); idx >= 0 {
				name = arn[idx+len(":stream/"):]
			}
		case "lambda":
			if idx := strings.Index(arn, ":function:"); idx >= 0 {
				name = arn[idx+len(":function:"):]
				if colon := strings.Index(name, ":"); colon >= 0 {
					name = name[:colon] // strip :version
				}
			}
		case "logs":
			if idx := strings.Index(arn, ":log-group:"); idx >= 0 {
				name = strings.TrimSuffix(arn[idx+len(":log-group:"):], ":*")
			}
		case "states":
			if idx := strings.Index(arn, ":stateMachine:"); idx >= 0 {
				name = arn[idx+len(":stateMachine:"):]
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
		return resource.RelatedCheckResult{TargetType: "kinesis", Count: -1}
	}
	return relatedResult("kinesis", ids)
}

func checkEbRuleLambda(ctx context.Context, clients any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	ids, ok := ebRuleTargetsByService(ctx, clients, res.ID, "lambda")
	if !ok {
		return resource.RelatedCheckResult{TargetType: "lambda", Count: -1}
	}
	return relatedResult("lambda", ids)
}

func checkEbRuleLogs(ctx context.Context, clients any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	ids, ok := ebRuleTargetsByService(ctx, clients, res.ID, "logs")
	if !ok {
		return resource.RelatedCheckResult{TargetType: "logs", Count: -1}
	}
	return relatedResult("logs", ids)
}

func checkEbRuleSFN(ctx context.Context, clients any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	ids, ok := ebRuleTargetsByService(ctx, clients, res.ID, "states")
	if !ok {
		return resource.RelatedCheckResult{TargetType: "sfn", Count: -1}
	}
	return relatedResult("sfn", ids)
}

func checkEbRuleSNS(ctx context.Context, clients any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	ids, ok := ebRuleTargetsByService(ctx, clients, res.ID, "sns")
	if !ok {
		return resource.RelatedCheckResult{TargetType: "sns", Count: -1}
	}
	return relatedResult("sns", ids)
}

func checkEbRuleSQS(ctx context.Context, clients any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	ids, ok := ebRuleTargetsByService(ctx, clients, res.ID, "sqs")
	if !ok {
		return resource.RelatedCheckResult{TargetType: "sqs", Count: -1}
	}
	return relatedResult("sqs", ids)
}






