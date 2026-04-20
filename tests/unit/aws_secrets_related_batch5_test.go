// aws_secrets_related_batch5_test.go contains TDD Red tests for the Secrets
// Manager related-panel checkers: secrets→codeartifact, secrets→eb,
// secrets→ecs-task, secrets→logs, secrets→role, secrets→sns.
// Tests are written before the coder replaces the stubs in stubs_related.go
// with real implementations — initial failures are expected.
package unit_test

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"
	"github.com/aws/aws-sdk-go-v2/service/elasticbeanstalk"
	ebtypes "github.com/aws/aws-sdk-go-v2/service/elasticbeanstalk/types"
	lambdatypes "github.com/aws/aws-sdk-go-v2/service/lambda/types"
	smtypes "github.com/aws/aws-sdk-go-v2/service/secretsmanager/types"

	_ "github.com/k2m30/a9s/v3/internal/aws"
	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// secretsCheckerByTarget is defined in aws_secrets_related_test.go.

// secretsSourceWithARN returns a secret resource with the given ARN and name.
func secretsSourceWithARN(secretARN, secretName string) resource.Resource {
	return resource.Resource{
		ID:   secretName,
		Name: secretName,
		Fields: map[string]string{
			"arn": secretARN,
		},
		RawStruct: smtypes.SecretListEntry{
			Name: aws.String(secretName),
			ARN:  aws.String(secretARN),
		},
	}
}

// secretsSourceWithRotation returns a secret with RotationLambdaARN set.
func secretsSourceWithRotation(secretARN, secretName, rotationLambdaARN string) resource.Resource {
	return resource.Resource{
		ID:   secretName,
		Name: secretName,
		Fields: map[string]string{
			"arn": secretARN,
		},
		RawStruct: smtypes.SecretListEntry{
			Name:              aws.String(secretName),
			ARN:               aws.String(secretARN),
			RotationLambdaARN: aws.String(rotationLambdaARN),
		},
	}
}

// ---------------------------------------------------------------------------
// secrets→codeartifact (heuristic: secret name contains "codeartifact")
// ---------------------------------------------------------------------------

// TestRelated_Secrets_CodeArtifact_MatchByName verifies that checkSecretsCodeArtifact
// returns Count=1 when the secret name contains "codeartifact".
func TestRelated_Secrets_CodeArtifact_MatchByName(t *testing.T) {
	source := resource.Resource{
		ID:   "prod/codeartifact/token",
		Name: "prod/codeartifact/token",
		RawStruct: smtypes.SecretListEntry{
			Name: aws.String("prod/codeartifact/token"),
			ARN:  aws.String("arn:aws:secretsmanager:us-east-1:123456789012:secret:prod/codeartifact/token"),
		},
	}

	checker := secretsCheckerByTarget(t, "codeartifact")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1 (name contains 'codeartifact')", result.Count)
	}
	if result.Err != nil {
		t.Errorf("unexpected error: %v", result.Err)
	}
}

// TestRelated_Secrets_CodeArtifact_NoMatch verifies that checkSecretsCodeArtifact
// returns Count=0 when the secret name does not contain "codeartifact" and has no
// relevant tags.
func TestRelated_Secrets_CodeArtifact_NoMatch(t *testing.T) {
	source := resource.Resource{
		ID:   "prod/db/postgres-password",
		Name: "prod/db/postgres-password",
		RawStruct: smtypes.SecretListEntry{
			Name: aws.String("prod/db/postgres-password"),
			ARN:  aws.String("arn:aws:secretsmanager:us-east-1:123456789012:secret:prod/db/postgres-password"),
		},
	}

	checker := secretsCheckerByTarget(t, "codeartifact")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (name does not contain 'codeartifact')", result.Count)
	}
	if result.Err != nil {
		t.Errorf("unexpected error: %v", result.Err)
	}
}

// TestRelated_Secrets_CodeArtifact_WrongRawStruct verifies that
// checkSecretsCodeArtifact returns Count=-1 for wrong RawStruct type.
func TestRelated_Secrets_CodeArtifact_WrongRawStruct(t *testing.T) {
	res := resource.Resource{
		ID:        "prod/codeartifact/token",
		RawStruct: "not-a-secret-list-entry",
	}

	checker := secretsCheckerByTarget(t, "codeartifact")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (wrong RawStruct)", result.Count)
	}
}

