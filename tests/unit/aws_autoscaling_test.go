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
	"github.com/k2m30/a9s/v3/internal/domain"
	"github.com/k2m30/a9s/v3/internal/resource"
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
					Status:                  nil,
					AvailabilityZones:       []string{"us-east-1a", "us-east-1b"},
					AutoScalingGroupARN:     aws.String("arn:aws:autoscaling:us-east-1:123456789012:autoScalingGroup:abc:autoScalingGroupName/prod-web-asg"),
					LaunchConfigurationName: aws.String("prod-web-lc-v3"),
					HealthCheckType:         aws.String("ELB"),
					CreatedTime:             &createdTime,
					DefaultCooldown:         aws.Int32(300),
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
					Status:              aws.String("Delete in progress"),
					AvailabilityZones:   []string{"us-east-1a"},
					AutoScalingGroupARN: aws.String("arn:aws:autoscaling:us-east-1:123456789012:autoScalingGroup:def:autoScalingGroupName/staging-api-asg"),
					HealthCheckType:     aws.String("EC2"),
					CreatedTime:         &createdTime,
					DefaultCooldown:     aws.Int32(120),
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
	// Post-fold contract: fetcher stops writing Status; emits wave1 Finding instead.
	if r1.Status != "" {
		t.Errorf("resource[1].Status: expected %q (fetcher must not write Status), got %q", "", r1.Status)
	}
	if len(r1.Findings) != 1 {
		t.Fatalf("resource[1].Findings: expected 1 for deleting ASG, got %d", len(r1.Findings))
	}
	if r1.Findings[0].Code != awsclient.CodeASGStateDeleting {
		t.Errorf("resource[1].Findings[0].Code: expected %q, got %q", awsclient.CodeASGStateDeleting, r1.Findings[0].Code)
	}
	if r1.Findings[0].Severity != domain.SevWarn {
		t.Errorf("resource[1].Findings[0].Severity: expected domain.SevWarn, got %v", r1.Findings[0].Severity)
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

// ---------------------------------------------------------------------------
// Attention-signal field population tests (Wave 1 ASG fields)
// ---------------------------------------------------------------------------

func TestFetchAutoScalingGroupsPage_PopulatesInstancesUnhealthyCount(t *testing.T) {
	mock := &mockASGDescribeAutoScalingGroupsClient{
		output: &autoscaling.DescribeAutoScalingGroupsOutput{
			AutoScalingGroups: []asgtypes.AutoScalingGroup{
				{
					AutoScalingGroupName: aws.String("prod-web-asg"),
					MinSize:              aws.Int32(2),
					MaxSize:              aws.Int32(10),
					DesiredCapacity:      aws.Int32(4),
					Instances: []asgtypes.Instance{
						{InstanceId: aws.String("i-0001"), HealthStatus: aws.String("Healthy"), LifecycleState: "InService"},
						{InstanceId: aws.String("i-0002"), HealthStatus: aws.String("Unhealthy"), LifecycleState: "InService"},
						{InstanceId: aws.String("i-0003"), HealthStatus: aws.String("Unhealthy"), LifecycleState: "InService"},
						{InstanceId: aws.String("i-0004"), HealthStatus: aws.String("Healthy"), LifecycleState: "InService"},
					},
				},
			},
		},
	}

	result, err := awsclient.FetchAutoScalingGroupsPage(context.Background(), mock, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(result.Resources))
	}

	got := result.Resources[0].Fields["instances_unhealthy_count"]
	if got != "2" {
		t.Errorf("Fields[\"instances_unhealthy_count\"]: expected %q, got %q", "2", got)
	}
}

