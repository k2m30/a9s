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
)

// ---------------------------------------------------------------------------
// EBS Volume fetcher tests
// ---------------------------------------------------------------------------

func TestFetchEBSVolumes_ParsesMultipleVolumes(t *testing.T) {
	createTime := time.Date(2025, 3, 10, 14, 0, 0, 0, time.UTC)

	mock := &mockEC2DescribeVolumesClient{
		output: &ec2.DescribeVolumesOutput{
			Volumes: []ec2types.Volume{
				{
					VolumeId:         aws.String("vol-111aabbcc"),
					State:            ec2types.VolumeStateInUse,
					Size:             aws.Int32(100),
					VolumeType:       ec2types.VolumeTypeGp3,
					Iops:             aws.Int32(3000),
					Encrypted:        aws.Bool(true),
					AvailabilityZone: aws.String("us-east-1a"),
					CreateTime:       &createTime,
					Tags: []ec2types.Tag{
						{Key: aws.String("Name"), Value: aws.String("prod-data-vol")},
					},
					Attachments: []ec2types.VolumeAttachment{
						{InstanceId: aws.String("i-0abc123456")},
					},
				},
				{
					VolumeId:         aws.String("vol-222ddeeff"),
					State:            ec2types.VolumeStateAvailable,
					Size:             aws.Int32(50),
					VolumeType:       ec2types.VolumeTypeGp2,
					Iops:             aws.Int32(150),
					Encrypted:        aws.Bool(false),
					AvailabilityZone: aws.String("us-east-1b"),
					CreateTime:       &createTime,
					Tags:             []ec2types.Tag{},
					Attachments:      []ec2types.VolumeAttachment{},
				},
			},
		},
	}

	resources, err := awsclient.FetchEBSVolumes(context.Background(), mock)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(resources) != 2 {
		t.Fatalf("expected 2 resources, got %d", len(resources))
	}

	// Verify first volume (in-use, with Name tag, with attachment)
	r0 := resources[0]
	if r0.ID != "vol-111aabbcc" {
		t.Errorf("resource[0].ID: expected %q, got %q", "vol-111aabbcc", r0.ID)
	}
	if r0.Name != "prod-data-vol" {
		t.Errorf("resource[0].Name: expected %q, got %q", "prod-data-vol", r0.Name)
	}
	// Post-fold contract: in-use state is healthy → no Status, no Finding.
	if len(r0.Findings) != 0 {
		t.Errorf("resource[0].Findings: expected 0 for in-use volume, got %d", len(r0.Findings))
	}

	// Verify second volume (available, no Name tag, no attachment)
	r1 := resources[1]
	if r1.ID != "vol-222ddeeff" {
		t.Errorf("resource[1].ID: expected %q, got %q", "vol-222ddeeff", r1.ID)
	}
	if r1.Name != "" {
		t.Errorf("resource[1].Name: expected empty string (no Name tag), got %q", r1.Name)
	}
	// Post-fold contract: available state is healthy for EBS volumes → no Status, no Finding.
	if len(r1.Findings) != 0 {
		t.Errorf("resource[1].Findings: expected 0 for available volume, got %d", len(r1.Findings))
	}
}

func TestFetchEBSVolumes_EmptyResponse(t *testing.T) {
	mock := &mockEC2DescribeVolumesClient{
		output: &ec2.DescribeVolumesOutput{
			Volumes: []ec2types.Volume{},
		},
	}

	resources, err := awsclient.FetchEBSVolumes(context.Background(), mock)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(resources) != 0 {
		t.Errorf("expected 0 resources, got %d", len(resources))
	}
}

func TestFetchEBSVolumes_ErrorResponse(t *testing.T) {
	mock := &mockEC2DescribeVolumesClient{
		output: nil,
		err:    fmt.Errorf("AWS API error: access denied"),
	}

	resources, err := awsclient.FetchEBSVolumes(context.Background(), mock)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	if resources != nil {
		t.Errorf("expected nil resources on error, got %d resources", len(resources))
	}
}

