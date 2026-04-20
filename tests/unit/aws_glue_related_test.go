package unit_test

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	cwtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	gluetypes "github.com/aws/aws-sdk-go-v2/service/glue/types"

	_ "github.com/k2m30/a9s/v3/internal/aws"
	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/resource"
)

func glueCheckerByTarget(t *testing.T, target string) resource.RelatedChecker {
	t.Helper()
	for _, def := range resource.GetRelated("glue") {
		if def.TargetType == target {
			if def.Checker == nil {
				t.Fatalf("glue related checker for %s is nil", target)
			}
			return def.Checker
		}
	}
	t.Fatalf("glue related checker for %s not found", target)
	return nil
}

// --- Navigable Field Registration ---

func TestNavigableFields_Glue_Registered(t *testing.T) {
	nav := resource.IsFieldNavigable("glue", "Role")
	if nav == nil {
		t.Fatal("expected navigable field Role not found for glue")
	}
	if nav.TargetType != "role" {
		t.Errorf("Role TargetType = %q, want %q", nav.TargetType, "role")
	}
}

// --- IAM Role checker (Pattern C — cache, name extracted from ARN) ---

func TestRelated_Glue_Role_Found(t *testing.T) {
	const roleARN = "arn:aws:iam::123456789012:role/GlueServiceRole"
	const roleName = "GlueServiceRole"

	roleRes := resource.Resource{
		ID:   roleName,
		Name: roleName,
	}
	cache := resource.ResourceCache{
		"role": resource.ResourceCacheEntry{Resources: []resource.Resource{roleRes}},
	}
	source := resource.Resource{
		ID:   "acme-etl-job",
		Name: "acme-etl-job",
		Fields: map[string]string{
			"role": roleARN,
		},
		RawStruct: gluetypes.Job{
			Name: aws.String("acme-etl-job"),
			Role: aws.String(roleARN),
		},
	}

	checker := glueCheckerByTarget(t, "role")
	result := checker(context.Background(), nil, source, cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != roleName {
		t.Errorf("ResourceIDs = %v, want [%s]", result.ResourceIDs, roleName)
	}
	if result.Err != nil {
		t.Errorf("unexpected error: %v", result.Err)
	}
}

func TestRelated_Glue_Role_NotFound(t *testing.T) {
	const roleARN = "arn:aws:iam::123456789012:role/GlueServiceRole"

	roleRes := resource.Resource{
		ID:   "DifferentRole",
		Name: "DifferentRole",
	}
	cache := resource.ResourceCache{
		"role": resource.ResourceCacheEntry{Resources: []resource.Resource{roleRes}},
	}
	source := resource.Resource{
		ID:   "acme-etl-job",
		Name: "acme-etl-job",
		Fields: map[string]string{
			"role": roleARN,
		},
		RawStruct: gluetypes.Job{
			Name: aws.String("acme-etl-job"),
			Role: aws.String(roleARN),
		},
	}

	checker := glueCheckerByTarget(t, "role")
	result := checker(context.Background(), nil, source, cache)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0", result.Count)
	}
}

func TestRelated_Glue_Role_EmptyRole(t *testing.T) {
	cache := resource.ResourceCache{
		"role": resource.ResourceCacheEntry{Resources: []resource.Resource{
			{ID: "SomeRole", Name: "SomeRole"},
		}},
	}
	source := resource.Resource{
		ID:   "acme-etl-job",
		Name: "acme-etl-job",
		Fields: map[string]string{
			"role": "",
		},
		RawStruct: gluetypes.Job{
			Name: aws.String("acme-etl-job"),
			Role: nil,
		},
	}

	checker := glueCheckerByTarget(t, "role")
	result := checker(context.Background(), nil, source, cache)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 for nil Role", result.Count)
	}
}

func TestRelated_Glue_Role_CacheMissNoClients(t *testing.T) {
	source := resource.Resource{
		ID:   "acme-etl-job",
		Name: "acme-etl-job",
		Fields: map[string]string{
			"role": "arn:aws:iam::123456789012:role/GlueServiceRole",
		},
		RawStruct: gluetypes.Job{
			Name: aws.String("acme-etl-job"),
			Role: aws.String("arn:aws:iam::123456789012:role/GlueServiceRole"),
		},
	}

	checker := glueCheckerByTarget(t, "role")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (unknown)", result.Count)
	}
}

