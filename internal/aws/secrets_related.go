// secrets_related.go contains Secrets Manager related-resource checker functions.
package aws

import (
	"context"
	"strings"

	cfntypes "github.com/aws/aws-sdk-go-v2/service/cloudformation/types"
	cbtypes "github.com/aws/aws-sdk-go-v2/service/codebuild/types"
	rdstypes "github.com/aws/aws-sdk-go-v2/service/rds/types"
	smtypes "github.com/aws/aws-sdk-go-v2/service/secretsmanager/types"

	"github.com/k2m30/a9s/v3/internal/resource"
)

func init() {
	resource.RegisterRelated("secrets", []resource.RelatedDef{
		{TargetType: "kms", DisplayName: "KMS Keys", Checker: checkSecretsKMS, NeedsTargetCache: true},
		{TargetType: "lambda", DisplayName: "Lambda (rotation)", Checker: checkSecretsLambda, NeedsTargetCache: true},
		{TargetType: "cfn", DisplayName: "CloudFormation", Checker: checkSecretsCFN, NeedsTargetCache: true},
		{TargetType: "dbi", DisplayName: "RDS Instances", Checker: checkSecretsDBI, NeedsTargetCache: true},
		{TargetType: "cb", DisplayName: "CodeBuild Projects", Checker: checkSecretsCB, NeedsTargetCache: true},
		{TargetType: "codeartifact", DisplayName: "CodeArtifact Domains", Checker: checkSecretsCodeArtifact},
		{TargetType: "eb", DisplayName: "Elastic Beanstalk", Checker: checkSecretsEB},
		{TargetType: "ecr", DisplayName: "ECR Repositories", Checker: checkSecretsECR},
		{TargetType: "ecs-task", DisplayName: "ECS Tasks", Checker: checkSecretsECSTask},
		{TargetType: "logs", DisplayName: "Log Groups", Checker: checkSecretsLogs},
		{TargetType: "pipeline", DisplayName: "CodePipelines", Checker: checkSecretsPipeline},
		{TargetType: "role", DisplayName: "IAM Roles", Checker: checkSecretsRole},
		{TargetType: "s3", DisplayName: "S3 Buckets", Checker: checkSecretsS3},
		{TargetType: "sns", DisplayName: "SNS Topics", Checker: checkSecretsSNS},
	})

	// smtypes.SecretListEntry: KmsKeyId (full ARN — UI resolves UUID suffix to kms cache),
	// RotationLambdaARN (full ARN — UI resolves function name suffix to lambda cache)
	resource.RegisterNavigableFields("secrets", []resource.NavigableField{
		{FieldPath: "KmsKeyId", TargetType: "kms"},
		{FieldPath: "RotationLambdaARN", TargetType: "lambda"},
	})
}

// checkSecretsKMS returns the KMS key used to encrypt this secret (Pattern F).
// KmsKeyId is a full ARN (arn:aws:kms:region:account:key/{uuid}); we extract the
// UUID after the last "/" and search the kms cache for a matching resource ID.
func checkSecretsKMS(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	secret, ok := assertStruct[smtypes.SecretListEntry](res.RawStruct)
	if !ok {
		return resource.RelatedCheckResult{TargetType: "kms", Count: -1}
	}
	if secret.KmsKeyId == nil || *secret.KmsKeyId == "" {
		return resource.RelatedCheckResult{TargetType: "kms", Count: 0}
	}
	val := *secret.KmsKeyId
	idx := strings.LastIndex(val, "/")
	var keyID string
	switch {
	case idx < 0:
		// Bare key ID (no ARN prefix)
		keyID = val
	case idx == len(val)-1:
		return resource.RelatedCheckResult{TargetType: "kms", Count: 0}
	default:
		keyID = val[idx+1:]
	}

	kmsList, truncated, err := secretsRelatedResources(ctx, clients, cache, "kms")
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "kms", Count: -1, Err: err}
	}
	if kmsList == nil {
		return resource.RelatedCheckResult{TargetType: "kms", Count: -1}
	}

	var ids []string
	for _, kmsRes := range kmsList {
		if kmsRes.ID == keyID {
			ids = append(ids, kmsRes.ID)
		}
	}
	if len(ids) == 0 && truncated {
		return resource.RelatedCheckResult{TargetType: "kms", Count: -1}
	}
	return relatedResult("kms", ids)
}

