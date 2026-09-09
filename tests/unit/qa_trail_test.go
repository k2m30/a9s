package unit

import (
	"context"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudtrail"
	cloudtrailtypes "github.com/aws/aws-sdk-go-v2/service/cloudtrail/types"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// ---------------------------------------------------------------------------
// T-TRAIL-001 - Test CloudTrail Trails response parsing
// ---------------------------------------------------------------------------

func TestFetchCloudTrailTrails_ParsesMultipleTrails(t *testing.T) {
	mock := &mockCloudTrailClient{
		output: &cloudtrail.DescribeTrailsOutput{
			TrailList: []cloudtrailtypes.Trail{
				{
					Name:                       aws.String("management-events"),
					TrailARN:                   aws.String("arn:aws:cloudtrail:us-east-1:123456789012:trail/management-events"),
					S3BucketName:               aws.String("my-trail-bucket"),
					HomeRegion:                 aws.String("us-east-1"),
					IsMultiRegionTrail:         aws.Bool(true),
					IsOrganizationTrail:        aws.Bool(false),
					LogFileValidationEnabled:   aws.Bool(true),
					IncludeGlobalServiceEvents: aws.Bool(true),
				},
				{
					Name:                     aws.String("data-events"),
					TrailARN:                 aws.String("arn:aws:cloudtrail:us-east-1:123456789012:trail/data-events"),
					S3BucketName:             aws.String("data-trail-bucket"),
					HomeRegion:               aws.String("us-west-2"),
					IsMultiRegionTrail:       aws.Bool(false),
					IsOrganizationTrail:      aws.Bool(false),
					LogFileValidationEnabled: aws.Bool(false),
				},
			},
		},
	}

	resources, err := awsclient.FetchCloudTrailTrails(context.Background(), mock)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(resources) != 2 {
		t.Fatalf("expected 2 resources, got %d", len(resources))
	}

	r := resources[0]
	if r.Name != "management-events" {
		t.Errorf("expected Name 'management-events', got %q", r.Name)
	}
	if r.ID != "management-events" {
		t.Errorf("expected ID 'management-events', got %q", r.ID)
	}
	if r.Fields["trail_name"] != "management-events" {
		t.Errorf("expected Fields[trail_name] 'management-events', got %q", r.Fields["trail_name"])
	}
	if r.Fields["s3_bucket"] != "my-trail-bucket" {
		t.Errorf("expected Fields[s3_bucket] 'my-trail-bucket', got %q", r.Fields["s3_bucket"])
	}
	if r.Fields["home_region"] != "us-east-1" {
		t.Errorf("expected Fields[home_region] 'us-east-1', got %q", r.Fields["home_region"])
	}
	if r.Fields["multi_region"] != "true" {
		t.Errorf("expected Fields[multi_region] 'true', got %q", r.Fields["multi_region"])
	}
	if r.Fields["log_file_validation_enabled"] != "true" {
		t.Errorf("expected Fields[log_file_validation_enabled] 'true', got %q", r.Fields["log_file_validation_enabled"])
	}

	if r.RawStruct == nil {
		t.Error("expected RawStruct to be set")
	}

	// Second trail
	r2 := resources[1]
	if r2.Fields["multi_region"] != "false" {
		t.Errorf("expected Fields[multi_region] 'false', got %q", r2.Fields["multi_region"])
	}
}

func TestFetchCloudTrailTrails_EmptyResponse(t *testing.T) {
	mock := &mockCloudTrailClient{
		output: &cloudtrail.DescribeTrailsOutput{
			TrailList: []cloudtrailtypes.Trail{},
		},
	}

	resources, err := awsclient.FetchCloudTrailTrails(context.Background(), mock)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(resources) != 0 {
		t.Fatalf("expected 0 resources, got %d", len(resources))
	}
}

func TestFetchCloudTrailTrails_APIError(t *testing.T) {
	mock := &mockCloudTrailClient{
		err: &MockAPIError{Code: "UnsupportedOperationException", Message: "unsupported"},
	}

	_, err := awsclient.FetchCloudTrailTrails(context.Background(), mock)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestFetchCloudTrailTrails_NilBoolFields(t *testing.T) {
	mock := &mockCloudTrailClient{
		output: &cloudtrail.DescribeTrailsOutput{
			TrailList: []cloudtrailtypes.Trail{
				{
					Name:         aws.String("bare-trail"),
					S3BucketName: aws.String("bucket"),
				},
			},
		},
	}

	resources, err := awsclient.FetchCloudTrailTrails(context.Background(), mock)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(resources))
	}

	r := resources[0]
	if r.Fields["multi_region"] != "false" {
		t.Errorf("expected Fields[multi_region] 'false', got %q", r.Fields["multi_region"])
	}
}