// --- CloudWatch Alarms checker (Pattern C — cache, JobName dimension) ---

func TestRelated_Glue_Alarms_Found(t *testing.T) {
	const jobName = "acme-etl-orders"

	alarmRes := resource.Resource{
		ID: "glue-job-failure-alarm",
		RawStruct: cwtypes.MetricAlarm{
			AlarmName: aws.String("glue-job-failure-alarm"),
			Dimensions: []cwtypes.Dimension{
				{Name: aws.String("JobName"), Value: aws.String(jobName)},
			},
		},
	}
	cache := resource.ResourceCache{
		"alarm": resource.ResourceCacheEntry{Resources: []resource.Resource{alarmRes}},
	}
	source := resource.Resource{
		ID:   jobName,
		Name: jobName,
		Fields: map[string]string{
			"job_name": jobName,
		},
		RawStruct: gluetypes.Job{
			Name: aws.String(jobName),
			Role: aws.String("arn:aws:iam::123456789012:role/acme-glue-role"),
		},
	}

	checker := glueCheckerByTarget(t, "alarm")
	result := checker(context.Background(), nil, source, cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != "glue-job-failure-alarm" {
		t.Errorf("ResourceIDs = %v, want [glue-job-failure-alarm]", result.ResourceIDs)
	}
	if result.Err != nil {
		t.Errorf("unexpected error: %v", result.Err)
	}
}

func TestRelated_Glue_Alarms_NotFound(t *testing.T) {
	const jobName = "acme-etl-orders"

	alarmRes := resource.Resource{
		ID: "other-job-alarm",
		RawStruct: cwtypes.MetricAlarm{
			AlarmName: aws.String("other-job-alarm"),
			Dimensions: []cwtypes.Dimension{
				{Name: aws.String("JobName"), Value: aws.String("different-job")},
			},
		},
	}
	cache := resource.ResourceCache{
		"alarm": resource.ResourceCacheEntry{Resources: []resource.Resource{alarmRes}},
	}
	source := resource.Resource{
		ID:   jobName,
		Name: jobName,
		Fields: map[string]string{
			"job_name": jobName,
		},
		RawStruct: gluetypes.Job{
			Name: aws.String(jobName),
			Role: aws.String("arn:aws:iam::123456789012:role/acme-glue-role"),
		},
	}

	checker := glueCheckerByTarget(t, "alarm")
	result := checker(context.Background(), nil, source, cache)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0", result.Count)
	}
}

func TestRelated_Glue_Alarms_CacheMissNoClients(t *testing.T) {
	source := resource.Resource{
		ID:   "acme-etl-orders",
		Name: "acme-etl-orders",
		Fields: map[string]string{
			"job_name": "acme-etl-orders",
		},
		RawStruct: gluetypes.Job{
			Name: aws.String("acme-etl-orders"),
			Role: aws.String("arn:aws:iam::123456789012:role/acme-glue-role"),
		},
	}

	checker := glueCheckerByTarget(t, "alarm")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (unknown)", result.Count)
	}
}

// --- glue→cfn: undeterminable without GetTags, returns Count: -1 ---

func TestRelated_Glue_CFN_Unknown(t *testing.T) {
	source := resource.Resource{
		ID:   "acme-etl-orders",
		Name: "acme-etl-orders",
	}
	checker := glueCheckerByTarget(t, "cfn")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})
	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (tags need GetTags enrichment)", result.Count)
	}
	if result.TargetType != "cfn" {
		t.Errorf("TargetType = %q, want %q", result.TargetType, "cfn")
	}
}

// --- checkGlueLogs tests (Pattern N — shared log groups /aws-glue/jobs/output and /aws-glue/jobs/error) ---

