// secrets_related_extra.go contains additional Secrets Manager related-resource
// checker functions (T048–T053 from the 019-related-panel-checkers spec).
package aws

import (
	"context"
	"encoding/json"
	"strings"

	ecspkg "github.com/aws/aws-sdk-go-v2/service/ecs"
	ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"
	"github.com/aws/aws-sdk-go-v2/service/elasticbeanstalk"
	ebtypes "github.com/aws/aws-sdk-go-v2/service/elasticbeanstalk/types"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
	smtypes "github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	secretstypes "github.com/aws/aws-sdk-go-v2/service/secretsmanager/types"

	"github.com/k2m30/a9s/v3/internal/resource"
)

// checkSecretsCodeArtifact checks whether this secret is linked to CodeArtifact.
// Weak pair (3-sometimes/2-no). Heuristic: name/tag match for CodeArtifact linkage.
// No AWS call — inspects secret Name and Tags only.
func checkSecretsCodeArtifact(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	secret, ok := assertStruct[secretstypes.SecretListEntry](res.RawStruct)
	if !ok {
		return resource.RelatedCheckResult{TargetType: "codeartifact", Count: -1}
	}
	// Check name contains "codeartifact" (case-insensitive)
	name := ""
	if secret.Name != nil {
		name = strings.ToLower(*secret.Name)
	}
	if strings.Contains(name, "codeartifact") {
		ids := []string{}
		if secret.Name != nil {
			ids = append(ids, *secret.Name)
		}
		return relatedResult("codeartifact", ids)
	}
	// Check tags for codeartifact key or CodeArtifact ARN value
	for _, tag := range secret.Tags {
		key := ""
		if tag.Key != nil {
			key = strings.ToLower(*tag.Key)
		}
		val := ""
		if tag.Value != nil {
			val = *tag.Value
		}
		if strings.Contains(key, "codeartifact") {
			match := val
			if match == "" && tag.Key != nil {
				match = *tag.Key
			}
			return relatedResult("codeartifact", []string{match})
		}
		// Value contains a CodeArtifact ARN pattern
		if strings.Contains(val, "codeartifact") || strings.Contains(val, ":codeartifact:") {
			return relatedResult("codeartifact", []string{val})
		}
	}
	return resource.RelatedCheckResult{TargetType: "codeartifact", Count: 0}
}

// checkSecretsEB is a reverse-scan checker for the secrets→eb relationship.
// Iterates cache["eb"]; for each EB environment, calls
// elasticbeanstalk:DescribeConfigurationSettings and scans OptionSettings[].Value
// for {{resolve:secretsmanager:<parent ARN> pattern.
// NeedsTargetCache: true; sets Approximate and FetchFilter.
func checkSecretsEB(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	// Validate source RawStruct — must be a SecretListEntry.
	if res.RawStruct == nil {
		return resource.RelatedCheckResult{TargetType: "eb", Count: 0}
	}
	if _, ok := assertStruct[secretstypes.SecretListEntry](res.RawStruct); !ok {
		return resource.RelatedCheckResult{TargetType: "eb", Count: -1}
	}

	secretARN, _ := secretIdentifiers(res)
	if secretARN == "" {
		return resource.RelatedCheckResult{TargetType: "eb", Count: 0}
	}

	entry, ok := cache["eb"]
	if !ok {
		return resource.RelatedCheckResult{TargetType: "eb"}
	}

	c, cok := clients.(*ServiceClients)
	if !cok || c == nil {
		return resource.RelatedCheckResult{TargetType: "eb", Count: -1}
	}

	resolveRef := "{{resolve:secretsmanager:" + secretARN
	var ids []string
	for _, ebRes := range entry.Resources {
		eb, ok := assertStruct[ebtypes.EnvironmentDescription](ebRes.RawStruct)
		if !ok {
			continue
		}
		appName := ""
		if eb.ApplicationName != nil {
			appName = *eb.ApplicationName
		}
		envName := ""
		if eb.EnvironmentName != nil {
			envName = *eb.EnvironmentName
		}
		if appName == "" || envName == "" {
			continue
		}
		cfgOut, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*elasticbeanstalk.DescribeConfigurationSettingsOutput, error) {
			return c.ElasticBeanstalk.DescribeConfigurationSettings(ctx, &elasticbeanstalk.DescribeConfigurationSettingsInput{
				ApplicationName: &appName,
				EnvironmentName: &envName,
			})
		})
		if err != nil {
			continue
		}
		for _, cfg := range cfgOut.ConfigurationSettings {
			for _, opt := range cfg.OptionSettings {
				if opt.Value == nil {
					continue
				}
				if strings.Contains(*opt.Value, resolveRef) {
					ids = append(ids, ebRes.ID)
					goto nextEB
				}
			}
		}
	nextEB:
	}

	result := relatedResult("eb", ids)
	result.Approximate = entry.IsTruncated
	return result
}