// checkSecretsLambda returns the Lambda rotation function associated with this
// secret (Pattern F). RotationLambdaARN has the form
// arn:aws:lambda:region:account:function:{name}; we extract the function name
// after the last ":" and search the lambda cache for a matching resource ID.
func checkSecretsLambda(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	secret, ok := assertStruct[smtypes.SecretListEntry](res.RawStruct)
	if !ok {
		return resource.RelatedCheckResult{TargetType: "lambda", Count: -1}
	}
	if secret.RotationLambdaARN == nil || *secret.RotationLambdaARN == "" {
		return resource.RelatedCheckResult{TargetType: "lambda", Count: 0}
	}
	arn := *secret.RotationLambdaARN
	idx := strings.LastIndex(arn, ":")
	if idx < 0 || idx == len(arn)-1 {
		return resource.RelatedCheckResult{TargetType: "lambda", Count: 0}
	}
	funcName := arn[idx+1:]

	lambdaList, truncated, err := secretsRelatedResources(ctx, clients, cache, "lambda")
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "lambda", Count: -1, Err: err}
	}
	if lambdaList == nil {
		return resource.RelatedCheckResult{TargetType: "lambda", Count: -1}
	}

	var ids []string
	for _, lambdaRes := range lambdaList {
		if lambdaRes.ID == funcName {
			ids = append(ids, lambdaRes.ID)
		}
	}
	if len(ids) == 0 && truncated {
		return resource.RelatedCheckResult{TargetType: "lambda", Count: -1}
	}
	return relatedResult("lambda", ids)
}

// checkSecretsCFN checks the secret's Tags for aws:cloudformation:stack-name
// and matches against the CFN stack cache.
func checkSecretsCFN(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	stackName := secretsCFNStackName(res)
	if stackName == "" {
		return resource.RelatedCheckResult{TargetType: "cfn", Count: 0}
	}

	cfnList, truncated, err := secretsRelatedResources(ctx, clients, cache, "cfn")
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
			continue
		}
		raw, ok := assertStruct[cfntypes.Stack](cfnRes.RawStruct)
		if ok && raw.StackName != nil && *raw.StackName == stackName {
			ids = append(ids, cfnRes.ID)
		}
	}
	if len(ids) == 0 && truncated {
		return resource.RelatedCheckResult{TargetType: "cfn", Count: -1}
	}
	return relatedResult("cfn", ids)
}

// secretsCFNStackName extracts the aws:cloudformation:stack-name tag value from
// the secret's Tags slice.
func secretsCFNStackName(res resource.Resource) string {
	secret, ok := assertStruct[smtypes.SecretListEntry](res.RawStruct)
	if !ok {
		return ""
	}
	for _, tag := range secret.Tags {
		if tag.Key != nil && *tag.Key == "aws:cloudformation:stack-name" && tag.Value != nil {
			return *tag.Value
		}
	}
	return ""
}