// ---------------------------------------------------------------------------
// secrets→eb (reverse-scan: EB environments whose option settings contain
//  {{resolve:secretsmanager:<arn>}})
// ---------------------------------------------------------------------------

// TestRelated_Secrets_EB_MatchByResolveReference verifies that checkSecretsEB
// counts EB environments whose DescribeConfigurationSettings contains a
// {{resolve:secretsmanager:<arn>}} reference.
func TestRelated_Secrets_EB_MatchByResolveReference(t *testing.T) {
	const secretARN = "arn:aws:secretsmanager:us-east-1:123456789012:secret:prod/db/password"

	source := secretsSourceWithARN(secretARN, "prod/db/password")

	// eb env that references the secret via resolve syntax
	matchingEnv := resource.Resource{
		ID:   "my-matching-env",
		Name: "my-matching-env",
		RawStruct: ebtypes.EnvironmentDescription{
			EnvironmentName: aws.String("my-matching-env"),
			ApplicationName: aws.String("my-app"),
		},
	}
	// eb env that does NOT reference this secret
	otherEnv := resource.Resource{
		ID:   "my-other-env",
		Name: "my-other-env",
		RawStruct: ebtypes.EnvironmentDescription{
			EnvironmentName: aws.String("my-other-env"),
			ApplicationName: aws.String("other-app"),
		},
	}

	// EB fake: DescribeConfigurationSettings returns option with secret reference for matching-env only
	fakeEB := &fakeEBBatch2{
		describeConfigSettingsFn: func(input *elasticbeanstalk.DescribeConfigurationSettingsInput) (*elasticbeanstalk.DescribeConfigurationSettingsOutput, error) {
			envName := ""
			if input.EnvironmentName != nil {
				envName = *input.EnvironmentName
			}
			if envName == "my-matching-env" {
				return &elasticbeanstalk.DescribeConfigurationSettingsOutput{
					ConfigurationSettings: []ebtypes.ConfigurationSettingsDescription{
						{
							OptionSettings: []ebtypes.ConfigurationOptionSetting{
								{
									Namespace:  aws.String("aws:elasticbeanstalk:application:environment"),
									OptionName: aws.String("DB_PASSWORD"),
									Value:      aws.String("{{resolve:secretsmanager:" + secretARN + "}}"),
								},
							},
						},
					},
				}, nil
			}
			return &elasticbeanstalk.DescribeConfigurationSettingsOutput{
				ConfigurationSettings: []ebtypes.ConfigurationSettingsDescription{},
			}, nil
		},
	}
	clients := &awsclient.ServiceClients{
		ElasticBeanstalk: fakeEB,
	}
	cache := resource.ResourceCache{
		"eb": resource.ResourceCacheEntry{
			Resources:   []resource.Resource{matchingEnv, otherEnv},
			IsTruncated: false,
		},
	}

	checker := secretsCheckerByTarget(t, "eb")
	result := checker(context.Background(), clients, source, cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1 (one EB env references the secret)", result.Count)
	}
	if result.Err != nil {
		t.Errorf("unexpected error: %v", result.Err)
	}
}

// TestRelated_Secrets_EB_MatchApproximate verifies that checkSecretsEB propagates
// Approximate=true when cache is truncated.
func TestRelated_Secrets_EB_MatchApproximate(t *testing.T) {
	const secretARN = "arn:aws:secretsmanager:us-east-1:123456789012:secret:prod/db/password"

	source := secretsSourceWithARN(secretARN, "prod/db/password")

	matchingEnv := resource.Resource{
		ID:   "my-env",
		Name: "my-env",
		RawStruct: ebtypes.EnvironmentDescription{
			EnvironmentName: aws.String("my-env"),
			ApplicationName: aws.String("my-app"),
		},
	}

	fakeEB := newFakeEBWithConfigSettings([]ebtypes.ConfigurationOptionSetting{
		{
			Namespace:  aws.String("aws:elasticbeanstalk:application:environment"),
			OptionName: aws.String("DB_PASSWORD"),
			Value:      aws.String("{{resolve:secretsmanager:" + secretARN + "}}"),
		},
	})
	clients := &awsclient.ServiceClients{
		ElasticBeanstalk: fakeEB,
	}
	cache := resource.ResourceCache{
		"eb": resource.ResourceCacheEntry{
			Resources:   []resource.Resource{matchingEnv},
			IsTruncated: true, // truncated — more envs may exist
		},
	}

	checker := secretsCheckerByTarget(t, "eb")
	result := checker(context.Background(), clients, source, cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1 (matching env)", result.Count)
	}
	if !result.Approximate {
		t.Errorf("Approximate = false, want true (cache is truncated)")
	}
}