// checkSecretsECSTask is a reverse-scan checker for the secrets→ecs-task relationship.
// Iterates cache["ecs-task"]; for each task, calls ecs:DescribeTaskDefinition and checks
// ContainerDefinitions[].Secrets[].ValueFrom == parent ARN or
// RepositoryCredentials.CredentialsParameter == parent ARN.
// NeedsTargetCache: true; sets Approximate.
func checkSecretsECSTask(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	// Validate source RawStruct — must be a SecretListEntry.
	if res.RawStruct == nil {
		return resource.RelatedCheckResult{TargetType: "ecs-task", Count: 0}
	}
	if _, ok := assertStruct[secretstypes.SecretListEntry](res.RawStruct); !ok {
		return resource.RelatedCheckResult{TargetType: "ecs-task", Count: -1}
	}

	secretARN, _ := secretIdentifiers(res)
	if secretARN == "" {
		return resource.RelatedCheckResult{TargetType: "ecs-task", Count: 0}
	}

	entry, ok := cache["ecs-task"]
	if !ok {
		return resource.RelatedCheckResult{TargetType: "ecs-task"}
	}

	c, cok := clients.(*ServiceClients)
	if !cok || c == nil {
		return resource.RelatedCheckResult{TargetType: "ecs-task", Count: -1}
	}

	ecsAPI, ok := c.ECS.(ECSDescribeTaskDefinitionAPI)
	if !ok {
		return resource.RelatedCheckResult{TargetType: "ecs-task", Count: -1}
	}

	var ids []string
	for _, taskRes := range entry.Resources {
		// Cache stores ecstypes.Task — extract TaskDefinitionArn
		task, ok := assertStruct[ecstypes.Task](taskRes.RawStruct)
		if !ok {
			continue
		}
		taskDefARN := ""
		if task.TaskDefinitionArn != nil {
			taskDefARN = *task.TaskDefinitionArn
		}
		if taskDefARN == "" {
			taskDefARN = taskRes.Fields["task_definition"]
		}
		if taskDefARN == "" {
			continue
		}
		// Fetch the task definition to inspect container secrets
		tdOut, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*ecspkg.DescribeTaskDefinitionOutput, error) {
			return ecsAPI.DescribeTaskDefinition(ctx, &ecspkg.DescribeTaskDefinitionInput{
				TaskDefinition: &taskDefARN,
			})
		})
		if err != nil || tdOut == nil || tdOut.TaskDefinition == nil {
			continue
		}
		if secretsECSTaskRefsSecret(*tdOut.TaskDefinition, secretARN) {
			ids = append(ids, taskRes.ID)
		}
	}

	result := relatedResult("ecs-task", ids)
	result.Approximate = entry.IsTruncated
	return result
}

