package unit_test

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	cwtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"

	_ "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/demo"
	"github.com/k2m30/a9s/v3/internal/resource"
)

func kinesisCheckerByTarget(t *testing.T, target string) resource.RelatedChecker {
	t.Helper()
	for _, def := range resource.GetRelated("kinesis") {
		if def.TargetType == target {
			if def.Checker == nil {
				t.Fatalf("kinesis related checker for %s is nil", target)
			}
			return def.Checker
		}
	}
	t.Fatalf("kinesis related checker for %s not found", target)
	return nil
}

// --- Navigable Fields ---

func TestNavigableFields_Kinesis_None(t *testing.T) {
	nav := resource.IsFieldNavigable("kinesis", "StreamName")
	if nav != nil {
		t.Errorf("expected no navigable fields for kinesis, but StreamName resolved to %v", nav)
	}
}

// --- CloudWatch Alarms checker (Pattern C — cache, StreamName dimension) ---

func TestRelated_Kinesis_Alarms_Found(t *testing.T) {
	const streamName = "clickstream-ingest"

	alarmRes := resource.Resource{
		ID: "kinesis-iterator-age",
		RawStruct: cwtypes.MetricAlarm{
			AlarmName: aws.String("kinesis-iterator-age"),
			Dimensions: []cwtypes.Dimension{
				{Name: aws.String("StreamName"), Value: aws.String(streamName)},
			},
		},
	}
	cache := resource.ResourceCache{
		"alarm": resource.ResourceCacheEntry{Resources: []resource.Resource{alarmRes}},
	}
	source := resource.Resource{
		ID:   streamName,
		Name: streamName,
		Fields: map[string]string{
			"stream_name": streamName,
		},
	}

	checker := kinesisCheckerByTarget(t, "alarm")
	result := checker(context.Background(), nil, source, cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != "kinesis-iterator-age" {
		t.Errorf("ResourceIDs = %v, want [kinesis-iterator-age]", result.ResourceIDs)
	}
	if result.Err != nil {
		t.Errorf("unexpected error: %v", result.Err)
	}
}

func TestRelated_Kinesis_Alarms_NotFound(t *testing.T) {
	const streamName = "clickstream-ingest"

	alarmRes := resource.Resource{
		ID: "other-stream-alarm",
		RawStruct: cwtypes.MetricAlarm{
			AlarmName: aws.String("other-stream-alarm"),
			Dimensions: []cwtypes.Dimension{
				{Name: aws.String("StreamName"), Value: aws.String("different-stream")},
			},
		},
	}
	cache := resource.ResourceCache{
		"alarm": resource.ResourceCacheEntry{Resources: []resource.Resource{alarmRes}},
	}
	source := resource.Resource{
		ID:   streamName,
		Name: streamName,
		Fields: map[string]string{
			"stream_name": streamName,
		},
	}

	checker := kinesisCheckerByTarget(t, "alarm")
	result := checker(context.Background(), nil, source, cache)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0", result.Count)
	}
}

func TestRelated_Kinesis_Alarms_CacheMissNoClients(t *testing.T) {
	source := resource.Resource{
		ID:   "clickstream-ingest",
		Name: "clickstream-ingest",
		Fields: map[string]string{
			"stream_name": "clickstream-ingest",
		},
	}

	checker := kinesisCheckerByTarget(t, "alarm")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (unknown)", result.Count)
	}
}

// --- kinesis→lambda: undeterminable from cache, returns Count: 0 ---

func TestRelated_Kinesis_Lambda_ReturnsZero(t *testing.T) {
	source := resource.Resource{
		ID:   "clickstream-ingest",
		Name: "clickstream-ingest",
	}
	checker := kinesisCheckerByTarget(t, "lambda")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})
	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (undeterminable from cache)", result.Count)
	}
	if result.TargetType != "lambda" {
		t.Errorf("TargetType = %q, want %q", result.TargetType, "lambda")
	}
}

// --- kinesis→cfn: undeterminable from cache, returns Count: 0 ---

func TestRelated_Kinesis_CFN_ReturnsZero(t *testing.T) {
	source := resource.Resource{
		ID:   "clickstream-ingest",
		Name: "clickstream-ingest",
	}
	checker := kinesisCheckerByTarget(t, "cfn")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})
	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (undeterminable from cache)", result.Count)
	}
	if result.TargetType != "cfn" {
		t.Errorf("TargetType = %q, want %q", result.TargetType, "cfn")
	}
}

// --- Demo Checker ---

func TestRelatedDemo_Kinesis_Registered(t *testing.T) {
	_ = demo.GetResources // ensure demo package is initialized
	checker := resource.GetRelatedDemo("kinesis")
	if checker == nil {
		t.Fatal("no demo checker registered for kinesis")
	}

	results := checker(resource.Resource{ID: "clickstream-ingest"})
	if len(results) == 0 {
		t.Fatal("demo checker returned no results")
	}
	for _, r := range results {
		if r.TargetType == "" {
			t.Error("demo result has empty TargetType")
		}
	}

	// Verify all expected target types are present.
	wantTargets := map[string]bool{"lambda": false, "alarm": false, "cfn": false}
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
}