// TestRelated_Secrets_EB_MissingCache verifies that checkSecretsEB returns
// Count=0 (not -1) when the eb cache is missing — the checker returns 0 until
// the cache is populated on the next panel open.
func TestRelated_Secrets_EB_MissingCache(t *testing.T) {
	source := secretsSourceWithARN(
		"arn:aws:secretsmanager:us-east-1:123456789012:secret:prod/db/password",
		"prod/db/password",
	)

	checker := secretsCheckerByTarget(t, "eb")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (missing cache — unknown, not error)", result.Count)
	}
	if result.Err != nil {
		t.Errorf("unexpected error: %v", result.Err)
	}
}

// TestRelated_Secrets_EB_WrongRawStruct verifies that checkSecretsEB returns
// Count=-1 for wrong RawStruct type.
func TestRelated_Secrets_EB_WrongRawStruct(t *testing.T) {
	res := resource.Resource{
		ID:        "prod/db/password",
		RawStruct: 99,
	}

	checker := secretsCheckerByTarget(t, "eb")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (wrong RawStruct)", result.Count)
	}
}

// ---------------------------------------------------------------------------
// secrets→ecs-task (reverse-scan: ECS tasks whose task definition Secrets
//  reference this secret's ARN via Secrets[].ValueFrom)
// ---------------------------------------------------------------------------

// TestRelated_Secrets_ECSTask_MatchBySecretsValueFrom verifies that
// checkSecretsECSTask counts ECS tasks whose task definition contains a
// container secret with ValueFrom = this secret's ARN.
func TestRelated_Secrets_ECSTask_MatchBySecretsValueFrom(t *testing.T) {
	const secretARN = "arn:aws:secretsmanager:us-east-1:123456789012:secret:prod/db/password"
	const taskDefARN = "arn:aws:ecs:us-east-1:123456789012:task-definition/api-task:7"
	const otherTaskDefARN = "arn:aws:ecs:us-east-1:123456789012:task-definition/worker-task:3"

	source := secretsSourceWithARN(secretARN, "prod/db/password")

	// ecs-task cache entry that references our secret (via TaskDefinitionArn)
	matchingTask := resource.Resource{
		ID:   "task-abc123",
		Name: "task-abc123",
		Fields: map[string]string{
			"task_definition": taskDefARN,
		},
		RawStruct: ecstypes.Task{
			TaskDefinitionArn: aws.String(taskDefARN),
		},
	}
	// ecs-task cache entry that does NOT reference our secret
	otherTask := resource.Resource{
		ID:   "task-def456",
		Name: "task-def456",
		Fields: map[string]string{
			"task_definition": otherTaskDefARN,
		},
		RawStruct: ecstypes.Task{
			TaskDefinitionArn: aws.String(otherTaskDefARN),
		},
	}

	// ECS fake: DescribeTaskDefinition returns a task def with secret reference for matching task
	fakeECS := &fakeECSBatch4{
		describeTaskDefFn: func(input *ecs.DescribeTaskDefinitionInput) (*ecs.DescribeTaskDefinitionOutput, error) {
			if input.TaskDefinition != nil && *input.TaskDefinition == taskDefARN {
				return &ecs.DescribeTaskDefinitionOutput{
					TaskDefinition: &ecstypes.TaskDefinition{
						TaskDefinitionArn: aws.String(taskDefARN),
						ContainerDefinitions: []ecstypes.ContainerDefinition{
							{
								Name: aws.String("api"),
								Secrets: []ecstypes.Secret{
									{
										Name:      aws.String("DB_PASSWORD"),
										ValueFrom: aws.String(secretARN),
									},
								},
							},
						},
					},
				}, nil
			}
			// Other task definitions have no secrets
			return &ecs.DescribeTaskDefinitionOutput{
				TaskDefinition: &ecstypes.TaskDefinition{
					TaskDefinitionArn: aws.String(""),
					ContainerDefinitions: []ecstypes.ContainerDefinition{
						{Name: aws.String("worker")},
					},
				},
			}, nil
		},
	}
	clients := &awsclient.ServiceClients{
		ECS: fakeECS,
	}
	cache := resource.ResourceCache{
		"ecs-task": resource.ResourceCacheEntry{
			Resources:   []resource.Resource{matchingTask, otherTask},
			IsTruncated: false,
		},
	}

	checker := secretsCheckerByTarget(t, "ecs-task")
	result := checker(context.Background(), clients, source, cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1 (one task references the secret)", result.Count)
	}
	if result.Err != nil {
		t.Errorf("unexpected error: %v", result.Err)
	}
}

