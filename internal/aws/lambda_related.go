// lambda_related.go contains Lambda function related-resource checker functions.
package aws

import (
	"context"
	"strings"

	cwtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
	lambdatypes "github.com/aws/aws-sdk-go-v2/service/lambda/types"

	"github.com/k2m30/a9s/v3/internal/resource"
)

// checkLambdaRole extracts the Role ARN from the Lambda FunctionConfiguration RawStruct.
// It extracts the role name from the last path segment of the ARN (after the last "/")
// and searches the role cache by name.
func checkLambdaRole(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	fn, ok := assertStruct[lambdatypes.FunctionConfiguration](res.RawStruct)
	if !ok {
		return resource.RelatedCheckResult{TargetType: "role", Count: -1}
	}
	if fn.Role == nil || *fn.Role == "" {
		return resource.RelatedCheckResult{TargetType: "role", Count: 0}
	}
	roleARN := *fn.Role
	roleName := roleARN
	if idx := strings.LastIndex(roleARN, "/"); idx >= 0 && idx < len(roleARN)-1 {
		roleName = roleARN[idx+1:]
	}

	roleList, _, err := lambdaRelatedResources(ctx, clients, cache, "role")
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "role", Count: -1, Err: err}
	}
	if roleList == nil {
		return resource.RelatedCheckResult{TargetType: "role", Count: -1}
	}

	var ids []string
	for _, roleRes := range roleList {
		if roleRes.Name == roleName || roleRes.Fields["role_name"] == roleName {
			ids = append(ids, roleRes.ID)
		}
	}
	return relatedResult("role", ids)
}

// checkLambdaAlarms searches the alarm cache for alarms with a "FunctionName" dimension
// matching this Lambda function's name (res.ID).
func checkLambdaAlarms(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	functionName := res.ID
	if functionName == "" {
		functionName = res.Name
	}
	if functionName == "" {
		return resource.RelatedCheckResult{TargetType: "alarm", Count: 0}
	}

	alarmList, truncated, err := lambdaRelatedResources(ctx, clients, cache, "alarm")
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "alarm", Count: -1, Err: err}
	}
	if alarmList == nil {
		return resource.RelatedCheckResult{TargetType: "alarm", Count: -1}
	}

	var ids []string
	for _, alarmRes := range alarmList {
		rawAlarm, ok := assertStruct[cwtypes.MetricAlarm](alarmRes.RawStruct)
		if !ok {
			continue
		}
		for _, d := range rawAlarm.Dimensions {
			if d.Name != nil && *d.Name == "FunctionName" && d.Value != nil && *d.Value == functionName {
				ids = append(ids, alarmRes.ID)
				break
			}
		}
	}
	if len(ids) == 0 && truncated {
		return resource.RelatedCheckResult{TargetType: "alarm", Count: -1}
	}
	return relatedResult("alarm", ids)
}



// checkLambdaLogs searches the logs cache for the CloudWatch log group for this function.
// Pattern N — default: /aws/lambda/{function-name}, with custom override via LoggingConfig.
func checkLambdaLogs(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	functionName := res.ID
	if functionName == "" {
		functionName = res.Name
	}
	if functionName == "" {
		return resource.RelatedCheckResult{TargetType: "logs", Count: 0}
	}

	// Check for custom log group via LoggingConfig
	expectedLogGroup := "/aws/lambda/" + functionName
	fn, ok := assertStruct[lambdatypes.FunctionConfiguration](res.RawStruct)
	if ok && fn.LoggingConfig != nil && fn.LoggingConfig.LogGroup != nil && *fn.LoggingConfig.LogGroup != "" {
		expectedLogGroup = *fn.LoggingConfig.LogGroup
	}

	logList, truncated, err := lambdaRelatedResources(ctx, clients, cache, "logs")
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
		return resource.RelatedCheckResult{TargetType: "logs", Count: -1}
	}
	return relatedResult("logs", ids)
}


