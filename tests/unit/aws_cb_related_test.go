package unit_test

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	cbtypes "github.com/aws/aws-sdk-go-v2/service/codebuild/types"
	cwtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"

	_ "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// TestRelated_CB_Registered verifies all 3 related defs are registered with correct checker presence.
func TestRelated_CB_Registered(t *testing.T) {
	defs := resource.GetRelated("cb")
	if len(defs) == 0 {
		t.Fatal("no related defs registered for cb")
	}

	type expectation struct {
		displayName string
		hasChecker  bool
	}
	expected := map[string]expectation{
		"logs":     {"Log Groups", true},
		"role":     {"IAM Roles", true},
		"pipeline": {"CodePipelines", true},
	}
	for target, want := range expected {
		found := false
		for _, def := range defs {
			if def.TargetType == target {
				found = true
				if want.hasChecker && def.Checker == nil {
					t.Errorf("cb %q: Checker should not be nil", target)
				}
				if !want.hasChecker && def.Checker != nil {
					t.Errorf("cb %q: Checker should be nil (stub)", target)
				}
				if def.DisplayName != want.displayName {
					t.Errorf("cb %q: DisplayName = %q, want %q", target, def.DisplayName, want.displayName)
				}
				break
			}
		}
		if !found {
			t.Errorf("expected related def for target %q not found", target)
		}
	}
}

// cbCheckerByTarget returns the RelatedChecker for the given target type registered
// under "cb". It fails the test immediately if the checker is nil or not found.
func cbCheckerByTarget(t *testing.T, target string) resource.RelatedChecker {
	t.Helper()
	for _, def := range resource.GetRelated("cb") {
		if def.TargetType == target {
			if def.Checker == nil {
				t.Fatalf("cb related checker for %s is nil", target)
			}
			return def.Checker
		}
	}
	t.Fatalf("cb related checker for %s not found", target)
	return nil
}

// --- checkCbRole tests (Pattern F — forward field lookup by ARN last segment) ---

func TestRelated_CB_Role_MatchByServiceRole(t *testing.T) {
	roleRes := resource.Resource{
		ID:     "codebuild-role",
		Name:   "codebuild-role",
		Fields: map[string]string{},
	}
	cache := resource.ResourceCache{
		"role": resource.ResourceCacheEntry{Resources: []resource.Resource{roleRes}},
	}

	res := resource.Resource{
		ID:     "my-project",
		Fields: map[string]string{},
		RawStruct: cbtypes.Project{
			ServiceRole: aws.String("arn:aws:iam::123456789012:role/codebuild-role"),
		},
	}

	checker := cbCheckerByTarget(t, "role")
	result := checker(context.Background(), nil, res, cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != "codebuild-role" {
		t.Errorf("ResourceIDs = %v, want [codebuild-role]", result.ResourceIDs)
	}
	if result.Err != nil {
		t.Errorf("unexpected error: %v", result.Err)
	}
}

func TestRelated_CB_Role_NoMatch(t *testing.T) {
	roleRes := resource.Resource{
		ID:     "different-role",
		Name:   "different-role",
		Fields: map[string]string{},
	}
	cache := resource.ResourceCache{
		"role": resource.ResourceCacheEntry{Resources: []resource.Resource{roleRes}},
	}

	res := resource.Resource{
		ID:     "my-project",
		Fields: map[string]string{},
		RawStruct: cbtypes.Project{
			ServiceRole: aws.String("arn:aws:iam::123456789012:role/codebuild-role"),
		},
	}

	checker := cbCheckerByTarget(t, "role")
	result := checker(context.Background(), nil, res, cache)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0", result.Count)
	}
}

func TestRelated_CB_Role_NilServiceRole(t *testing.T) {
	roleRes := resource.Resource{
		ID:     "codebuild-role",
		Name:   "codebuild-role",
		Fields: map[string]string{},
	}
	cache := resource.ResourceCache{
		"role": resource.ResourceCacheEntry{Resources: []resource.Resource{roleRes}},
	}

	res := resource.Resource{
		ID:     "my-project",
		Fields: map[string]string{},
		RawStruct: cbtypes.Project{
			ServiceRole: nil,
		},
	}

	checker := cbCheckerByTarget(t, "role")
	result := checker(context.Background(), nil, res, cache)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (nil ServiceRole)", result.Count)
	}
}

