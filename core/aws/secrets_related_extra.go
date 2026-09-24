// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// secrets_related_extra.go contains additional Secrets Manager related-resource
// checker functions.
package aws

import (
	"context"
	"slices"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	ecspkg "github.com/aws/aws-sdk-go-v2/service/ecs"
	ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"
	"github.com/aws/aws-sdk-go-v2/service/elasticbeanstalk"
	ebtypes "github.com/aws/aws-sdk-go-v2/service/elasticbeanstalk/types"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
	smtypes "github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	secretstypes "github.com/aws/aws-sdk-go-v2/service/secretsmanager/types"

	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// checkSecretsCodeArtifact reports the CodeArtifact repositories this secret
// is for. Weak pair (3-sometimes/2-no), no AWS call: a secret is linked to
// CodeArtifact when its name or a tag says so, and the repositories it is for
// are the loaded ones its name, description or tag values name.
func checkSecretsCodeArtifact(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	secret, ok := assertStruct[secretstypes.SecretListEntry](res.RawStruct)
	if !ok {
		return NotRead("codeartifact")
	}
	text := []string{aws.ToString(secret.Name), aws.ToString(secret.Description)}
	linked := strings.Contains(strings.ToLower(text[0]), "codeartifact")
	for _, tag := range secret.Tags {
		key, val := aws.ToString(tag.Key), aws.ToString(tag.Value)
		linked = linked || strings.Contains(strings.ToLower(key), "codeartifact") || strings.Contains(val, ":codeartifact:")
		text = append(text, val)
	}
	if !linked {
		return foundNone("codeartifact", "the secret's name, description and tags")
	}
	repos, truncated, err := relatedResourcesFor(ctx, clients, cache, "codeartifact")
	if err != nil {
		return ReadFailed("codeartifact", err)
	}
	if repos == nil {
		return NotRead("codeartifact")
	}
	var ids []string
	for _, repo := range repos {
		if slices.ContainsFunc(text, func(t string) bool { return strings.Contains(t, repo.Name) }) {
			ids = append(ids, repo.ID)
		}
	}
	return heuristicResult("codeartifact", ids, truncated)
}

// checkSecretsEB is a reverse-scan checker for the secrets→eb relationship.
// Iterates cache["eb"]; for each EB environment, calls
// elasticbeanstalk:DescribeConfigurationSettings and scans OptionSettings[].Value
// for {{resolve:secretsmanager:<parent ARN> pattern.
// NeedsTargetCache: true; sets Truncated and FetchFilter.
func checkSecretsEB(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	// A struct, when the row carries one, must be a SecretListEntry. A row
	// carrying none is not the wrong shape — secretIdentifiers still reads
	// the ARN and the name off Fields, which survive the disk cache.
	if res.RawStruct != nil {
		if _, ok := assertStruct[secretstypes.SecretListEntry](res.RawStruct); !ok {
			return NotRead("eb")
		}
	}

	secretARN, _ := secretIdentifiers(res)
	if secretARN == "" {
		return foundNone("eb", "secretARN")
	}

	ebList, truncated, err := relatedResourcesFor(ctx, clients, cache, "eb")
	if err != nil {
		return ReadFailed("eb", err)
	}
	if ebList == nil {
		return NotRead("eb")
	}

	c, cok := clients.(*ServiceClients)
	if !cok || c == nil {
		return NotRead("eb")
	}

	resolveRef := "{{resolve:secretsmanager:" + secretARN
	var ids []string
	var reads rowReads
	for _, ebRes := range ebList {
		eb, ok := assertStruct[ebtypes.EnvironmentDescription](ebRes.RawStruct)
		if !ok {
			reads.missed()
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
			reads.missed()
			continue
		}
		cfgOut, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*elasticbeanstalk.DescribeConfigurationSettingsOutput, error) {
			return c.ElasticBeanstalk.DescribeConfigurationSettings(ctx, &elasticbeanstalk.DescribeConfigurationSettingsInput{
				ApplicationName: &appName,
				EnvironmentName: &envName,
			})
		})
		if err != nil {
			reads.fail(ebRes.ID, err)
			continue
		}
		reads.read++
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

	return reads.answer("eb", "secrets-related: DescribeConfigurationSettings", ids, truncated)
}

