package unit

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudtrail"
	cloudtrailtypes "github.com/aws/aws-sdk-go-v2/service/cloudtrail/types"

	"github.com/k2m30/a9s/v3/internal/demo/fakes"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// ═══════════════════════════════════════════════════════════════════════════
// AppendRelated and BuildCloudTrailFilter unit tests
// Issue #247: CloudTrail Events related view
// ═══════════════════════════════════════════════════════════════════════════

// TestAppendRelated_AddsToExisting verifies that AppendRelated appends a new
// entry to an existing RelatedDef slice without replacing it.
func TestAppendRelated_AddsToExisting(t *testing.T) {
	resource.SetRelatedForTest("test_append", []resource.RelatedDef{
		{TargetType: "vpc", DisplayName: "VPCs", Checker: resource.NoopChecker},
	})
	t.Cleanup(func() { resource.CleanupRelatedForTest("test_append") })

	resource.AppendRelated("test_append", resource.RelatedDef{
		TargetType:  "ct-events",
		DisplayName: "CloudTrail Events",
		Checker:     resource.NoopChecker,
	})

	got := resource.GetRelated("test_append")
	if len(got) != 2 {
		t.Fatalf("GetRelated length = %d, want 2", len(got))
	}
	if got[1].TargetType != "ct-events" {
		t.Errorf("got[1].TargetType = %q, want %q", got[1].TargetType, "ct-events")
	}
}

// TestAppendRelated_CreatesNew verifies that AppendRelated creates a new entry
// when no prior registration exists for the short name.
func TestAppendRelated_CreatesNew(t *testing.T) {
	t.Cleanup(func() { resource.CleanupRelatedForTest("test_append_new") })

	resource.AppendRelated("test_append_new", resource.RelatedDef{
		TargetType:  "ct-events",
		DisplayName: "CloudTrail Events",
		Checker:     resource.NoopChecker,
	})

	got := resource.GetRelated("test_append_new")
	if len(got) != 1 {
		t.Fatalf("GetRelated length = %d, want 1", len(got))
	}
	if got[0].TargetType != "ct-events" {
		t.Errorf("got[0].TargetType = %q, want %q", got[0].TargetType, "ct-events")
	}
}

// TestAppendRelated_NoDuplicate verifies that calling AppendRelated twice with
// the same TargetType does not create a duplicate entry.
func TestAppendRelated_NoDuplicate(t *testing.T) {
	resource.SetRelatedForTest("test_append_dedup", []resource.RelatedDef{
		{TargetType: "ct-events", DisplayName: "CloudTrail Events", Checker: resource.NoopChecker},
	})
	t.Cleanup(func() { resource.CleanupRelatedForTest("test_append_dedup") })

	resource.AppendRelated("test_append_dedup", resource.RelatedDef{
		TargetType:  "ct-events",
		DisplayName: "CloudTrail Events",
		Checker:     resource.NoopChecker,
	})

	got := resource.GetRelated("test_append_dedup")
	if len(got) != 1 {
		t.Errorf("GetRelated length = %d, want 1 (no duplicate)", len(got))
	}
}

// TestBuildCloudTrailFilter_FieldsArn verifies that SQS (CloudTrailKey "ResourceName:Fields.arn")
// uses the Fields["arn"] value as the filter.
func TestBuildCloudTrailFilter_FieldsArn(t *testing.T) {
	res := resource.Resource{
		ID: "my-queue",
		Fields: map[string]string{
			"arn": "arn:aws:sqs:us-east-1:000000000000:my-queue",
		},
	}

	got := resource.BuildCloudTrailFilter(res, "sqs")
	want := map[string]string{
		"ResourceName": "arn:aws:sqs:us-east-1:000000000000:my-queue",
	}
	if len(got) != len(want) {
		t.Fatalf("filter length = %d, want %d; got %v", len(got), len(want), got)
	}
	if got["ResourceName"] != want["ResourceName"] {
		t.Errorf("filter[ResourceName] = %q, want %q", got["ResourceName"], want["ResourceName"])
	}
}

