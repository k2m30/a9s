// sfn_related.go contains Step Functions related-resource checker functions.
package aws

import (
	"context"
	"encoding/json"
	"strings"

	cwtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	"github.com/aws/aws-sdk-go-v2/service/eventbridge"
	"github.com/aws/aws-sdk-go-v2/service/sfn"

	"github.com/k2m30/a9s/v3/internal/resource"
)

// sfnDescribe wraps DescribeStateMachine in RetryOnThrottle. Returns nil on any
// failure (unsupported client, API error, empty output).
func sfnDescribe(ctx context.Context, clients any, stateMachineARN string) *sfn.DescribeStateMachineOutput {
	c, ok := clients.(*ServiceClients)
	if !ok || c == nil || c.SFN == nil {
		return nil
	}
	api, ok := c.SFN.(SFNDescribeStateMachineAPI)
	if !ok {
		return nil
	}
	out, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*sfn.DescribeStateMachineOutput, error) {
		return api.DescribeStateMachine(ctx, &sfn.DescribeStateMachineInput{StateMachineArn: &stateMachineARN})
	})
	if err != nil {
		return nil
	}
	return out
}

// checkSFNLogs searches the logs cache for the vendedlogs log group associated
// with this state machine by naming convention.
// Pattern N — naming convention: /aws/vendedlogs/states/{sfnName}
func checkSFNLogs(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	sfnName := res.ID
	if sfnName == "" {
		return resource.RelatedCheckResult{TargetType: "logs", Count: 0}
	}

	expectedLogGroup := "/aws/vendedlogs/states/" + sfnName

	logList, truncated, err := sfnRelatedResources(ctx, clients, cache, "logs")
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "logs", Count: -1, Err: err}
	}
	if logList == nil {
		return resource.RelatedCheckResult{TargetType: "logs", Count: -1}
	}

	var ids []string
	for _, logRes := range logList {
		if logRes.ID == expectedLogGroup {
			ids = append(ids, logRes.ID)
		}
	}
	if len(ids) == 0 && truncated {
		return resource.ApproximateZero("logs")
	}
	return relatedResult("logs", ids)
}

// checkSFNAlarm searches the alarm cache for alarms with a "StateMachineArn" dimension
// matching this state machine's ARN.
// Pattern D — dimension-based lookup.
func checkSFNAlarm(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	sfnARN := res.Fields["arn"]
	if sfnARN == "" {
		return resource.RelatedCheckResult{TargetType: "alarm", Count: -1}
	}

	alarmList, truncated, err := sfnRelatedResources(ctx, clients, cache, "alarm")
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "alarm", Count: -1, Err: err}
	}
	if alarmList == nil {
		return resource.RelatedCheckResult{TargetType: "alarm", Count: -1}
	}

	var ids []string
	for _, alarmRes := range alarmList {
		alarm, ok := assertStruct[cwtypes.MetricAlarm](alarmRes.RawStruct)
		if !ok {
			continue
		}
		for _, d := range alarm.Dimensions {
			if d.Name != nil && *d.Name == "StateMachineArn" && d.Value != nil && *d.Value == sfnARN {
				ids = append(ids, alarmRes.ID)
				break
			}
		}
	}
	if len(ids) == 0 && truncated {
		return resource.ApproximateZero("alarm")
	}
	return relatedResult("alarm", ids)
}

// checkSFNRole resolves the IAM execution role for this state machine via
// DescribeStateMachine (Pattern C: 1 API call, RoleArn → role name).
func checkSFNRole(ctx context.Context, clients any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	arn := res.Fields["arn"]
	if arn == "" {
		return resource.RelatedCheckResult{TargetType: "role", Count: 0}
	}
	out := sfnDescribe(ctx, clients, arn)
	if out == nil {
		return resource.RelatedCheckResult{TargetType: "role", Count: -1}
	}
	if out.RoleArn == nil || *out.RoleArn == "" {
		return resource.RelatedCheckResult{TargetType: "role", Count: 0}
	}
	return relatedResult("role", []string{arnRoleName(*out.RoleArn)})
}

// checkSFNKMS resolves the state machine's encryption KMS key via DescribeStateMachine
// (Pattern C: 1 API call, EncryptionConfiguration.KmsKeyId → key ID).
func checkSFNKMS(ctx context.Context, clients any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	arn := res.Fields["arn"]
	if arn == "" {
		return resource.RelatedCheckResult{TargetType: "kms", Count: 0}
	}
	out := sfnDescribe(ctx, clients, arn)
	if out == nil {
		return resource.RelatedCheckResult{TargetType: "kms", Count: -1}
	}
	if out.EncryptionConfiguration == nil || out.EncryptionConfiguration.KmsKeyId == nil ||
		*out.EncryptionConfiguration.KmsKeyId == "" {
		return resource.RelatedCheckResult{TargetType: "kms", Count: 0}
	}
	return relatedResult("kms", []string{arnLastSegment(*out.EncryptionConfiguration.KmsKeyId)})
}

