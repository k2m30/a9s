// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// cb_related.go contains CodeBuild related-resource checker functions.
package aws

import (
	"context"

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
		return NotRead("role")
	}
	if project.ServiceRole == nil || *project.ServiceRole == "" {
		return foundNone("role", "project.ServiceRole")
	}
	// In-body: the project's ServiceRole ARN normalizes to the role name (== the
	// role's Resource.ID). Resolve by identity — no role-list fetch.
	return relatedRefs("role", []string{*project.ServiceRole}, refContext(clients, cache, "role"))
}

// checkCbLogs searches the logs cache for the CloudWatch log group associated
// with this CodeBuild project.
// Uses explicit LogsConfig.CloudWatchLogs.GroupName if set, otherwise the
// naming convention /aws/codebuild/{projectName}. With status DISABLED
// "CloudWatch Logs are not enabled for this build project"
// (https://docs.aws.amazon.com/codebuild/latest/APIReference/API_CloudWatchLogsConfig.html).
func checkCbLogs(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	project, ok := assertStruct[cbtypes.Project](res.RawStruct)
	if !ok {
		return NotRead("logs")
	}

	if project.LogsConfig != nil && project.LogsConfig.CloudWatchLogs != nil && project.LogsConfig.CloudWatchLogs.Status == cbtypes.LogsConfigStatusTypeDisabled {
		return foundNone("logs", "LogsConfig.CloudWatchLogs.Status")
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
		return ReadFailed("logs", err)
	}
	if logList == nil {
		return NotRead("logs")
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
		return NotRead("sg")
	}
	if project.VpcConfig == nil {
		return foundNone("sg", "project.VpcConfig")
	}
	var ids []string
	for _, sgID := range project.VpcConfig.SecurityGroupIds {
		if sgID != "" {
			ids = append(ids, sgID)
		}
	}
	return relatedResultTrunc("sg", ids, false)
}

// checkCbVPC returns the VPC this CodeBuild project runs in.
// Reads Project.VpcConfig.VpcId from the RawStruct.
// Returns Count: 0 for projects not configured with VPC access.
func checkCbVPC(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	project, ok := assertStruct[cbtypes.Project](res.RawStruct)
	if !ok {
		return NotRead("vpc")
	}
	if project.VpcConfig == nil || project.VpcConfig.VpcId == nil || *project.VpcConfig.VpcId == "" {
		return foundNone("vpc", "project.VpcConfig.VpcId")
	}
	return relatedResultTrunc("vpc", []string{*project.VpcConfig.VpcId}, false)
}

// checkCbKMS extracts the KMS key from the CodeBuild Project's EncryptionKey field.
// EncryptionKey is a KMS key ARN or alias ARN. Returns the key ID (last segment after "/").
func checkCbKMS(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	project, ok := assertStruct[cbtypes.Project](res.RawStruct)
	if !ok || project.EncryptionKey == nil || *project.EncryptionKey == "" {
		if res.RawStruct == nil {
			return NotRead("kms")
		}
		return foundNone("kms", "project.EncryptionKey")
	}
	keyID := kmsRefFromField(*project.EncryptionKey, res.Type)
	return kmsRelated(ctx, clients, cache, []string{keyID})
}

// checkCbSubnet extracts subnet IDs from cbtypes.Project.VpcConfig.Subnets.
func checkCbSubnet(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	project, ok := assertStruct[cbtypes.Project](res.RawStruct)
	if !ok {
		return NotRead("subnet")
	}
	if project.VpcConfig == nil {
		return foundNone("subnet", "project.VpcConfig")
	}
	var ids []string
	for _, sid := range project.VpcConfig.Subnets {
		if sid != "" {
			ids = append(ids, sid)
		}
	}
	return relatedResultTrunc("subnet", ids, false)
}

// checkCbAlarm scans the alarm cache for CloudWatch alarms with a "ProjectName"
// dimension matching this project's name.
func checkCbAlarm(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	return alarmIDsByDimension(ctx, clients, cache, "cb", res)
}

// checkCbECR maps the CodeBuild project's build image to an ECR repository when the
// Environment.Image references an ECR URI.
func checkCbECR(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	project, ok := assertStruct[cbtypes.Project](res.RawStruct)
	if !ok {
		return NotRead("ecr")
	}

	if project.Environment == nil || aws.ToString(project.Environment.Image) == "" {
		return foundNone("ecr", "project.Environment.Image")
	}
	return ecrWorkloadRepos(ctx, clients, cache, []string{*project.Environment.Image})
}

// checkCbS3 counts the buckets the project reads and writes: its S3 source
// and secondary sources ("<bucket-name>/<path>",
// https://docs.aws.amazon.com/codebuild/latest/APIReference/API_ProjectSource.html),
// its S3 artifacts, and the bucket its S3 build logs go to while enabled
// ("my-bucket/build-log" or the bucket ARN,
// https://docs.aws.amazon.com/codebuild/latest/APIReference/API_S3LogsConfig.html).
func checkCbS3(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	project, ok := assertStruct[cbtypes.Project](res.RawStruct)
	if !ok {
		return NotRead("s3")
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
	sources := project.SecondarySources
	if project.Source != nil {
		sources = append([]cbtypes.ProjectSource{*project.Source}, sources...)
	}
	for _, src := range sources {
		if src.Type == cbtypes.SourceTypeS3 && src.Location != nil {
			locations = append(locations, *src.Location)
		}
	}
	if l := project.LogsConfig; l != nil && l.S3Logs != nil && l.S3Logs.Status == cbtypes.LogsConfigStatusTypeEnabled && l.S3Logs.Location != nil {
		locations = append(locations, *l.S3Logs.Location)
	}

	if len(locations) == 0 {
		return foundNone("s3", "locations")
	}

	s3List, _, err := relatedResourcesFor(ctx, clients, cache, "s3")
	if err != nil {
		return ReadFailed("s3", err)
	}
	if s3List == nil {
		return NotRead("s3")
	}
	ids, lowerBound := listedRefs("s3", locations, refContext(clients, cache, "s3"), s3List)
	return relatedResultTrunc("s3", ids, lowerBound)
}

// checkCbSecrets extracts Secrets Manager secret references from project environment
// variables (Type=SECRETS_MANAGER). The Value is either the secret name or an ARN
// with an optional ":json-key" suffix.
func checkCbSecrets(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	project, ok := assertStruct[cbtypes.Project](res.RawStruct)
	if !ok {
		return NotRead("secrets")
	}
	if project.Environment == nil {
		return foundNone("secrets", "project.Environment")
	}
	var refs []string
	for _, env := range project.Environment.EnvironmentVariables {
		if env.Type == cbtypes.EnvironmentVariableTypeSecretsManager {
			refs = append(refs, aws.ToString(env.Value))
		}
	}
	return listedRelated(ctx, clients, cache, "secrets", refs, false)
}

// checkCbSSM extracts SSM parameter references from project environment variables
// (Type=PARAMETER_STORE).
func checkCbSSM(_ context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	project, ok := assertStruct[cbtypes.Project](res.RawStruct)
	if !ok {
		return NotRead("ssm")
	}
	if project.Environment == nil {
		return foundNone("ssm", "project.Environment")
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
	return relatedRefs("ssm", ids, refContext(clients, cache, "ssm"))
}