func TestRelated_Glue_Logs_MatchBothSharedGroups(t *testing.T) {
	outputLogRes := resource.Resource{ID: "/aws-glue/jobs/output", Name: "/aws-glue/jobs/output"}
	errorLogRes := resource.Resource{ID: "/aws-glue/jobs/error", Name: "/aws-glue/jobs/error"}
	otherLogRes := resource.Resource{ID: "/aws/lambda/my-function", Name: "/aws/lambda/my-function"}
	cache := resource.ResourceCache{
		"logs": resource.ResourceCacheEntry{Resources: []resource.Resource{outputLogRes, errorLogRes, otherLogRes}},
	}
	source := resource.Resource{
		ID:   "acme-etl-job",
		Name: "acme-etl-job",
		RawStruct: gluetypes.Job{
			Name: aws.String("acme-etl-job"),
		},
	}

	checker := glueCheckerByTarget(t, "logs")
	result := checker(context.Background(), nil, source, cache)

	if result.Count != 2 {
		t.Errorf("Count = %d, want 2 (both shared Glue log groups)", result.Count)
	}
	found := map[string]bool{}
	for _, id := range result.ResourceIDs {
		found[id] = true
	}
	if !found["/aws-glue/jobs/output"] {
		t.Errorf("missing /aws-glue/jobs/output in ResourceIDs: %v", result.ResourceIDs)
	}
	if !found["/aws-glue/jobs/error"] {
		t.Errorf("missing /aws-glue/jobs/error in ResourceIDs: %v", result.ResourceIDs)
	}
}

func TestRelated_Glue_Logs_NoGlueGroups(t *testing.T) {
	otherLogRes := resource.Resource{ID: "/aws/lambda/my-function", Name: "/aws/lambda/my-function"}
	cache := resource.ResourceCache{
		"logs": resource.ResourceCacheEntry{Resources: []resource.Resource{otherLogRes}},
	}
	source := resource.Resource{ID: "acme-etl-job", Name: "acme-etl-job"}

	checker := glueCheckerByTarget(t, "logs")
	result := checker(context.Background(), nil, source, cache)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0", result.Count)
	}
}

func TestRelated_Glue_Logs_CacheMissNoClients(t *testing.T) {
	source := resource.Resource{ID: "acme-etl-job", Name: "acme-etl-job"}

	checker := glueCheckerByTarget(t, "logs")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (cache miss, no clients)", result.Count)
	}
}

// --- checkGlueS3 tests (Pattern F — s3:// script location from Command.ScriptLocation) ---

func TestRelated_Glue_S3_MatchScriptBucket(t *testing.T) {
	source := resource.Resource{
		ID:   "acme-etl-job",
		Name: "acme-etl-job",
		RawStruct: gluetypes.Job{
			Name: aws.String("acme-etl-job"),
			Command: &gluetypes.JobCommand{
				ScriptLocation: aws.String("s3://acme-glue-scripts/jobs/etl.py"),
			},
		},
	}

	checker := glueCheckerByTarget(t, "s3")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != "acme-glue-scripts" {
		t.Errorf("ResourceIDs = %v, want [acme-glue-scripts]", result.ResourceIDs)
	}
}

func TestRelated_Glue_S3_NilCommand(t *testing.T) {
	source := resource.Resource{
		ID:   "acme-etl-job",
		Name: "acme-etl-job",
		RawStruct: gluetypes.Job{
			Name:    aws.String("acme-etl-job"),
			Command: nil,
		},
	}

	checker := glueCheckerByTarget(t, "s3")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (nil Command)", result.Count)
	}
}

func TestRelated_Glue_S3_InvalidRawStruct(t *testing.T) {
	source := resource.Resource{
		ID:        "acme-etl-job",
		RawStruct: "not-a-glue-job",
	}

	checker := glueCheckerByTarget(t, "s3")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 for invalid RawStruct", result.Count)
	}
}

// --- checkGlueSecrets tests (Pattern F — DefaultArguments secrets ARNs) ---

func TestRelated_Glue_Secrets_MatchSecretARN(t *testing.T) {
	const secretARN = "arn:aws:secretsmanager:us-east-1:123456789012:secret:acme/db-password-AbcDef"
	source := resource.Resource{
		ID:   "acme-etl-job",
		Name: "acme-etl-job",
		RawStruct: gluetypes.Job{
			Name: aws.String("acme-etl-job"),
			DefaultArguments: map[string]string{
				"--db-password": secretARN,
			},
		},
	}

	checker := glueCheckerByTarget(t, "secrets")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	// Name extracted after ":secret:" prefix
	const wantName = "acme/db-password-AbcDef"
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != wantName {
		t.Errorf("ResourceIDs = %v, want [%s]", result.ResourceIDs, wantName)
	}
}

