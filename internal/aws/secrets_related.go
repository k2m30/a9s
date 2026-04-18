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
		{TargetType: "eb", DisplayName: "Elastic Beanstalk", Checker: checkSecretsEB, NeedsTargetCache: true},
		{TargetType: "ecs-task", DisplayName: "ECS Tasks", Checker: checkSecretsECSTask, NeedsTargetCache: true},
		{TargetType: "logs", DisplayName: "Log Groups", Checker: checkSecretsLogs},
		{TargetType: "role", DisplayName: "IAM Roles", Checker: checkSecretsRole},
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
		return resource.RelatedCheckResult{TargetType: "kms", Count: 0}
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
		return resource.ApproximateZero("kms")
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
		return resource.RelatedCheckResult{TargetType: "lambda", Count: 0}
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
		return resource.ApproximateZero("lambda")
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
		return resource.ApproximateZero("cfn")
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
		return resource.ApproximateZero("dbi")
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
		return resource.ApproximateZero("cb")
	}
	return relatedResult("cb", ids)
}
