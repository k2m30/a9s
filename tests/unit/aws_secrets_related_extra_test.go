package unit_test

import (
	"context"
	"slices"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"
	"github.com/aws/aws-sdk-go-v2/service/elasticbeanstalk"
	ebtypes "github.com/aws/aws-sdk-go-v2/service/elasticbeanstalk/types"
	lambdatypes "github.com/aws/aws-sdk-go-v2/service/lambda/types"
	smtypes "github.com/aws/aws-sdk-go-v2/service/secretsmanager/types"

	_ "github.com/k2m30/a9s/v3/core/aws"
	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

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

// The secret's own name is no codeartifact row, so it is never the ID.
func TestRelated_Secrets_CodeArtifact_MatchByName(t *testing.T) {
	source := resource.Resource{
		ID:   "prod/codeartifact/acme-npm/token",
		Name: "prod/codeartifact/acme-npm/token",
		RawStruct: smtypes.SecretListEntry{
			Name: aws.String("prod/codeartifact/acme-npm/token"),
			ARN:  aws.String("arn:aws:secretsmanager:us-east-1:123456789012:secret:prod/codeartifact/acme-npm/token-AbCdEf"),
		},
	}
	cache := resource.ResourceCache{"codeartifact": resource.ResourceCacheEntry{Resources: []resource.Resource{
		{ID: "acme-npm"}, {ID: "acme-pypi"},
	}}}

	checker := secretsCheckerByTarget(t, "codeartifact")
	result := checker(context.Background(), nil, source, cache)

	if result.Count() != 1 || result.ResourceIDs()[0] != "acme-npm" {
		t.Errorf("Count = %d, IDs %v, want [acme-npm]", result.Count(), result.ResourceIDs())
	}
	if result.Err() != nil {
		t.Errorf("unexpected error: %v", result.Err())
	}
}

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

	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0 (name does not contain 'codeartifact')", result.Count())
	}
	if result.Err() != nil {
		t.Errorf("unexpected error: %v", result.Err())
	}
}

func TestRelated_Secrets_CodeArtifact_WrongRawStruct(t *testing.T) {
	res := resource.Resource{
		ID:        "prod/codeartifact/token",
		RawStruct: "not-a-secret-list-entry",
	}

	checker := secretsCheckerByTarget(t, "codeartifact")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.State() != domain.RelatedUnknown {
		t.Errorf("Count = %d, want -1 (wrong RawStruct)", result.Count())
	}
}