// checkSecretsDBI does a reverse lookup — scans the dbi cache for DBInstance
// entries whose MasterUserSecret.SecretArn matches this secret's ARN (Pattern C).
func checkSecretsDBI(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	secretARN := ""
	if raw, ok := assertStruct[smtypes.SecretListEntry](res.RawStruct); ok && raw.ARN != nil {
		secretARN = *raw.ARN
	}
	if secretARN == "" {
		secretARN = res.Fields["arn"]
	}
	if secretARN == "" {
		return resource.RelatedCheckResult{TargetType: "dbi", Count: 0}
	}

	dbiList, truncated, err := secretsRelatedResources(ctx, clients, cache, "dbi")
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "dbi", Count: -1, Err: err}
	}
	if dbiList == nil {
		return resource.RelatedCheckResult{TargetType: "dbi", Count: -1}
	}

	var ids []string
	for _, dbRes := range dbiList {
		db, ok := assertStruct[rdstypes.DBInstance](dbRes.RawStruct)
		if !ok {
			continue
		}
		if db.MasterUserSecret == nil || db.MasterUserSecret.SecretArn == nil {
			continue
		}
		if *db.MasterUserSecret.SecretArn == secretARN {
			ids = append(ids, dbRes.ID)
		}
	}
	if len(ids) == 0 && truncated {
		return resource.RelatedCheckResult{TargetType: "dbi", Count: -1}
	}
	return relatedResult("dbi", ids)
}

// secretsRelatedResources returns the resource list for target from cache or by
// fetching the first page via the registered paginated fetcher.
func secretsRelatedResources(ctx context.Context, clients any, cache resource.ResourceCache, target string) ([]resource.Resource, bool, error) {
	resources, isTruncated, err := FetchRelatedTarget(ctx, clients, cache, target)
	if err != nil {
		if _, ok := clients.(*ServiceClients); !ok {
			return nil, false, nil
		}
	}
	return resources, isTruncated, err
}

// secretIdentifiers returns the (arn, name) pair for the source secret,
// preferring the raw struct's ARN/Name over Fields.
func secretIdentifiers(res resource.Resource) (arn, name string) {
	if raw, ok := assertStruct[smtypes.SecretListEntry](res.RawStruct); ok {
		if raw.ARN != nil {
			arn = *raw.ARN
		}
		if raw.Name != nil {
			name = *raw.Name
		}
	}
	if arn == "" {
		arn = res.Fields["arn"]
	}
	if name == "" {
		name = res.Name
	}
	if name == "" {
		name = res.ID
	}
	return arn, name
}

// checkSecretsCB does a reverse lookup — scans the cb (CodeBuild) cache for
// projects whose Environment.EnvironmentVariables contains a SECRETS_MANAGER
// variable whose Value references this secret's ARN or name.
func checkSecretsCB(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	secretARN, secretName := secretIdentifiers(res)
	if secretARN == "" && secretName == "" {
		return resource.RelatedCheckResult{TargetType: "cb", Count: 0}
	}

	cbList, truncated, err := secretsRelatedResources(ctx, clients, cache, "cb")
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "cb", Count: -1, Err: err}
	}
	if cbList == nil {
		return resource.RelatedCheckResult{TargetType: "cb", Count: -1}
	}

	var ids []string
	for _, cbRes := range cbList {
		proj, ok := assertStruct[cbtypes.Project](cbRes.RawStruct)
		if !ok || proj.Environment == nil {
			continue
		}
		for _, ev := range proj.Environment.EnvironmentVariables {
			if ev.Type != cbtypes.EnvironmentVariableTypeSecretsManager || ev.Value == nil {
				continue
			}
			val := *ev.Value
			if (secretARN != "" && val == secretARN) ||
				(secretName != "" && (val == secretName || strings.HasPrefix(val, secretName+":"))) {
				ids = append(ids, cbRes.ID)
				break
			}
		}
	}
	if len(ids) == 0 && truncated {
		return resource.RelatedCheckResult{TargetType: "cb", Count: -1}
	}
	return relatedResult("cb", ids)
}

// checkSecretsCodeArtifact returns Count: -1 (unknown). CodeArtifact domains /
// repositories authenticate via CodeArtifact-issued tokens rather than Secrets
// Manager, and the golden doc's expectation here is that users store
// externally-generated registry credentials as secrets — but that association
// is not exposed in any AWS API response. It would require customer-specific
// naming conventions or tag analysis to discover.
func checkSecretsCodeArtifact(_ context.Context, _ any, _ resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	return resource.RelatedCheckResult{TargetType: "codeartifact", Count: -1}
}