func TestRelated_Glue_Secrets_NoneOfSMPrefix(t *testing.T) {
	source := resource.Resource{
		ID:   "acme-etl-job",
		Name: "acme-etl-job",
		RawStruct: gluetypes.Job{
			Name: aws.String("acme-etl-job"),
			DefaultArguments: map[string]string{
				"--output-path": "s3://my-bucket/output",
			},
		},
	}

	checker := glueCheckerByTarget(t, "secrets")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (no secrets manager ARNs)", result.Count)
	}
}

func TestRelated_Glue_Secrets_EmptyArguments(t *testing.T) {
	source := resource.Resource{
		ID:   "acme-etl-job",
		Name: "acme-etl-job",
		RawStruct: gluetypes.Job{
			Name:             aws.String("acme-etl-job"),
			DefaultArguments: map[string]string{},
		},
	}

	checker := glueCheckerByTarget(t, "secrets")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (empty DefaultArguments)", result.Count)
	}
}

func TestRelated_Glue_Secrets_InvalidRawStruct(t *testing.T) {
	source := resource.Resource{
		ID:        "acme-etl-job",
		RawStruct: "not-a-glue-job",
	}

	checker := glueCheckerByTarget(t, "secrets")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 for invalid RawStruct", result.Count)
	}
}

// --- checkGlueAthena tests (Pattern C — athena workgroup cache, glue_job field match) ---

func TestRelated_Glue_Athena_MatchByGlueJobField(t *testing.T) {
	const jobName = "acme-etl-job"
	wgRes := resource.Resource{
		ID:     "acme-workgroup",
		Name:   "acme-workgroup",
		Fields: map[string]string{"glue_job": jobName},
	}
	cache := resource.ResourceCache{
		"athena": resource.ResourceCacheEntry{Resources: []resource.Resource{wgRes}},
	}
	source := resource.Resource{ID: jobName, Name: jobName}

	checker := glueCheckerByTarget(t, "athena")
	result := checker(context.Background(), nil, source, cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != "acme-workgroup" {
		t.Errorf("ResourceIDs = %v, want [acme-workgroup]", result.ResourceIDs)
	}
}

func TestRelated_Glue_Athena_NoMatch(t *testing.T) {
	wgRes := resource.Resource{
		ID:     "acme-workgroup",
		Name:   "acme-workgroup",
		Fields: map[string]string{"glue_job": "other-job"},
	}
	cache := resource.ResourceCache{
		"athena": resource.ResourceCacheEntry{Resources: []resource.Resource{wgRes}},
	}
	source := resource.Resource{ID: "acme-etl-job", Name: "acme-etl-job"}

	checker := glueCheckerByTarget(t, "athena")
	result := checker(context.Background(), nil, source, cache)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0", result.Count)
	}
}

func TestRelated_Glue_Athena_CacheMissNoClients(t *testing.T) {
	source := resource.Resource{ID: "acme-etl-job", Name: "acme-etl-job"}

	checker := glueCheckerByTarget(t, "athena")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (cache miss, no clients)", result.Count)
	}
}

// ---------------------------------------------------------------------------
// checkGlueKMS — Pattern C: GetSecurityConfiguration → KMS key ARNs
// ---------------------------------------------------------------------------

// TestRelated_Glue_KMS_InvalidRawStruct verifies Count=-1 when the RawStruct
// is not a gluetypes.Job.
func TestRelated_Glue_KMS_InvalidRawStruct(t *testing.T) {
	source := resource.Resource{ID: "acme-etl-job", RawStruct: "not-a-job"}
	checker := glueCheckerByTarget(t, "kms")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})
	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (bad raw struct)", result.Count)
	}
}

// TestRelated_Glue_KMS_NoSecurityConfigReturnsZero verifies Count=0 when the
// job has no SecurityConfiguration set.
func TestRelated_Glue_KMS_NoSecurityConfigReturnsZero(t *testing.T) {
	source := resource.Resource{
		ID:        "acme-etl-job",
		RawStruct: gluetypes.Job{Name: aws.String("acme-etl-job")},
	}
	checker := glueCheckerByTarget(t, "kms")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})
	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (no security config)", result.Count)
	}
}

