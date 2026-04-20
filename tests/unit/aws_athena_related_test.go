package unit_test

import (
	"context"
	"testing"

	_ "github.com/k2m30/a9s/v3/internal/aws"
	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// TestRelated_Athena_Registered verifies all related defs are registered with non-nil checkers.
func TestRelated_Athena_Registered(t *testing.T) {
	defs := resource.GetRelated("athena")
	if len(defs) == 0 {
		t.Fatal("no related defs registered for athena")
	}

	expected := map[string]string{
		"s3":  "S3 Buckets (results)",
		"kms": "KMS Keys",
	}
	for target, wantDisplay := range expected {
		found := false
		for _, def := range defs {
			if def.TargetType == target {
				found = true
				if def.Checker == nil {
					t.Errorf("athena %q: Checker should not be nil", target)
				}
				if def.DisplayName != wantDisplay {
					t.Errorf("athena %q: DisplayName = %q, want %q", target, def.DisplayName, wantDisplay)
				}
				break
			}
		}
		if !found {
			t.Errorf("expected related def for target %q not found", target)
		}
	}
}

// athenaCheckerByTarget returns the RelatedChecker for the given target type
// registered under "athena". Fails immediately if not found or nil.
func athenaCheckerByTarget(t *testing.T, target string) resource.RelatedChecker {
	t.Helper()
	for _, def := range resource.GetRelated("athena") {
		if def.TargetType == target {
			if def.Checker == nil {
				t.Fatalf("athena related checker for %s is nil", target)
			}
			return def.Checker
		}
	}
	t.Fatalf("athena related checker for %s not found", target)
	return nil
}

// ---------------------------------------------------------------------------
// checkAthenaS3 tests — reads Fields["result_output_location"] populated by
// GetWorkGroup enrichment. Without enrichment, Count: -1 (unknown).
// ---------------------------------------------------------------------------

func TestRelated_Athena_S3_Unknown(t *testing.T) {
	res := resource.Resource{
		ID:     "primary",
		Name:   "primary",
		Fields: map[string]string{},
	}
	checker := athenaCheckerByTarget(t, "s3")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})
	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (GetWorkGroup enrichment needed)", result.Count)
	}
	if result.TargetType != "s3" {
		t.Errorf("TargetType = %q, want %q", result.TargetType, "s3")
	}
}

// NOTE: The athena→s3 "Found" path now makes a live GetWorkGroup API call
// (Pattern C). Verifying a positive match requires a mocked *ServiceClients
// whose Athena satisfies AthenaGetWorkGroupAPI — set up in the integration
// test suite. Unit tests cover the nil-clients path (above) and the
// s3://-URI parser via bucketFromS3URI in its own test.

func TestRelated_Athena_KMS_Unknown(t *testing.T) {
	res := resource.Resource{
		ID:     "primary",
		Name:   "primary",
		Fields: map[string]string{},
	}
	checker := athenaCheckerByTarget(t, "kms")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})
	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (GetWorkGroup enrichment needed)", result.Count)
	}
}

// Same pattern for athena→kms: the positive match now requires a mocked
// GetWorkGroup response. See integration tests.

// ---------------------------------------------------------------------------
// checkAthenaS3 — positive match with fake AthenaAPI
// ---------------------------------------------------------------------------

// TestRelated_Athena_S3_Match verifies that a workgroup with an s3:// OutputLocation
// returns Count=1 and the bucket name extracted from the URI.
func TestRelated_Athena_S3_Match(t *testing.T) {
	res := resource.Resource{
		ID:     "primary",
		Name:   "primary",
		Fields: map[string]string{},
	}
	clients := &awsclient.ServiceClients{
		Athena: newFakeAthenaWithS3URI("s3://my-athena-results/prefix/"),
	}
	checker := athenaCheckerByTarget(t, "s3")
	result := checker(context.Background(), clients, res, resource.ResourceCache{})

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) == 0 || result.ResourceIDs[0] != "my-athena-results" {
		t.Errorf("ResourceIDs = %v, want [my-athena-results]", result.ResourceIDs)
	}
}

// TestRelated_Athena_S3_NoOutputLocation verifies Count=0 when the workgroup
// carries no OutputLocation. Requires a non-nil Configuration so cfg != nil.
func TestRelated_Athena_S3_NoOutputLocation(t *testing.T) {
	res := resource.Resource{ID: "primary", Fields: map[string]string{}}
	clients := &awsclient.ServiceClients{
		Athena: newFakeAthenaWithEmptyConfig(),
	}
	checker := athenaCheckerByTarget(t, "s3")
	result := checker(context.Background(), clients, res, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (no OutputLocation)", result.Count)
	}
}

// ---------------------------------------------------------------------------
// checkAthenaKMS — positive match with fake AthenaAPI
// ---------------------------------------------------------------------------

