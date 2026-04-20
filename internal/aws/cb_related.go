// cb_related.go contains CodeBuild related-resource checker functions.
package aws

import (
	"context"
	"strings"

	cwtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	cbtypes "github.com/aws/aws-sdk-go-v2/service/codebuild/types"

	"github.com/k2m30/a9s/v3/internal/resource"
)

// checkCbRole extracts the ServiceRole ARN from the CodeBuild Project RawStruct.
// It extracts the role name from the last path segment of the ARN (after the last "/")
// and searches the role cache by name.
// Pattern F — forward field lookup.
func checkCbRole(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	project, ok := assertStruct[cbtypes.Project](res.RawStruct)
	if !ok {
		return resource.RelatedCheckResult{TargetType: "role", Count: -1}
	}
	if project.ServiceRole == nil || *project.ServiceRole == "" {
		return resource.RelatedCheckResult{TargetType: "role", Count: 0}
	}
	roleARN := *project.ServiceRole
	roleName := roleARN
	if idx := strings.LastIndex(roleARN, "/"); idx >= 0 && idx < len(roleARN)-1 {
		roleName = roleARN[idx+1:]
	}

	roleList, _, err := cbRelatedResources(ctx, clients, cache, "role")
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "role", Count: -1, Err: err}
	}
	if roleList == nil {
		return resource.RelatedCheckResult{TargetType: "role", Count: -1}
	}

	var ids []string
	for _, roleRes := range roleList {
		if roleRes.Name == roleName || roleRes.ID == roleName {
			ids = append(ids, roleRes.ID)
		}
	}
	return relatedResult("role", ids)
}

// checkCbLogs searches the logs cache for the CloudWatch log group associated
// with this CodeBuild project.
// Pattern F+N — uses explicit LogsConfig.CloudWatchLogs.GroupName if set,
// otherwise falls back to naming convention: /aws/codebuild/{projectName}.
func checkCbLogs(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	project, ok := assertStruct[cbtypes.Project](res.RawStruct)
	if !ok {
		return resource.RelatedCheckResult{TargetType: "logs", Count: -1}
	}

	// Determine expected log group name: explicit config or naming convention.
	expectedLogGroup := "/aws/codebuild/" + res.ID
	if project.LogsConfig != nil &&
		project.LogsConfig.CloudWatchLogs != nil &&
		project.LogsConfig.CloudWatchLogs.GroupName != nil &&
		*project.LogsConfig.CloudWatchLogs.GroupName != "" {
		expectedLogGroup = *project.LogsConfig.CloudWatchLogs.GroupName
	}

	logList, truncated, err := cbRelatedResources(ctx, clients, cache, "logs")
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
		return resource.ApproximateZero("logs")
	}
	return relatedResult("logs", ids)
}

// checkCbSG extracts security group IDs from the CodeBuild Project's VpcConfig.
// Pattern F — no cache needed.
func checkCbSG(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	project, ok := assertStruct[cbtypes.Project](res.RawStruct)
	if !ok {
		return resource.RelatedCheckResult{TargetType: "sg", Count: -1}
	}
	if project.VpcConfig == nil {
		return resource.RelatedCheckResult{TargetType: "sg", Count: 0}
	}
	var ids []string
	for _, sgID := range project.VpcConfig.SecurityGroupIds {
		if sgID != "" {
			ids = append(ids, sgID)
		}
	}
	return relatedResult("sg", ids)
}

// checkCbVPC returns the VPC this CodeBuild project runs in (Pattern R).
// Reads Project.VpcConfig.VpcId from the RawStruct.
// Returns Count: 0 for projects not configured with VPC access.
func checkCbVPC(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	project, ok := assertStruct[cbtypes.Project](res.RawStruct)
	if !ok {
		return resource.RelatedCheckResult{TargetType: "vpc", Count: -1}
	}
	if project.VpcConfig == nil || project.VpcConfig.VpcId == nil || *project.VpcConfig.VpcId == "" {
		return resource.RelatedCheckResult{TargetType: "vpc", Count: 0}
	}
	return relatedResult("vpc", []string{*project.VpcConfig.VpcId})
}

