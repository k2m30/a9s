// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// sfn_related.go contains Step Functions related-resource checker functions.
package aws

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/sfn"

	"github.com/k2m30/a9s/v3/core/resource"
)

// sfnDescribe wraps DescribeStateMachine in RetryOnThrottle. Returns (nil, nil)
// when the client is unsupported or absent (no API call attempted — the caller
// renders an UnknownRelated result without a FlashMsg). Returns (nil, err) on API failure so
// callers can surface the underlying error via Result.Err → FlashMsg → error log.
func sfnDescribe(ctx context.Context, clients any, stateMachineARN string) (*sfn.DescribeStateMachineOutput, error) {
	c, ok := clients.(*ServiceClients)
	if !ok || c == nil || c.SFN == nil {
		return nil, nil
	}
	api, ok := c.SFN.(SFNDescribeStateMachineAPI)
	if !ok {
		return nil, nil
	}
	return RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*sfn.DescribeStateMachineOutput, error) {
		return api.DescribeStateMachine(ctx, &sfn.DescribeStateMachineInput{StateMachineArn: &stateMachineARN})
	})
}

// checkSFNLogs searches the logs cache for the vendedlogs log group associated
// with this state machine by naming convention.
// Pattern N — naming convention: /aws/vendedlogs/states/{sfnName}
func checkSFNLogs(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	sfnName := res.ID
	if sfnName == "" {
		return resource.ProvenZero("logs", "sfnName")
	}

	expectedLogGroup := "/aws/vendedlogs/states/" + sfnName

	logList, truncated, err := relatedResourcesFor(ctx, clients, cache, "logs")
	if err != nil {
		return resource.ErrorRelated("logs", err)
	}
	if logList == nil {
		return resource.UnknownRelated("logs")
	}

	var ids []string
	for _, logRes := range logList {
		if logRes.ID == expectedLogGroup {
			ids = append(ids, logRes.ID)
		}
	}
	return relatedResultTrunc("logs", ids, truncated)
}

func checkSFNAlarm(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	return alarmIDsByDimension(ctx, clients, cache, "sfn", res)
}

// checkSFNRole resolves the IAM execution role for this state machine via
// DescribeStateMachine (Pattern C: 1 API call, RoleArn → role name).
func checkSFNRole(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	arn := res.Fields["arn"]
	if arn == "" {
		return resource.ProvenZero("role", "arn")
	}
	out, err := sfnDescribe(ctx, clients, arn)
	if err != nil {
		return resource.ErrorRelated("role", err)
	}
	if out == nil {
		return resource.UnknownRelated("role")
	}
	if out.RoleArn == nil || *out.RoleArn == "" {
		return resource.ProvenZero("role", "out.RoleArn")
	}
	return relatedRefs("role", []string{*out.RoleArn}, refContext(clients, cache, "role"))
}

// checkSFNKMS resolves the state machine's encryption KMS key via DescribeStateMachine
// (Pattern C: 1 API call, EncryptionConfiguration.KmsKeyId → key ID).
func checkSFNKMS(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	arn := res.Fields["arn"]
	if arn == "" {
		return resource.ProvenZero("kms", "arn")
	}
	out, err := sfnDescribe(ctx, clients, arn)
	if err != nil {
		return resource.ErrorRelated("kms", err)
	}
	if out == nil {
		return resource.UnknownRelated("kms")
	}
	if out.EncryptionConfiguration == nil || out.EncryptionConfiguration.KmsKeyId == nil ||
		*out.EncryptionConfiguration.KmsKeyId == "" {
		return resource.ProvenZero("kms", "out")
	}
	return kmsRelated(ctx, clients, cache, []string{*out.EncryptionConfiguration.KmsKeyId})
}

// checkSFNLambda parses the state machine's ASL definition JSON (returned by
// DescribeStateMachine) and extracts Lambda function ARNs referenced as Task
// Resource values. Pattern C: 1 API call, offline JSON walk.
func checkSFNLambda(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	arn := res.Fields["arn"]
	if arn == "" {
		return resource.ProvenZero("lambda", "arn")
	}
	out, err := sfnDescribe(ctx, clients, arn)
	if err != nil {
		return resource.ErrorRelated("lambda", err)
	}
	if out == nil {
		return resource.UnknownRelated("lambda")
	}
	if out.Definition == nil || *out.Definition == "" {
		return resource.ProvenZero("lambda", "out.Definition")
	}

	var refs []string
	sfnCollectLambdaRefs([]byte(*out.Definition), &refs)
	return relatedRefs("lambda", refs, refContext(clients, cache, "lambda"))
}

// sfnCollectLambdaRefs walks an ASL definition JSON and appends to refs the
// Lambda function references found in Resource (a function ARN; a service
// integration such as arn:aws:states:::lambda:invoke names no function) and
// Parameters.FunctionName fields.
func sfnCollectLambdaRefs(def []byte, refs *[]string) {
	var raw any
	if err := json.Unmarshal(def, &raw); err != nil {
		return
	}
	var walk func(v any)
	walk = func(v any) {
		switch x := v.(type) {
		case map[string]any:
			for k, val := range x {
				if s, ok := val.(string); ok && (k == "FunctionName" || k == "Resource" && strings.Contains(s, ":function:")) {
					*refs = append(*refs, s)
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

// checkSFNEbRule resolves EventBridge rules that target this state machine.
// Pattern C: one events:ListRuleNamesByTarget call using the state machine ARN
// from res.Fields["arn"]. Count = len(RuleNames).
func checkSFNEbRule(ctx context.Context, clients any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	sfnARN := res.Fields["arn"]
	if sfnARN == "" {
		return resource.ProvenZero("eb-rule", "sfnARN")
	}
	return ebRulesTargeting(ctx, clients, sfnARN)
}
