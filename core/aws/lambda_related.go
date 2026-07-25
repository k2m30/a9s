// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// lambda_related.go contains Lambda function related-resource checker functions.
package aws

import (
	"context"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/lambda"
	lambdatypes "github.com/aws/aws-sdk-go-v2/service/lambda/types"

	"github.com/k2m30/a9s/v3/core/resource"
)

// checkLambdaRole extracts the Role ARN from the Lambda FunctionConfiguration RawStruct.
// It extracts the role name from the last path segment of the ARN (after the last "/")
// and searches the role cache by name.
func checkLambdaRole(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	fn, ok := assertStruct[lambdatypes.FunctionConfiguration](res.RawStruct)
	if !ok {
		return resource.KnownRelated("role", nil, false)
	}
	if fn.Role == nil || *fn.Role == "" {
		return resource.KnownRelated("role", nil, false)
	}
	// In-body: the execution Role ARN normalizes to the role name, which IS the
	// role's Resource.ID (roles keyed by name; role FetchByIDs drives the drill).
	// Resolve by identity — no role-list fetch.
	return relatedResult("role", []string{roleNameFromARN(*fn.Role)})
}

// checkLambdaAlarms searches the alarm cache for alarms with a "FunctionName" dimension
// matching this Lambda function's name (res.ID).
func checkLambdaAlarms(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	functionName := res.ID
	if functionName == "" {
		functionName = res.Name
	}
	return alarmIDsByDimension(ctx, clients, cache, "", "FunctionName", functionName)
}

// checkLambdaLogs searches the logs cache for the CloudWatch log group for this function.
// Pattern N — default: /aws/lambda/{function-name}, with custom override via LoggingConfig.
func checkLambdaLogs(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	functionName := res.ID
	if functionName == "" {
		functionName = res.Name
	}
	if functionName == "" {
		return resource.KnownRelated("logs", nil, false)
	}

	// Check for custom log group via LoggingConfig
	expectedLogGroup := "/aws/lambda/" + functionName
	fn, ok := assertStruct[lambdatypes.FunctionConfiguration](res.RawStruct)
	if ok && fn.LoggingConfig != nil && fn.LoggingConfig.LogGroup != nil && *fn.LoggingConfig.LogGroup != "" {
		expectedLogGroup = *fn.LoggingConfig.LogGroup
	}

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

// checkLambdaSG extracts security group IDs from the Lambda FunctionConfiguration's
// VpcConfig.SecurityGroupIds (only present for VPC-attached functions).
// Pattern F — no cache needed.
func checkLambdaSG(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	fn, ok := assertStruct[lambdatypes.FunctionConfiguration](res.RawStruct)
	if !ok {
		return resource.UnknownRelated("sg")
	}
	if fn.VpcConfig == nil {
		return resource.KnownRelated("sg", nil, false)
	}
	var ids []string
	for _, sgID := range fn.VpcConfig.SecurityGroupIds {
		if sgID != "" {
			ids = append(ids, sgID)
		}
	}
	return relatedResult("sg", ids)
}

// checkLambdaVPC returns the VPC this Lambda function runs in (Pattern R).
// Reads FunctionConfiguration.VpcConfig.VpcId from the RawStruct.
// Returns Count: 0 for functions not attached to a VPC.
func checkLambdaVPC(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	fn, ok := assertStruct[lambdatypes.FunctionConfiguration](res.RawStruct)
	if !ok {
		return resource.UnknownRelated("vpc")
	}
	if fn.VpcConfig == nil || fn.VpcConfig.VpcId == nil || *fn.VpcConfig.VpcId == "" {
		return resource.KnownRelated("vpc", nil, false)
	}
	return relatedResult("vpc", []string{*fn.VpcConfig.VpcId})
}

// checkLambdaKMS extracts the KMS key ARN from the Lambda FunctionConfiguration
// KMSKeyArn field (used for environment variable encryption). Pattern F — no
// cache needed. The ARN last segment after "/" is used as the key ID.
func checkLambdaKMS(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	fn, ok := assertStruct[lambdatypes.FunctionConfiguration](res.RawStruct)
	if !ok || fn.KMSKeyArn == nil || *fn.KMSKeyArn == "" {
		return resource.KnownRelated("kms", nil, false)
	}
	keyID := kmsKeyIDFromField(*fn.KMSKeyArn, res.Type)
	return relatedResult("kms", []string{keyID})
}

// checkLambdaSQS finds SQS queues wired to this Lambda as event sources
// (Pattern A — live API). Calls lambda:ListEventSourceMappings scoped to the
// function and extracts SQS queue names from the returned EventSourceArn values.
// Returns an unknown result when no live clients are available, since the
// Lambda FunctionConfiguration struct does not embed event source mappings.
func checkLambdaSQS(ctx context.Context, clients any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	functionName := res.ID
	if functionName == "" {
		functionName = res.Name
	}
	if functionName == "" {
		return resource.KnownRelated("sqs", nil, false)
	}
	c, ok := clients.(*ServiceClients)
	if !ok || c == nil || c.Lambda == nil {
		return resource.UnknownRelated("sqs")
	}
	out, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*lambda.ListEventSourceMappingsOutput, error) {
		return c.Lambda.ListEventSourceMappings(ctx, &lambda.ListEventSourceMappingsInput{
			FunctionName: &functionName,
		})
	})
	if err != nil {
		return resource.ErrorRelated("sqs", err)
	}
	var ids []string
	for _, m := range out.EventSourceMappings {
		if m.EventSourceArn == nil {
			continue
		}
		arn := *m.EventSourceArn
		if !strings.Contains(arn, ":sqs:") {
			continue
		}
		parts := strings.Split(arn, ":")
		name := parts[len(parts)-1]
		if name != "" {
			ids = append(ids, name)
		}
	}
	return relatedResult("sqs", ids)
}

