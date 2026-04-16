// lambda_related.go contains Lambda function related-resource checker functions.
package aws

import (
	"context"
	"strings"

	cwtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
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




