// checkSecretsECSTask is a reverse-scan checker for the secrets→ecs-task relationship.
// Iterates cache["ecs-task"]; for each task, calls ecs:DescribeTaskDefinition and checks
// ContainerDefinitions[].Secrets[].ValueFrom == parent ARN or
// RepositoryCredentials.CredentialsParameter == parent ARN.
// NeedsTargetCache: true; sets Truncated.
func checkSecretsECSTask(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	// A struct, when the row carries one, must be a SecretListEntry. A row
	// carrying none is not the wrong shape — secretIdentifiers still reads
	// the ARN and the name off Fields, which survive the disk cache.
	if res.RawStruct != nil {
		if _, ok := assertStruct[secretstypes.SecretListEntry](res.RawStruct); !ok {
			return NotRead("ecs-task")
		}
	}

	secretARN, _ := secretIdentifiers(res)
	if secretARN == "" {
		return foundNone("ecs-task", "secretARN")
	}

	ecsTaskList, truncated, err := relatedResourcesFor(ctx, clients, cache, "ecs-task")
	if err != nil {
		return ReadFailed("ecs-task", err)
	}
	if ecsTaskList == nil {
		return NotRead("ecs-task")
	}

	c, cok := clients.(*ServiceClients)
	if !cok || c == nil {
		return NotRead("ecs-task")
	}

	ecsAPI, ok := c.ECS.(ECSDescribeTaskDefinitionAPI)
	if !ok {
		return NotRead("ecs-task")
	}

	var ids []string
	var reads rowReads
	for _, taskRes := range ecsTaskList {
		// Cache stores ecstypes.Task — extract TaskDefinitionArn
		task, ok := assertStruct[ecstypes.Task](taskRes.RawStruct)
		if !ok {
			reads.missed()
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
			reads.missed()
			continue
		}
		tdOut, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*ecspkg.DescribeTaskDefinitionOutput, error) {
			return ecsAPI.DescribeTaskDefinition(ctx, &ecspkg.DescribeTaskDefinitionInput{
				TaskDefinition: &taskDefARN,
			})
		})
		if err != nil {
			// "Task definition does not exist" (ClientException) is definitive
			// absence, not a real failure — skip without aggregating. Every
			// other error (AccessDenied, Throttling, transient) aggregates.
			if ErrCodeIs(err, "ClientException") {
				reads.read++
				continue
			}
			reads.fail(taskRes.ID, err)
			continue
		}
		reads.read++
		if tdOut == nil || tdOut.TaskDefinition == nil {
			continue
		}
		if secretsECSTaskRefsSecret(*tdOut.TaskDefinition, res, refContext(clients, nil, "")) {
			ids = append(ids, taskRes.ID)
		}
	}

	return reads.answer("ecs-task", "secrets-related: DescribeTaskDefinition", ids, truncated)
}

// secretsECSTaskRefsSecret returns true if the TaskDefinition references the
// secret row via ContainerDefinitions[].Secrets[].ValueFrom (which may carry a
// json-key tail) or ContainerDefinitions[].RepositoryCredentials.CredentialsParameter.
func secretsECSTaskRefsSecret(td ecstypes.TaskDefinition, secret resource.Resource, rc domain.RefContext) bool {
	rc.Targets = []resource.Resource{secret}
	names := func(ref *string) bool {
		id, ok := resource.ResolveRef("secrets", aws.ToString(ref), rc)
		return ok && id == secret.ID
	}
	for _, c := range td.ContainerDefinitions {
		for _, s := range c.Secrets {
			if names(s.ValueFrom) {
				return true
			}
		}
		if c.RepositoryCredentials != nil && names(c.RepositoryCredentials.CredentialsParameter) {
			return true
		}
	}
	return false
}

// checkSecretsLogs returns the CloudWatch log group for the rotation Lambda function
// associated with this secret. Reads RotationLambdaARN and calls
// lambda:GetFunction → FunctionConfiguration.LoggingConfig.LogGroup (or derives
// /aws/lambda/<function-name> as default).
// If no RotationLambdaARN is set, returns Count: 0; a function in another
// account or region has its log group there, so the count is a lower bound.
func checkSecretsLogs(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	secret, ok := assertStruct[secretstypes.SecretListEntry](res.RawStruct)
	if !ok {
		return NotRead("logs")
	}
	if secret.RotationLambdaARN == nil || *secret.RotationLambdaARN == "" {
		return foundNone("logs", "secret.RotationLambdaARN")
	}
	rotationARN := *secret.RotationLambdaARN

	funcName, local := resource.ResolveRef("lambda", rotationARN, refContext(clients, cache, "lambda"))
	if !local {
		return relatedResultTrunc("logs", nil, true)
	}

	defaultLogGroup := "/aws/lambda/" + funcName

	// /aws/lambda/<name> is Lambda's default log group; a function with a
	// custom LoggingConfig logs elsewhere, so without the function's
	// configuration the default is a candidate, not a read.
	c, cok := clients.(*ServiceClients)
	if !cok || c == nil {
		return heuristicResult("logs", []string{defaultLogGroup}, false)
	}
	lambdaAPI, ok := c.Lambda.(LambdaGetFunctionAPI)
	if !ok {
		return heuristicResult("logs", []string{defaultLogGroup}, false)
	}

	out, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*lambda.GetFunctionOutput, error) {
		return lambdaAPI.GetFunction(ctx, &lambda.GetFunctionInput{FunctionName: &rotationARN})
	})
	// no finding: the row carries the default log group as a candidate.
	if err != nil || out == nil || out.Configuration == nil {
		return heuristicResult("logs", []string{defaultLogGroup}, false)
	}

	logGroup := defaultLogGroup
	if out.Configuration.LoggingConfig != nil && out.Configuration.LoggingConfig.LogGroup != nil &&
		*out.Configuration.LoggingConfig.LogGroup != "" {
		logGroup = *out.Configuration.LoggingConfig.LogGroup
	}
	return relatedResultTrunc("logs", []string{logGroup}, false)
}