// TestRelated_Athena_KMS_Match verifies that a KMS key ARN extracts the UUID
// (last "/" segment) as the resource ID.
func TestRelated_Athena_KMS_Match(t *testing.T) {
	res := resource.Resource{ID: "primary", Fields: map[string]string{}}
	clients := &awsclient.ServiceClients{
		Athena: newFakeAthenaWithKMSKey("arn:aws:kms:us-east-1:123456789012:key/a1b2c3d4-1234-5678-abcd-111111111111"),
	}
	checker := athenaCheckerByTarget(t, "kms")
	result := checker(context.Background(), clients, res, resource.ResourceCache{})

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) == 0 || result.ResourceIDs[0] != "a1b2c3d4-1234-5678-abcd-111111111111" {
		t.Errorf("ResourceIDs = %v, want [a1b2c3d4-1234-5678-abcd-111111111111]", result.ResourceIDs)
	}
}

// TestRelated_Athena_KMS_NoKey verifies Count=0 when the workgroup has no KMS config.
// Requires a non-nil Configuration so cfg != nil.
func TestRelated_Athena_KMS_NoKey(t *testing.T) {
	res := resource.Resource{ID: "primary", Fields: map[string]string{}}
	clients := &awsclient.ServiceClients{
		Athena: newFakeAthenaWithEmptyConfig(),
	}
	checker := athenaCheckerByTarget(t, "kms")
	result := checker(context.Background(), clients, res, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (no KMS key)", result.Count)
	}
}

// ---------------------------------------------------------------------------
// checkAthenaLogs — CW metrics enabled/disabled branches
// ---------------------------------------------------------------------------

// TestRelated_Athena_Logs_CWEnabled verifies that when PublishCloudWatchMetricsEnabled
// is true the log group /aws/athena/<wgName> is returned.
func TestRelated_Athena_Logs_CWEnabled(t *testing.T) {
	res := resource.Resource{ID: "primary", Fields: map[string]string{}}
	clients := &awsclient.ServiceClients{
		Athena: newFakeAthenaWithCWLogsEnabled(),
	}
	checker := athenaCheckerByTarget(t, "logs")
	result := checker(context.Background(), clients, res, resource.ResourceCache{})

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	want := "/aws/athena/primary"
	if len(result.ResourceIDs) == 0 || result.ResourceIDs[0] != want {
		t.Errorf("ResourceIDs = %v, want [%s]", result.ResourceIDs, want)
	}
}

// TestRelated_Athena_Logs_CWDisabled verifies Count=0 when CW metrics are off.
// Requires a non-nil Configuration so cfg != nil.
func TestRelated_Athena_Logs_CWDisabled(t *testing.T) {
	res := resource.Resource{ID: "primary", Fields: map[string]string{}}
	clients := &awsclient.ServiceClients{
		Athena: newFakeAthenaWithEmptyConfig(), // non-nil config, PublishCWMetrics=nil → disabled
	}
	checker := athenaCheckerByTarget(t, "logs")
	result := checker(context.Background(), clients, res, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (CW metrics disabled)", result.Count)
	}
}

// ---------------------------------------------------------------------------
// checkAthenaRole — ExecutionRole extraction
// ---------------------------------------------------------------------------

// TestRelated_Athena_Role_Match verifies that the ExecutionRole ARN last segment
// is returned as the role name.
func TestRelated_Athena_Role_Match(t *testing.T) {
	res := resource.Resource{ID: "spark-wg", Fields: map[string]string{}}
	clients := &awsclient.ServiceClients{
		Athena: newFakeAthenaWithExecutionRole("arn:aws:iam::123456789012:role/AthenaSparkExecutionRole"),
	}
	checker := athenaCheckerByTarget(t, "role")
	result := checker(context.Background(), clients, res, resource.ResourceCache{})

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) == 0 || result.ResourceIDs[0] != "AthenaSparkExecutionRole" {
		t.Errorf("ResourceIDs = %v, want [AthenaSparkExecutionRole]", result.ResourceIDs)
	}
}

// TestRelated_Athena_Role_NoRole verifies Count=0 when ExecutionRole is not set.
// Requires a non-nil Configuration so cfg != nil.
func TestRelated_Athena_Role_NoRole(t *testing.T) {
	res := resource.Resource{ID: "primary", Fields: map[string]string{}}
	clients := &awsclient.ServiceClients{
		Athena: newFakeAthenaWithEmptyConfig(),
	}
	checker := athenaCheckerByTarget(t, "role")
	result := checker(context.Background(), clients, res, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (no ExecutionRole)", result.Count)
	}
}

// ---------------------------------------------------------------------------
// checkAthenaGlue — always returns Count: 0 (no structured Glue field)
// ---------------------------------------------------------------------------

// TestRelated_Athena_Glue_AlwaysZero verifies the glue checker always returns 0
// because no structured per-job Glue linkage is available on the WorkGroup config.
// Requires a non-nil Configuration with a non-nil EngineVersion (so the EngineVersion
// nil-check branch returns 0, not the top-level cfg==nil branch returning -1).
func TestRelated_Athena_Glue_AlwaysZero(t *testing.T) {
	res := resource.Resource{ID: "primary", Fields: map[string]string{}}
	clients := &awsclient.ServiceClients{
		Athena: newFakeAthenaWithEmptyConfig(),
	}
	checker := athenaCheckerByTarget(t, "glue")
	result := checker(context.Background(), clients, res, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (no structured Glue linkage on WG config)", result.Count)
	}
}