// checkSFNLambda parses the state machine's ASL definition JSON (returned by
// DescribeStateMachine) and extracts Lambda function ARNs referenced as Task
// Resource values. Pattern C: 1 API call, offline JSON walk.
func checkSFNLambda(ctx context.Context, clients any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	arn := res.Fields["arn"]
	if arn == "" {
		return resource.RelatedCheckResult{TargetType: "lambda", Count: 0}
	}
	out := sfnDescribe(ctx, clients, arn)
	if out == nil {
		return resource.RelatedCheckResult{TargetType: "lambda", Count: -1}
	}
	if out.Definition == nil || *out.Definition == "" {
		return resource.RelatedCheckResult{TargetType: "lambda", Count: 0}
	}

	seen := map[string]struct{}{}
	sfnCollectLambdaRefs([]byte(*out.Definition), seen)
	names := make([]string, 0, len(seen))
	for n := range seen {
		names = append(names, n)
	}
	return relatedResult("lambda", names)
}

// sfnCollectLambdaRefs walks an ASL definition JSON and records lambda function
// names found in Resource / Parameters.FunctionName fields. Function names are
// extracted from Lambda ARNs (arn:aws:lambda:...:function:NAME[:alias]).
func sfnCollectLambdaRefs(def []byte, seen map[string]struct{}) {
	var raw any
	if err := json.Unmarshal(def, &raw); err != nil {
		return
	}
	var walk func(v any)
	walk = func(v any) {
		switch x := v.(type) {
		case map[string]any:
			for k, val := range x {
				if k == "Resource" {
					if s, ok := val.(string); ok {
						if name := lambdaFuncNameFromARN(s); name != "" {
							seen[name] = struct{}{}
						}
					}
				}
				if k == "FunctionName" {
					if s, ok := val.(string); ok && s != "" {
						seen[lambdaFuncNameFromARN(s)] = struct{}{}
					}
				}
				walk(val)
			}
		case []any:
			for _, item := range x {
				walk(item)
			}
		}
	}
	walk(raw)
}

// lambdaFuncNameFromARN extracts the function name from a Lambda ARN. Returns
// the input if it does not look like a Lambda ARN.
func lambdaFuncNameFromARN(s string) string {
	if !strings.HasPrefix(s, "arn:") {
		return s
	}
	// arn:aws:lambda:REGION:ACCT:function:NAME or arn:aws:states:::lambda:invoke
	parts := strings.Split(s, ":")
	if len(parts) < 6 {
		return ""
	}
	// Only treat as Lambda if "lambda" is the service and the 5th slot is "function"
	if parts[2] == "lambda" && len(parts) >= 7 && parts[5] == "function" {
		return parts[6]
	}
	return ""
}

// sfnRelatedResources returns the resource list for target from cache or by fetching the first page.
func sfnRelatedResources(ctx context.Context, clients any, cache resource.ResourceCache, target string) ([]resource.Resource, bool, error) {
	resources, isTruncated, err := FetchRelatedTarget(ctx, clients, cache, target)
	if err != nil {
		if _, ok := clients.(*ServiceClients); !ok {
			return nil, false, nil
		}
	}
	return resources, isTruncated, err
}

// checkSFNEbRule resolves EventBridge rules that target this state machine.
// Pattern C: one events:ListRuleNamesByTarget call using the state machine ARN
// from res.Fields["arn"]. Count = len(RuleNames).
func checkSFNEbRule(ctx context.Context, clients any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	sfnARN := res.Fields["arn"]
	if sfnARN == "" {
		return resource.RelatedCheckResult{TargetType: "eb-rule", Count: 0}
	}
	c, ok := clients.(*ServiceClients)
	if !ok || c == nil || c.EventBridge == nil {
		return resource.RelatedCheckResult{TargetType: "eb-rule", Count: -1}
	}
	api, ok := c.EventBridge.(EventBridgeListRuleNamesByTargetAPI)
	if !ok {
		return resource.RelatedCheckResult{TargetType: "eb-rule", Count: -1}
	}
	out, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*eventbridge.ListRuleNamesByTargetOutput, error) {
		return api.ListRuleNamesByTarget(ctx, &eventbridge.ListRuleNamesByTargetInput{TargetArn: &sfnARN})
	})
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "eb-rule", Count: -1, Err: err}
	}
	return relatedResult("eb-rule", out.RuleNames)
}
