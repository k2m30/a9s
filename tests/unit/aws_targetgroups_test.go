package unit

import (
	"context"
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	elbv2 "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2"
	elbv2types "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types"

	awsclient "github.com/k2m30/a9s/internal/aws"
)

// ---------------------------------------------------------------------------
// T-TG01 - Test Target Groups response parsing
// ---------------------------------------------------------------------------

func TestFetchTargetGroups_ParsesMultipleTargetGroups(t *testing.T) {
	port80 := int32(80)
	port443 := int32(443)
	interval30 := int32(30)
	interval15 := int32(15)
	healthyThreshold := int32(5)
	unhealthyThreshold := int32(2)

	mock := &mockELBv2DescribeTargetGroupsClient{
		output: &elbv2.DescribeTargetGroupsOutput{
			TargetGroups: []elbv2types.TargetGroup{
				{
					TargetGroupName:            aws.String("prod-web-tg"),
					Port:                       &port80,
					Protocol:                   elbv2types.ProtocolEnumHttp,
					VpcId:                      aws.String("vpc-abc123"),
					TargetType:                 elbv2types.TargetTypeEnumInstance,
					HealthCheckPath:            aws.String("/health"),
					TargetGroupArn:             aws.String("arn:aws:elasticloadbalancing:us-east-1:123456789012:targetgroup/prod-web-tg/abc123"),
					HealthCheckPort:            aws.String("traffic-port"),
					HealthCheckProtocol:        elbv2types.ProtocolEnumHttp,
					HealthCheckIntervalSeconds: &interval30,
					HealthyThresholdCount:      &healthyThreshold,
					UnhealthyThresholdCount:    &unhealthyThreshold,
				},
				{
					TargetGroupName:            aws.String("prod-api-tg"),
					Port:                       &port443,
					Protocol:                   elbv2types.ProtocolEnumHttps,
					VpcId:                      aws.String("vpc-def456"),
					TargetType:                 elbv2types.TargetTypeEnumIp,
					HealthCheckPath:            aws.String("/api/health"),
					TargetGroupArn:             aws.String("arn:aws:elasticloadbalancing:us-east-1:123456789012:targetgroup/prod-api-tg/def456"),
					HealthCheckPort:            aws.String("443"),
					HealthCheckProtocol:        elbv2types.ProtocolEnumHttps,
					HealthCheckIntervalSeconds: &interval15,
					HealthyThresholdCount:      &healthyThreshold,
					UnhealthyThresholdCount:    &unhealthyThreshold,
				},
			},
		},
	}

	resources, err := awsclient.FetchTargetGroups(context.Background(), mock)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(resources) != 2 {
		t.Fatalf("expected 2 resources, got %d", len(resources))
	}

	// Verify required fields exist
	requiredFields := []string{"target_group_name", "port", "protocol", "vpc_id", "target_type", "health_check_path"}
	for i, r := range resources {
		for _, key := range requiredFields {
			if _, ok := r.Fields[key]; !ok {
				t.Errorf("resource[%d].Fields missing key %q", i, key)
			}
		}
	}

	// Verify first target group
	r0 := resources[0]
	if r0.ID != "prod-web-tg" {
		t.Errorf("resource[0].ID: expected %q, got %q", "prod-web-tg", r0.ID)
	}
	if r0.Name != "prod-web-tg" {
		t.Errorf("resource[0].Name: expected %q, got %q", "prod-web-tg", r0.Name)
	}
	if r0.Fields["target_group_name"] != "prod-web-tg" {
		t.Errorf("resource[0].Fields[\"target_group_name\"]: expected %q, got %q", "prod-web-tg", r0.Fields["target_group_name"])
	}
	if r0.Fields["port"] != "80" {
		t.Errorf("resource[0].Fields[\"port\"]: expected %q, got %q", "80", r0.Fields["port"])
	}
	if r0.Fields["protocol"] != "HTTP" {
		t.Errorf("resource[0].Fields[\"protocol\"]: expected %q, got %q", "HTTP", r0.Fields["protocol"])
	}
	if r0.Fields["vpc_id"] != "vpc-abc123" {
		t.Errorf("resource[0].Fields[\"vpc_id\"]: expected %q, got %q", "vpc-abc123", r0.Fields["vpc_id"])
	}
	if r0.Fields["target_type"] != "instance" {
		t.Errorf("resource[0].Fields[\"target_type\"]: expected %q, got %q", "instance", r0.Fields["target_type"])
	}
	if r0.Fields["health_check_path"] != "/health" {
		t.Errorf("resource[0].Fields[\"health_check_path\"]: expected %q, got %q", "/health", r0.Fields["health_check_path"])
	}

	// Verify second target group
	r1 := resources[1]
	if r1.ID != "prod-api-tg" {
		t.Errorf("resource[1].ID: expected %q, got %q", "prod-api-tg", r1.ID)
	}
	if r1.Fields["port"] != "443" {
		t.Errorf("resource[1].Fields[\"port\"]: expected %q, got %q", "443", r1.Fields["port"])
	}
	if r1.Fields["protocol"] != "HTTPS" {
		t.Errorf("resource[1].Fields[\"protocol\"]: expected %q, got %q", "HTTPS", r1.Fields["protocol"])
	}
	if r1.Fields["target_type"] != "ip" {
		t.Errorf("resource[1].Fields[\"target_type\"]: expected %q, got %q", "ip", r1.Fields["target_type"])
	}
	if r1.Fields["health_check_path"] != "/api/health" {
		t.Errorf("resource[1].Fields[\"health_check_path\"]: expected %q, got %q", "/api/health", r1.Fields["health_check_path"])
	}
}

func TestFetchTargetGroups_ErrorResponse(t *testing.T) {
	mock := &mockELBv2DescribeTargetGroupsClient{
		output: nil,
		err:    fmt.Errorf("AWS API error: access denied"),
	}

	resources, err := awsclient.FetchTargetGroups(context.Background(), mock)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	if resources != nil {
		t.Errorf("expected nil resources on error, got %d resources", len(resources))
	}
}

func TestFetchTargetGroups_EmptyResponse(t *testing.T) {
	mock := &mockELBv2DescribeTargetGroupsClient{
		output: &elbv2.DescribeTargetGroupsOutput{
			TargetGroups: []elbv2types.TargetGroup{},
		},
	}

	resources, err := awsclient.FetchTargetGroups(context.Background(), mock)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(resources) != 0 {
		t.Errorf("expected 0 resources, got %d", len(resources))
	}
}
