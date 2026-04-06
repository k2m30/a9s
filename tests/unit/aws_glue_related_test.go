package unit_test

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	cwtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	gluetypes "github.com/aws/aws-sdk-go-v2/service/glue/types"

	_ "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/demo"
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

func TestNavigableFields_Glue_FieldPathsResolve(t *testing.T) {
	resources, ok := demo.GetResources("glue")
	if !ok || len(resources) == 0 {
		t.Fatal("no glue demo fixtures available")
	}

	// Verify that the first fixture has a non-empty Role in its RawStruct.
	raw, ok := resources[0].RawStruct.(gluetypes.Job)
	if !ok {
		t.Fatalf("RawStruct is not gluetypes.Job, got %T", resources[0].RawStruct)
	}
	if raw.Role == nil || *raw.Role == "" {
		t.Error("fixture RawStruct.Role is nil or empty — Role field path cannot resolve")
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

// --- CloudFormation checker (stub — nil Checker) ---

func TestRelated_Glue_CFN_IsStub(t *testing.T) {
	defs := resource.GetRelated("glue")
	if len(defs) == 0 {
		t.Fatal("no related defs registered for glue")
	}
	for _, def := range defs {
		if def.TargetType == "cfn" {
			if def.Checker != nil {
				t.Errorf("glue cfn Checker should be nil (stub)")
			}
			return
		}
	}
	t.Error("expected related def for target cfn not found for glue")
}

// --- Demo Checker ---

func TestRelatedDemo_Glue_Registered(t *testing.T) {
	_ = demo.GetResources // ensure demo package is initialized
	checker := resource.GetRelatedDemo("glue")
	if checker == nil {
		t.Fatal("no demo checker registered for glue")
	}

	results := checker(resource.Resource{ID: "acme-etl-orders"})
	if len(results) == 0 {
		t.Fatal("demo checker returned no results")
	}
	for _, r := range results {
		if r.TargetType == "" {
			t.Error("demo result has empty TargetType")
		}
	}

	// Verify all expected target types are present.
	wantTargets := map[string]bool{"role": false, "alarm": false, "cfn": false}
	for _, r := range results {
		if _, ok := wantTargets[r.TargetType]; ok {
			wantTargets[r.TargetType] = true
		}
	}
	for target, found := range wantTargets {
		if !found {
			t.Errorf("demo checker missing result for target %q", target)
		}
	}

	// At least one result must have Count > 0 (role).
	hasPositive := false
	for _, r := range results {
		if r.Count > 0 {
			hasPositive = true
			break
		}
	}
	if !hasPositive {
		t.Error("demo checker returned no result with Count > 0")
	}
}