// checkSecretsRole resolves IAM roles associated with this secret via two paths:
//  1. secretsmanager:GetResourcePolicy → the roles the resource policy grants.
//  2. If parent has RotationLambdaARN: lambda:GetFunction → FunctionConfiguration.Role.
//
// Deduplicates results.
func checkSecretsRole(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
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
		return unreadZero(res, foundNone("role", "secretID"))
	}

	c, cok := clients.(*ServiceClients)
	if !cok || c == nil {
		return NotRead("role")
	}

	rc := policyRefContext(clients, cache, "role", secretID)
	var ids []string
	// partial tracks whether either independent path below could not be
	// attempted or failed, so ids (whatever the other path found) is reported
	// as a lower bound rather than an exact count.
	partial := false

	// Path 1: resource-based policy
	smAPI, ok := c.SecretsManager.(SecretsManagerGetResourcePolicyAPI)
	if !ok {
		partial = true
	} else {
		policyOut, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*smtypes.GetResourcePolicyOutput, error) {
			return smAPI.GetResourcePolicy(ctx, &smtypes.GetResourcePolicyInput{
				SecretId: &secretID,
			})
		})
		if err != nil {
			partial = true
		} else if policyOut != nil && policyOut.ResourcePolicy != nil && *policyOut.ResourcePolicy != "" {
			refs, parsed := grantedPrincipalRefs(*policyOut.ResourcePolicy, "role/")
			partial = partial || !parsed
			ids = append(ids, refs...)
		}
	}

	// Path 2: rotation Lambda execution role
	secret, ok := assertStruct[secretstypes.SecretListEntry](res.RawStruct)
	if ok && secret.RotationLambdaARN != nil && *secret.RotationLambdaARN != "" {
		lambdaAPI, lok := c.Lambda.(LambdaGetFunctionAPI)
		if !lok {
			partial = true
		} else {
			rotationARN := *secret.RotationLambdaARN
			out, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*lambda.GetFunctionOutput, error) {
				return lambdaAPI.GetFunction(ctx, &lambda.GetFunctionInput{FunctionName: &rotationARN})
			})
			if err != nil {
				partial = true
			} else if out != nil && out.Configuration != nil &&
				out.Configuration.Role != nil && *out.Configuration.Role != "" {
				ids = append(ids, *out.Configuration.Role)
			}
		}
	}

	finalIDs, dropped := resolveRefs("role", ids, rc)
	if len(finalIDs) == 0 && partial {
		// Neither path could be checked (or both failed): nothing was
		// confirmed, so this is unresolved, not a proven zero or a lower bound.
		return NotRead("role")
	}
	return unreadZero(res, relatedResultTrunc("role", finalIDs, partial || dropped))
}

// checkSecretsSNS checks whether the rotation Lambda for this secret has an SNS
// DLQ (DeadLetterConfig.TargetArn starting with arn:aws:sns:).
// Weak pair (3-sometimes/2-no). No direct SecretsManager→SNS API; check rotation
// lambda's DLQ if SNS.
// If no RotationLambdaARN, Count: 0.
func checkSecretsSNS(ctx context.Context, clients any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	secret, ok := assertStruct[secretstypes.SecretListEntry](res.RawStruct)
	if !ok {
		return NotRead("sns")
	}
	if secret.RotationLambdaARN == nil || *secret.RotationLambdaARN == "" {
		return foundNone("sns", "secret.RotationLambdaARN")
	}
	rotationARN := *secret.RotationLambdaARN

	c, cok := clients.(*ServiceClients)
	if !cok || c == nil {
		return NotRead("sns")
	}
	lambdaAPI, ok := c.Lambda.(LambdaGetFunctionAPI)
	if !ok {
		return NotRead("sns")
	}

	out, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*lambda.GetFunctionOutput, error) {
		return lambdaAPI.GetFunction(ctx, &lambda.GetFunctionInput{FunctionName: &rotationARN})
	})
	// The rotation function's configuration is where the dead-letter topic is
	// named, so a call that did not answer leaves the count unknown; reporting
	// zero read as "checked, no topic".
	// no finding: this arm answers with the related panel's unknown.
	if err != nil || out == nil || out.Configuration == nil {
		return NotRead("sns")
	}
	dlc := out.Configuration.DeadLetterConfig
	if dlc == nil || dlc.TargetArn == nil || *dlc.TargetArn == "" {
		return foundNone("sns", "dlc.TargetArn")
	}
	if _, isTopic := ARNForService(*dlc.TargetArn, "sns"); !isTopic {
		return foundNone("sns", "isTopic")
	}
	return relatedResultTrunc("sns", []string{*dlc.TargetArn}, false)
}