func TestRelated_CB_Role_NilCache(t *testing.T) {
	cache := resource.ResourceCache{}

	res := resource.Resource{
		ID:     "my-project",
		Fields: map[string]string{},
		RawStruct: cbtypes.Project{
			ServiceRole: aws.String("arn:aws:iam::123456789012:role/codebuild-role"),
		},
	}

	checker := cbCheckerByTarget(t, "role")
	result := checker(context.Background(), nil, res, cache)

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (empty cache, no clients)", result.Count)
	}
}

// --- checkCbLogs tests (Pattern F+N — explicit GroupName or naming convention) ---

func TestRelated_CB_Logs_MatchByExplicitGroupName(t *testing.T) {
	logRes := resource.Resource{
		ID:     "/custom/my-logs",
		Fields: map[string]string{},
	}
	cache := resource.ResourceCache{
		"logs": resource.ResourceCacheEntry{Resources: []resource.Resource{logRes}},
	}

	res := resource.Resource{
		ID:     "my-project",
		Fields: map[string]string{},
		RawStruct: cbtypes.Project{
			LogsConfig: &cbtypes.LogsConfig{
				CloudWatchLogs: &cbtypes.CloudWatchLogsConfig{
					GroupName: aws.String("/custom/my-logs"),
				},
			},
		},
	}

	checker := cbCheckerByTarget(t, "logs")
	result := checker(context.Background(), nil, res, cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != "/custom/my-logs" {
		t.Errorf("ResourceIDs = %v, want [/custom/my-logs]", result.ResourceIDs)
	}
	if result.Err != nil {
		t.Errorf("unexpected error: %v", result.Err)
	}
}

func TestRelated_CB_Logs_MatchByNamingConvention(t *testing.T) {
	logRes := resource.Resource{
		ID:     "/aws/codebuild/my-project",
		Fields: map[string]string{},
	}
	cache := resource.ResourceCache{
		"logs": resource.ResourceCacheEntry{Resources: []resource.Resource{logRes}},
	}

	res := resource.Resource{
		ID:        "my-project",
		Fields:    map[string]string{},
		RawStruct: cbtypes.Project{},
	}

	checker := cbCheckerByTarget(t, "logs")
	result := checker(context.Background(), nil, res, cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != "/aws/codebuild/my-project" {
		t.Errorf("ResourceIDs = %v, want [/aws/codebuild/my-project]", result.ResourceIDs)
	}
	if result.Err != nil {
		t.Errorf("unexpected error: %v", result.Err)
	}
}

func TestRelated_CB_Logs_NoMatch(t *testing.T) {
	logRes := resource.Resource{
		ID:     "/aws/codebuild/other-project",
		Fields: map[string]string{},
	}
	cache := resource.ResourceCache{
		"logs": resource.ResourceCacheEntry{Resources: []resource.Resource{logRes}},
	}

	res := resource.Resource{
		ID:        "my-project",
		Fields:    map[string]string{},
		RawStruct: cbtypes.Project{},
	}

	checker := cbCheckerByTarget(t, "logs")
	result := checker(context.Background(), nil, res, cache)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0", result.Count)
	}
}

func TestRelated_CB_Logs_NilCache(t *testing.T) {
	cache := resource.ResourceCache{}

	res := resource.Resource{
		ID:        "my-project",
		Fields:    map[string]string{},
		RawStruct: cbtypes.Project{},
	}

	checker := cbCheckerByTarget(t, "logs")
	result := checker(context.Background(), nil, res, cache)

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (empty cache, no clients)", result.Count)
	}
}

// --- cb→pipeline tests (undeterminable — pipeline cache lacks stage data) ---

// TestRelated_CB_Pipeline_ReturnsUnknown verifies cb→pipeline reports Count=-1 because
// the pipeline list cache only carries cptypes.PipelineSummary — stages/actions are
// only available via GetPipeline per pipeline, so reverse CodeBuild-project lookup
// cannot be done from cache.
func TestRelated_CB_Pipeline_ReturnsUnknown(t *testing.T) {
	res := resource.Resource{
		ID:     "my-project",
		Fields: map[string]string{},
		RawStruct: cbtypes.Project{
			Name: aws.String("my-project"),
		},
	}
	cache := resource.ResourceCache{
		"pipeline": resource.ResourceCacheEntry{Resources: []resource.Resource{
			{ID: "some-pipeline", Name: "some-pipeline"},
		}},
	}

	checker := cbCheckerByTarget(t, "pipeline")
	result := checker(context.Background(), nil, res, cache)

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (undeterminable — pipeline cache lacks stages)", result.Count)
	}
	if result.TargetType != "pipeline" {
		t.Errorf("TargetType = %q, want %q", result.TargetType, "pipeline")
	}
}

