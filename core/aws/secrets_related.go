// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// secrets_related.go contains Secrets Manager related-resource checker functions.
package aws

import (
	"context"
	"strings"

	cfntypes "github.com/aws/aws-sdk-go-v2/service/cloudformation/types"
	cbtypes "github.com/aws/aws-sdk-go-v2/service/codebuild/types"
	rdstypes "github.com/aws/aws-sdk-go-v2/service/rds/types"
	smtypes "github.com/aws/aws-sdk-go-v2/service/secretsmanager/types"

	"github.com/k2m30/a9s/v3/core/resource"
)

// checkSecretsKMS returns the KMS key used to encrypt this secret (Pattern F).
// KmsKeyId is a full ARN (arn:aws:kms:region:account:key/{uuid}); we extract the
// UUID after the last "/" and search the kms cache for a matching resource ID.
func checkSecretsKMS(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	secret, ok := assertStruct[smtypes.SecretListEntry](res.RawStruct)
	if !ok {
		if res.RawStruct == nil {
			return resource.UnknownRelated("kms")
		}
		return resource.KnownRelated("kms", nil, false)
	}
	if secret.KmsKeyId == nil || *secret.KmsKeyId == "" {
		return resource.KnownRelated("kms", nil, false)
	}
	val := *secret.KmsKeyId
	idx := strings.LastIndex(val, "/")
	var keyID string
	switch {
	case idx < 0:
		// Bare key ID (no ARN prefix)
		keyID = val
	case idx == len(val)-1:
		return resource.KnownRelated("kms", nil, false)
	default:
		keyID = val[idx+1:]
	}

	kmsList, truncated, err := relatedResourcesFor(ctx, clients, cache, "kms")
	if err != nil {
		return resource.ErrorRelated("kms", err)
	}
	if kmsList == nil {
		return resource.UnknownRelated("kms")
	}

	var ids []string
	for _, kmsRes := range kmsList {
		if kmsRes.ID == keyID {
			ids = append(ids, kmsRes.ID)
		}
	}
	return relatedResultTrunc("kms", ids, truncated)
}

// checkSecretsLambda returns the Lambda rotation function associated with this
// secret (Pattern F). RotationLambdaARN has the form
// arn:aws:lambda:region:account:function:{name}; we extract the function name
// after the last ":" and search the lambda cache for a matching resource ID.
func checkSecretsLambda(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	secret, ok := assertStruct[smtypes.SecretListEntry](res.RawStruct)
	if !ok {
		if res.RawStruct == nil {
			return resource.UnknownRelated("lambda")
		}
		return resource.KnownRelated("lambda", nil, false)
	}
	if secret.RotationLambdaARN == nil || *secret.RotationLambdaARN == "" {
		return resource.KnownRelated("lambda", nil, false)
	}
	arn := *secret.RotationLambdaARN
	idx := strings.LastIndex(arn, ":")
	if idx < 0 || idx == len(arn)-1 {
		return resource.KnownRelated("lambda", nil, false)
	}
	funcName := arn[idx+1:]

	lambdaList, truncated, err := relatedResourcesFor(ctx, clients, cache, "lambda")
	if err != nil {
		return resource.ErrorRelated("lambda", err)
	}
	if lambdaList == nil {
		return resource.UnknownRelated("lambda")
	}

	var ids []string
	for _, lambdaRes := range lambdaList {
		if lambdaRes.ID == funcName {
			ids = append(ids, lambdaRes.ID)
		}
	}
	return relatedResultTrunc("lambda", ids, truncated)
}

// checkSecretsCFN checks the secret's Tags for aws:cloudformation:stack-name
// and matches against the CFN stack cache.
func checkSecretsCFN(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	stackName := secretsCFNStackName(res)
	if stackName == "" {
		return unreadZero(res, resource.KnownRelated("cfn", nil, false))
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
			continue
		}
		raw, ok := assertStruct[cfntypes.Stack](cfnRes.RawStruct)
		if ok && raw.StackName != nil && *raw.StackName == stackName {
			ids = append(ids, cfnRes.ID)
		}
	}
	return unreadZeroScanned(res, len(cfnList), relatedResultTrunc("cfn", ids, truncated))
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
		return resource.KnownRelated("dbi", nil, false)
	}

	dbiList, truncated, err := relatedResourcesFor(ctx, clients, cache, "dbi")
	if err != nil {
		return resource.ErrorRelated("dbi", err)
	}
	if dbiList == nil {
		return resource.UnknownRelated("dbi")
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
	return relatedResultTrunc("dbi", ids, truncated)
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
		return resource.KnownRelated("cb", nil, false)
	}

	cbList, truncated, err := relatedResourcesFor(ctx, clients, cache, "cb")
	if err != nil {
		return resource.ErrorRelated("cb", err)
	}
	if cbList == nil {
		return resource.UnknownRelated("cb")
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
	return relatedResultTrunc("cb", ids, truncated)
}