// TestRelated_Secrets_ECSTask_MissingCache verifies that checkSecretsECSTask
// returns Count=0 when ecs-task cache is missing.
func TestRelated_Secrets_ECSTask_MissingCache(t *testing.T) {
	source := secretsSourceWithARN(
		"arn:aws:secretsmanager:us-east-1:123456789012:secret:prod/db/password",
		"prod/db/password",
	)

	checker := secretsCheckerByTarget(t, "ecs-task")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (missing cache)", result.Count)
	}
	if result.Err != nil {
		t.Errorf("unexpected error: %v", result.Err)
	}
}

// TestRelated_Secrets_ECSTask_Truncated verifies that checkSecretsECSTask
// propagates Approximate=true when the ecs-task cache is truncated.
func TestRelated_Secrets_ECSTask_Truncated(t *testing.T) {
	const secretARN = "arn:aws:secretsmanager:us-east-1:123456789012:secret:prod/db/password"
	const taskDefARN = "arn:aws:ecs:us-east-1:123456789012:task-definition/api-task:7"

	source := secretsSourceWithARN(secretARN, "prod/db/password")

	matchingTask := resource.Resource{
		ID:     "task-abc123",
		Fields: map[string]string{"task_definition": taskDefARN},
		RawStruct: ecstypes.Task{
			TaskDefinitionArn: aws.String(taskDefARN),
		},
	}
	fakeECS := newFakeECSWithTaskDefinition(&ecstypes.TaskDefinition{
		TaskDefinitionArn: aws.String(taskDefARN),
		ContainerDefinitions: []ecstypes.ContainerDefinition{
			{
				Name: aws.String("api"),
				Secrets: []ecstypes.Secret{
					{Name: aws.String("DB_PASSWORD"), ValueFrom: aws.String(secretARN)},
				},
			},
		},
	})
	clients := &awsclient.ServiceClients{
		ECS: fakeECS,
	}
	cache := resource.ResourceCache{
		"ecs-task": resource.ResourceCacheEntry{
			Resources:   []resource.Resource{matchingTask},
			IsTruncated: true, // more tasks may exist in full scan
		},
	}

	checker := secretsCheckerByTarget(t, "ecs-task")
	result := checker(context.Background(), clients, source, cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if !result.Approximate {
		t.Errorf("Approximate = false, want true (cache is truncated)")
	}
}

// TestRelated_Secrets_ECSTask_WrongRawStruct verifies that checkSecretsECSTask
// returns Count=-1 for wrong RawStruct type.
func TestRelated_Secrets_ECSTask_WrongRawStruct(t *testing.T) {
	res := resource.Resource{
		ID:        "prod/db/password",
		RawStruct: "not-a-secret",
	}

	checker := secretsCheckerByTarget(t, "ecs-task")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (wrong RawStruct)", result.Count)
	}
}

// ---------------------------------------------------------------------------
// secrets→logs (forward: RotationLambdaARN → Lambda GetFunction →
//  FunctionName → /aws/lambda/<name> log group)
// ---------------------------------------------------------------------------