// secretsECSTaskRefsSecret returns true if the TaskDefinition references the given
// secret ARN via ContainerDefinitions[].Secrets[].ValueFrom or
// ContainerDefinitions[].RepositoryCredentials.CredentialsParameter.
func secretsECSTaskRefsSecret(td ecstypes.TaskDefinition, secretARN string) bool {
	for _, c := range td.ContainerDefinitions {
		for _, s := range c.Secrets {
			if s.ValueFrom != nil && *s.ValueFrom == secretARN {
				return true
			}
		}
		if c.RepositoryCredentials != nil && c.RepositoryCredentials.CredentialsParameter != nil &&
			*c.RepositoryCredentials.CredentialsParameter == secretARN {
			return true
		}
	}
	return false
}

// checkSecretsLogs returns the CloudWatch log group for the rotation Lambda function
// associated with this secret. Reads RotationLambdaARN and calls
// lambda:GetFunction → FunctionConfiguration.LoggingConfig.LogGroup (or derives
// /aws/lambda/<function-name> as default).
// If no RotationLambdaARN is set, returns Count: 0.
func checkSecretsLogs(ctx context.Context, clients any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	secret, ok := assertStruct[secretstypes.SecretListEntry](res.RawStruct)
	if !ok {
		return resource.RelatedCheckResult{TargetType: "logs", Count: -1}
	}
	if secret.RotationLambdaARN == nil || *secret.RotationLambdaARN == "" {
		return resource.RelatedCheckResult{TargetType: "logs", Count: 0}
	}
	rotationARN := *secret.RotationLambdaARN

	// Extract function name from ARN (arn:aws:lambda:region:account:function:<name>)
	funcName := rotationARN
	if idx := strings.LastIndex(rotationARN, ":"); idx >= 0 && idx < len(rotationARN)-1 {
		funcName = rotationARN[idx+1:]
	}

	defaultLogGroup := "/aws/lambda/" + funcName

	c, cok := clients.(*ServiceClients)
	if !cok || c == nil {
		// Return the default log group name derived from the ARN
		return relatedResult("logs", []string{defaultLogGroup})
	}
	lambdaAPI, ok := c.Lambda.(LambdaGetFunctionAPI)
	if !ok {
		return relatedResult("logs", []string{defaultLogGroup})
	}

	out, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*lambda.GetFunctionOutput, error) {
		return lambdaAPI.GetFunction(ctx, &lambda.GetFunctionInput{FunctionName: &rotationARN})
	})
	if err != nil || out == nil || out.Configuration == nil {
		return relatedResult("logs", []string{defaultLogGroup})
	}

	logGroup := defaultLogGroup
	if out.Configuration.LoggingConfig != nil && out.Configuration.LoggingConfig.LogGroup != nil &&
		*out.Configuration.LoggingConfig.LogGroup != "" {
		logGroup = *out.Configuration.LoggingConfig.LogGroup
	}
	return relatedResult("logs", []string{logGroup})
}

// checkSecretsRole resolves IAM roles associated with this secret via two paths:
//  1. secretsmanager:GetResourcePolicy → Statement[].Principal.AWS for role ARNs.
//  2. If parent has RotationLambdaARN: lambda:GetFunction → FunctionConfiguration.Role.
//
// Deduplicates results.
func checkSecretsRole(ctx context.Context, clients any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	_, secretName := secretIdentifiers(res)
	secretID := res.Fields["arn"]
	if secretID == "" {
		if raw, ok := assertStruct[secretstypes.SecretListEntry](res.RawStruct); ok && raw.ARN != nil {
			secretID = *raw.ARN
		}
	}
	if secretID == "" {
		secretID = secretName
	}
	if secretID == "" {
		return resource.RelatedCheckResult{TargetType: "role", Count: 0}
	}

	c, cok := clients.(*ServiceClients)
	if !cok || c == nil {
		return resource.RelatedCheckResult{TargetType: "role", Count: -1}
	}

	var ids []string

	// Path 1: resource-based policy
	smAPI, ok := c.SecretsManager.(SecretsManagerGetResourcePolicyAPI)
	if ok {
		policyOut, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*smtypes.GetResourcePolicyOutput, error) {
			return smAPI.GetResourcePolicy(ctx, &smtypes.GetResourcePolicyInput{
				SecretId: &secretID,
			})
		})
		if err == nil && policyOut != nil && policyOut.ResourcePolicy != nil && *policyOut.ResourcePolicy != "" {
			ids = append(ids, secretsPolicyRoleARNs(*policyOut.ResourcePolicy)...)
		}
	}

	// Path 2: rotation Lambda execution role
	secret, ok := assertStruct[secretstypes.SecretListEntry](res.RawStruct)
	if ok && secret.RotationLambdaARN != nil && *secret.RotationLambdaARN != "" {
		lambdaAPI, ok := c.Lambda.(LambdaGetFunctionAPI)
		if ok {
			rotationARN := *secret.RotationLambdaARN
			out, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*lambda.GetFunctionOutput, error) {
				return lambdaAPI.GetFunction(ctx, &lambda.GetFunctionInput{FunctionName: &rotationARN})
			})
			if err == nil && out != nil && out.Configuration != nil &&
				out.Configuration.Role != nil && *out.Configuration.Role != "" {
				ids = append(ids, *out.Configuration.Role)
			}
		}
	}

	return relatedResult("role", ids)
}