// TestBuildCloudTrailFilter_IAMUser verifies that iam-user (CloudTrailKey "Username:ID")
// returns a Username filter using res.ID.
func TestBuildCloudTrailFilter_IAMUser(t *testing.T) {
	res := resource.Resource{
		ID: "admin-user",
		Fields: map[string]string{
			"user_name": "admin-user",
		},
	}

	got := resource.BuildCloudTrailFilter(res, "iam-user")
	want := map[string]string{
		"Username": "admin-user",
	}
	if len(got) != len(want) {
		t.Fatalf("filter length = %d, want %d; got %v", len(got), len(want), got)
	}
	if got["Username"] != want["Username"] {
		t.Errorf("filter[Username] = %q, want %q", got["Username"], want["Username"])
	}
}

// TestBuildCloudTrailFilter_IAMRole verifies that role (CloudTrailKey "Username:Name")
// returns a Username filter using res.Name.
func TestBuildCloudTrailFilter_IAMRole(t *testing.T) {
	res := resource.Resource{
		ID:   "arn:aws:iam::000000000000:role/MyRole",
		Name: "MyRole",
		Fields: map[string]string{
			"role_name": "MyRole",
		},
	}

	got := resource.BuildCloudTrailFilter(res, "role")
	want := map[string]string{
		"Username": "MyRole",
	}
	if len(got) != len(want) {
		t.Fatalf("filter length = %d, want %d; got %v", len(got), len(want), got)
	}
	if got["Username"] != want["Username"] {
		t.Errorf("filter[Username] = %q, want %q", got["Username"], want["Username"])
	}
}

// TestBuildCloudTrailFilter_EC2UsesID verifies that ec2 (CloudTrailKey "ResourceName:ID")
// returns a ResourceName filter using res.ID.
func TestBuildCloudTrailFilter_EC2UsesID(t *testing.T) {
	res := resource.Resource{
		ID:     "i-0abc123",
		Fields: map[string]string{},
	}

	filter := resource.BuildCloudTrailFilter(res, "ec2")
	if filter == nil {
		t.Fatal("expected non-nil filter")
	}
	if filter["ResourceName"] != "i-0abc123" {
		t.Errorf("expected ResourceName=%q, got %q", "i-0abc123", filter["ResourceName"])
	}
}

func TestBuildCloudTrailFilter_EmptyID(t *testing.T) {
	res := resource.Resource{ID: "", Fields: map[string]string{}}
	filter := resource.BuildCloudTrailFilter(res, "ec2")
	if filter != nil {
		t.Errorf("expected nil filter for empty ID, got %v", filter)
	}
}

func TestBuildCloudTrailFilter_SQSUsesFieldsArn(t *testing.T) {
	// SQS stores ARN in Fields["arn"] — CloudTrailKey "ResourceName:Fields.arn"
	res := resource.Resource{
		ID:     "my-queue",
		Fields: map[string]string{"arn": "arn:aws:sqs:us-east-1:000000000000:my-queue"},
	}
	filter := resource.BuildCloudTrailFilter(res, "sqs")
	if filter == nil {
		t.Fatal("expected non-nil filter")
	}
	if filter["ResourceName"] != "arn:aws:sqs:us-east-1:000000000000:my-queue" {
		t.Errorf("expected SQS ARN, got %q", filter["ResourceName"])
	}
}

