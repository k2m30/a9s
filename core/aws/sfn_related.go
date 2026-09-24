// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// sfn_related.go contains Step Functions related-resource checker functions.
package aws

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sfn"
	sfntypes "github.com/aws/aws-sdk-go-v2/service/sfn/types"

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

// checkSFNLogs resolves the log group a state machine's LoggingConfiguration
// sends its execution history to; at level OFF it sends none. A destination
// names its group by an ARN ending in ":*"
// (https://docs.aws.amazon.com/step-functions/latest/apireference/API_LoggingConfiguration.html,
// https://docs.aws.amazon.com/step-functions/latest/apireference/API_CloudWatchLogsLogGroup.html).
func checkSFNLogs(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	arn := res.Fields["arn"]
	if arn == "" {
		return keyMissing("logs", "arn")
	}
	out, err := sfnDescribe(ctx, clients, arn)
	if err != nil {
		return ReadFailed("logs", err)
	}
	if out == nil {
		return NotRead("logs")
	}
	lc := out.LoggingConfiguration
	if lc == nil || lc.Level == sfntypes.LogLevelOff {
		return foundNone("logs", "out.LoggingConfiguration.Level")
	}
	var refs []string
	for _, d := range lc.Destinations {
		if g := d.CloudWatchLogsLogGroup; g != nil {
			refs = append(refs, aws.ToString(g.LogGroupArn))
		}
	}
	return relatedRefs("logs", refs, refContext(clients, cache, "logs"))
}

func checkSFNAlarm(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	return alarmIDsByDimension(ctx, clients, cache, "sfn", res)
}

// checkSFNRole resolves the IAM execution role for this state machine via
// DescribeStateMachine (Pattern C: 1 API call, RoleArn → role name).
func checkSFNRole(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	arn := res.Fields["arn"]
	if arn == "" {
		return keyMissing("role", "arn")
	}
	out, err := sfnDescribe(ctx, clients, arn)
	if err != nil {
		return ReadFailed("role", err)
	}
	if out == nil {
		return NotRead("role")
	}
	if out.RoleArn == nil || *out.RoleArn == "" {
		return foundNone("role", "out.RoleArn")
	}
	return relatedRefs("role", []string{*out.RoleArn}, refContext(clients, cache, "role"))
}

// checkSFNKMS resolves the state machine's encryption KMS key via DescribeStateMachine
// (Pattern C: 1 API call, EncryptionConfiguration.KmsKeyId → key ID).
func checkSFNKMS(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	arn := res.Fields["arn"]
	if arn == "" {
		return keyMissing("kms", "arn")
	}
	out, err := sfnDescribe(ctx, clients, arn)
	if err != nil {
		return ReadFailed("kms", err)
	}
	if out == nil {
		return NotRead("kms")
	}
	if out.EncryptionConfiguration == nil || out.EncryptionConfiguration.KmsKeyId == nil ||
		*out.EncryptionConfiguration.KmsKeyId == "" {
		return foundNone("kms", "out")
	}
	return kmsRelated(ctx, clients, cache, []string{*out.EncryptionConfiguration.KmsKeyId})
}

// checkSFNLambda parses the state machine's ASL definition JSON (returned by
// DescribeStateMachine) and extracts Lambda function ARNs referenced as Task
// Resource values. Pattern C: 1 API call, offline JSON walk.
func checkSFNLambda(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	arn := res.Fields["arn"]
	if arn == "" {
		return keyMissing("lambda", "arn")
	}
	out, err := sfnDescribe(ctx, clients, arn)
	if err != nil {
		return ReadFailed("lambda", err)
	}
	if out == nil {
		return NotRead("lambda")
	}
	if out.Definition == nil || *out.Definition == "" {
		return foundNone("lambda", "out.Definition")
	}

	var refs []string
	sfnCollectLambdaRefs([]byte(*out.Definition), &refs)
	return listedRelated(ctx, clients, cache, "lambda", refs, false)
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
func checkSFNEbRule(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	return ebRulesTargeting(ctx, clients, cache, res.Fields["arn"])
}
