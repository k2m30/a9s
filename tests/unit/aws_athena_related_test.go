package unit_test

import (
	"context"
	"testing"

	_ "github.com/k2m30/a9s/v3/internal/aws"
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
