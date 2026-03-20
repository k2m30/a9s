package unit

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/efs"
	efstypes "github.com/aws/aws-sdk-go-v2/service/efs/types"

	awsclient "github.com/k2m30/a9s/internal/aws"
)

// ---------------------------------------------------------------------------
// EFS File System fetcher tests
// ---------------------------------------------------------------------------

func TestFetchEFSFileSystems_ParsesMultiple(t *testing.T) {
	now := time.Now()
	mock := &mockEFSClient{
		output: &efs.DescribeFileSystemsOutput{
			FileSystems: []efstypes.FileSystemDescription{
				{
					FileSystemId:        aws.String("fs-12345678"),
					FileSystemArn:       aws.String("arn:aws:elasticfilesystem:us-east-1:123456789012:file-system/fs-12345678"),
					CreationTime:        &now,
					CreationToken:       aws.String("console-abc123"),
					LifeCycleState:      efstypes.LifeCycleStateAvailable,
					Name:                aws.String("shared-data"),
					NumberOfMountTargets: 3,
					OwnerId:             aws.String("123456789012"),
					PerformanceMode:     efstypes.PerformanceModeGeneralPurpose,
					ThroughputMode:      efstypes.ThroughputModeBursting,
					Encrypted:           aws.Bool(true),
					SizeInBytes: &efstypes.FileSystemSize{
						Value: int64(1073741824),
					},
					Tags: []efstypes.Tag{
						{Key: aws.String("Name"), Value: aws.String("shared-data")},
						{Key: aws.String("Env"), Value: aws.String("prod")},
					},
				},
				{
					FileSystemId:        aws.String("fs-87654321"),
					FileSystemArn:       aws.String("arn:aws:elasticfilesystem:us-east-1:123456789012:file-system/fs-87654321"),
					CreationTime:        &now,
					CreationToken:       aws.String("console-def456"),
					LifeCycleState:      efstypes.LifeCycleStateCreating,
					NumberOfMountTargets: 0,
					OwnerId:             aws.String("123456789012"),
					PerformanceMode:     efstypes.PerformanceModeMaxIo,
					ThroughputMode:      efstypes.ThroughputModeProvisioned,
					Encrypted:           aws.Bool(false),
					SizeInBytes: &efstypes.FileSystemSize{
						Value: int64(0),
					},
					Tags: []efstypes.Tag{},
				},
			},
		},
	}

	resources, err := awsclient.FetchEFSFileSystems(context.Background(), mock)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(resources) != 2 {
		t.Fatalf("expected 2 resources, got %d", len(resources))
	}

	// Verify first file system
	r0 := resources[0]
	if r0.ID != "fs-12345678" {
		t.Errorf("resource[0].ID: expected %q, got %q", "fs-12345678", r0.ID)
	}
	if r0.Name != "shared-data" {
		t.Errorf("resource[0].Name: expected %q, got %q", "shared-data", r0.Name)
	}
	if r0.Status != "available" {
		t.Errorf("resource[0].Status: expected %q, got %q", "available", r0.Status)
	}

	// Verify required fields
	requiredFields := []string{"file_system_id", "name", "life_cycle_state", "performance_mode", "throughput_mode", "encrypted", "mount_targets"}
	for i, r := range resources {
		for _, key := range requiredFields {
			if _, ok := r.Fields[key]; !ok {
				t.Errorf("resource[%d].Fields missing key %q", i, key)
			}
		}
	}

	if r0.Fields["file_system_id"] != "fs-12345678" {
		t.Errorf("resource[0].Fields[\"file_system_id\"]: expected %q, got %q", "fs-12345678", r0.Fields["file_system_id"])
	}
	if r0.Fields["encrypted"] != "true" {
		t.Errorf("resource[0].Fields[\"encrypted\"]: expected %q, got %q", "true", r0.Fields["encrypted"])
	}
	if r0.Fields["mount_targets"] != "3" {
		t.Errorf("resource[0].Fields[\"mount_targets\"]: expected %q, got %q", "3", r0.Fields["mount_targets"])
	}
	if r0.Fields["performance_mode"] != "generalPurpose" {
		t.Errorf("resource[0].Fields[\"performance_mode\"]: expected %q, got %q", "generalPurpose", r0.Fields["performance_mode"])
	}

	// Verify second file system (creating, no name)
	r1 := resources[1]
	if r1.Status != "creating" {
		t.Errorf("resource[1].Status: expected %q, got %q", "creating", r1.Status)
	}
	if r1.Name != "" {
		t.Errorf("resource[1].Name: expected empty, got %q", r1.Name)
	}
}

func TestFetchEFSFileSystems_RawStructPopulated(t *testing.T) {
	now := time.Now()
	mock := &mockEFSClient{
		output: &efs.DescribeFileSystemsOutput{
			FileSystems: []efstypes.FileSystemDescription{
				{
					FileSystemId:        aws.String("fs-raw123"),
					CreationTime:        &now,
					CreationToken:       aws.String("token-raw"),
					LifeCycleState:      efstypes.LifeCycleStateAvailable,
					NumberOfMountTargets: 1,
					OwnerId:             aws.String("123456789012"),
					PerformanceMode:     efstypes.PerformanceModeGeneralPurpose,
					SizeInBytes:         &efstypes.FileSystemSize{Value: int64(0)},
					Tags:                []efstypes.Tag{},
				},
			},
		},
	}

	resources, err := awsclient.FetchEFSFileSystems(context.Background(), mock)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	r := resources[0]
	if r.RawStruct == nil {
		t.Fatal("RawStruct must not be nil")
	}
	fs, ok := r.RawStruct.(efstypes.FileSystemDescription)
	if !ok {
		t.Fatalf("RawStruct should be efstypes.FileSystemDescription, got %T", r.RawStruct)
	}
	if fs.FileSystemId == nil || *fs.FileSystemId != "fs-raw123" {
		t.Errorf("RawStruct.FileSystemId: expected %q", "fs-raw123")
	}
}

func TestFetchEFSFileSystems_ErrorResponse(t *testing.T) {
	mock := &mockEFSClient{
		err: fmt.Errorf("AWS API error: access denied"),
	}

	resources, err := awsclient.FetchEFSFileSystems(context.Background(), mock)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	if resources != nil {
		t.Errorf("expected nil resources on error, got %d", len(resources))
	}
}

func TestFetchEFSFileSystems_EmptyResponse(t *testing.T) {
	mock := &mockEFSClient{
		output: &efs.DescribeFileSystemsOutput{
			FileSystems: []efstypes.FileSystemDescription{},
		},
	}

	resources, err := awsclient.FetchEFSFileSystems(context.Background(), mock)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(resources) != 0 {
		t.Errorf("expected 0 resources, got %d", len(resources))
	}
}
