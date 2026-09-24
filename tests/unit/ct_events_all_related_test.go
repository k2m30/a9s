package unit

import (
	"context"
	"maps"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudtrail"
	cloudtrailtypes "github.com/aws/aws-sdk-go-v2/service/cloudtrail/types"

	"github.com/k2m30/a9s/v3/core/demo/fakes"
	"github.com/k2m30/a9s/v3/core/resource"
)

func TestAppendRelated_AddsToExisting(t *testing.T) {
	resource.SetRelatedForTest("test_append", []resource.RelatedDef{
		{TargetType: "vpc", DisplayName: "VPCs", Checker: resource.NoopCheckerForTest},
	})
	t.Cleanup(func() { resource.CleanupRelatedForTest("test_append") })

	resource.AppendRelated("test_append", resource.RelatedDef{
		TargetType:  "ct-events",
		DisplayName: "CloudTrail Events",
		Checker:     resource.NoopCheckerForTest,
	})

	got := resource.GetRelated("test_append")
	if len(got) != 2 {
		t.Fatalf("GetRelated length = %d, want 2", len(got))
	}
	if got[1].TargetType != "ct-events" {
		t.Errorf("got[1].TargetType = %q, want %q", got[1].TargetType, "ct-events")
	}
}

func TestAppendRelated_CreatesNew(t *testing.T) {
	t.Cleanup(func() { resource.CleanupRelatedForTest("test_append_new") })

	resource.AppendRelated("test_append_new", resource.RelatedDef{
		TargetType:  "ct-events",
		DisplayName: "CloudTrail Events",
		Checker:     resource.NoopCheckerForTest,
	})

	got := resource.GetRelated("test_append_new")
	if len(got) != 1 {
		t.Fatalf("GetRelated length = %d, want 1", len(got))
	}
	if got[0].TargetType != "ct-events" {
		t.Errorf("got[0].TargetType = %q, want %q", got[0].TargetType, "ct-events")
	}
}

func TestAppendRelated_NoDuplicate(t *testing.T) {
	resource.SetRelatedForTest("test_append_dedup", []resource.RelatedDef{
		{TargetType: "ct-events", DisplayName: "CloudTrail Events", Checker: resource.NoopCheckerForTest},
	})
	t.Cleanup(func() { resource.CleanupRelatedForTest("test_append_dedup") })

	resource.AppendRelated("test_append_dedup", resource.RelatedDef{
		TargetType:  "ct-events",
		DisplayName: "CloudTrail Events",
		Checker:     resource.NoopCheckerForTest,
	})

	got := resource.GetRelated("test_append_dedup")
	if len(got) != 1 {
		t.Errorf("GetRelated length = %d, want 1 (no duplicate)", len(got))
	}
}

// TestBuildCloudTrailFilter_FieldsSource verifies that a type whose
// CloudTrailKey names a Fields entry (ng, "ResourceName:Fields.nodegroup_name")
// takes the filter value from that entry rather than from the row's ID.
func TestBuildCloudTrailFilter_FieldsSource(t *testing.T) {
	res := resource.Resource{
		ID: "acme-prod/workers",
		Fields: map[string]string{
			"nodegroup_name": "workers",
		},
	}

	got := resource.BuildCloudTrailFilter(res, "ng")
	want := map[string]string{
		"ResourceName": "workers",
	}
	if len(got) != len(want) {
		t.Fatalf("filter length = %d, want %d; got %v", len(got), len(want), got)
	}
	if got["ResourceName"] != want["ResourceName"] {
		t.Errorf("filter[ResourceName] = %q, want %q", got["ResourceName"], want["ResourceName"])
	}
}

// TestBuildCloudTrailFilter_IAMUser pins the events of an IAM user: LookupEvents'
// Username attribute also matches an assumed-role session whose session name
// is the user's name, so the lookup keeps only the events whose
// userIdentity.type is IAMUser.
// https://docs.aws.amazon.com/awscloudtrail/latest/userguide/cloudtrail-event-reference-user-identity.html
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
		"_where":   "userIdentity.type=IAMUser",
	}
	if !maps.Equal(got, want) {
		t.Errorf("filter = %v, want %v", got, want)
	}
}

