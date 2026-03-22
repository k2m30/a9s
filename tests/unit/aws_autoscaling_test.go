package unit

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/autoscaling"
	asgtypes "github.com/aws/aws-sdk-go-v2/service/autoscaling/types"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
)

// ---------------------------------------------------------------------------
// ASG - Test FetchAutoScalingGroups response parsing
// ---------------------------------------------------------------------------

func TestFetchAutoScalingGroups_ParsesMultipleGroups(t *testing.T) {
	createdTime := time.Date(2025, 3, 1, 9, 0, 0, 0, time.UTC)

	mock := &mockASGDescribeAutoScalingGroupsClient{
		output: &autoscaling.DescribeAutoScalingGroupsOutput{
			AutoScalingGroups: []asgtypes.AutoScalingGroup{
				{
					AutoScalingGroupName: aws.String("prod-web-asg"),
					MinSize:              aws.Int32(2),
					MaxSize:              aws.Int32(10),
					DesiredCapacity:      aws.Int32(4),
					Instances: []asgtypes.Instance{
						{InstanceId: aws.String("i-0aaa1111bbbb2222c")},
						{InstanceId: aws.String("i-0aaa1111bbbb2222d")},
						{InstanceId: aws.String("i-0aaa1111bbbb2222e")},
						{InstanceId: aws.String("i-0aaa1111bbbb2222f")},
					},
					Status:               nil,
					AvailabilityZones:    []string{"us-east-1a", "us-east-1b"},
					AutoScalingGroupARN:  aws.String("arn:aws:autoscaling:us-east-1:123456789012:autoScalingGroup:abc:autoScalingGroupName/prod-web-asg"),
					LaunchConfigurationName: aws.String("prod-web-lc-v3"),
					HealthCheckType:      aws.String("ELB"),
					CreatedTime:          &createdTime,
					DefaultCooldown:      aws.Int32(300),
					Tags: []asgtypes.TagDescription{
						{Key: aws.String("Environment"), Value: aws.String("production")},
						{Key: aws.String("Team"), Value: aws.String("platform")},
					},
				},
				{
					AutoScalingGroupName: aws.String("staging-api-asg"),
					MinSize:              aws.Int32(1),
					MaxSize:              aws.Int32(5),
					DesiredCapacity:      aws.Int32(2),
					Instances: []asgtypes.Instance{
						{InstanceId: aws.String("i-0bbb2222cccc3333a")},
						{InstanceId: aws.String("i-0bbb2222cccc3333b")},
					},
					Status:            aws.String("Delete in progress"),
					AvailabilityZones: []string{"us-east-1a"},
					AutoScalingGroupARN: aws.String("arn:aws:autoscaling:us-east-1:123456789012:autoScalingGroup:def:autoScalingGroupName/staging-api-asg"),
					HealthCheckType:   aws.String("EC2"),
					CreatedTime:       &createdTime,
					DefaultCooldown:   aws.Int32(120),
				},
			},
		},
	}

	resources, err := awsclient.FetchAutoScalingGroups(context.Background(), mock)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(resources) != 2 {
		t.Fatalf("expected 2 resources, got %d", len(resources))
	}

	// Verify required fields exist
	requiredFields := []string{"asg_name", "min_size", "max_size", "desired", "instances", "status"}
	for i, r := range resources {
		for _, key := range requiredFields {
			if _, ok := r.Fields[key]; !ok {
				t.Errorf("resource[%d].Fields missing key %q", i, key)
			}
		}
	}

	// Verify first ASG
	r0 := resources[0]
	if r0.ID != "prod-web-asg" {
		t.Errorf("resource[0].ID: expected %q, got %q", "prod-web-asg", r0.ID)
	}
	if r0.Name != "prod-web-asg" {
		t.Errorf("resource[0].Name: expected %q, got %q", "prod-web-asg", r0.Name)
	}
	if r0.Fields["asg_name"] != "prod-web-asg" {
		t.Errorf("resource[0].Fields[\"asg_name\"]: expected %q, got %q", "prod-web-asg", r0.Fields["asg_name"])
	}
	if r0.Fields["min_size"] != "2" {
		t.Errorf("resource[0].Fields[\"min_size\"]: expected %q, got %q", "2", r0.Fields["min_size"])
	}
	if r0.Fields["max_size"] != "10" {
		t.Errorf("resource[0].Fields[\"max_size\"]: expected %q, got %q", "10", r0.Fields["max_size"])
	}
	if r0.Fields["desired"] != "4" {
		t.Errorf("resource[0].Fields[\"desired\"]: expected %q, got %q", "4", r0.Fields["desired"])
	}
	if r0.Fields["instances"] != "4" {
		t.Errorf("resource[0].Fields[\"instances\"]: expected %q, got %q", "4", r0.Fields["instances"])
	}
	if r0.Fields["status"] != "" {
		t.Errorf("resource[0].Fields[\"status\"]: expected empty, got %q", r0.Fields["status"])
	}

	// Verify second ASG
	r1 := resources[1]
	if r1.ID != "staging-api-asg" {
		t.Errorf("resource[1].ID: expected %q, got %q", "staging-api-asg", r1.ID)
	}
	if r1.Status != "Delete in progress" {
		t.Errorf("resource[1].Status: expected %q, got %q", "Delete in progress", r1.Status)
	}
	if r1.Fields["instances"] != "2" {
		t.Errorf("resource[1].Fields[\"instances\"]: expected %q, got %q", "2", r1.Fields["instances"])
	}
	if r1.Fields["min_size"] != "1" {
		t.Errorf("resource[1].Fields[\"min_size\"]: expected %q, got %q", "1", r1.Fields["min_size"])
	}
	if r1.Fields["max_size"] != "5" {
		t.Errorf("resource[1].Fields[\"max_size\"]: expected %q, got %q", "5", r1.Fields["max_size"])
	}
	if r1.Fields["status"] != "Delete in progress" {
		t.Errorf("resource[1].Fields[\"status\"]: expected %q, got %q", "Delete in progress", r1.Fields["status"])
	}
}

func TestFetchAutoScalingGroups_ErrorResponse(t *testing.T) {
	mock := &mockASGDescribeAutoScalingGroupsClient{
		output: nil,
		err:    fmt.Errorf("AWS API error: access denied"),
	}

	resources, err := awsclient.FetchAutoScalingGroups(context.Background(), mock)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	if resources != nil {
		t.Errorf("expected nil resources on error, got %d resources", len(resources))
	}
}

func TestFetchAutoScalingGroups_EmptyResponse(t *testing.T) {
	mock := &mockASGDescribeAutoScalingGroupsClient{
		output: &autoscaling.DescribeAutoScalingGroupsOutput{
			AutoScalingGroups: []asgtypes.AutoScalingGroup{},
		},
	}

	resources, err := awsclient.FetchAutoScalingGroups(context.Background(), mock)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(resources) != 0 {
		t.Errorf("expected 0 resources, got %d", len(resources))
	}
}