// checkLambdaSG extracts security group IDs from the Lambda FunctionConfiguration's
// VpcConfig.SecurityGroupIds (only present for VPC-attached functions).
// Pattern F — no cache needed.
func checkLambdaSG(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	fn, ok := assertStruct[lambdatypes.FunctionConfiguration](res.RawStruct)
	if !ok {
		return resource.RelatedCheckResult{TargetType: "sg", Count: -1}
	}
	if fn.VpcConfig == nil {
		return resource.RelatedCheckResult{TargetType: "sg", Count: 0}
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
		return resource.RelatedCheckResult{TargetType: "vpc", Count: -1}
	}
	if fn.VpcConfig == nil || fn.VpcConfig.VpcId == nil || *fn.VpcConfig.VpcId == "" {
		return resource.RelatedCheckResult{TargetType: "vpc", Count: 0}
	}
	return relatedResult("vpc", []string{*fn.VpcConfig.VpcId})
}

// checkLambdaKMS extracts the KMS key ARN from the Lambda FunctionConfiguration
// KMSKeyArn field (used for environment variable encryption). Pattern F — no
// cache needed. The ARN last segment after "/" is used as the key ID.
func checkLambdaKMS(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	fn, ok := assertStruct[lambdatypes.FunctionConfiguration](res.RawStruct)
	if !ok || fn.KMSKeyArn == nil || *fn.KMSKeyArn == "" {
		return resource.RelatedCheckResult{TargetType: "kms", Count: 0}
	}
	keyID := *fn.KMSKeyArn
	if idx := strings.LastIndex(keyID, "/"); idx >= 0 && idx < len(keyID)-1 {
		keyID = keyID[idx+1:]
	}
	return relatedResult("kms", []string{keyID})
}

// lambdaRelatedResources returns the resource list for target from cache or by fetching the first page.
func lambdaRelatedResources(ctx context.Context, clients any, cache resource.ResourceCache, target string) ([]resource.Resource, bool, error) {
	resources, isTruncated, err := FetchRelatedTarget(ctx, clients, cache, target)
	if err != nil {
		if _, ok := clients.(*ServiceClients); !ok {
			return nil, false, nil
		}
	}
	return resources, isTruncated, err
}

// checkLambdaSQS finds SQS queues wired to this Lambda as event sources
// (Pattern A — live API). Calls lambda:ListEventSourceMappings scoped to the
// function and extracts SQS queue names from the returned EventSourceArn values.
// Returns Count: -1 when no live clients are available, since the Lambda
// FunctionConfiguration struct does not embed event source mappings.
func checkLambdaSQS(ctx context.Context, clients any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	functionName := res.ID
	if functionName == "" {
		functionName = res.Name
	}
	if functionName == "" {
		return resource.RelatedCheckResult{TargetType: "sqs", Count: 0}
	}
	c, ok := clients.(*ServiceClients)
	if !ok || c == nil || c.Lambda == nil {
		return resource.RelatedCheckResult{TargetType: "sqs", Count: -1}
	}
	out, err := c.Lambda.ListEventSourceMappings(ctx, &lambda.ListEventSourceMappingsInput{
		FunctionName: &functionName,
	})
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "sqs", Count: -1, Err: err}
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
// Returns Count: -1 when neither clients nor a usable ARN are available.
func checkLambdaCFN(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	fn, ok := assertStruct[lambdatypes.FunctionConfiguration](res.RawStruct)
	if !ok {
		return resource.RelatedCheckResult{TargetType: "cfn", Count: -1}
	}
	if fn.FunctionArn == nil || *fn.FunctionArn == "" {
		return resource.RelatedCheckResult{TargetType: "cfn", Count: 0}
	}
	c, sok := clients.(*ServiceClients)
	if !sok || c == nil || c.Lambda == nil {
		return resource.RelatedCheckResult{TargetType: "cfn", Count: -1}
	}
	tagsOut, err := c.Lambda.ListTags(ctx, &lambda.ListTagsInput{Resource: fn.FunctionArn})
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "cfn", Count: -1, Err: err}
	}
	stackName := tagsOut.Tags["aws:cloudformation:stack-name"]
	if stackName == "" {
		return resource.RelatedCheckResult{TargetType: "cfn", Count: 0}
	}
	cfnList, truncated, err := lambdaRelatedResources(ctx, clients, cache, "cfn")
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "cfn", Count: -1, Err: err}
	}
	if cfnList == nil {
		return resource.RelatedCheckResult{TargetType: "cfn", Count: -1}
	}
	var ids []string
	for _, cfnRes := range cfnList {
		if cfnRes.ID == stackName || cfnRes.Name == stackName || cfnRes.Fields["stack_name"] == stackName {
			ids = append(ids, cfnRes.ID)
		}
	}
	if len(ids) == 0 && truncated {
		return resource.RelatedCheckResult{TargetType: "cfn", Count: -1}
	}
	return relatedResult("cfn", ids)
}