// TestBuildCloudTrailFilter_IAMRole pins the question a role's CloudTrail row
// answers: the events recorded FOR the role — its creation, its policy
// attachments, its trust changes — looked up by the role's own ARN in
// us-east-1, where IAM records them. What a role DID cannot be asked through
// one lookup attribute: for an assumed-role session CloudTrail's Username is
// the session name, and the role lives only in a field no attribute selects.
func TestBuildCloudTrailFilter_IAMRole(t *testing.T) {
	res := resource.Resource{
		ID:   "MyRole",
		Name: "MyRole",
		Fields: map[string]string{
			"role_name": "MyRole",
			"arn":       "arn:aws:iam::000000000000:role/MyRole",
		},
	}

	got := resource.BuildCloudTrailFilter(res, "role")
	want := map[string]string{
		"ResourceName":              "arn:aws:iam::000000000000:role/MyRole",
		resource.CTRegionFilterKey:  "us-east-1",
		resource.CTAltNameFilterKey: "MyRole",
	}
	if len(got) != len(want) {
		t.Fatalf("filter length = %d, want %d; got %v", len(got), len(want), got)
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("filter[%q] = %q, want %q", k, got[k], v)
		}
	}
	if _, ok := got["Username"]; ok {
		t.Errorf("role filter must not carry a server-side Username key: got %v", got)
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

// TestBuildCloudTrailFilter_SQSUsesQueueName pins the value a LookupEvents
// ResourceName lookup matches: the user-created name of the resource — a queue
// name for SQS, "i-1234567" for an EC2 instance — not its ARN
// (docs.aws.amazon.com/awscloudtrail/latest/APIReference/API_Resource.html).
func TestBuildCloudTrailFilter_SQSUsesQueueName(t *testing.T) {
	res := resource.Resource{
		ID:     "my-queue",
		Fields: map[string]string{"arn": "arn:aws:sqs:us-east-1:000000000000:my-queue"},
	}
	filter := resource.BuildCloudTrailFilter(res, "sqs")
	if filter == nil {
		t.Fatal("expected non-nil filter")
	}
	if filter["ResourceName"] != "my-queue" {
		t.Errorf("expected the queue name, got %q", filter["ResourceName"])
	}
}

// TestAllResourceTypesHaveCloudTrailRelated verifies that every registered
// resource type (except ct-events itself) has a CloudTrail Events related entry.
// Each top-level catalog entry declares its own ct-events RelatedDef in its
// struct literal; this sweep catches a type whose declaration was dropped.
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
// alongside all of its other related entries (>= 9 total).
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
// entry alongside its iam-group and policy entries (>= 3 total).
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

// TestCloudTrailFilter_LooksUpTheRowsName pins the value these types' `t`
// hotkey sends as the ResourceName lookup: the user-created name, which is the
// shape CloudTrail records for each of them
// (docs.aws.amazon.com/awscloudtrail/latest/APIReference/API_Resource.html).
// A service that records the ARN instead is keyed on the ARN, as ECR is.
func TestCloudTrailFilter_LooksUpTheRowsName(t *testing.T) {
	cases := []struct {
		shortName string
		id        string
		arn       string
	}{
		{"lambda", "process-orders", "arn:aws:lambda:us-east-1:123456789012:function:process-orders"},
		{"dbi", "prod-api-primary", "arn:aws:rds:us-east-1:123456789012:db:prod-api-primary"},
		{"eks", "acme-prod", "arn:aws:eks:us-east-1:123456789012:cluster/acme-prod"},
		{"secrets", "prod/database/primary", "arn:aws:secretsmanager:us-east-1:123456789012:secret:prod/database/primary-AbCdEf"},
		{"dbc", "acme-docdb-prod", "arn:aws:rds:us-east-1:123456789012:cluster:acme-docdb-prod"},
	}
	for _, tc := range cases {
		t.Run(tc.shortName, func(t *testing.T) {
			res := resource.Resource{ID: tc.id, Name: tc.id, Fields: map[string]string{"arn": tc.arn}}
			filter := resource.BuildCloudTrailFilter(res, tc.shortName)
			if filter == nil {
				t.Fatalf("BuildCloudTrailFilter(%s) returned nil", tc.shortName)
			}
			if got := filter["ResourceName"]; got != tc.id {
				t.Errorf("ResourceName = %q, want %q", got, tc.id)
			}
		})
	}
}

// TestCloudTrailFake_MatchesTheRecordedName verifies that the demo
// CloudTrailFake answers a ResourceName lookup the way LookupEvents does:
// only an event whose recorded ResourceName equals the attribute value.
func TestCloudTrailFake_MatchesTheRecordedName(t *testing.T) {
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
		t.Error("expected at least one event recorded for \"process-orders\", got 0")
	}

	input.LookupAttributes[0].AttributeValue = aws.String("arn:aws:lambda:us-east-1:123456789012:function:process-orders")
	byARN, err := fake.LookupEvents(context.Background(), input)
	if err != nil {
		t.Fatalf("LookupEvents error: %v", err)
	}
	if len(byARN.Events) != 0 {
		t.Errorf("a lookup for the ARN returned %d events; the fixture records the name", len(byARN.Events))
	}
}