// TestFetchCloudTrailTrails_LogFileValidationFieldKey verifies that the
// fetcher stores the log file validation flag under the key
// "log_file_validation_enabled", which is exactly the key the colorer reads;
// with the two keys apart the colorer always sees "" and skips the check.
func TestFetchCloudTrailTrails_LogFileValidationFieldKey(t *testing.T) {
	mock := &mockCloudTrailClient{
		output: &cloudtrail.DescribeTrailsOutput{
			TrailList: []cloudtrailtypes.Trail{
				{
					Name:                     aws.String("audit-trail"),
					TrailARN:                 aws.String("arn:aws:cloudtrail:us-east-1:123456789012:trail/audit-trail"),
					S3BucketName:             aws.String("audit-bucket"),
					HomeRegion:               aws.String("us-east-1"),
					IsMultiRegionTrail:       aws.Bool(true),
					IsOrganizationTrail:      aws.Bool(false),
					LogFileValidationEnabled: aws.Bool(false),
				},
			},
		},
		statusByName: map[string]*cloudtrail.GetTrailStatusOutput{
			"audit-trail": {
				IsLogging:           aws.Bool(true),
				LatestDeliveryError: nil,
			},
		},
	}

	resources, err := awsclient.FetchCloudTrailTrails(context.Background(), mock)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(resources))
	}

	r := resources[0]

	// The fetcher MUST store the log file validation flag under the key
	// "log_file_validation_enabled" so the colorer can read it.
	// Currently the fetcher writes "log_validation" — this assertion will FAIL.
	got := r.Fields["log_file_validation_enabled"]
	if got != "false" {
		t.Errorf("Fields[\"log_file_validation_enabled\"] = %q, want %q — fetcher likely writes wrong key (\"log_validation\")", got, "false")
	}

	// Also verify the colorer reaches ColorWarning when is_logging=true,
	// latest_delivery_error="-", and log_file_validation_enabled="false".
	// This exercises the full type → color path after the field-key fix.
	td := resource.FindResourceType("trail")
	if td == nil {
		t.Fatal("trail type not registered")
	}
	colorableFields := map[string]string{
		"is_logging":                  "true",
		"latest_delivery_error":       "-",
		"log_file_validation_enabled": "false",
	}
	got2 := td.Color(resource.Resource{Fields: colorableFields})
	if got2 != resource.ColorWarning {
		t.Errorf("trail Color with log_file_validation_enabled=false should be ColorWarning, got %v", got2)
	}
}

// TestFetchCloudTrailTrails_StaleDeliveryIsBroken pins docs/resources/trail.md
// §3.2: "Signal: LatestDeliveryTime >1h ago on IsLogging==true trail → Broken
// (silent delivery)." A trail that is actively logging (IsLogging==true) but
// whose most recent successful S3 delivery is more than an hour old must be
// classified Broken — CloudTrail is silently failing to ship log files even
// though it reports itself as "logging"; trail.go reads
// GetTrailStatusOutput.LatestDeliveryTime so colorTrail can see it.
func TestFetchCloudTrailTrails_StaleDeliveryIsBroken(t *testing.T) {
	staleDelivery := time.Now().Add(-2 * time.Hour)

	mock := &mockCloudTrailClient{
		output: &cloudtrail.DescribeTrailsOutput{
			TrailList: []cloudtrailtypes.Trail{
				{
					Name:                     aws.String("silent-delivery-trail"),
					TrailARN:                 aws.String("arn:aws:cloudtrail:us-east-1:123456789012:trail/silent-delivery-trail"),
					S3BucketName:             aws.String("audit-bucket"),
					HomeRegion:               aws.String("us-east-1"),
					IsMultiRegionTrail:       aws.Bool(true),
					IsOrganizationTrail:      aws.Bool(false),
					LogFileValidationEnabled: aws.Bool(true),
				},
			},
		},
		statusByName: map[string]*cloudtrail.GetTrailStatusOutput{
			"arn:aws:cloudtrail:us-east-1:123456789012:trail/silent-delivery-trail": {
				IsLogging:          aws.Bool(true),
				LatestDeliveryTime: &staleDelivery,
			},
		},
	}

	resources, err := awsclient.FetchCloudTrailTrails(context.Background(), mock)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(resources))
	}

	r := resources[0]

	foundBroken := false
	for _, f := range r.Findings {
		if f.Severity == domain.SevBroken && f.Source == "wave2" {
			foundBroken = true
		}
	}
	if !foundBroken {
		t.Errorf("expected a wave2 SevBroken Finding for a >1h-stale delivery on a logging trail, got %+v", r.Findings)
	}

	td := resource.FindResourceType("trail")
	if td == nil {
		t.Fatal("trail type not registered")
	}
	gotColor := td.Color(r)
	if gotColor != resource.ColorBroken {
		t.Errorf("trail Color for stale delivery on a logging trail = %v, want ColorBroken (red row per trail.md §4)", gotColor)
	}
}