// TestRelated_Secrets_Logs_MatchByRotationLambda verifies that checkSecretsLogs
// derives the expected log group path from the rotation Lambda's function name.
func TestRelated_Secrets_Logs_MatchByRotationLambda(t *testing.T) {
	const lambdaARN = "arn:aws:lambda:us-east-1:123456789012:function:rotate-docdb-credentials"
	const lambdaName = "rotate-docdb-credentials"
	const expectedLogGroup = "/aws/lambda/rotate-docdb-credentials"

	source := secretsSourceWithRotation(
		"arn:aws:secretsmanager:us-east-1:123456789012:secret:prod/docdb/password",
		"prod/docdb/password",
		lambdaARN,
	)

	fakeLambda := newFakeLambdaWithFunctionConfig(&lambdatypes.FunctionConfiguration{
		FunctionName: aws.String(lambdaName),
		FunctionArn:  aws.String(lambdaARN),
	})
	clients := &awsclient.ServiceClients{
		Lambda: fakeLambda,
	}

	checker := secretsCheckerByTarget(t, "logs")
	result := checker(context.Background(), clients, source, resource.ResourceCache{})

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1 (one log group for rotation Lambda)", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != expectedLogGroup {
		t.Errorf("ResourceIDs = %v, want [%s]", result.ResourceIDs, expectedLogGroup)
	}
	if result.Err != nil {
		t.Errorf("unexpected error: %v", result.Err)
	}
}

// TestRelated_Secrets_Logs_NoRotationLambda verifies that checkSecretsLogs
// returns Count=0 when the secret has no rotation Lambda ARN.
func TestRelated_Secrets_Logs_NoRotationLambda(t *testing.T) {
	source := resource.Resource{
		ID:   "prod/api/stripe-key",
		Name: "prod/api/stripe-key",
		RawStruct: smtypes.SecretListEntry{
			Name:              aws.String("prod/api/stripe-key"),
			ARN:               aws.String("arn:aws:secretsmanager:us-east-1:123456789012:secret:prod/api/stripe-key"),
			RotationLambdaARN: nil,
		},
	}

	checker := secretsCheckerByTarget(t, "logs")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (no rotation Lambda)", result.Count)
	}
	if result.Err != nil {
		t.Errorf("unexpected error: %v", result.Err)
	}
}

// TestRelated_Secrets_Logs_WrongRawStruct verifies that checkSecretsLogs
// returns Count=-1 for wrong RawStruct type.
func TestRelated_Secrets_Logs_WrongRawStruct(t *testing.T) {
	res := resource.Resource{
		ID:        "prod/docdb/password",
		RawStruct: 0,
	}

	checker := secretsCheckerByTarget(t, "logs")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (wrong RawStruct)", result.Count)
	}
}

// ---------------------------------------------------------------------------
// secrets→role (forward: GetResourcePolicy → Principal role ARNs +
//  RotationLambdaARN → Lambda GetFunction → Configuration.Role)
// ---------------------------------------------------------------------------

// TestRelated_Secrets_Role_MatchByResourcePolicy verifies that checkSecretsRole
// extracts role ARNs from the secret's resource policy Principal.
func TestRelated_Secrets_Role_MatchByResourcePolicy(t *testing.T) {
	const secretARN = "arn:aws:secretsmanager:us-east-1:123456789012:secret:prod/db/password"
	const role1ARN = "arn:aws:iam::123456789012:role/api-service-role"
	const role2ARN = "arn:aws:iam::123456789012:role/batch-processor-role"

	// IAM policy granting two roles access to the secret
	policyJSON := `{
		"Version": "2012-10-17",
		"Statement": [
			{
				"Effect": "Allow",
				"Principal": {
					"AWS": [
						"` + role1ARN + `",
						"` + role2ARN + `"
					]
				},
				"Action": ["secretsmanager:GetSecretValue"],
				"Resource": "*"
			}
		]
	}`

	source := secretsSourceWithARN(secretARN, "prod/db/password")

	fakeSM := newFakeSecretsManagerWithResourcePolicy(policyJSON)
	clients := &awsclient.ServiceClients{
		SecretsManager: fakeSM,
	}

	checker := secretsCheckerByTarget(t, "role")
	result := checker(context.Background(), clients, source, resource.ResourceCache{})

	if result.Count < 2 {
		t.Errorf("Count = %d, want >= 2 (two role ARNs in resource policy)", result.Count)
	}
	roleFound := map[string]bool{role1ARN: false, role2ARN: false}
	for _, id := range result.ResourceIDs {
		if _, ok := roleFound[id]; ok {
			roleFound[id] = true
		}
	}
	for arn, found := range roleFound {
		if !found {
			t.Errorf("expected role ARN %q in ResourceIDs, got %v", arn, result.ResourceIDs)
		}
	}
	if result.Err != nil {
		t.Errorf("unexpected error: %v", result.Err)
	}
}