// TestRelated_Glue_KMS_NilClientsReturnsMinusOne verifies Count=-1 when the
// job has a SecurityConfiguration but no ServiceClients are available.
func TestRelated_Glue_KMS_NilClientsReturnsMinusOne(t *testing.T) {
	source := resource.Resource{
		ID: "acme-etl-job",
		RawStruct: gluetypes.Job{
			Name:                  aws.String("acme-etl-job"),
			SecurityConfiguration: aws.String("acme-sec-cfg"),
		},
	}
	checker := glueCheckerByTarget(t, "kms")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})
	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (nil clients)", result.Count)
	}
}

// TestRelated_Glue_KMS_FoundViaSecurityConfig verifies that a KMS key ARN in
// the CloudWatch encryption block is extracted and returned as a resource ID.
func TestRelated_Glue_KMS_FoundViaSecurityConfig(t *testing.T) {
	const keyID = "a1b2c3d4-5678-90ab-cdef-111111111111"
	const kmsARN = "arn:aws:kms:us-east-1:123456789012:key/" + keyID
	source := resource.Resource{
		ID: "acme-etl-job",
		RawStruct: gluetypes.Job{
			Name:                  aws.String("acme-etl-job"),
			SecurityConfiguration: aws.String("acme-sec-cfg"),
		},
	}
	clients := &awsclient.ServiceClients{
		Glue: newFakeGlueWithKMSConfig(kmsARN),
	}
	checker := glueCheckerByTarget(t, "kms")
	result := checker(context.Background(), clients, source, resource.ResourceCache{})
	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != keyID {
		t.Errorf("ResourceIDs = %v, want [%s]", result.ResourceIDs, keyID)
	}
}

// TestRelated_Glue_KMS_EmptyEncryptionReturnsZero verifies Count=0 when
// GetSecurityConfiguration returns an empty EncryptionConfiguration (no KMS keys).
func TestRelated_Glue_KMS_EmptyEncryptionReturnsZero(t *testing.T) {
	source := resource.Resource{
		ID: "acme-etl-job",
		RawStruct: gluetypes.Job{
			Name:                  aws.String("acme-etl-job"),
			SecurityConfiguration: aws.String("acme-sec-cfg"),
		},
	}
	clients := &awsclient.ServiceClients{
		Glue: newFakeGlueWithEmptyEncryption(),
	}
	checker := glueCheckerByTarget(t, "kms")
	result := checker(context.Background(), clients, source, resource.ResourceCache{})
	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (empty encryption config)", result.Count)
	}
}

// ---------------------------------------------------------------------------
// checkGlueCFN — Pattern C: GetTags → aws:cloudformation:stack-name
// ---------------------------------------------------------------------------

// TestRelated_Glue_CFN_EmptyJobIDReturnsZero verifies Count=0 when the job
// has no ID (short-circuit before any API call).
func TestRelated_Glue_CFN_EmptyJobIDReturnsZero(t *testing.T) {
	source := resource.Resource{ID: "", Name: ""}
	checker := glueCheckerByTarget(t, "cfn")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})
	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (empty job ID)", result.Count)
	}
}

// TestRelated_Glue_CFN_GlueDoesNotImplementGetTagsReturnsMinusOne verifies
// Count=-1 when c.Glue does not implement GlueGetTagsAPI (type assertion fails).
func TestRelated_Glue_CFN_GlueDoesNotImplementGetTagsReturnsMinusOne(t *testing.T) {
	source := resource.Resource{ID: "acme-etl-job", Name: "acme-etl-job"}
	// fakeGlueWithSecurityConfig implements GlueAPI but NOT GlueGetTagsAPI.
	clients := &awsclient.ServiceClients{
		Glue: newFakeGlueWithEmptyEncryption(),
	}
	checker := glueCheckerByTarget(t, "cfn")
	result := checker(context.Background(), clients, source, resource.ResourceCache{})
	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (Glue client lacks GetTags)", result.Count)
	}
}