// checkCbKMS extracts the KMS key from the CodeBuild Project's EncryptionKey field.
// EncryptionKey is a KMS key ARN or alias ARN. Returns the key ID (last segment after "/").
// Pattern F — no cache needed.
func checkCbKMS(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	project, ok := assertStruct[cbtypes.Project](res.RawStruct)
	if !ok || project.EncryptionKey == nil || *project.EncryptionKey == "" {
		return resource.RelatedCheckResult{TargetType: "kms", Count: 0}
	}
	keyID := *project.EncryptionKey
	if idx := strings.LastIndex(keyID, "/"); idx >= 0 && idx < len(keyID)-1 {
		keyID = keyID[idx+1:]
	}
	return relatedResult("kms", []string{keyID})
}

// checkCbSubnet extracts subnet IDs from cbtypes.Project.VpcConfig.Subnets.
// Pattern F — no cache needed.
func checkCbSubnet(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	project, ok := assertStruct[cbtypes.Project](res.RawStruct)
	if !ok {
		return resource.RelatedCheckResult{TargetType: "subnet", Count: -1}
	}
	if project.VpcConfig == nil {
		return resource.RelatedCheckResult{TargetType: "subnet", Count: 0}
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
// dimension matching this project's name. Pattern D.
func checkCbAlarm(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	projectName := res.ID
	if projectName == "" {
		return resource.RelatedCheckResult{TargetType: "alarm", Count: 0}
	}
	alarmList, truncated, err := cbRelatedResources(ctx, clients, cache, "alarm")
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "alarm", Count: -1, Err: err}
	}
	if alarmList == nil {
		return resource.RelatedCheckResult{TargetType: "alarm", Count: -1}
	}
	var ids []string
	for _, a := range alarmList {
		alarm, ok := assertStruct[cwtypes.MetricAlarm](a.RawStruct)
		if !ok {
			continue
		}
		if alarm.Namespace == nil || *alarm.Namespace != "AWS/CodeBuild" {
			continue
		}
		for _, d := range alarm.Dimensions {
			if d.Name != nil && *d.Name == "ProjectName" && d.Value != nil && *d.Value == projectName {
				ids = append(ids, a.ID)
				break
			}
		}
	}
	if len(ids) == 0 && truncated {
		return resource.ApproximateZero("alarm")
	}
	return relatedResult("alarm", ids)
}

// checkCbECR maps the CodeBuild project's build image to an ECR repository when the
// Environment.Image references an ECR URI. Pattern F+C.
//
// ECR URIs look like: {account}.dkr.ecr.{region}.amazonaws.com/{repo}[:tag|@sha256:...]
func checkCbECR(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	project, ok := assertStruct[cbtypes.Project](res.RawStruct)
	if !ok {
		return resource.RelatedCheckResult{TargetType: "ecr", Count: -1}
	}

	candidates := map[string]struct{}{}
	if project.Environment != nil && project.Environment.Image != nil {
		if name := cbRepoNameFromImage(*project.Environment.Image); name != "" {
			candidates[name] = struct{}{}
		}
	}
	if len(candidates) == 0 {
		return resource.RelatedCheckResult{TargetType: "ecr", Count: 0}
	}

	ecrList, truncated, err := cbRelatedResources(ctx, clients, cache, "ecr")
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "ecr", Count: -1, Err: err}
	}
	if ecrList == nil {
		return resource.RelatedCheckResult{TargetType: "ecr", Count: -1}
	}
	var ids []string
	for _, r := range ecrList {
		if _, hit := candidates[r.ID]; hit {
			ids = append(ids, r.ID)
			continue
		}
		if _, hit := candidates[r.Name]; hit {
			ids = append(ids, r.ID)
		}
	}
	if len(ids) == 0 && truncated {
		return resource.ApproximateZero("ecr")
	}
	return relatedResult("ecr", ids)
}

// cbRepoNameFromImage parses an ECR image URI and returns the repo name.
// Returns "" if the image does not appear to reference ECR.
func cbRepoNameFromImage(img string) string {
	if !strings.Contains(img, ".dkr.ecr.") {
		return ""
	}
	slash := strings.Index(img, "/")
	if slash < 0 || slash == len(img)-1 {
		return ""
	}
	tail := img[slash+1:]
	if colon := strings.IndexAny(tail, ":@"); colon > 0 {
		tail = tail[:colon]
	}
	return tail
}

