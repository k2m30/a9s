// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// cb_related.go contains CodeBuild related-resource checker functions.
package aws

import (
	"context"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	cbtypes "github.com/aws/aws-sdk-go-v2/service/codebuild/types"

	"github.com/k2m30/a9s/v3/core/resource"
)

// checkCbRole extracts the ServiceRole ARN from the CodeBuild Project RawStruct.
// It extracts the role name from the last path segment of the ARN (after the last "/")
// and searches the role cache by name.
func checkCbRole(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	project, ok := assertStruct[cbtypes.Project](res.RawStruct)
	if !ok {
		return resource.UnknownRelated("role")
	}
	if project.ServiceRole == nil || *project.ServiceRole == "" {
		return resource.KnownRelated("role", nil, false)
	}
	// In-body: the project's ServiceRole ARN normalizes to the role name (== the
	// role's Resource.ID). Resolve by identity — no role-list fetch.
	return relatedRefs("role", []string{*project.ServiceRole}, refContext(clients, cache, "role"))
}

// checkCbLogs searches the logs cache for the CloudWatch log group associated
// with this CodeBuild project.
// Uses explicit LogsConfig.CloudWatchLogs.GroupName if set, otherwise the
// naming convention /aws/codebuild/{projectName}.
func checkCbLogs(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	project, ok := assertStruct[cbtypes.Project](res.RawStruct)
	if !ok {
		return resource.UnknownRelated("logs")
	}

	expectedLogGroup := "/aws/codebuild/" + res.ID
	if project.LogsConfig != nil &&
		project.LogsConfig.CloudWatchLogs != nil &&
		project.LogsConfig.CloudWatchLogs.GroupName != nil &&
		*project.LogsConfig.CloudWatchLogs.GroupName != "" {
		expectedLogGroup = *project.LogsConfig.CloudWatchLogs.GroupName
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

// checkCbSG extracts security group IDs from the CodeBuild Project's VpcConfig.
func checkCbSG(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	project, ok := assertStruct[cbtypes.Project](res.RawStruct)
	if !ok {
		return resource.UnknownRelated("sg")
	}
	if project.VpcConfig == nil {
		return resource.KnownRelated("sg", nil, false)
	}
	var ids []string
	for _, sgID := range project.VpcConfig.SecurityGroupIds {
		if sgID != "" {
			ids = append(ids, sgID)
		}
	}
	return relatedResult("sg", ids)
}

// checkCbVPC returns the VPC this CodeBuild project runs in.
// Reads Project.VpcConfig.VpcId from the RawStruct.
// Returns Count: 0 for projects not configured with VPC access.
func checkCbVPC(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	project, ok := assertStruct[cbtypes.Project](res.RawStruct)
	if !ok {
		return resource.UnknownRelated("vpc")
	}
	if project.VpcConfig == nil || project.VpcConfig.VpcId == nil || *project.VpcConfig.VpcId == "" {
		return resource.KnownRelated("vpc", nil, false)
	}
	return relatedResult("vpc", []string{*project.VpcConfig.VpcId})
}

// checkCbKMS extracts the KMS key from the CodeBuild Project's EncryptionKey field.
// EncryptionKey is a KMS key ARN or alias ARN. Returns the key ID (last segment after "/").
func checkCbKMS(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	project, ok := assertStruct[cbtypes.Project](res.RawStruct)
	if !ok || project.EncryptionKey == nil || *project.EncryptionKey == "" {
		if res.RawStruct == nil {
			return resource.UnknownRelated("kms")
		}
		return resource.KnownRelated("kms", nil, false)
	}
	keyID := kmsRefFromField(*project.EncryptionKey, res.Type)
	return kmsRelated(ctx, clients, cache, []string{keyID})
}

// checkCbSubnet extracts subnet IDs from cbtypes.Project.VpcConfig.Subnets.
func checkCbSubnet(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	project, ok := assertStruct[cbtypes.Project](res.RawStruct)
	if !ok {
		return resource.UnknownRelated("subnet")
	}
	if project.VpcConfig == nil {
		return resource.KnownRelated("subnet", nil, false)
	}
	var ids []string
	for _, sid := range project.VpcConfig.Subnets {
		if sid != "" {
			ids = append(ids, sid)
		}
	}
	return relatedResult("subnet", ids)
}

// checkCbAlarm scans the alarm cache for CloudWatch alarms with a "ProjectName"
// dimension matching this project's name.
func checkCbAlarm(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	return alarmIDsByDimension(ctx, clients, cache, "AWS/CodeBuild", "ProjectName", res.ID)
}

// checkCbECR maps the CodeBuild project's build image to an ECR repository when the
// Environment.Image references an ECR URI.
func checkCbECR(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	project, ok := assertStruct[cbtypes.Project](res.RawStruct)
	if !ok {
		return resource.UnknownRelated("ecr")
	}

	if project.Environment == nil || project.Environment.Image == nil || !strings.Contains(*project.Environment.Image, ".dkr.ecr.") {
		return resource.KnownRelated("ecr", nil, false)
	}

	ecrList, truncated, err := relatedResourcesFor(ctx, clients, cache, "ecr")
	if err != nil {
		return resource.ErrorRelated("ecr", err)
	}
	if ecrList == nil {
		return resource.UnknownRelated("ecr")
	}
	ids, dropped := listedRefs("ecr", []string{*project.Environment.Image}, refContext(clients, cache, "ecr"), ecrList)
	return relatedResultTrunc("ecr", ids, truncated || dropped)
}

// checkCbS3 scans Artifacts/SecondaryArtifacts/Source for S3 bucket locations and
// matches against the S3 cache.
func checkCbS3(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	project, ok := assertStruct[cbtypes.Project](res.RawStruct)
	if !ok {
		return resource.UnknownRelated("s3")
	}

	var locations []string
	if a := project.Artifacts; a != nil && a.Type == cbtypes.ArtifactsTypeS3 && a.Location != nil {
		locations = append(locations, *a.Location)
	}
	for i := range project.SecondaryArtifacts {
		a := project.SecondaryArtifacts[i]
		if a.Type == cbtypes.ArtifactsTypeS3 && a.Location != nil {
			locations = append(locations, *a.Location)
		}
	}
	if s := project.Source; s != nil && s.Type == cbtypes.SourceTypeS3 && s.Location != nil {
		locations = append(locations, *s.Location)
	}

	if len(locations) == 0 {
		return resource.KnownRelated("s3", nil, false)
	}

	s3List, truncated, err := relatedResourcesFor(ctx, clients, cache, "s3")
	if err != nil {
		return resource.ErrorRelated("s3", err)
	}
	if s3List == nil {
		return resource.UnknownRelated("s3")
	}
	ids, _ := listedRefs("s3", locations, refContext(clients, cache, "s3"), s3List)
	return relatedResultTrunc("s3", ids, truncated)
}

// checkCbSecrets extracts Secrets Manager secret references from project environment
// variables (Type=SECRETS_MANAGER). The Value is either the secret name or an ARN
// with an optional ":json-key" suffix.
func checkCbSecrets(_ context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	project, ok := assertStruct[cbtypes.Project](res.RawStruct)
	if !ok {
		return resource.UnknownRelated("secrets")
	}
	if project.Environment == nil {
		return resource.KnownRelated("secrets", nil, false)
	}
	var refs []string
	for _, env := range project.Environment.EnvironmentVariables {
		if env.Type == cbtypes.EnvironmentVariableTypeSecretsManager {
			refs = append(refs, aws.ToString(env.Value))
		}
	}
	return relatedRefs("secrets", refs, refContext(clients, cache, "secrets"))
}

// checkCbSSM extracts SSM parameter references from project environment variables
// (Type=PARAMETER_STORE).
func checkCbSSM(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	project, ok := assertStruct[cbtypes.Project](res.RawStruct)
	if !ok {
		return resource.UnknownRelated("ssm")
	}
	if project.Environment == nil {
		return resource.KnownRelated("ssm", nil, false)
	}
	var ids []string
	for _, env := range project.Environment.EnvironmentVariables {
		if env.Type != cbtypes.EnvironmentVariableTypeParameterStore || env.Value == nil {
			continue
		}
		if *env.Value != "" {
			ids = append(ids, *env.Value)
		}
	}
	return relatedResult("ssm", ids)
}