// TestAllResourceTypesHaveCloudTrailRelated verifies that every registered
// resource type (except ct-events itself) has a CloudTrail Events related entry.
// This exercises the bulk registration in zzz_ct_events_all_related.go.
func TestAllResourceTypesHaveCloudTrailRelated(t *testing.T) {
	shortNames := resource.AllShortNames()
	if len(shortNames) == 0 {
		t.Fatal("AllShortNames returned empty slice — registry not initialized")
	}

	for _, sn := range shortNames {
		if sn == "ct-events" {
			continue // CloudTrail Events doesn't need a self-reference
		}
		related := resource.GetRelated(sn)
		found := false
		for _, def := range related {
			if def.TargetType == "ct-events" {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("resource type %q has no ct-events related entry", sn)
		}
	}
}

// TestEC2StillHasCloudTrailRelated verifies that EC2 has a ct-events entry
// and still has all of its other related entries (>= 9 total).
func TestEC2StillHasCloudTrailRelated(t *testing.T) {
	related := resource.GetRelated("ec2")
	if related == nil {
		t.Fatal("GetRelated(\"ec2\") returned nil")
	}

	foundCT := false
	for _, def := range related {
		if def.TargetType == "ct-events" {
			foundCT = true
			if def.DisplayName != "CloudTrail Events" {
				t.Errorf("ct-events entry DisplayName = %q, want %q", def.DisplayName, "CloudTrail Events")
			}
			break
		}
	}
	if !foundCT {
		t.Error("ec2 related defs missing ct-events entry")
	}
	if len(related) < 9 {
		t.Errorf("ec2 related defs length = %d, want >= 9 (other entries must still be present)", len(related))
	}
}

// TestIAMUserStillHasCloudTrailRelated verifies that iam-user has a ct-events
// entry and still has its iam-group and policy entries (>= 3 total).
func TestIAMUserStillHasCloudTrailRelated(t *testing.T) {
	related := resource.GetRelated("iam-user")
	if related == nil {
		t.Fatal("GetRelated(\"iam-user\") returned nil")
	}

	foundCT := false
	for _, def := range related {
		if def.TargetType == "ct-events" {
			foundCT = true
			break
		}
	}
	if !foundCT {
		t.Error("iam-user related defs missing ct-events entry")
	}
	if len(related) < 3 {
		t.Errorf("iam-user related defs length = %d, want >= 3 (iam-group + policy entries must still be present)", len(related))
	}
}

// ---------------------------------------------------------------------------
// Demo-mode filter construction: ARN-based resource types
// ---------------------------------------------------------------------------

// TestCloudTrailFilter_DemoMode_Lambda verifies that BuildCloudTrailFilter for
// a lambda resource produces a ResourceName filter containing the full ARN.
func TestCloudTrailFilter_DemoMode_Lambda(t *testing.T) {
	res := resource.Resource{
		ID:   "process-orders",
		Name: "process-orders",
		Fields: map[string]string{
			"arn": "arn:aws:lambda:us-east-1:123456789012:function:process-orders",
		},
	}
	filter := resource.BuildCloudTrailFilter(res, "lambda")
	if filter == nil {
		t.Fatal("BuildCloudTrailFilter(lambda) returned nil")
	}
	want := "arn:aws:lambda:us-east-1:123456789012:function:process-orders"
	if got := filter["ResourceName"]; got != want {
		t.Errorf("ResourceName = %q, want %q", got, want)
	}
}

// TestCloudTrailFilter_DemoMode_RDS verifies that BuildCloudTrailFilter for a
// dbi resource produces a ResourceName filter containing the full RDS ARN.
func TestCloudTrailFilter_DemoMode_RDS(t *testing.T) {
	res := resource.Resource{
		ID:   "prod-api-primary",
		Name: "prod-api-primary",
		Fields: map[string]string{
			"arn": "arn:aws:rds:us-east-1:123456789012:db:prod-api-primary",
		},
	}
	filter := resource.BuildCloudTrailFilter(res, "dbi")
	if filter == nil {
		t.Fatal("BuildCloudTrailFilter(dbi) returned nil")
	}
	want := "arn:aws:rds:us-east-1:123456789012:db:prod-api-primary"
	if got := filter["ResourceName"]; got != want {
		t.Errorf("ResourceName = %q, want %q", got, want)
	}
}

// TestCloudTrailFilter_DemoMode_EKS verifies that BuildCloudTrailFilter for an
// eks resource produces a ResourceName filter containing the full EKS ARN.
func TestCloudTrailFilter_DemoMode_EKS(t *testing.T) {
	res := resource.Resource{
		ID:   "acme-prod",
		Name: "acme-prod",
		Fields: map[string]string{
			"arn": "arn:aws:eks:us-east-1:123456789012:cluster/acme-prod",
		},
	}
	filter := resource.BuildCloudTrailFilter(res, "eks")
	if filter == nil {
		t.Fatal("BuildCloudTrailFilter(eks) returned nil")
	}
	want := "arn:aws:eks:us-east-1:123456789012:cluster/acme-prod"
	if got := filter["ResourceName"]; got != want {
		t.Errorf("ResourceName = %q, want %q", got, want)
	}
}

// TestCloudTrailFilter_DemoMode_Secrets verifies that BuildCloudTrailFilter for
// a secrets resource produces a ResourceName filter with the full Secrets ARN.
func TestCloudTrailFilter_DemoMode_Secrets(t *testing.T) {
	res := resource.Resource{
		ID:   "prod/database/primary",
		Name: "prod/database/primary",
		Fields: map[string]string{
			"arn": "arn:aws:secretsmanager:us-east-1:123456789012:secret:prod/database/primary-AbCdEf",
		},
	}
	filter := resource.BuildCloudTrailFilter(res, "secrets")
	if filter == nil {
		t.Fatal("BuildCloudTrailFilter(secrets) returned nil")
	}
	want := "arn:aws:secretsmanager:us-east-1:123456789012:secret:prod/database/primary-AbCdEf"
	if got := filter["ResourceName"]; got != want {
		t.Errorf("ResourceName = %q, want %q", got, want)
	}
}

// TestCloudTrailFilter_DemoMode_DocDB verifies that BuildCloudTrailFilter for a
// dbc (DocDB) resource produces a ResourceName filter with the full RDS ARN.
// Note: the demo CT fixture has no DocDB events, so this only verifies filter
// construction — not event lookup.
func TestCloudTrailFilter_DemoMode_DocDB(t *testing.T) {
	res := resource.Resource{
		ID:   "acme-docdb-prod",
		Name: "acme-docdb-prod",
		Fields: map[string]string{
			"arn": "arn:aws:rds:us-east-1:123456789012:cluster:acme-docdb-prod",
		},
	}
	filter := resource.BuildCloudTrailFilter(res, "dbc")
	if filter == nil {
		t.Fatal("BuildCloudTrailFilter(dbc) returned nil")
	}
	want := "arn:aws:rds:us-east-1:123456789012:cluster:acme-docdb-prod"
	if got := filter["ResourceName"]; got != want {
		t.Errorf("ResourceName = %q, want %q", got, want)
	}
}

// TestCloudTrailFake_SuffixMatching verifies that the CloudTrailFake matches
// events by bare resource name via the ":<name>" suffix rule. The demo fixture
// contains a lambda event with ResourceName
// "arn:aws:lambda:us-east-1:123456789012:function:process-orders"; looking up
// by the bare name "process-orders" must return at least one event.
func TestCloudTrailFake_SuffixMatching(t *testing.T) {
	fake := fakes.NewCloudTrail()
	input := &cloudtrail.LookupEventsInput{
		LookupAttributes: []cloudtrailtypes.LookupAttribute{
			{
				AttributeKey:   cloudtrailtypes.LookupAttributeKeyResourceName,
				AttributeValue: aws.String("process-orders"),
			},
		},
	}
	out, err := fake.LookupEvents(context.Background(), input)
	if err != nil {
		t.Fatalf("LookupEvents error: %v", err)
	}
	if len(out.Events) == 0 {
		t.Error("expected at least one event matching suffix ':process-orders', got 0")
	}
}
