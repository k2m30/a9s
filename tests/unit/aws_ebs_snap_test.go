package unit

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/domain"
)

// ---------------------------------------------------------------------------
// EBS Snapshot fetcher tests
// ---------------------------------------------------------------------------

func TestFetchEBSSnapshots_ParsesMultipleSnapshots(t *testing.T) {
	startTime := time.Date(2025, 2, 20, 9, 15, 0, 0, time.UTC)

	mock := &mockEC2DescribeSnapshotsClient{
		output: &ec2.DescribeSnapshotsOutput{
			Snapshots: []ec2types.Snapshot{
				{
					SnapshotId:  aws.String("snap-0aabb11cc"),
					State:       ec2types.SnapshotStateCompleted,
					VolumeId:    aws.String("vol-111aabbcc"),
					VolumeSize:  aws.Int32(100),
					Encrypted:   aws.Bool(true),
					Description: aws.String("Daily backup snapshot"),
					StartTime:   &startTime,
					Progress:    aws.String("100%"),
					Tags: []ec2types.Tag{
						{Key: aws.String("Name"), Value: aws.String("prod-snap-daily")},
					},
				},
				{
					SnapshotId:  aws.String("snap-0ccdd22ee"),
					State:       ec2types.SnapshotStatePending,
					VolumeId:    aws.String("vol-222ddeeff"),
					VolumeSize:  aws.Int32(50),
					Encrypted:   aws.Bool(false),
					Description: aws.String(""),
					StartTime:   &startTime,
					Progress:    aws.String("42%"),
					Tags:        []ec2types.Tag{},
				},
			},
		},
	}

	resources, err := awsclient.FetchEBSSnapshots(context.Background(), mock)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(resources) != 2 {
		t.Fatalf("expected 2 resources, got %d", len(resources))
	}

	// Verify first snapshot (completed, with Name tag)
	r0 := resources[0]
	if r0.ID != "snap-0aabb11cc" {
		t.Errorf("resource[0].ID: expected %q, got %q", "snap-0aabb11cc", r0.ID)
	}
	if r0.Name != "prod-snap-daily" {
		t.Errorf("resource[0].Name: expected %q, got %q", "prod-snap-daily", r0.Name)
	}
	// Post-fold contract: completed state is healthy → no Status, no Finding.
	if len(r0.Findings) != 0 {
		t.Errorf("resource[0].Findings: expected 0 for completed snapshot, got %d", len(r0.Findings))
	}

	// Verify second snapshot (pending, no Name tag)
	r1 := resources[1]
	if r1.ID != "snap-0ccdd22ee" {
		t.Errorf("resource[1].ID: expected %q, got %q", "snap-0ccdd22ee", r1.ID)
	}
	if r1.Name != "" {
		t.Errorf("resource[1].Name: expected empty string (no Name tag), got %q", r1.Name)
	}
	// Post-fold contract: pending state emits SevWarn Finding, not Status.
	if len(r1.Findings) != 1 {
		t.Fatalf("resource[1].Findings: expected 1 for pending snapshot, got %d", len(r1.Findings))
	}
	if r1.Findings[0].Code != awsclient.CodeEBSSnapStatePending {
		t.Errorf("resource[1].Findings[0].Code: expected %q, got %q", awsclient.CodeEBSSnapStatePending, r1.Findings[0].Code)
	}
	if r1.Findings[0].Severity != domain.SevWarn {
		t.Errorf("resource[1].Findings[0].Severity: expected domain.SevWarn, got %v", r1.Findings[0].Severity)
	}
}

func TestFetchEBSSnapshots_EmptyResponse(t *testing.T) {
	mock := &mockEC2DescribeSnapshotsClient{
		output: &ec2.DescribeSnapshotsOutput{
			Snapshots: []ec2types.Snapshot{},
		},
	}

	resources, err := awsclient.FetchEBSSnapshots(context.Background(), mock)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(resources) != 0 {
		t.Errorf("expected 0 resources, got %d", len(resources))
	}
}

func TestFetchEBSSnapshots_ErrorResponse(t *testing.T) {
	mock := &mockEC2DescribeSnapshotsClient{
		output: nil,
		err:    fmt.Errorf("AWS API error: access denied"),
	}

	resources, err := awsclient.FetchEBSSnapshots(context.Background(), mock)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	if resources != nil {
		t.Errorf("expected nil resources on error, got %d resources", len(resources))
	}
}