func TestRelated_Secrets_EB_MatchByResolveReference(t *testing.T) {
	const secretARN = "arn:aws:secretsmanager:us-east-1:123456789012:secret:prod/db/password"

	source := secretsSourceWithARN(secretARN, "prod/db/password")

	matchingEnv := resource.Resource{
		ID:   "my-matching-env",
		Name: "my-matching-env",
		RawStruct: ebtypes.EnvironmentDescription{
			EnvironmentName: aws.String("my-matching-env"),
			ApplicationName: aws.String("my-app"),
		},
	}
	otherEnv := resource.Resource{
		ID:   "my-other-env",
		Name: "my-other-env",
		RawStruct: ebtypes.EnvironmentDescription{
			EnvironmentName: aws.String("my-other-env"),
			ApplicationName: aws.String("other-app"),
		},
	}

	fakeEB := &fakeEBChecker{
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

	if result.Count() != 1 {
		t.Errorf("Count = %d, want 1 (one EB env references the secret)", result.Count())
	}
	if result.Err() != nil {
		t.Errorf("unexpected error: %v", result.Err())
	}
}

func TestRelated_Secrets_EB_MatchTruncated(t *testing.T) {
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

	if result.Count() != 1 {
		t.Errorf("Count = %d, want 1 (matching env)", result.Count())
	}
	if !result.Truncated() {
		t.Errorf("Truncated = false, want true (cache is truncated)")
	}
}

func TestRelated_Secrets_EB_WrongRawStruct(t *testing.T) {
	res := resource.Resource{
		ID:        "prod/db/password",
		RawStruct: 99,
	}

	checker := secretsCheckerByTarget(t, "eb")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.State() != domain.RelatedUnknown {
		t.Errorf("Count = %d, want -1 (wrong RawStruct)", result.Count())
	}
}

func TestRelated_Secrets_ECSTask_MatchBySecretsValueFrom(t *testing.T) {
	const secretARN = "arn:aws:secretsmanager:us-east-1:123456789012:secret:prod/db/password"
	const taskDefARN = "arn:aws:ecs:us-east-1:123456789012:task-definition/api-task:7"
	const otherTaskDefARN = "arn:aws:ecs:us-east-1:123456789012:task-definition/worker-task:3"

	source := secretsSourceWithARN(secretARN, "prod/db/password")

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

	fakeECS := &fakeECSForSvcPivots{
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

	if result.Count() != 1 {
		t.Errorf("Count = %d, want 1 (one task references the secret)", result.Count())
	}
	if result.Err() != nil {
		t.Errorf("unexpected error: %v", result.Err())
	}
}

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

	if result.Count() != 1 {
		t.Errorf("Count = %d, want 1", result.Count())
	}
	if !result.Truncated() {
		t.Errorf("Truncated = false, want true (cache is truncated)")
	}
}

func TestRelated_Secrets_ECSTask_WrongRawStruct(t *testing.T) {
	res := resource.Resource{
		ID:        "prod/db/password",
		RawStruct: "not-a-secret",
	}

	checker := secretsCheckerByTarget(t, "ecs-task")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.State() != domain.RelatedUnknown {
		t.Errorf("Count = %d, want -1 (wrong RawStruct)", result.Count())
	}
}

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

	if result.Count() != 1 {
		t.Errorf("Count = %d, want 1 (one log group for rotation Lambda)", result.Count())
	}
	if len(result.ResourceIDs()) != 1 || result.ResourceIDs()[0] != expectedLogGroup {
		t.Errorf("ResourceIDs = %v, want [%s]", result.ResourceIDs(), expectedLogGroup)
	}
	if result.Err() != nil {
		t.Errorf("unexpected error: %v", result.Err())
	}
}

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

	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0 (no rotation Lambda)", result.Count())
	}
	if result.Err() != nil {
		t.Errorf("unexpected error: %v", result.Err())
	}
}

func TestRelated_Secrets_Logs_WrongRawStruct(t *testing.T) {
	res := resource.Resource{
		ID:        "prod/docdb/password",
		RawStruct: 0,
	}

	checker := secretsCheckerByTarget(t, "logs")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.State() != domain.RelatedUnknown {
		t.Errorf("Count = %d, want -1 (wrong RawStruct)", result.Count())
	}
}

func TestRelated_Secrets_Role_MatchByResourcePolicy(t *testing.T) {
	const secretARN = "arn:aws:secretsmanager:us-east-1:123456789012:secret:prod/db/password"
	const role1ARN = "arn:aws:iam::123456789012:role/api-service-role"
	const role2ARN = "arn:aws:iam::123456789012:role/batch-processor-role"

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

	if result.Count() < 2 {
		t.Errorf("Count = %d, want >= 2 (two role ARNs in resource policy)", result.Count())
	}
	// IDs must be bare role names (== role.ID / iam:GetRole RoleName), never
	// full ARNs — a full ARN fails GetRole with ValidationError.
	roleFound := map[string]bool{"api-service-role": false, "batch-processor-role": false}
	for _, id := range result.ResourceIDs() {
		if _, ok := roleFound[id]; ok {
			roleFound[id] = true
		}
	}
	for name, found := range roleFound {
		if !found {
			t.Errorf("expected bare role name %q in ResourceIDs, got %v", name, result.ResourceIDs())
		}
	}
	if result.Err() != nil {
		t.Errorf("unexpected error: %v", result.Err())
	}
}

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

	if result.Count() < 1 {
		t.Errorf("Count = %d, want >= 1 (rotation Lambda execution role)", result.Count())
	}
	const lambdaRoleName = "rotate-db-creds-execution-role"
	if !slices.Contains(result.ResourceIDs(), lambdaRoleName) {
		t.Errorf("expected rotation Lambda role name %q in ResourceIDs, got %v", lambdaRoleName, result.ResourceIDs())
	}
	if result.Err() != nil {
		t.Errorf("unexpected error: %v", result.Err())
	}
}