// checkLambdaCFN finds the CloudFormation stack that owns this Lambda by reading
// the function's tags (Pattern A — live API). FunctionConfiguration does NOT
// embed tags, so this calls lambda:ListTags on the function ARN and then matches
// the aws:cloudformation:stack-name tag against the cfn cache.
// Returns an unknown result when neither clients nor a usable ARN are available.
func checkLambdaCFN(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	fn, ok := assertStruct[lambdatypes.FunctionConfiguration](res.RawStruct)
	if !ok {
		return resource.UnknownRelated("cfn")
	}
	if fn.FunctionArn == nil || *fn.FunctionArn == "" {
		return resource.KnownRelated("cfn", nil, false)
	}
	c, sok := clients.(*ServiceClients)
	if !sok || c == nil || c.Lambda == nil {
		return resource.UnknownRelated("cfn")
	}
	tagsOut, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*lambda.ListTagsOutput, error) {
		return c.Lambda.ListTags(ctx, &lambda.ListTagsInput{Resource: fn.FunctionArn})
	})
	if err != nil {
		return resource.ErrorRelated("cfn", err)
	}
	stackName := tagsOut.Tags["aws:cloudformation:stack-name"]
	if stackName == "" {
		return resource.KnownRelated("cfn", nil, false)
	}
	cfnList, truncated, err := relatedResourcesFor(ctx, clients, cache, "cfn")
	if err != nil {
		return resource.ErrorRelated("cfn", err)
	}
	if cfnList == nil {
		return resource.UnknownRelated("cfn")
	}
	var ids []string
	for _, cfnRes := range cfnList {
		if cfnRes.ID == stackName || cfnRes.Name == stackName || cfnRes.Fields["stack_name"] == stackName {
			ids = append(ids, cfnRes.ID)
		}
	}
	return relatedResultTrunc("cfn", ids, truncated)
}

// checkLambdaECR resolves the ECR repository for container-image Lambda
// functions. Short-circuits unless Fields["package_type"]=="Image" (set by
// the lambda fetcher). The image URI is only returned by GetFunction's
// Code.ImageUri — never by ListFunctions/FunctionConfiguration — so this
// checker calls GetFunction for this one function (in budget: one call per
// open Image-package function, per docs/resources/lambda.md). ECR image URIs
// follow the pattern <account>.dkr.ecr.<region>.amazonaws.com/<repo>[:<tag>|@<digest>].
func checkLambdaECR(ctx context.Context, clients any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	if res.Fields["package_type"] != "Image" {
		return resource.KnownRelated("ecr", nil, false)
	}
	fnName := res.ID
	if fnName == "" {
		fnName = res.Name
	}
	if fnName == "" {
		return resource.UnknownRelated("ecr")
	}
	c, ok := clients.(*ServiceClients)
	if !ok || c == nil || c.Lambda == nil {
		return resource.UnknownRelated("ecr")
	}
	out, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*lambda.GetFunctionOutput, error) {
		return c.Lambda.GetFunction(ctx, &lambda.GetFunctionInput{FunctionName: &fnName})
	})
	if err != nil {
		return resource.ErrorRelated("ecr", err)
	}
	if out == nil || out.Code == nil || out.Code.ImageUri == nil || *out.Code.ImageUri == "" {
		return resource.UnknownRelated("ecr")
	}
	imageURI := *out.Code.ImageUri
	// URI form: <account>.dkr.ecr.<region>.amazonaws.com/<repo>[:<tag>|@<digest>]
	// We need the <repo> portion — everything after the hostname "/" and before ":" or "@".
	slashIdx := strings.Index(imageURI, "/")
	if slashIdx < 0 || slashIdx == len(imageURI)-1 {
		return resource.UnknownRelated("ecr")
	}
	repoAndTag := imageURI[slashIdx+1:]
	// Strip tag/digest suffix.
	if idx := strings.Index(repoAndTag, ":"); idx >= 0 {
		repoAndTag = repoAndTag[:idx]
	}
	if idx := strings.Index(repoAndTag, "@"); idx >= 0 {
		repoAndTag = repoAndTag[:idx]
	}
	if repoAndTag == "" {
		return resource.UnknownRelated("ecr")
	}
	return relatedResult("ecr", []string{repoAndTag})
}