// secretsPolicyRoleARNs parses a Secrets Manager resource policy JSON and returns
// all IAM role ARNs found in Statement[].Principal.AWS.
func secretsPolicyRoleARNs(policyText string) []string {
	var policy struct {
		Statement []struct {
			Principal json.RawMessage `json:"Principal"`
		} `json:"Statement"`
	}
	if err := json.Unmarshal([]byte(policyText), &policy); err != nil {
		return nil
	}
	seen := map[string]struct{}{}
	for _, stmt := range policy.Statement {
		if stmt.Principal == nil {
			continue
		}
		var principalObj map[string]json.RawMessage
		if err := json.Unmarshal(stmt.Principal, &principalObj); err == nil {
			if awsRaw, ok := principalObj["AWS"]; ok {
				addRoleARNs(awsRaw, seen)
			}
		}
	}
	ids := make([]string, 0, len(seen))
	for arn := range seen {
		ids = append(ids, arn)
	}
	return ids
}

// checkSecretsSNS checks whether the rotation Lambda for this secret has an SNS
// DLQ (DeadLetterConfig.TargetArn starting with arn:aws:sns:).
// Weak pair (3-sometimes/2-no). No direct SecretsManager→SNS API; check rotation
// lambda's DLQ if SNS.
// If no RotationLambdaARN, Count: 0.
func checkSecretsSNS(ctx context.Context, clients any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	secret, ok := assertStruct[secretstypes.SecretListEntry](res.RawStruct)
	if !ok {
		return resource.RelatedCheckResult{TargetType: "sns", Count: -1}
	}
	if secret.RotationLambdaARN == nil || *secret.RotationLambdaARN == "" {
		return resource.RelatedCheckResult{TargetType: "sns", Count: 0}
	}
	rotationARN := *secret.RotationLambdaARN

	c, cok := clients.(*ServiceClients)
	if !cok || c == nil {
		return resource.RelatedCheckResult{TargetType: "sns", Count: -1}
	}
	lambdaAPI, ok := c.Lambda.(LambdaGetFunctionAPI)
	if !ok {
		return resource.RelatedCheckResult{TargetType: "sns", Count: -1}
	}

	out, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*lambda.GetFunctionOutput, error) {
		return lambdaAPI.GetFunction(ctx, &lambda.GetFunctionInput{FunctionName: &rotationARN})
	})
	if err != nil || out == nil || out.Configuration == nil {
		return resource.RelatedCheckResult{TargetType: "sns", Count: 0}
	}
	dlc := out.Configuration.DeadLetterConfig
	if dlc == nil || dlc.TargetArn == nil || *dlc.TargetArn == "" {
		return resource.RelatedCheckResult{TargetType: "sns", Count: 0}
	}
	if !strings.HasPrefix(*dlc.TargetArn, "arn:aws:sns:") {
		return resource.RelatedCheckResult{TargetType: "sns", Count: 0}
	}
	return relatedResult("sns", []string{*dlc.TargetArn})
}