func TestFetchEBSVolumes_FieldExtraction(t *testing.T) {
	createTime := time.Date(2025, 3, 10, 14, 0, 0, 0, time.UTC)

	mock := &mockEC2DescribeVolumesClient{
		output: &ec2.DescribeVolumesOutput{
			Volumes: []ec2types.Volume{
				{
					VolumeId:         aws.String("vol-111aabbcc"),
					State:            ec2types.VolumeStateInUse,
					Size:             aws.Int32(100),
					VolumeType:       ec2types.VolumeTypeGp3,
					Iops:             aws.Int32(3000),
					Encrypted:        aws.Bool(true),
					AvailabilityZone: aws.String("us-east-1a"),
					CreateTime:       &createTime,
					Tags: []ec2types.Tag{
						{Key: aws.String("Name"), Value: aws.String("prod-data-vol")},
					},
					Attachments: []ec2types.VolumeAttachment{
						{InstanceId: aws.String("i-0abc123456")},
					},
				},
			},
		},
	}

	resources, err := awsclient.FetchEBSVolumes(context.Background(), mock)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(resources))
	}

	r := resources[0]

	// Verify all FieldKeys are present and have exact values
	if r.Fields["volume_id"] != "vol-111aabbcc" {
		t.Errorf("Fields[\"volume_id\"]: expected %q, got %q", "vol-111aabbcc", r.Fields["volume_id"])
	}
	if r.Fields["name"] != "prod-data-vol" {
		t.Errorf("Fields[\"name\"]: expected %q, got %q", "prod-data-vol", r.Fields["name"])
	}
	if r.Fields["state"] != "in-use" {
		t.Errorf("Fields[\"state\"]: expected %q, got %q", "in-use", r.Fields["state"])
	}
	if r.Fields["size"] != "100" {
		t.Errorf("Fields[\"size\"]: expected %q, got %q", "100", r.Fields["size"])
	}
	if r.Fields["type"] != "gp3" {
		t.Errorf("Fields[\"type\"]: expected %q, got %q", "gp3", r.Fields["type"])
	}
	if r.Fields["iops"] != "3000" {
		t.Errorf("Fields[\"iops\"]: expected %q, got %q", "3000", r.Fields["iops"])
	}
	if r.Fields["encrypted"] != "true" {
		t.Errorf("Fields[\"encrypted\"]: expected %q, got %q", "true", r.Fields["encrypted"])
	}
	if r.Fields["attached_to"] != "i-0abc123456" {
		t.Errorf("Fields[\"attached_to\"]: expected %q, got %q", "i-0abc123456", r.Fields["attached_to"])
	}
	if r.Fields["az"] != "us-east-1a" {
		t.Errorf("Fields[\"az\"]: expected %q, got %q", "us-east-1a", r.Fields["az"])
	}
	if r.Fields["created"] != "2025-03-10 14:00" {
		t.Errorf("Fields[\"created\"]: expected %q, got %q", "2025-03-10 14:00", r.Fields["created"])
	}
}

func TestFetchEBSVolumes_NoAttachment(t *testing.T) {
	mock := &mockEC2DescribeVolumesClient{
		output: &ec2.DescribeVolumesOutput{
			Volumes: []ec2types.Volume{
				{
					VolumeId:    aws.String("vol-unattached"),
					State:       ec2types.VolumeStateAvailable,
					Size:        aws.Int32(20),
					VolumeType:  ec2types.VolumeTypeGp2,
					Attachments: []ec2types.VolumeAttachment{},
				},
			},
		},
	}

	resources, err := awsclient.FetchEBSVolumes(context.Background(), mock)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(resources))
	}

	if resources[0].Fields["attached_to"] != "" {
		t.Errorf("Fields[\"attached_to\"]: expected empty string for unattached volume, got %q", resources[0].Fields["attached_to"])
	}
}

func TestFetchEBSVolumes_NoNameTag(t *testing.T) {
	mock := &mockEC2DescribeVolumesClient{
		output: &ec2.DescribeVolumesOutput{
			Volumes: []ec2types.Volume{
				{
					VolumeId:   aws.String("vol-noname"),
					State:      ec2types.VolumeStateAvailable,
					VolumeType: ec2types.VolumeTypeGp2,
					Tags:       []ec2types.Tag{},
				},
			},
		},
	}

	resources, err := awsclient.FetchEBSVolumes(context.Background(), mock)
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

func TestFetchEBSVolumes_RawStructIsVolume(t *testing.T) {
	mock := &mockEC2DescribeVolumesClient{
		output: &ec2.DescribeVolumesOutput{
			Volumes: []ec2types.Volume{
				{
					VolumeId:   aws.String("vol-rawstruct"),
					State:      ec2types.VolumeStateAvailable,
					VolumeType: ec2types.VolumeTypeGp3,
				},
			},
		},
	}

	resources, err := awsclient.FetchEBSVolumes(context.Background(), mock)
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

	vol, ok := r.RawStruct.(ec2types.Volume)
	if !ok {
		t.Fatalf("RawStruct should be ec2types.Volume, got %T", r.RawStruct)
	}
	if vol.VolumeId == nil || *vol.VolumeId != "vol-rawstruct" {
		t.Errorf("RawStruct.VolumeId: expected %q, got %v", "vol-rawstruct", vol.VolumeId)
	}
}