// checkLambdaECR resolves the ECR repository for container-image Lambda functions.
// Pattern F — no AWS call needed. Reads PackageType from FunctionConfiguration
// and the image URI from res.Fields["image_uri"]. ECR image URIs follow the
// pattern <account>.dkr.ecr.<region>.amazonaws.com/<repo>[:<tag>|@<digest>].
// Returns Count: 0 if PackageType != Image, Count: -1 if the URI is missing or
// the repository name cannot be parsed.
func checkLambdaECR(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	fn, ok := assertStruct[lambdatypes.FunctionConfiguration](res.RawStruct)
	if !ok {
		return resource.RelatedCheckResult{TargetType: "ecr", Count: -1}
	}
	if fn.PackageType != "Image" {
		return resource.RelatedCheckResult{TargetType: "ecr", Count: 0}
	}
	// Image URI is only available via lambda:GetFunction (not ListFunctions).
	// If the fetcher has stored it in Fields["image_uri"] use it; otherwise
	// we cannot determine the repository without an extra API call.
	imageURI := res.Fields["image_uri"]
	if imageURI == "" {
		return resource.RelatedCheckResult{TargetType: "ecr", Count: -1}
	}
	// URI form: <account>.dkr.ecr.<region>.amazonaws.com/<repo>[:<tag>|@<digest>]
	// We need the <repo> portion — everything after the hostname "/" and before ":" or "@".
	slashIdx := strings.Index(imageURI, "/")
	if slashIdx < 0 || slashIdx == len(imageURI)-1 {
		return resource.RelatedCheckResult{TargetType: "ecr", Count: -1}
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
		return resource.RelatedCheckResult{TargetType: "ecr", Count: -1}
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
		return resource.RelatedCheckResult{TargetType: "eb-rule", Count: -1}
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
		return resource.RelatedCheckResult{TargetType: "eb-rule", Count: 0}
	}
	c, sok := clients.(*ServiceClients)
	if !sok || c == nil || c.EventBridge == nil {
		// Without live EventBridge access there is no cached field on the rule
		// struct that links to Lambda targets — targets come from a separate API.
		return resource.RelatedCheckResult{TargetType: "eb-rule", Count: -1}
	}
	ruleList, truncated, err := lambdaRelatedResources(ctx, clients, cache, "eb-rule")
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "eb-rule", Count: -1, Err: err}
	}
	if ruleList == nil {
		return resource.RelatedCheckResult{TargetType: "eb-rule", Count: -1}
	}
	idSet := make(map[string]struct{})
	for _, ruleRes := range ruleList {
		parentCtx := map[string]string{
			"rule_name": ruleRes.ID,
			"event_bus": ruleRes.Fields["event_bus"],
		}
		targets, err := FetchEventBridgeRuleTargets(ctx, c.EventBridge, parentCtx, "")
		if err != nil {
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
	if len(ids) == 0 && truncated {
		return resource.RelatedCheckResult{TargetType: "eb-rule", Count: -1}
	}
	return relatedResult("eb-rule", ids)
}




