// Only a same-account role is fetchable via iam:GetRole here, so a
// cross-account principal is dropped: never counted and never passed to
// FetchByIDs. A full ARN of any account fails GetRole with ValidationError.
func TestRelated_Secrets_Role_CrossAccountExcluded(t *testing.T) {
	const secretARN = "arn:aws:secretsmanager:us-east-1:123456789012:secret:prod/app/creds"
	const localRoleARN = "arn:aws:iam::123456789012:role/local-access-role"
	const foreignRoleARN = "arn:aws:iam::210987654321:role/foreign-access-role"

	policyJSON := `{
		"Version": "2012-10-17",
		"Statement": [
			{
				"Effect": "Allow",
				"Principal": {
					"AWS": [
						"` + localRoleARN + `",
						"` + foreignRoleARN + `"
					]
				},
				"Action": ["secretsmanager:GetSecretValue"],
				"Resource": "*"
			}
		]
	}`

	source := secretsSourceWithARN(secretARN, "prod/app/creds")
	fakeSM := newFakeSecretsManagerWithResourcePolicy(policyJSON)
	clients := &awsclient.ServiceClients{SecretsManager: fakeSM}

	checker := secretsCheckerByTarget(t, "role")
	result := checker(context.Background(), clients, source, resource.ResourceCache{})

	if result.Err() != nil {
		t.Fatalf("unexpected error: %v", result.Err())
	}
	if result.Count() != 1 {
		t.Fatalf("Count = %d, want 1 (same-account role only; cross-account dropped)", result.Count())
	}
	if len(result.ResourceIDs()) != 1 || result.ResourceIDs()[0] != "local-access-role" {
		t.Fatalf("ResourceIDs = %v, want [local-access-role] (bare name, cross-account foreign-access-role excluded)", result.ResourceIDs())
	}
}

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

	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0 (no resource policy, no rotation Lambda)", result.Count())
	}
	if result.Err() != nil {
		t.Errorf("unexpected error: %v", result.Err())
	}
}

func TestRelated_Secrets_Role_WrongRawStruct(t *testing.T) {
	res := resource.Resource{
		ID:        "prod/db/password",
		RawStruct: struct{ X int }{X: 1},
	}

	checker := secretsCheckerByTarget(t, "role")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.State() != domain.RelatedUnknown {
		t.Errorf("Count = %d, want -1 (wrong RawStruct)", result.Count())
	}
}

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

	if result.Count() != 1 {
		t.Errorf("Count = %d, want 1 (rotation Lambda DLQ is SNS topic)", result.Count())
	}
	if len(result.ResourceIDs()) != 1 || result.ResourceIDs()[0] != snsTopicARN {
		t.Errorf("ResourceIDs = %v, want [%s]", result.ResourceIDs(), snsTopicARN)
	}
	if result.Err() != nil {
		t.Errorf("unexpected error: %v", result.Err())
	}
}

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

	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0 (DLQ is SQS, not SNS)", result.Count())
	}
	if result.Err() != nil {
		t.Errorf("unexpected error: %v", result.Err())
	}
}

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

	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0 (no rotation Lambda)", result.Count())
	}
	if result.Err() != nil {
		t.Errorf("unexpected error: %v", result.Err())
	}
}

func TestRelated_Secrets_Sns_WrongRawStruct(t *testing.T) {
	res := resource.Resource{
		ID:        "prod/docdb/password",
		RawStruct: true,
	}

	checker := secretsCheckerByTarget(t, "sns")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.State() != domain.RelatedUnknown {
		t.Errorf("Count = %d, want -1 (wrong RawStruct)", result.Count())
	}
}