// TestRelated_Secrets_Role_MatchIncludesRotationLambdaRole verifies that
// checkSecretsRole also includes the execution role of the rotation Lambda.
func TestRelated_Secrets_Role_MatchIncludesRotationLambdaRole(t *testing.T) {
	const secretARN = "arn:aws:secretsmanager:us-east-1:123456789012:secret:prod/db/password"
	const lambdaARN = "arn:aws:lambda:us-east-1:123456789012:function:rotate-db-creds"
	const lambdaName = "rotate-db-creds"
	const lambdaRoleARN = "arn:aws:iam::123456789012:role/rotate-db-creds-execution-role"

	policyJSON := `{"Version":"2012-10-17","Statement":[]}`

	source := secretsSourceWithRotation(secretARN, "prod/db/password", lambdaARN)

	fakeSM := newFakeSecretsManagerWithResourcePolicy(policyJSON)
	fakeLambda := newFakeLambdaWithFunctionConfig(&lambdatypes.FunctionConfiguration{
		FunctionName: aws.String(lambdaName),
		FunctionArn:  aws.String(lambdaARN),
		Role:         aws.String(lambdaRoleARN),
	})
	clients := &awsclient.ServiceClients{
		SecretsManager: fakeSM,
		Lambda:         fakeLambda,
	}

	checker := secretsCheckerByTarget(t, "role")
	result := checker(context.Background(), clients, source, resource.ResourceCache{})

	if result.Count < 1 {
		t.Errorf("Count = %d, want >= 1 (rotation Lambda execution role)", result.Count)
	}
	found := false
	for _, id := range result.ResourceIDs {
		if id == lambdaRoleARN {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected rotation Lambda role %q in ResourceIDs, got %v", lambdaRoleARN, result.ResourceIDs)
	}
	if result.Err != nil {
		t.Errorf("unexpected error: %v", result.Err)
	}
}

// TestRelated_Secrets_Role_NoPolicy verifies that checkSecretsRole returns
// Count=0 when the secret has no resource policy and no rotation Lambda.
func TestRelated_Secrets_Role_NoPolicy(t *testing.T) {
	source := resource.Resource{
		ID:   "prod/api/stripe-key",
		Name: "prod/api/stripe-key",
		RawStruct: smtypes.SecretListEntry{
			Name:              aws.String("prod/api/stripe-key"),
			ARN:               aws.String("arn:aws:secretsmanager:us-east-1:123456789012:secret:prod/api/stripe-key"),
			RotationLambdaARN: nil,
		},
	}

	fakeSM := newFakeSecretsManagerNoPolicy()
	clients := &awsclient.ServiceClients{
		SecretsManager: fakeSM,
	}

	checker := secretsCheckerByTarget(t, "role")
	result := checker(context.Background(), clients, source, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (no resource policy, no rotation Lambda)", result.Count)
	}
	if result.Err != nil {
		t.Errorf("unexpected error: %v", result.Err)
	}
}

// TestRelated_Secrets_Role_WrongRawStruct verifies that checkSecretsRole
// returns Count=-1 for wrong RawStruct type.
func TestRelated_Secrets_Role_WrongRawStruct(t *testing.T) {
	res := resource.Resource{
		ID:        "prod/db/password",
		RawStruct: struct{ X int }{X: 1},
	}

	checker := secretsCheckerByTarget(t, "role")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (wrong RawStruct)", result.Count)
	}
}