func TestFetchEBSSnapshots_FieldExtraction(t *testing.T) {
	startTime := time.Date(2025, 2, 20, 9, 15, 0, 0, time.UTC)

	mock := &mockEC2DescribeSnapshotsClient{
		output: &ec2.DescribeSnapshotsOutput{
			Snapshots: []ec2types.Snapshot{
				{
					SnapshotId:  aws.String("snap-0aabb11cc"),
					State:       ec2types.SnapshotStateCompleted,
					VolumeId:    aws.String("vol-111aabbcc"),
					VolumeSize:  aws.Int32(100),
					Encrypted:   aws.Bool(true),
					Description: aws.String("Daily backup snapshot"),
					StartTime:   &startTime,
					Progress:    aws.String("100%"),
					Tags: []ec2types.Tag{
						{Key: aws.String("Name"), Value: aws.String("prod-snap-daily")},
					},
				},
			},
		},
	}

	resources, err := awsclient.FetchEBSSnapshots(context.Background(), mock)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(resources))
	}

	r := resources[0]

	// Verify all FieldKeys are present and have exact values
	if r.Fields["snapshot_id"] != "snap-0aabb11cc" {
		t.Errorf("Fields[\"snapshot_id\"]: expected %q, got %q", "snap-0aabb11cc", r.Fields["snapshot_id"])
	}
	if r.Fields["name"] != "prod-snap-daily" {
		t.Errorf("Fields[\"name\"]: expected %q, got %q", "prod-snap-daily", r.Fields["name"])
	}
	if r.Fields["state"] != "completed" {
		t.Errorf("Fields[\"state\"]: expected %q, got %q", "completed", r.Fields["state"])
	}
	if r.Fields["volume_id"] != "vol-111aabbcc" {
		t.Errorf("Fields[\"volume_id\"]: expected %q, got %q", "vol-111aabbcc", r.Fields["volume_id"])
	}
	if r.Fields["size"] != "100" {
		t.Errorf("Fields[\"size\"]: expected %q, got %q", "100", r.Fields["size"])
	}
	if r.Fields["encrypted"] != "true" {
		t.Errorf("Fields[\"encrypted\"]: expected %q, got %q", "true", r.Fields["encrypted"])
	}
	if r.Fields["description"] != "Daily backup snapshot" {
		t.Errorf("Fields[\"description\"]: expected %q, got %q", "Daily backup snapshot", r.Fields["description"])
	}
	if r.Fields["started"] != "2025-02-20 09:15" {
		t.Errorf("Fields[\"started\"]: expected %q, got %q", "2025-02-20 09:15", r.Fields["started"])
	}
	if r.Fields["progress"] != "100%" {
		t.Errorf("Fields[\"progress\"]: expected %q, got %q", "100%", r.Fields["progress"])
	}
}

func TestFetchEBSSnapshots_NoNameTag(t *testing.T) {
	mock := &mockEC2DescribeSnapshotsClient{
		output: &ec2.DescribeSnapshotsOutput{
			Snapshots: []ec2types.Snapshot{
				{
					SnapshotId: aws.String("snap-noname"),
					State:      ec2types.SnapshotStateCompleted,
					VolumeId:   aws.String("vol-abc123"),
					Tags:       []ec2types.Tag{},
				},
			},
		},
	}

	resources, err := awsclient.FetchEBSSnapshots(context.Background(), mock)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(resources))
	}

	if resources[0].Name != "" {
		t.Errorf("Name: expected empty string (no Name tag), got %q", resources[0].Name)
	}
	if resources[0].Fields["name"] != "" {
		t.Errorf("Fields[\"name\"]: expected empty string (no Name tag), got %q", resources[0].Fields["name"])
	}
}

func TestFetchEBSSnapshots_RawStructIsSnapshot(t *testing.T) {
	mock := &mockEC2DescribeSnapshotsClient{
		output: &ec2.DescribeSnapshotsOutput{
			Snapshots: []ec2types.Snapshot{
				{
					SnapshotId: aws.String("snap-rawstruct"),
					State:      ec2types.SnapshotStateCompleted,
					VolumeId:   aws.String("vol-abc123"),
				},
			},
		},
	}

	resources, err := awsclient.FetchEBSSnapshots(context.Background(), mock)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(resources))
	}

	r := resources[0]
	if r.RawStruct == nil {
		t.Fatal("RawStruct must not be nil")
	}

	snap, ok := r.RawStruct.(ec2types.Snapshot)
	if !ok {
		t.Fatalf("RawStruct should be ec2types.Snapshot, got %T", r.RawStruct)
	}
	if snap.SnapshotId == nil || *snap.SnapshotId != "snap-rawstruct" {
		t.Errorf("RawStruct.SnapshotId: expected %q, got %v", "snap-rawstruct", snap.SnapshotId)
	}
}