// ---------------------------------------------------------------------------
// checkCbSG — Pattern F: VpcConfig.SecurityGroupIds
// ---------------------------------------------------------------------------

func TestRelated_CB_SG_Match(t *testing.T) {
	res := resource.Resource{
		ID:     "my-project",
		Fields: map[string]string{},
		RawStruct: cbtypes.Project{
			VpcConfig: &cbtypes.VpcConfig{
				SecurityGroupIds: []string{"sg-aaa111", "sg-bbb222"},
			},
		},
	}
	checker := cbCheckerByTarget(t, "sg")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.Count != 2 {
		t.Errorf("Count = %d, want 2", result.Count)
	}
}

func TestRelated_CB_SG_NoVPCConfig(t *testing.T) {
	res := resource.Resource{
		ID:        "my-project",
		Fields:    map[string]string{},
		RawStruct: cbtypes.Project{},
	}
	checker := cbCheckerByTarget(t, "sg")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (no VpcConfig)", result.Count)
	}
}

func TestRelated_CB_SG_WrongRawStruct(t *testing.T) {
	res := resource.Resource{
		ID:        "my-project",
		Fields:    map[string]string{},
		RawStruct: "not-a-project",
	}
	checker := cbCheckerByTarget(t, "sg")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (wrong RawStruct type)", result.Count)
	}
}

// ---------------------------------------------------------------------------
// checkCbVPC — Pattern F: VpcConfig.VpcId
// ---------------------------------------------------------------------------

func TestRelated_CB_VPC_Match(t *testing.T) {
	res := resource.Resource{
		ID:     "my-project",
		Fields: map[string]string{},
		RawStruct: cbtypes.Project{
			VpcConfig: &cbtypes.VpcConfig{
				VpcId: aws.String("vpc-abc123"),
			},
		},
	}
	checker := cbCheckerByTarget(t, "vpc")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) == 0 || result.ResourceIDs[0] != "vpc-abc123" {
		t.Errorf("ResourceIDs = %v, want [vpc-abc123]", result.ResourceIDs)
	}
}

func TestRelated_CB_VPC_NoVPCConfig(t *testing.T) {
	res := resource.Resource{
		ID:        "my-project",
		Fields:    map[string]string{},
		RawStruct: cbtypes.Project{},
	}
	checker := cbCheckerByTarget(t, "vpc")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (no VpcConfig)", result.Count)
	}
}

// ---------------------------------------------------------------------------
// checkCbKMS — Pattern F: EncryptionKey ARN last segment
// ---------------------------------------------------------------------------

func TestRelated_CB_KMS_Match(t *testing.T) {
	res := resource.Resource{
		ID:     "my-project",
		Fields: map[string]string{},
		RawStruct: cbtypes.Project{
			EncryptionKey: aws.String("arn:aws:kms:us-east-1:123456789012:key/a1b2c3d4-5678-abcd-ef01-234567890abc"),
		},
	}
	checker := cbCheckerByTarget(t, "kms")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) == 0 || result.ResourceIDs[0] != "a1b2c3d4-5678-abcd-ef01-234567890abc" {
		t.Errorf("ResourceIDs = %v, want [a1b2c3d4-5678-abcd-ef01-234567890abc]", result.ResourceIDs)
	}
}

func TestRelated_CB_KMS_NoKey(t *testing.T) {
	res := resource.Resource{
		ID:        "my-project",
		Fields:    map[string]string{},
		RawStruct: cbtypes.Project{},
	}
	checker := cbCheckerByTarget(t, "kms")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (no EncryptionKey)", result.Count)
	}
}

// ---------------------------------------------------------------------------
// checkCbSubnet — Pattern F: VpcConfig.Subnets
// ---------------------------------------------------------------------------

func TestRelated_CB_Subnet_Match(t *testing.T) {
	res := resource.Resource{
		ID:     "my-project",
		Fields: map[string]string{},
		RawStruct: cbtypes.Project{
			VpcConfig: &cbtypes.VpcConfig{
				Subnets: []string{"subnet-11111111", "subnet-22222222"},
			},
		},
	}
	checker := cbCheckerByTarget(t, "subnet")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.Count != 2 {
		t.Errorf("Count = %d, want 2", result.Count)
	}
}