// ---------------------------------------------------------------------------
// secrets→sns (forward weak: RotationLambdaARN → Lambda GetFunction →
//  DeadLetterConfig.TargetArn starting with "arn:aws:sns:")
// ---------------------------------------------------------------------------

// TestRelated_Secrets_Sns_MatchByDLQ verifies that checkSecretsSNS returns
// Count=1 when the rotation Lambda's DLQ TargetArn is an SNS topic ARN.
func TestRelated_Secrets_Sns_MatchByDLQ(t *testing.T) {
	const lambdaARN = "arn:aws:lambda:us-east-1:123456789012:function:rotate-docdb-credentials"
	const lambdaName = "rotate-docdb-credentials"
	const snsTopicARN = "arn:aws:sns:us-east-1:123456789012:rotation-dlq-topic"

	source := secretsSourceWithRotation(
		"arn:aws:secretsmanager:us-east-1:123456789012:secret:prod/docdb/password",
		"prod/docdb/password",
		lambdaARN,
	)

	fakeLambda := newFakeLambdaWithDLQ(lambdaName, snsTopicARN)
	clients := &awsclient.ServiceClients{
		Lambda: fakeLambda,
	}

	checker := secretsCheckerByTarget(t, "sns")
	result := checker(context.Background(), clients, source, resource.ResourceCache{})

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1 (rotation Lambda DLQ is SNS topic)", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != snsTopicARN {
		t.Errorf("ResourceIDs = %v, want [%s]", result.ResourceIDs, snsTopicARN)
	}
	if result.Err != nil {
		t.Errorf("unexpected error: %v", result.Err)
	}
}

// TestRelated_Secrets_Sns_DLQNotSNS verifies that checkSecretsSNS returns
// Count=0 when the rotation Lambda's DLQ is an SQS queue (not SNS).
func TestRelated_Secrets_Sns_DLQNotSNS(t *testing.T) {
	const lambdaARN = "arn:aws:lambda:us-east-1:123456789012:function:rotate-docdb-credentials"
	const sqsARN = "arn:aws:sqs:us-east-1:123456789012:rotation-dlq"

	source := secretsSourceWithRotation(
		"arn:aws:secretsmanager:us-east-1:123456789012:secret:prod/docdb/password",
		"prod/docdb/password",
		lambdaARN,
	)

	fakeLambda := newFakeLambdaWithDLQ("rotate-docdb-credentials", sqsARN)
	clients := &awsclient.ServiceClients{
		Lambda: fakeLambda,
	}

	checker := secretsCheckerByTarget(t, "sns")
	result := checker(context.Background(), clients, source, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (DLQ is SQS, not SNS)", result.Count)
	}
	if result.Err != nil {
		t.Errorf("unexpected error: %v", result.Err)
	}
}

// TestRelated_Secrets_Sns_NoRotationLambda verifies that checkSecretsSNS returns
// Count=0 when the secret has no rotation Lambda.
func TestRelated_Secrets_Sns_NoRotationLambda(t *testing.T) {
	source := resource.Resource{
		ID:   "prod/api/stripe-key",
		Name: "prod/api/stripe-key",
		RawStruct: smtypes.SecretListEntry{
			Name:              aws.String("prod/api/stripe-key"),
			ARN:               aws.String("arn:aws:secretsmanager:us-east-1:123456789012:secret:prod/api/stripe-key"),
			RotationLambdaARN: nil,
		},
	}

	checker := secretsCheckerByTarget(t, "sns")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (no rotation Lambda)", result.Count)
	}
	if result.Err != nil {
		t.Errorf("unexpected error: %v", result.Err)
	}
}

// TestRelated_Secrets_Sns_WrongRawStruct verifies that checkSecretsSNS returns
// Count=-1 for wrong RawStruct type.
func TestRelated_Secrets_Sns_WrongRawStruct(t *testing.T) {
	res := resource.Resource{
		ID:        "prod/docdb/password",
		RawStruct: true,
	}

	checker := secretsCheckerByTarget(t, "sns")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (wrong RawStruct)", result.Count)
	}
}