// checkLambdaEBRule finds EventBridge rules that target this Lambda
// (Pattern A — live API). There is no field on FunctionConfiguration that
// enumerates incoming rules, and scanning the eb-rule cache alone is
// insufficient because Rule structs do not include targets — each would require
// a separate events:ListTargetsByRule call. We iterate the cached rules and
// look for Lambda ARN targets when live clients are available.
func checkLambdaEBRule(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	fn, ok := assertStruct[lambdatypes.FunctionConfiguration](res.RawStruct)
	if !ok {
		return resource.UnknownRelated("eb-rule")
	}
	functionARN := ""
	if fn.FunctionArn != nil {
		functionARN = *fn.FunctionArn
	}
	functionName := ""
	if fn.FunctionName != nil {
		functionName = *fn.FunctionName
	}
	if functionName == "" {
		functionName = res.ID
	}
	if functionARN == "" && functionName == "" {
		return resource.KnownRelated("eb-rule", nil, false)
	}
	c, sok := clients.(*ServiceClients)
	if !sok || c == nil || c.EventBridge == nil {
		// Without live EventBridge access there is no cached field on the rule
		// struct that links to Lambda targets — targets come from a separate API.
		return resource.UnknownRelated("eb-rule")
	}
	ruleList, truncated, err := relatedResourcesFor(ctx, clients, cache, "eb-rule")
	if err != nil {
		return resource.ErrorRelated("eb-rule", err)
	}
	if ruleList == nil {
		return resource.UnknownRelated("eb-rule")
	}
	idSet := make(map[string]struct{})
	var failures []string
	for _, ruleRes := range ruleList {
		parentCtx := map[string]string{
			"rule_name": ruleRes.ID,
			"event_bus": ruleRes.Fields["event_bus"],
		}
		targets, err := FetchEventBridgeRuleTargets(ctx, c.EventBridge, parentCtx, "")
		if err != nil {
			failures = append(failures, fmt.Sprintf("%s: %v", ruleRes.ID, err))
			continue
		}
		for _, tgt := range targets.Resources {
			arn := tgt.Fields["target_arn"]
			if arn == "" {
				continue
			}
			if (functionARN != "" && arn == functionARN) ||
				(functionName != "" && strings.HasSuffix(arn, ":function:"+functionName)) {
				idSet[ruleRes.ID] = struct{}{}
				break
			}
		}
	}
	ids := make([]string, 0, len(idSet))
	for id := range idSet {
		ids = append(ids, id)
	}
	if aggErr := AggregateFailures("lambda-related: ListTargetsByRule", failures, len(ruleList)); aggErr != nil {
		if len(ids) == 0 {
			// Nothing was confirmed: the failures establish nothing about the
			// population size, only that the attempt failed.
			return resource.ErrorRelated("eb-rule", aggErr)
		}
		// Some ListTargetsByRule calls failed: ids is a proven subset, not the
		// exhaustive answer. Truncated (not Errored) keeps the row actionable —
		// "at least N, could not verify the rest" — rather than discarding the
		// confirmed matches as a dead end.
		return resource.KnownRelated("eb-rule", ids, true)
	}
	return relatedResultTrunc("eb-rule", ids, truncated)
}

// lambdaRelatedResources returns the resource list for target from cache or by
// fetching the first page via the registered paginated fetcher.
func lambdaRelatedResources(ctx context.Context, clients any, cache resource.ResourceCache, target string) ([]resource.Resource, bool, error) {
	return relatedResourcesFor(ctx, clients, cache, target)
}
