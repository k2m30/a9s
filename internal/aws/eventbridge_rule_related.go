package aws

import (
	"context"
	"strings"

	eventbridgetypes "github.com/aws/aws-sdk-go-v2/service/eventbridge/types"

	"github.com/k2m30/a9s/v3/internal/resource"
)

func init() {
	resource.RegisterRelated("eb-rule", []resource.RelatedDef{
		{TargetType: "role", DisplayName: "IAM Role", Checker: checkEbRuleRole, NeedsTargetCache: false},
		{TargetType: "kinesis", DisplayName: "Kinesis Streams", Checker: checkEbRuleKinesis},
		{TargetType: "lambda", DisplayName: "Lambda Functions", Checker: checkEbRuleLambda},
		{TargetType: "logs", DisplayName: "Log Groups", Checker: checkEbRuleLogs},
		{TargetType: "sfn", DisplayName: "Step Functions", Checker: checkEbRuleSFN},
		{TargetType: "sns", DisplayName: "SNS Topics", Checker: checkEbRuleSNS},
		{TargetType: "sqs", DisplayName: "SQS Queues", Checker: checkEbRuleSQS},
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

func checkEbRuleKinesis(_ context.Context, _ any, _ resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	return resource.RelatedCheckResult{TargetType: "kinesis", Count: 0}
}

func checkEbRuleLambda(_ context.Context, _ any, _ resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	return resource.RelatedCheckResult{TargetType: "lambda", Count: 0}
}

func checkEbRuleLogs(_ context.Context, _ any, _ resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	return resource.RelatedCheckResult{TargetType: "logs", Count: 0}
}

func checkEbRuleSFN(_ context.Context, _ any, _ resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	return resource.RelatedCheckResult{TargetType: "sfn", Count: 0}
}

func checkEbRuleSNS(_ context.Context, _ any, _ resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	return resource.RelatedCheckResult{TargetType: "sns", Count: 0}
}

func checkEbRuleSQS(_ context.Context, _ any, _ resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	return resource.RelatedCheckResult{TargetType: "sqs", Count: 0}
}
