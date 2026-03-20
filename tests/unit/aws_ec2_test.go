package unit

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"

	awsclient "github.com/k2m30/a9s/internal/aws"
)

// ---------------------------------------------------------------------------
// T021 - Test EC2 response parsing
// ---------------------------------------------------------------------------

func TestFetchEC2Instances_ParsesMultipleReservations(t *testing.T) {
	launchTime := time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC)

	mock := &mockEC2Client{
		output: &ec2.DescribeInstancesOutput{
			Reservations: []ec2types.Reservation{
				{
					Instances: []ec2types.Instance{
						{
							InstanceId:   aws.String("i-0001"),
							InstanceType: ec2types.InstanceTypeT3Micro,
							State: &ec2types.InstanceState{
								Name: ec2types.InstanceStateNameRunning,
							},
							PrivateIpAddress: aws.String("10.0.0.1"),
							PublicIpAddress:   aws.String("54.1.2.3"),
							LaunchTime:       &launchTime,
							Tags: []ec2types.Tag{
								{Key: aws.String("Name"), Value: aws.String("web-server-1")},
							},
						},
						{
							InstanceId:   aws.String("i-0002"),
							InstanceType: ec2types.InstanceTypeT3Small,
							State: &ec2types.InstanceState{
								Name: ec2types.InstanceStateNameStopped,
							},
							PrivateIpAddress: aws.String("10.0.0.2"),
							LaunchTime:       &launchTime,
							Tags: []ec2types.Tag{
								{Key: aws.String("Name"), Value: aws.String("web-server-2")},
							},
						},
					},
				},
				{
					Instances: []ec2types.Instance{
						{
							InstanceId:   aws.String("i-0003"),
							InstanceType: ec2types.InstanceTypeM5Large,
							State: &ec2types.InstanceState{
								Name: ec2types.InstanceStateNameRunning,
							},
							PrivateIpAddress: aws.String("10.0.1.1"),
							PublicIpAddress:   aws.String("54.4.5.6"),
							LaunchTime:       &launchTime,
							Tags: []ec2types.Tag{
								{Key: aws.String("Name"), Value: aws.String("api-server")},
								{Key: aws.String("Env"), Value: aws.String("prod")},
							},
						},
					},
				},
			},
		},
	}

	resources, err := awsclient.FetchEC2Instances(context.Background(), mock)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(resources) != 3 {
		t.Fatalf("expected 3 resources, got %d", len(resources))
	}

	// Verify first instance
	r0 := resources[0]
	if r0.ID != "i-0001" {
		t.Errorf("resource[0].ID: expected %q, got %q", "i-0001", r0.ID)
	}
	if r0.Name != "web-server-1" {
		t.Errorf("resource[0].Name: expected %q, got %q", "web-server-1", r0.Name)
	}
	if r0.Status != "running" {
		t.Errorf("resource[0].Status: expected %q, got %q", "running", r0.Status)
	}

	// Verify second instance
	r1 := resources[1]
	if r1.ID != "i-0002" {
		t.Errorf("resource[1].ID: expected %q, got %q", "i-0002", r1.ID)
	}
	if r1.Name != "web-server-2" {
		t.Errorf("resource[1].Name: expected %q, got %q", "web-server-2", r1.Name)
	}
	if r1.Status != "stopped" {
		t.Errorf("resource[1].Status: expected %q, got %q", "stopped", r1.Status)
	}

	// Verify third instance
	r2 := resources[2]
	if r2.ID != "i-0003" {
		t.Errorf("resource[2].ID: expected %q, got %q", "i-0003", r2.ID)
	}
	if r2.Name != "api-server" {
		t.Errorf("resource[2].Name: expected %q, got %q", "api-server", r2.Name)
	}
	if r2.Status != "running" {
		t.Errorf("resource[2].Status: expected %q, got %q", "running", r2.Status)
	}

	// Verify Fields contain the expected keys
	requiredFields := []string{"instance_id", "name", "state", "type", "private_ip", "public_ip", "launch_time"}
	for i, r := range resources {
		for _, key := range requiredFields {
			if _, ok := r.Fields[key]; !ok {
				t.Errorf("resource[%d].Fields missing key %q", i, key)
			}
		}
	}

	// Verify specific field values on the first instance
	if r0.Fields["instance_id"] != "i-0001" {
		t.Errorf("resource[0].Fields[\"instance_id\"]: expected %q, got %q", "i-0001", r0.Fields["instance_id"])
	}
	if r0.Fields["type"] != "t3.micro" {
		t.Errorf("resource[0].Fields[\"type\"]: expected %q, got %q", "t3.micro", r0.Fields["type"])
	}
	if r0.Fields["private_ip"] != "10.0.0.1" {
		t.Errorf("resource[0].Fields[\"private_ip\"]: expected %q, got %q", "10.0.0.1", r0.Fields["private_ip"])
	}
	if r0.Fields["public_ip"] != "54.1.2.3" {
		t.Errorf("resource[0].Fields[\"public_ip\"]: expected %q, got %q", "54.1.2.3", r0.Fields["public_ip"])
	}

	// Second instance has no public IP - should be empty or "-"
	if r1.Fields["public_ip"] != "" && r1.Fields["public_ip"] != "-" {
		t.Errorf("resource[1].Fields[\"public_ip\"]: expected empty or \"-\", got %q", r1.Fields["public_ip"])
	}
}

func TestFetchEC2Instances_ErrorResponse(t *testing.T) {
	mock := &mockEC2Client{
		output: nil,
		err:    fmt.Errorf("AWS API error: access denied"),
	}

	resources, err := awsclient.FetchEC2Instances(context.Background(), mock)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	if resources != nil {
		t.Errorf("expected nil resources on error, got %d resources", len(resources))
	}
}

func TestFetchEC2Instances_EmptyResponse(t *testing.T) {
	mock := &mockEC2Client{
		output: &ec2.DescribeInstancesOutput{
			Reservations: []ec2types.Reservation{},
		},
	}

	resources, err := awsclient.FetchEC2Instances(context.Background(), mock)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(resources) != 0 {
		t.Errorf("expected 0 resources, got %d", len(resources))
	}
}