// checkSecretsEB returns Count: -1 (unknown). Elastic Beanstalk environments
// can reference secrets via option settings (platform-specific), but
// DescribeEnvironments does not include option settings in the response.
// Resolving this association requires a DescribeConfigurationSettings call
// per environment which the fetcher does not perform.
func checkSecretsEB(_ context.Context, _ any, _ resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	return resource.RelatedCheckResult{TargetType: "eb", Count: -1}
}

// checkSecretsECR returns Count: -1 (unknown). ECR repositories do not
// natively reference Secrets Manager in any AWS API response. This
// relationship exists only when users manually name registry-auth secrets
// after repositories; there is no deterministic way to resolve it without
// customer-specific conventions.
func checkSecretsECR(_ context.Context, _ any, _ resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	return resource.RelatedCheckResult{TargetType: "ecr", Count: -1}
}

// checkSecretsECSTask returns Count: -1 (unknown). ECS Task containers can
// reference Secrets Manager ARNs via their container definitions'
// `Secrets` array, but that information lives on the *TaskDefinition*, not
// the running ecs-task. Resolving this relationship would require
// DescribeTaskDefinition for each task's taskDefinitionArn — a per-task
// (N+1) API call the fetcher does not perform.
func checkSecretsECSTask(_ context.Context, _ any, _ resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	return resource.RelatedCheckResult{TargetType: "ecs-task", Count: -1}
}

// checkSecretsLogs returns Count: -1 (unknown). CloudWatch Log groups are
// never direct consumers of Secrets Manager secrets; the golden-doc link here
// refers to the rotation-lambda's log group, which we could only resolve by
// (a) looking up the rotation lambda (already covered by secrets→lambda) and
// (b) calling GetFunction to read its LoggingConfig.LogGroup. That two-hop
// chain cannot be answered from the secrets/list response alone.
func checkSecretsLogs(_ context.Context, _ any, _ resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	return resource.RelatedCheckResult{TargetType: "logs", Count: -1}
}

// checkSecretsPipeline returns Count: -1 (unknown). CodePipeline actions can
// reference Secrets Manager via action-configuration parameters, but those
// parameters live on the pipeline's stages/actions which ListPipelines does
// NOT return — only GetPipeline does, per pipeline. Without that N+1 lookup
// we cannot deterministically resolve consumers.
func checkSecretsPipeline(_ context.Context, _ any, _ resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	return resource.RelatedCheckResult{TargetType: "pipeline", Count: -1}
}

// checkSecretsRole returns Count: -1 (unknown). Finding IAM roles that can
// access this secret requires inspecting every role's attached policies for
// Action: secretsmanager:* with Resource matching this secret's ARN. Policy
// evaluation is not represented in the role cache (which stores role
// metadata, not policy documents). Requires an iam:SimulatePrincipalPolicy
// or a per-role ListRolePolicies+GetRolePolicy crawl — neither is performed.
func checkSecretsRole(_ context.Context, _ any, _ resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	return resource.RelatedCheckResult{TargetType: "role", Count: -1}
}

// checkSecretsS3 returns Count: -1 (unknown). S3 buckets do not reference
// Secrets Manager secrets in any AWS API response. Any connection would be
// user-established (e.g. a secret named after a bucket) — not resolvable
// without customer-specific conventions.
func checkSecretsS3(_ context.Context, _ any, _ resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	return resource.RelatedCheckResult{TargetType: "s3", Count: -1}
}

// checkSecretsSNS returns Count: -1 (unknown). SNS topics do not reference
// Secrets Manager secrets. Secrets Manager can publish rotation events to an
// SNS topic, but that configuration lives on the Secrets Manager side as
// part of secret rotation rules — not exposed in ListSecrets. Requires
// DescribeSecret per secret which the fetcher does not do for list entries.
func checkSecretsSNS(_ context.Context, _ any, _ resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	return resource.RelatedCheckResult{TargetType: "sns", Count: -1}
}