func TestFetchAutoScalingGroupsPage_PopulatesInServiceCount(t *testing.T) {
	mock := &mockASGDescribeAutoScalingGroupsClient{
		output: &autoscaling.DescribeAutoScalingGroupsOutput{
			AutoScalingGroups: []asgtypes.AutoScalingGroup{
				{
					AutoScalingGroupName: aws.String("prod-api-asg"),
					MinSize:              aws.Int32(2),
					MaxSize:              aws.Int32(8),
					DesiredCapacity:      aws.Int32(4),
					Instances: []asgtypes.Instance{
						{InstanceId: aws.String("i-0001"), HealthStatus: aws.String("Healthy"), LifecycleState: "InService"},
						{InstanceId: aws.String("i-0002"), HealthStatus: aws.String("Healthy"), LifecycleState: "InService"},
						{InstanceId: aws.String("i-0003"), HealthStatus: aws.String("Healthy"), LifecycleState: "Pending"},
						{InstanceId: aws.String("i-0004"), HealthStatus: aws.String("Unhealthy"), LifecycleState: "Terminating"},
					},
				},
			},
		},
	}

	result, err := awsclient.FetchAutoScalingGroupsPage(context.Background(), mock, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(result.Resources))
	}

	got := result.Resources[0].Fields["in_service_count"]
	if got != "2" {
		t.Errorf("Fields[\"in_service_count\"]: expected %q, got %q", "2", got)
	}
}

func TestFetchAutoScalingGroupsPage_PopulatesSuspendedProcesses(t *testing.T) {
	mock := &mockASGDescribeAutoScalingGroupsClient{
		output: &autoscaling.DescribeAutoScalingGroupsOutput{
			AutoScalingGroups: []asgtypes.AutoScalingGroup{
				{
					AutoScalingGroupName: aws.String("worker-asg"),
					MinSize:              aws.Int32(1),
					MaxSize:              aws.Int32(5),
					DesiredCapacity:      aws.Int32(3),
					SuspendedProcesses: []asgtypes.SuspendedProcess{
						{ProcessName: aws.String("Launch"), SuspensionReason: aws.String("User suspended the process")},
						{ProcessName: aws.String("HealthCheck"), SuspensionReason: aws.String("User suspended the process")},
					},
				},
			},
		},
	}

	result, err := awsclient.FetchAutoScalingGroupsPage(context.Background(), mock, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(result.Resources))
	}

	got := result.Resources[0].Fields["suspended_processes"]
	if got != "Launch,HealthCheck" {
		t.Errorf("Fields[\"suspended_processes\"]: expected %q, got %q", "Launch,HealthCheck", got)
	}
}

func TestFetchAutoScalingGroupsPage_ZeroInstances(t *testing.T) {
	mock := &mockASGDescribeAutoScalingGroupsClient{
		output: &autoscaling.DescribeAutoScalingGroupsOutput{
			AutoScalingGroups: []asgtypes.AutoScalingGroup{
				{
					AutoScalingGroupName: aws.String("empty-asg"),
					MinSize:              aws.Int32(0),
					MaxSize:              aws.Int32(10),
					DesiredCapacity:      aws.Int32(0),
					Instances:            []asgtypes.Instance{},
				},
			},
		},
	}

	result, err := awsclient.FetchAutoScalingGroupsPage(context.Background(), mock, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(result.Resources))
	}

	r := result.Resources[0]
	if got := r.Fields["instances_unhealthy_count"]; got != "0" {
		t.Errorf("Fields[\"instances_unhealthy_count\"]: expected %q, got %q", "0", got)
	}
	if got := r.Fields["in_service_count"]; got != "0" {
		t.Errorf("Fields[\"in_service_count\"]: expected %q, got %q", "0", got)
	}
}

func TestFetchAutoScalingGroupsPage_NoSuspendedProcesses(t *testing.T) {
	mock := &mockASGDescribeAutoScalingGroupsClient{
		output: &autoscaling.DescribeAutoScalingGroupsOutput{
			AutoScalingGroups: []asgtypes.AutoScalingGroup{
				{
					AutoScalingGroupName: aws.String("healthy-asg"),
					MinSize:              aws.Int32(2),
					MaxSize:              aws.Int32(10),
					DesiredCapacity:      aws.Int32(4),
					SuspendedProcesses:   []asgtypes.SuspendedProcess{},
				},
			},
		},
	}

	result, err := awsclient.FetchAutoScalingGroupsPage(context.Background(), mock, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(result.Resources))
	}

	// Key must exist and be empty string, not missing.
	got, ok := result.Resources[0].Fields["suspended_processes"]
	if !ok {
		t.Error("Fields[\"suspended_processes\"] key is missing; expected empty string")
	} else if got != "" {
		t.Errorf("Fields[\"suspended_processes\"]: expected %q, got %q", "", got)
	}
}