// checkCbS3 scans Artifacts/SecondaryArtifacts/Source for S3 bucket locations and
// matches against the S3 cache. Pattern F+C.
func checkCbS3(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	project, ok := assertStruct[cbtypes.Project](res.RawStruct)
	if !ok {
		return resource.RelatedCheckResult{TargetType: "s3", Count: -1}
	}

	buckets := map[string]struct{}{}
	addBucketFrom := func(loc string) {
		if loc == "" {
			return
		}
		if idx := strings.Index(loc, "/"); idx > 0 {
			loc = loc[:idx]
		}
		buckets[loc] = struct{}{}
	}
	if a := project.Artifacts; a != nil && a.Type == cbtypes.ArtifactsTypeS3 && a.Location != nil {
		addBucketFrom(*a.Location)
	}
	for i := range project.SecondaryArtifacts {
		a := project.SecondaryArtifacts[i]
		if a.Type == cbtypes.ArtifactsTypeS3 && a.Location != nil {
			addBucketFrom(*a.Location)
		}
	}
	if s := project.Source; s != nil && s.Type == cbtypes.SourceTypeS3 && s.Location != nil {
		addBucketFrom(*s.Location)
	}

	if len(buckets) == 0 {
		return resource.RelatedCheckResult{TargetType: "s3", Count: 0}
	}

	s3List, truncated, err := cbRelatedResources(ctx, clients, cache, "s3")
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "s3", Count: -1, Err: err}
	}
	if s3List == nil {
		return resource.RelatedCheckResult{TargetType: "s3", Count: -1}
	}
	var ids []string
	for _, b := range s3List {
		if _, hit := buckets[b.ID]; hit {
			ids = append(ids, b.ID)
			continue
		}
		if _, hit := buckets[b.Name]; hit {
			ids = append(ids, b.ID)
		}
	}
	if len(ids) == 0 && truncated {
		return resource.ApproximateZero("s3")
	}
	return relatedResult("s3", ids)
}

// checkCbSecrets extracts Secrets Manager secret references from project environment
// variables (Type=SECRETS_MANAGER). The Value is either the secret name or an ARN
// with an optional ":json-key" suffix. Pattern F.
func checkCbSecrets(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	project, ok := assertStruct[cbtypes.Project](res.RawStruct)
	if !ok {
		return resource.RelatedCheckResult{TargetType: "secrets", Count: -1}
	}
	if project.Environment == nil {
		return resource.RelatedCheckResult{TargetType: "secrets", Count: 0}
	}
	var ids []string
	for _, env := range project.Environment.EnvironmentVariables {
		if env.Type != cbtypes.EnvironmentVariableTypeSecretsManager || env.Value == nil {
			continue
		}
		name := *env.Value
		if strings.HasPrefix(name, "arn:") {
			if sec := strings.Index(name, ":secret:"); sec >= 0 {
				tail := name[sec+len(":secret:"):]
				if colon := strings.Index(tail, ":"); colon >= 0 {
					tail = tail[:colon]
				}
				name = tail
			}
		}
		if name != "" {
			ids = append(ids, name)
		}
	}
	return relatedResult("secrets", ids)
}

// checkCbSSM extracts SSM parameter references from project environment variables
// (Type=PARAMETER_STORE). Pattern F.
func checkCbSSM(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	project, ok := assertStruct[cbtypes.Project](res.RawStruct)
	if !ok {
		return resource.RelatedCheckResult{TargetType: "ssm", Count: -1}
	}
	if project.Environment == nil {
		return resource.RelatedCheckResult{TargetType: "ssm", Count: 0}
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

// cbRelatedResources returns the resource list for target from cache or by fetching the first page.
func cbRelatedResources(ctx context.Context, clients any, cache resource.ResourceCache, target string) ([]resource.Resource, bool, error) {
	resources, isTruncated, err := FetchRelatedTarget(ctx, clients, cache, target)
	if err != nil {
		if _, ok := clients.(*ServiceClients); !ok {
			return nil, false, nil
		}
	}
	return resources, isTruncated, err
}
