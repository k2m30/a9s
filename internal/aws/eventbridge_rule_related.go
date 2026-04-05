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
	})
}

// checkEbRuleRole reads RoleArn from the Rule RawStruct and extracts the role name.
// Pattern F — no cache needed.
func checkEbRuleRole(_ context.Context, _ interface{}, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
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