func TestRelated_CB_Subnet_NoVPCConfig(t *testing.T) {
	res := resource.Resource{
		ID:        "my-project",
		Fields:    map[string]string{},
		RawStruct: cbtypes.Project{},
	}
	checker := cbCheckerByTarget(t, "subnet")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (no VpcConfig)", result.Count)
	}
}

// ---------------------------------------------------------------------------
// checkCbAlarm — Pattern D: CW Alarm Namespace=AWS/CodeBuild + ProjectName dim
// ---------------------------------------------------------------------------

func TestRelated_CB_Alarm_Match(t *testing.T) {
	const projectName = "my-project"
	alarmRes := resource.Resource{
		ID: "cb-build-failures",
		RawStruct: cwtypes.MetricAlarm{
			AlarmName: aws.String("cb-build-failures"),
			Namespace: aws.String("AWS/CodeBuild"),
			Dimensions: []cwtypes.Dimension{
				{Name: aws.String("ProjectName"), Value: aws.String(projectName)},
			},
		},
	}
	cache := resource.ResourceCache{
		"alarm": resource.ResourceCacheEntry{Resources: []resource.Resource{alarmRes}},
	}
	res := resource.Resource{
		ID:        projectName,
		Fields:    map[string]string{},
		RawStruct: cbtypes.Project{Name: aws.String(projectName)},
	}

	checker := cbCheckerByTarget(t, "alarm")
	result := checker(context.Background(), nil, res, cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) == 0 || result.ResourceIDs[0] != "cb-build-failures" {
		t.Errorf("ResourceIDs = %v, want [cb-build-failures]", result.ResourceIDs)
	}
}

func TestRelated_CB_Alarm_WrongNamespace(t *testing.T) {
	alarmRes := resource.Resource{
		ID: "other-alarm",
		RawStruct: cwtypes.MetricAlarm{
			AlarmName: aws.String("other-alarm"),
			Namespace: aws.String("AWS/EC2"),
			Dimensions: []cwtypes.Dimension{
				{Name: aws.String("ProjectName"), Value: aws.String("my-project")},
			},
		},
	}
	cache := resource.ResourceCache{
		"alarm": resource.ResourceCacheEntry{Resources: []resource.Resource{alarmRes}},
	}
	res := resource.Resource{
		ID:        "my-project",
		Fields:    map[string]string{},
		RawStruct: cbtypes.Project{},
	}

	checker := cbCheckerByTarget(t, "alarm")
	result := checker(context.Background(), nil, res, cache)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (wrong Namespace)", result.Count)
	}
}

func TestRelated_CB_Alarm_NilCache(t *testing.T) {
	res := resource.Resource{
		ID:        "my-project",
		Fields:    map[string]string{},
		RawStruct: cbtypes.Project{},
	}
	checker := cbCheckerByTarget(t, "alarm")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (empty cache, no clients)", result.Count)
	}
}

// ---------------------------------------------------------------------------
// checkCbECR — Pattern F+C: ECR URI parsing
// ---------------------------------------------------------------------------

func TestRelated_CB_ECR_Match(t *testing.T) {
	const repoName = "my-app"
	res := resource.Resource{
		ID:     "my-project",
		Fields: map[string]string{},
		RawStruct: cbtypes.Project{
			Environment: &cbtypes.ProjectEnvironment{
				Image: aws.String("123456789012.dkr.ecr.us-east-1.amazonaws.com/my-app:latest"),
			},
		},
	}
	cache := resource.ResourceCache{
		"ecr": resource.ResourceCacheEntry{Resources: []resource.Resource{
			{ID: repoName, Name: repoName},
		}},
	}

	checker := cbCheckerByTarget(t, "ecr")
	result := checker(context.Background(), nil, res, cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) == 0 || result.ResourceIDs[0] != repoName {
		t.Errorf("ResourceIDs = %v, want [%s]", result.ResourceIDs, repoName)
	}
}

func TestRelated_CB_ECR_NonECRImage(t *testing.T) {
	res := resource.Resource{
		ID:     "my-project",
		Fields: map[string]string{},
		RawStruct: cbtypes.Project{
			Environment: &cbtypes.ProjectEnvironment{
				Image: aws.String("ubuntu:22.04"),
			},
		},
	}

	checker := cbCheckerByTarget(t, "ecr")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (non-ECR image)", result.Count)
	}
}