func TestFetchAutoScalingGroupsPage_RegistersAttentionFields(t *testing.T) {
	keys := resource.GetFieldKeys("asg")
	if keys == nil {
		t.Fatal("no field keys registered for \"asg\"")
	}

	required := []string{"instances_unhealthy_count", "in_service_count", "suspended_processes"}
	keySet := make(map[string]bool, len(keys))
	for _, k := range keys {
		keySet[k] = true
	}

	for _, want := range required {
		if !keySet[want] {
			t.Errorf("registered field keys for \"asg\" missing %q; got: %v", want, keys)
		}
	}
}

// ---------------------------------------------------------------------------
// Color function tests for ASG attention rules
// ---------------------------------------------------------------------------

func TestColorASG_BrokenWhenInServiceBelowMinSize(t *testing.T) {
	td := resource.FindResourceType("asg")
	if td == nil {
		t.Fatal("asg type not registered")
	}

	r := resource.Resource{
		Fields: map[string]string{
			"min_size":                  "4",
			"in_service_count":          "2",
			"instances_unhealthy_count": "0",
			"suspended_processes":       "",
			"status":                    "",
		},
	}

	got := td.Color(r)
	if got != resource.ColorBroken {
		t.Errorf("Color with in_service_count=2 < min_size=4: expected ColorBroken (%v), got %v", resource.ColorBroken, got)
	}
}

func TestColorASG_WarningWhenUnhealthyInstances(t *testing.T) {
	td := resource.FindResourceType("asg")
	if td == nil {
		t.Fatal("asg type not registered")
	}

	r := resource.Resource{
		Fields: map[string]string{
			"min_size":                  "2",
			"in_service_count":          "2",
			"instances_unhealthy_count": "1",
			"suspended_processes":       "",
			"status":                    "",
		},
	}

	got := td.Color(r)
	if got != resource.ColorWarning {
		t.Errorf("Color with instances_unhealthy_count=1: expected ColorWarning (%v), got %v", resource.ColorWarning, got)
	}
}

func TestColorASG_WarningWhenSuspendedLaunch(t *testing.T) {
	td := resource.FindResourceType("asg")
	if td == nil {
		t.Fatal("asg type not registered")
	}

	cases := []struct {
		name      string
		suspended string
	}{
		{"Launch", "Launch"},
		{"Terminate", "Terminate"},
		{"HealthCheck", "HealthCheck"},
		{"Launch_and_HealthCheck", "Launch,HealthCheck"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := resource.Resource{
				Fields: map[string]string{
					"min_size":                  "2",
					"in_service_count":          "2",
					"instances_unhealthy_count": "0",
					"suspended_processes":       tc.suspended,
					"status":                    "",
				},
			}
			got := td.Color(r)
			if got != resource.ColorWarning {
				t.Errorf("Color with suspended_processes=%q: expected ColorWarning (%v), got %v", tc.suspended, resource.ColorWarning, got)
			}
		})
	}
}

func TestColorASG_HealthyWhenAllGood(t *testing.T) {
	td := resource.FindResourceType("asg")
	if td == nil {
		t.Fatal("asg type not registered")
	}

	r := resource.Resource{
		Fields: map[string]string{
			"min_size":                  "2",
			"in_service_count":          "4",
			"instances_unhealthy_count": "0",
			"suspended_processes":       "",
			"status":                    "",
		},
	}

	got := td.Color(r)
	if got != resource.ColorHealthy {
		t.Errorf("Color with all-healthy fields: expected ColorHealthy (%v), got %v", resource.ColorHealthy, got)
	}
}