// ---------------------------------------------------------------------------
// checkCbS3 — Pattern F+C: S3 artifact/source bucket
// ---------------------------------------------------------------------------

func TestRelated_CB_S3_Match(t *testing.T) {
	res := resource.Resource{
		ID:     "my-project",
		Fields: map[string]string{},
		RawStruct: cbtypes.Project{
			Artifacts: &cbtypes.ProjectArtifacts{
				Type:     cbtypes.ArtifactsTypeS3,
				Location: aws.String("my-artifacts-bucket"),
			},
		},
	}
	cache := resource.ResourceCache{
		"s3": resource.ResourceCacheEntry{Resources: []resource.Resource{
			{ID: "my-artifacts-bucket", Name: "my-artifacts-bucket"},
		}},
	}

	checker := cbCheckerByTarget(t, "s3")
	result := checker(context.Background(), nil, res, cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
}

func TestRelated_CB_S3_NoS3Artifacts(t *testing.T) {
	res := resource.Resource{
		ID:     "my-project",
		Fields: map[string]string{},
		RawStruct: cbtypes.Project{
			Artifacts: &cbtypes.ProjectArtifacts{
				Type: cbtypes.ArtifactsTypeNoArtifacts,
			},
		},
	}

	checker := cbCheckerByTarget(t, "s3")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (no S3 artifacts)", result.Count)
	}
}

// ---------------------------------------------------------------------------
// checkCbSecrets — Pattern F: SECRETS_MANAGER env vars
// ---------------------------------------------------------------------------

func TestRelated_CB_Secrets_Match(t *testing.T) {
	res := resource.Resource{
		ID:     "my-project",
		Fields: map[string]string{},
		RawStruct: cbtypes.Project{
			Environment: &cbtypes.ProjectEnvironment{
				EnvironmentVariables: []cbtypes.EnvironmentVariable{
					{
						Name:  aws.String("DB_PASSWORD"),
						Value: aws.String("prod/db/password"),
						Type:  cbtypes.EnvironmentVariableTypeSecretsManager,
					},
				},
			},
		},
	}

	checker := cbCheckerByTarget(t, "secrets")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) == 0 || result.ResourceIDs[0] != "prod/db/password" {
		t.Errorf("ResourceIDs = %v, want [prod/db/password]", result.ResourceIDs)
	}
}

func TestRelated_CB_Secrets_NoSecretsVars(t *testing.T) {
	res := resource.Resource{
		ID:     "my-project",
		Fields: map[string]string{},
		RawStruct: cbtypes.Project{
			Environment: &cbtypes.ProjectEnvironment{
				EnvironmentVariables: []cbtypes.EnvironmentVariable{
					{Name: aws.String("NODE_ENV"), Value: aws.String("prod"), Type: cbtypes.EnvironmentVariableTypePlaintext},
				},
			},
		},
	}

	checker := cbCheckerByTarget(t, "secrets")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (no SECRETS_MANAGER vars)", result.Count)
	}
}

// ---------------------------------------------------------------------------
// checkCbSSM — Pattern F: PARAMETER_STORE env vars
// ---------------------------------------------------------------------------

func TestRelated_CB_SSM_Match(t *testing.T) {
	res := resource.Resource{
		ID:     "my-project",
		Fields: map[string]string{},
		RawStruct: cbtypes.Project{
			Environment: &cbtypes.ProjectEnvironment{
				EnvironmentVariables: []cbtypes.EnvironmentVariable{
					{
						Name:  aws.String("API_KEY"),
						Value: aws.String("/prod/api/key"),
						Type:  cbtypes.EnvironmentVariableTypeParameterStore,
					},
				},
			},
		},
	}

	checker := cbCheckerByTarget(t, "ssm")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) == 0 || result.ResourceIDs[0] != "/prod/api/key" {
		t.Errorf("ResourceIDs = %v, want [/prod/api/key]", result.ResourceIDs)
	}
}

func TestRelated_CB_SSM_NoSSMVars(t *testing.T) {
	res := resource.Resource{
		ID:        "my-project",
		Fields:    map[string]string{},
		RawStruct: cbtypes.Project{Environment: &cbtypes.ProjectEnvironment{}},
	}

	checker := cbCheckerByTarget(t, "ssm")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (no PARAMETER_STORE vars)", result.Count)
	}
}
