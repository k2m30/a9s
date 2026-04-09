package unit_test

// related_smoketest_table_test.go consolidates the 19 per-resource-type
// aws_*_related_smoketest_test.go files into a single table-driven test.
//
// Each entry in relatedSmokeTable covers S01-S06 for one resource type:
//   S01 – right column visible at width=120 with RELATED header
//   S02 – correct labels in right column
//   S03 – counts render correctly after results delivered
//   S04 – Tab + Enter on first count>0 row emits RelatedNavigateMsg
//   S05 – Enter on all-count=0 right column emits no RelatedNavigateMsg
//   S06 – checker registration + demo checker coverage (per-type assertions)

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/aws/aws-sdk-go-v2/aws"
	cloudtrailtypes "github.com/aws/aws-sdk-go-v2/service/cloudtrail/types"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"
	efstypes "github.com/aws/aws-sdk-go-v2/service/efs/types"
	ekstypes "github.com/aws/aws-sdk-go-v2/service/eks/types"
	ebtypes "github.com/aws/aws-sdk-go-v2/service/elasticbeanstalk/types"
	elbv2types "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types"
	eventbridgetypes "github.com/aws/aws-sdk-go-v2/service/eventbridge/types"
	snstypes "github.com/aws/aws-sdk-go-v2/service/sns/types"
	ssmtypes "github.com/aws/aws-sdk-go-v2/service/ssm/types"
	wafv2types "github.com/aws/aws-sdk-go-v2/service/wafv2/types"

	internalaws "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/tui/keys"
	"github.com/k2m30/a9s/v3/internal/tui/messages"
	"github.com/k2m30/a9s/v3/internal/tui/views"
)

// ---------------------------------------------------------------------------
// Shared delivery helper (replaces the 19 per-type deliver*RelatedResult funcs)
// ---------------------------------------------------------------------------

func deliverSmokeRelatedResult(d views.DetailModel, resourceType, targetType string, count int, ids ...string) views.DetailModel {
	msg := messages.RelatedCheckResultMsg{
		ResourceType: resourceType,
		Result: resource.RelatedCheckResult{
			TargetType:  targetType,
			Count:       count,
			ResourceIDs: ids,
		},
	}
	updated, _ := d.Update(msg)
	return updated
}

// ---------------------------------------------------------------------------
// smokeDelivery describes one RelatedCheckResultMsg delivery for S03/S04/S05
// ---------------------------------------------------------------------------

type smokeDelivery struct {
	targetType string
	count      int
	ids        []string
}

// ---------------------------------------------------------------------------
// smokeTestCase holds all data needed for S01-S06 for one resource type
// ---------------------------------------------------------------------------

type smokeTestCase struct {
	shortName      string
	resource       resource.Resource
	expectedLabels []string
	deliveries     []smokeDelivery // used in S03/S04 (positive counts first, then zeros)
	zeroDeliveries []smokeDelivery // all-zero variant for S05
	firstNavTarget string          // expected TargetType from first Enter in S04
	s06            func(t *testing.T)
}

// ---------------------------------------------------------------------------
// Table
// ---------------------------------------------------------------------------

var relatedSmokeTable = []smokeTestCase{
	// ------------------------------------------------------------------
	// AMI
	// ------------------------------------------------------------------
	{
		shortName: "ami",
		resource: resource.Resource{
			ID:   "ami-0a1b2c3d4e5f60001",
			Name: "golden-image-2026-01",
			Fields: map[string]string{
				"image_id":     "ami-0a1b2c3d4e5f60001",
				"name":         "golden-image-2026-01",
				"state":        "available",
				"architecture": "x86_64",
			},
			RawStruct: ec2types.Image{},
		},
		expectedLabels: []string{"EC2 Instances", "EBS Snapshots", "Auto Scaling Groups"},
		deliveries: []smokeDelivery{
			{"ec2", 1, []string{"i-0a1b2c3d4e5f60001"}},
			{"ebs-snap", 1, []string{"snap-0a1b2c3d4e5f60001"}},
			{"asg", 0, nil},
		},
		zeroDeliveries: []smokeDelivery{
			{"ec2", 0, nil},
			{"ebs-snap", 0, nil},
			{"asg", 0, nil},
		},
		firstNavTarget: "ec2",
		s06: func(t *testing.T) {
			t.Helper()
			defs := resource.GetRelated("ami")
			var asgDef *resource.RelatedDef
			for i := range defs {
				if defs[i].TargetType == "asg" {
					asgDef = &defs[i]
					break
				}
			}
			if asgDef == nil {
				t.Fatal("AMI-S06: asg related def not registered")
			}
			if asgDef.Checker == nil {
				t.Fatal("AMI-S06: asg Checker must not be nil — implementation missing?")
			}
		},
	},

	// ------------------------------------------------------------------
	// EB (Elastic Beanstalk)
	// ------------------------------------------------------------------
	{
		shortName: "eb",
		resource: resource.Resource{
			ID:   "e-acmeprodapi",
			Name: "acme-prod-api",
			Fields: map[string]string{
				"environment_id":   "e-acmeprodapi",
				"environment_name": "acme-prod-api",
				"status":           "Ready",
			},
			RawStruct: ebtypes.EnvironmentDescription{},
		},
		expectedLabels: []string{"CloudFormation Stack", "Log Groups", "Auto Scaling Groups"},
		deliveries: []smokeDelivery{
			{"cfn", 1, []string{"awseb-e-acmeprodapi-stack"}},
			{"logs", 0, nil},
			{"asg", 0, nil},
		},
		zeroDeliveries: []smokeDelivery{
			{"cfn", 0, nil},
			{"logs", 0, nil},
			{"asg", 0, nil},
		},
		firstNavTarget: "cfn",
		s06: func(t *testing.T) {
			t.Helper()
			defs := resource.GetRelated("eb")
			for _, target := range []string{"cfn", "logs", "asg"} {
				found := false
				for _, def := range defs {
					if def.TargetType == target {
						found = true
						if def.Checker == nil {
							t.Errorf("EB-S06: %q Checker must not be nil — expected real checker", target)
						}
						break
					}
				}
				if !found {
					t.Errorf("EB-S06: related def for target %q not registered", target)
				}
			}
		},
	},

	// ------------------------------------------------------------------
	// EB-Rule (EventBridge Rule)
	// ------------------------------------------------------------------
	{
		shortName: "eb-rule",
		resource: resource.Resource{
			ID:   "nightly-db-backup",
			Name: "nightly-db-backup",
			Fields: map[string]string{
				"name":  "nightly-db-backup",
				"state": "ENABLED",
			},
			RawStruct: eventbridgetypes.Rule{},
		},
		expectedLabels: []string{"IAM Role"},
		deliveries: []smokeDelivery{
			{"role", 1, []string{"acme-ci-deploy-role"}},
		},
		zeroDeliveries: []smokeDelivery{
			{"role", 0, nil},
		},
		firstNavTarget: "role",
		s06: func(t *testing.T) {
			t.Helper()
			defs := resource.GetRelated("eb-rule")
			var roleDef *resource.RelatedDef
			for i := range defs {
				if defs[i].TargetType == "role" {
					roleDef = &defs[i]
					break
				}
			}
			if roleDef == nil {
				t.Fatal("EbRule-S06: role related def not registered")
			}
			if roleDef.Checker == nil {
				t.Fatal("EbRule-S06: role Checker must not be nil; got nil — implementation missing?")
			}
		},
	},

	// ------------------------------------------------------------------
	// EBS (EBS Volumes)
	// ------------------------------------------------------------------
	{
		shortName: "ebs",
		resource: resource.Resource{
			ID:   "vol-0a1b2c3d4e5f60001",
			Name: "web-prod-01-root",
			Fields: map[string]string{
				"volume_id":   "vol-0a1b2c3d4e5f60001",
				"name":        "web-prod-01-root",
				"state":       "in-use",
				"size":        "50",
				"type":        "gp3",
				"encrypted":   "true",
				"attached_to": "i-0a1b2c3d4e5f60001",
			},
			RawStruct: ec2types.Volume{},
		},
		expectedLabels: []string{"EC2 Instance", "EBS Snapshots", "KMS Key"},
		deliveries: []smokeDelivery{
			{"ec2", 1, []string{"i-0a1b2c3d4e5f60001"}},
			{"ebs-snap", 1, []string{"snap-0a1b2c3d4e5f60001"}},
			{"kms", 1, []string{"a1b2c3d4-5678-90ab-cdef-111111111111"}},
		},
		zeroDeliveries: []smokeDelivery{
			{"ec2", 0, nil},
			{"ebs-snap", 0, nil},
			{"kms", 0, nil},
		},
		firstNavTarget: "ec2",
		s06: func(t *testing.T) {
			t.Helper()
			defs := resource.GetRelated("ebs")
			for _, def := range defs {
				if def.Checker == nil {
					t.Errorf("EBS-S06: checker for target %q is nil (stub); all ebs checkers should be non-nil", def.TargetType)
				}
			}
		},
	},

	// ------------------------------------------------------------------
	// ECS Service
	// ------------------------------------------------------------------
	{
		shortName: "ecs-svc",
		resource: resource.Resource{
			ID:   "api-gateway",
			Name: "api-gateway",
			Fields: map[string]string{
				"service_name":  "api-gateway",
				"cluster":       "acme-services",
				"status":        "ACTIVE",
				"desired_count": "3",
				"running_count": "3",
				"launch_type":   "FARGATE",
			},
			RawStruct: ecstypes.Service{},
		},
		expectedLabels: []string{"ECS Clusters", "Target Groups", "CloudWatch Alarms", "CloudFormation Stacks"},
		deliveries: []smokeDelivery{
			{"ecs", 1, []string{"acme-services"}},
			{"tg", 1, []string{"api-tg"}},
			{"alarm", 1, []string{"ecs-svc-cpu-high"}},
			{"cfn", 0, nil},
		},
		zeroDeliveries: []smokeDelivery{
			{"ecs", 0, nil},
			{"tg", 0, nil},
			{"alarm", 0, nil},
			{"cfn", 0, nil},
		},
		firstNavTarget: "ecs",
		s06: func(t *testing.T) {
			t.Helper()
			defs := resource.GetRelated("ecs-svc")
			var cfnDef *resource.RelatedDef
			for i := range defs {
				if defs[i].TargetType == "cfn" {
					cfnDef = &defs[i]
					break
				}
			}
			if cfnDef == nil {
				t.Fatal("ECSSvc-S06: cfn related def not registered")
			}
			if cfnDef.Checker == nil {
				t.Fatal("ECSSvc-S06: cfn Checker must not be nil")
			}
		},
	},

	// ------------------------------------------------------------------
	// ECS Task
	// ------------------------------------------------------------------
	{
		shortName: "ecs-task",
		resource: resource.Resource{
			ID:   "abc123def456",
			Name: "abc123def456",
			Fields: map[string]string{
				"task_id":         "abc123def456",
				"cluster":         "arn:aws:ecs:us-east-1:123456789012:cluster/acme-services",
				"status":          "RUNNING",
				"task_definition": "arn:aws:ecs:us-east-1:123456789012:task-definition/api:5",
				"launch_type":     "FARGATE",
				"cpu":             "256",
				"memory":          "512",
			},
			RawStruct: ecstypes.Task{
				Group:      aws.String("service:api-gateway"),
				ClusterArn: aws.String("arn:aws:ecs:us-east-1:123456789012:cluster/acme-services"),
			},
		},
		expectedLabels: []string{"ECS Services", "ECS Clusters"},
		deliveries: []smokeDelivery{
			{"ecs-svc", 1, []string{"api-gateway"}},
			{"ecs", 1, []string{"acme-services"}},
		},
		zeroDeliveries: []smokeDelivery{
			{"ecs-svc", 0, nil},
			{"ecs", 0, nil},
		},
		firstNavTarget: "ecs-svc",
		s06: func(t *testing.T) {
			t.Helper()
		},
	},

	// ------------------------------------------------------------------
	// EFS
	// ------------------------------------------------------------------
	{
		shortName: "efs",
		resource: func() resource.Resource {
			encrypted := true
			return resource.Resource{
				ID:   "fs-001",
				Name: "shared-data",
				Fields: map[string]string{
					"file_system_id":   "fs-001",
					"name":             "shared-data",
					"life_cycle_state": "available",
					"encrypted":        "true",
				},
				RawStruct: efstypes.FileSystemDescription{
					FileSystemId: aws.String("fs-001"),
					Name:         aws.String("shared-data"),
					Encrypted:    &encrypted,
					KmsKeyId:     aws.String("a1b2c3d4-5678-90ab-cdef-111111111111"),
				},
			}
		}(),
		expectedLabels: []string{"KMS Keys", "CloudFormation Stacks", "Lambda Functions"},
		deliveries: []smokeDelivery{
			{"kms", 1, []string{"a1b2c3d4"}},
			{"cfn", 0, nil},
			{"lambda", 0, nil},
		},
		zeroDeliveries: []smokeDelivery{
			{"kms", 0, nil},
			{"cfn", 0, nil},
			{"lambda", 0, nil},
		},
		firstNavTarget: "kms",
		s06: func(t *testing.T) {
			t.Helper()
		},
	},

	// ------------------------------------------------------------------
	// EIP
	// ------------------------------------------------------------------
	{
		shortName: "eip",
		resource: resource.Resource{
			ID:   "eipalloc-001",
			Name: "web-server-eip",
			Fields: map[string]string{
				"allocation_id": "eipalloc-001",
				"public_ip":     "54.200.1.1",
				"instance_id":   "i-0a1b2c3d4e5f60001",
			},
			RawStruct: ec2types.Address{
				AllocationId:       aws.String("eipalloc-001"),
				InstanceId:         aws.String("i-0a1b2c3d4e5f60001"),
				NetworkInterfaceId: aws.String("eni-0a1b2c3d4e5f60001"),
			},
		},
		expectedLabels: []string{"EC2 Instances", "Network Interfaces", "NAT Gateways"},
		deliveries: []smokeDelivery{
			{"ec2", 1, []string{"i-001"}},
			{"eni", 1, []string{"eni-001"}},
			{"nat", 0, nil},
		},
		zeroDeliveries: []smokeDelivery{
			{"ec2", 0, nil},
			{"eni", 0, nil},
			{"nat", 0, nil},
		},
		firstNavTarget: "ec2",
		s06: func(t *testing.T) {
			t.Helper()
		},
	},

	// ------------------------------------------------------------------
	// EKS
	// ------------------------------------------------------------------
	{
		shortName: "eks",
		resource: resource.Resource{
			ID:   "acme-prod",
			Name: "acme-prod",
			Fields: map[string]string{
				"cluster_name": "acme-prod",
				"version":      "1.29",
				"status":       "ACTIVE",
			},
			RawStruct: &ekstypes.Cluster{
				Name:    aws.String("acme-prod"),
				Version: aws.String("1.29"),
				Status:  ekstypes.ClusterStatusActive,
				ResourcesVpcConfig: &ekstypes.VpcConfigResponse{
					VpcId:                  aws.String("vpc-0abc123def456789a"),
					ClusterSecurityGroupId: aws.String("sg-0ccc333333333333c"),
				},
			},
		},
		expectedLabels: []string{"Node Groups", "CloudWatch Alarms", "CloudFormation Stacks"},
		deliveries: []smokeDelivery{
			{"ng", 2, []string{"general-pool", "gpu-pool"}},
			{"alarm", 0, nil},
			{"cfn", 0, nil},
		},
		zeroDeliveries: []smokeDelivery{
			{"ng", 0, nil},
			{"alarm", 0, nil},
			{"cfn", 0, nil},
		},
		firstNavTarget: "ng",
		s06: func(t *testing.T) {
			t.Helper()
		},
	},

	// ------------------------------------------------------------------
	// SNS Subscription
	// ------------------------------------------------------------------
	{
		shortName: "sns-sub",
		resource: resource.Resource{
			ID:   "arn:aws:sns:us-east-1:123456789012:order-events:c3d4e5f6-a7b8-9012-cdef-123456789012",
			Name: "alarm-notifications",
			Fields: map[string]string{
				"topic_arn":        "arn:aws:sns:us-east-1:123456789012:alarm-notifications",
				"protocol":         "sqs",
				"endpoint":         "arn:aws:sqs:us-east-1:123456789012:alarm-queue",
				"subscription_arn": "arn:aws:sns:us-east-1:123456789012:order-events:c3d4e5f6-a7b8-9012-cdef-123456789012",
			},
			RawStruct: snstypes.Subscription{},
		},
		expectedLabels: []string{"SNS Topic", "Lambda Function", "SQS Queue"},
		deliveries: []smokeDelivery{
			{"sns", 1, []string{"arn:aws:sns:us-east-1:123456789012:order-events"}},
			{"sqs", 1, []string{"order-processing-queue"}},
			{"lambda", 0, nil},
		},
		zeroDeliveries: []smokeDelivery{
			{"sns", 0, nil},
			{"sqs", 0, nil},
			{"lambda", 0, nil},
		},
		firstNavTarget: "sns",
		s06: func(t *testing.T) {
			t.Helper()
			defs := resource.GetRelated("sns-sub")
			if len(defs) == 0 {
				t.Fatal("SNSSub-S06: no related defs registered for sns-sub")
			}
			for i := range defs {
				if defs[i].Checker == nil {
					t.Errorf("SNSSub-S06: Checker for target %q must be non-nil (real checker); got nil", defs[i].TargetType)
				}
			}
		},
	},

	// ------------------------------------------------------------------
	// SQS
	// ------------------------------------------------------------------
	{
		shortName: "sqs",
		resource: resource.Resource{
			ID:   "payment-processing",
			Name: "payment-processing",
			Fields: map[string]string{
				"queue_name":         "payment-processing",
				"queue_url":          "https://sqs.us-east-1.amazonaws.com/123456789012/payment-processing",
				"approx_messages":    "42",
				"approx_not_visible": "3",
				"delay_seconds":      "0",
			},
			RawStruct: internalaws.SQSQueueAttributesRow{},
		},
		expectedLabels: []string{"SNS Subscriptions", "CloudWatch Alarms", "Lambda Functions"},
		deliveries: []smokeDelivery{
			{"sns-sub", 1, []string{"arn:aws:sns:us-east-1:123456789012:payment-events:sub-001"}},
			{"alarm", 1, []string{"payment-queue-depth-alarm"}},
			{"lambda", 0, nil},
		},
		zeroDeliveries: []smokeDelivery{
			{"sns-sub", 0, nil},
			{"alarm", 0, nil},
			{"lambda", 0, nil},
		},
		firstNavTarget: "sns-sub",
		s06: func(t *testing.T) {
			t.Helper()
			defs := resource.GetRelated("sqs")
			if len(defs) == 0 {
				t.Fatal("SQS-S06: no related defs registered for sqs")
			}
			nonNilCheckers := map[string]bool{"sns-sub": true, "alarm": true, "lambda": true}
			for i := range defs {
				tt := defs[i].TargetType
				if nonNilCheckers[tt] {
					if defs[i].Checker == nil {
						t.Errorf("SQS-S06: Checker for target %q must be non-nil (real checker); got nil", tt)
					}
				}
				if tt == "cfn" {
					t.Errorf("SQS-S06: cfn related def must not be registered (removed); found unexpected def")
				}
			}
		},
	},

	// ------------------------------------------------------------------
	// SSM Parameter
	// ------------------------------------------------------------------
	{
		shortName: "ssm",
		resource: resource.Resource{
			ID:   "/acme/prod/app/config",
			Name: "/acme/prod/app/config",
			Fields: map[string]string{
				"name":          "/acme/prod/app/config",
				"type":          "SecureString",
				"version":       "3",
				"last_modified": "2026-01-15 10:30",
				"description":   "Application config",
			},
			RawStruct: ssmtypes.ParameterMetadata{},
		},
		expectedLabels: []string{"KMS Key"},
		deliveries: []smokeDelivery{
			{"kms", 1, []string{"arn:aws:kms:us-east-1:123456789012:key/demo-key-001"}},
		},
		zeroDeliveries: []smokeDelivery{
			{"kms", 0, nil},
		},
		firstNavTarget: "kms",
		s06: func(t *testing.T) {
			t.Helper()
			defs := resource.GetRelated("ssm")
			if len(defs) == 0 {
				t.Fatal("SSM-S06: no related defs registered for ssm")
			}
			for i := range defs {
				tt := defs[i].TargetType
				if tt == "kms" {
					if defs[i].Checker == nil {
						t.Errorf("SSM-S06: Checker for target %q must be non-nil (real checker); got nil", tt)
					}
				}
				if tt == "cfn" {
					t.Errorf("SSM-S06: cfn related def must not be registered (removed); found unexpected def")
				}
			}
		},
	},

	// ------------------------------------------------------------------
	// Subnet
	// ------------------------------------------------------------------
	{
		shortName: "subnet",
		resource: resource.Resource{
			ID:   "subnet-0a1b2c3d4e5f60001",
			Name: "prod-public-a",
			Fields: map[string]string{
				"subnet_id":         "subnet-0a1b2c3d4e5f60001",
				"name":              "prod-public-a",
				"vpc_id":            "vpc-0a1b2c3d4e5f60001",
				"cidr_block":        "10.0.1.0/24",
				"availability_zone": "us-east-1a",
				"state":             "available",
				"available_ips":     "251",
			},
			RawStruct: ec2types.Subnet{},
		},
		expectedLabels: []string{"EC2 Instances", "Network Interfaces", "NAT Gateways", "Load Balancers", "Route Tables", "CloudFormation"},
		deliveries: []smokeDelivery{
			{"ec2", 3, []string{"i-0a1b2c3d4e5f60001", "i-0a1b2c3d4e5f60002", "i-0a1b2c3d4e5f60003"}},
			{"eni", 1, []string{"eni-0a1b2c3d4e5f60001"}},
			{"nat", 1, []string{"nat-0a1b2c3d4e5f60001"}},
			{"elb", 2, []string{"elb-0a1b2c3d4e5f60001", "elb-0a1b2c3d4e5f60002"}},
			{"rtb", 0, nil},
			{"cfn", 0, nil},
		},
		zeroDeliveries: []smokeDelivery{
			{"ec2", 0, nil},
			{"eni", 0, nil},
			{"nat", 0, nil},
			{"elb", 0, nil},
			{"rtb", 0, nil},
			{"cfn", 0, nil},
		},
		firstNavTarget: "ec2",
		s06: func(t *testing.T) {
			t.Helper()
			defs := resource.GetRelated("subnet")
			nonNilCheckers := map[string]bool{"ec2": true, "eni": true, "nat": true, "elb": true, "rtb": true, "cfn": true}
			for i := range defs {
				tt := defs[i].TargetType
				if nonNilCheckers[tt] {
					if defs[i].Checker == nil {
						t.Errorf("Subnet-S06: %s Checker must be non-nil; got nil", tt)
					}
				}
			}
		},
	},

	// ------------------------------------------------------------------
	// TG (Target Group)
	// ------------------------------------------------------------------
	{
		shortName: "tg",
		resource: resource.Resource{
			ID:   "my-tg",
			Name: "my-tg",
			Fields: map[string]string{
				"target_group_name": "my-tg",
				"target_group_arn":  "arn:aws:elasticloadbalancing:us-east-1:123456789012:targetgroup/my-tg/abc123",
				"vpc_id":            "vpc-abc123",
				"target_type":       "instance",
			},
			RawStruct: elbv2types.TargetGroup{},
		},
		expectedLabels: []string{"Load Balancers", "ECS Services", "Auto Scaling Groups", "CW Alarms"},
		deliveries: []smokeDelivery{
			{"elb", 1, []string{"prod-alb"}},
			{"ecs-svc", 1, []string{"api-gateway"}},
			{"asg", 0, nil},
			{"alarm", 0, nil},
		},
		zeroDeliveries: []smokeDelivery{
			{"elb", 0, nil},
			{"ecs-svc", 0, nil},
			{"asg", 0, nil},
			{"alarm", 0, nil},
		},
		firstNavTarget: "elb",
		s06: func(t *testing.T) {
			t.Helper()
			defs := resource.GetRelated("tg")
			for _, targetType := range []string{"elb", "ecs-svc", "asg", "alarm"} {
				var found *resource.RelatedDef
				for i := range defs {
					if defs[i].TargetType == targetType {
						found = &defs[i]
						break
					}
				}
				if found == nil {
					t.Errorf("TG-S06: %s related def not registered", targetType)
					continue
				}
				if found.Checker == nil {
					t.Errorf("TG-S06: %s Checker must be non-nil; got nil", targetType)
				}
			}
			for i := range defs {
				if defs[i].TargetType == "cfn" {
					t.Errorf("TG-S06: cfn related def must not be registered (removed); found unexpected def")
				}
			}
		},
	},

	// ------------------------------------------------------------------
	// TGW (Transit Gateway)
	// ------------------------------------------------------------------
	{
		shortName: "tgw",
		resource: resource.Resource{
			ID:   "tgw-0a1b2c3d4e5f67890",
			Name: "prod-tgw",
			Fields: map[string]string{
				"tgw_id": "tgw-0a1b2c3d4e5f67890",
				"name":   "prod-tgw",
				"state":  "available",
			},
			RawStruct: ec2types.TransitGateway{},
		},
		expectedLabels: []string{"VPCs", "Route Tables", "CloudFormation"},
		deliveries: []smokeDelivery{
			{"rtb", 1, []string{"rtb-0aaa111111111111a"}},
			{"vpc", 0, nil},
			{"cfn", 0, nil},
		},
		zeroDeliveries: []smokeDelivery{
			{"rtb", 0, nil},
			{"vpc", 0, nil},
			{"cfn", 0, nil},
		},
		firstNavTarget: "rtb",
		s06: func(t *testing.T) {
			t.Helper()
			defs := resource.GetRelated("tgw")
			defMap := make(map[string]*resource.RelatedDef)
			for i := range defs {
				defMap[defs[i].TargetType] = &defs[i]
			}
			for _, targetType := range []string{"vpc", "cfn", "rtb"} {
				def, found := defMap[targetType]
				if !found {
					t.Errorf("TGW-S06: related def for %q not registered", targetType)
					continue
				}
				if def.Checker == nil {
					t.Errorf("TGW-S06: %q Checker must be non-nil; got nil", targetType)
				}
			}
		},
	},

	// ------------------------------------------------------------------
	// Trail (CloudTrail)
	// ------------------------------------------------------------------
	{
		shortName: "trail",
		resource: resource.Resource{
			ID:   "my-trail",
			Name: "my-trail",
			Fields: map[string]string{
				"trail_name": "my-trail",
				"s3_bucket":  "my-audit-bucket",
			},
			RawStruct: cloudtrailtypes.Trail{},
		},
		expectedLabels: []string{"S3 Bucket", "Log Groups", "SNS Topic", "KMS Key"},
		deliveries: []smokeDelivery{
			{"s3", 1, []string{"my-audit-bucket"}},
			{"logs", 1, []string{"/aws/cloudtrail/management"}},
			{"sns", 0, nil},
			{"kms", 1, []string{"arn:aws:kms:us-east-1:123456789012:key/trail-key-id"}},
		},
		zeroDeliveries: []smokeDelivery{
			{"s3", 0, nil},
			{"logs", 0, nil},
			{"sns", 0, nil},
			{"kms", 0, nil},
		},
		firstNavTarget: "s3",
		s06: func(t *testing.T) {
			t.Helper()
			defs := resource.GetRelated("trail")
			if len(defs) == 0 {
				t.Fatal("Trail-S06: no related defs registered for trail")
			}
			expectedTargets := []string{"s3", "logs", "sns", "kms"}
			for _, targetType := range expectedTargets {
				var found *resource.RelatedDef
				for i := range defs {
					if defs[i].TargetType == targetType {
						found = &defs[i]
						break
					}
				}
				if found == nil {
					t.Errorf("Trail-S06: related def for target %q not registered", targetType)
					continue
				}
				if found.Checker == nil {
					t.Errorf("Trail-S06: Checker for target %q must be non-nil (real implementation); got nil", targetType)
				}
			}
		},
	},

	// ------------------------------------------------------------------
	// VPC
	// ------------------------------------------------------------------
	{
		shortName: "vpc",
		resource: resource.Resource{
			ID:   "vpc-0abc123def456789a",
			Name: "prod-vpc",
			Fields: map[string]string{
				"vpc_id":     "vpc-0abc123def456789a",
				"name":       "prod-vpc",
				"cidr_block": "10.0.0.0/16",
				"state":      "available",
				"is_default": "false",
			},
			RawStruct: ec2types.Vpc{},
		},
		expectedLabels: []string{
			"Subnets",
			"Security Groups",
			"EC2 Instances",
			"Load Balancers",
			"NAT Gateways",
			"Internet Gateways",
			"Route Tables",
			"VPC Endpoints",
			"CloudFormation",
		},
		deliveries: []smokeDelivery{
			{"subnet", 4, []string{"subnet-1", "subnet-2", "subnet-3", "subnet-4"}},
			{"sg", 4, []string{"sg-1", "sg-2", "sg-3", "sg-4"}},
			{"ec2", 2, []string{"i-1", "i-2"}},
			{"elb", 1, []string{"arn:aws:elasticloadbalancing:us-east-1:123456789012:loadbalancer/app/prod-elb/1234"}},
			{"nat", 2, []string{"nat-1", "nat-2"}},
			{"igw", 1, []string{"igw-1"}},
			{"rtb", 3, []string{"rtb-1", "rtb-2", "rtb-3"}},
			{"vpce", 1, []string{"vpce-1"}},
			{"cfn", 0, nil},
		},
		zeroDeliveries: []smokeDelivery{
			{"subnet", 0, nil},
			{"sg", 0, nil},
			{"ec2", 0, nil},
			{"elb", 0, nil},
			{"nat", 0, nil},
			{"igw", 0, nil},
			{"rtb", 0, nil},
			{"vpce", 0, nil},
			{"cfn", 0, nil},
		},
		firstNavTarget: "subnet",
		s06: func(t *testing.T) {
			t.Helper()
			defs := resource.GetRelated("vpc")
			realCheckers := []string{"subnet", "sg", "ec2", "elb", "nat", "igw", "rtb", "vpce", "cfn"}
			for _, targetType := range realCheckers {
				var found *resource.RelatedDef
				for i := range defs {
					if defs[i].TargetType == targetType {
						found = &defs[i]
						break
					}
				}
				if found == nil {
					t.Errorf("VPC-S06: related def for %q not registered", targetType)
					continue
				}
				if found.Checker == nil {
					t.Errorf("VPC-S06: Checker for %q must be non-nil; got nil", targetType)
				}
			}
		},
	},

	// ------------------------------------------------------------------
	// VPCE (VPC Endpoint)
	// ------------------------------------------------------------------
	{
		shortName: "vpce",
		resource: resource.Resource{
			ID:   "vpce-0aaa111111111111a",
			Name: "com.amazonaws.us-east-1.s3",
			Fields: map[string]string{
				"vpce_id":      "vpce-0aaa111111111111a",
				"service_name": "com.amazonaws.us-east-1.s3",
				"type":         "Gateway",
				"state":        "available",
				"vpc_id":       "vpc-abc123",
			},
			RawStruct: ec2types.VpcEndpoint{},
		},
		expectedLabels: []string{"Subnets", "Security Groups", "Route Tables", "Network Interfaces"},
		deliveries: []smokeDelivery{
			{"subnet", 2, []string{"subnet-vpce1", "subnet-vpce2"}},
			{"sg", 1, []string{"sg-vpce1"}},
			{"rtb", 2, []string{"rtb-vpce1", "rtb-vpce2"}},
			{"eni", 0, nil},
		},
		zeroDeliveries: []smokeDelivery{
			{"subnet", 0, nil},
			{"sg", 0, nil},
			{"rtb", 0, nil},
			{"eni", 0, nil},
		},
		firstNavTarget: "subnet",
		s06: func(t *testing.T) {
			t.Helper()
			defs := resource.GetRelated("vpce")
			if len(defs) != 4 {
				t.Fatalf("VPCE-S06: expected 4 related defs for vpce, got %d", len(defs))
			}
			expectedTargets := []string{"subnet", "sg", "rtb", "eni"}
			for _, target := range expectedTargets {
				var found *resource.RelatedDef
				for i := range defs {
					if defs[i].TargetType == target {
						found = &defs[i]
						break
					}
				}
				if found == nil {
					t.Errorf("VPCE-S06: related def for target %q not registered", target)
					continue
				}
				if found.Checker == nil {
					t.Errorf("VPCE-S06: Checker for target %q must be non-nil (real implementation); got nil", target)
				}
			}
		},
	},

	// ------------------------------------------------------------------
	// WAF
	// ------------------------------------------------------------------
	{
		shortName: "waf",
		resource: resource.Resource{
			ID:   "my-waf-id",
			Name: "my-waf",
			Fields: map[string]string{
				"name":        "my-waf",
				"id":          "my-waf-id",
				"description": "Test WAF",
			},
			RawStruct: wafv2types.WebACLSummary{},
		},
		expectedLabels: []string{"Load Balancers", "API Gateways", "CloudFront"},
		deliveries: []smokeDelivery{
			{"elb", 1, []string{"arn:aws:elasticloadbalancing:us-east-1:123456789012:loadbalancer/app/my-alb/abc123"}},
			{"apigw", 0, nil},
			{"cf", 0, nil},
		},
		zeroDeliveries: []smokeDelivery{
			{"elb", 0, nil},
			{"apigw", 0, nil},
			{"cf", 0, nil},
		},
		firstNavTarget: "elb",
		s06: func(t *testing.T) {
			t.Helper()
			defs := resource.GetRelated("waf")
			for _, targetType := range []string{"elb", "apigw", "cf"} {
				var def *resource.RelatedDef
				for i := range defs {
					if defs[i].TargetType == targetType {
						def = &defs[i]
						break
					}
				}
				if def == nil {
					t.Fatalf("WAF-S06: %s related def not registered", targetType)
				}
				if def.Checker == nil {
					t.Fatalf("WAF-S06: %s Checker must be non-nil (real checker); got nil", targetType)
				}
			}
		},
	},
}

// ---------------------------------------------------------------------------
// Table runner helpers
// ---------------------------------------------------------------------------

func buildSmokeDetail(tc smokeTestCase, width, height int) views.DetailModel {
	k := keys.Default()
	d := views.NewDetail(tc.resource, tc.shortName, nil, k)
	d.SetSize(width, height)
	return d
}

func deliverAll(d views.DetailModel, shortName string, deliveries []smokeDelivery) views.DetailModel {
	for _, del := range deliveries {
		d = deliverSmokeRelatedResult(d, shortName, del.targetType, del.count, del.ids...)
	}
	return d
}

// ---------------------------------------------------------------------------
// Test_RelatedSmoke — single entry point for all 19 × 6 = 114 subtests
// ---------------------------------------------------------------------------

func Test_RelatedSmoke(t *testing.T) {
	for _, tc := range relatedSmokeTable {
		t.Run(tc.shortName, func(t *testing.T) {
			// S01 — right column visible at width=120
			t.Run("S01_RightColVisible", func(t *testing.T) {
				d := buildSmokeDetail(tc, 120, 30)
				v := d.View()
				if !strings.Contains(v, "RELATED") {
					t.Fatalf("%s-S01: right column must auto-show at width=120; 'RELATED' header not found in View()", tc.shortName)
				}
				if !strings.Contains(v, "│") {
					t.Fatalf("%s-S01: column separator │ must be present at width=120", tc.shortName)
				}
			})

			// S02 — correct labels in right column
			t.Run("S02_CorrectLabels", func(t *testing.T) {
				d := buildSmokeDetail(tc, 120, 30)
				plain := stripAnsi(d.View())
				if !strings.Contains(plain, "RELATED") {
					t.Skipf("%s-S02: right column not visible; skipping label check", tc.shortName)
				}
				for _, label := range tc.expectedLabels {
					if !strings.Contains(plain, label) {
						t.Errorf("%s-S02: expected label %q in right column; not found\nview:\n%s", tc.shortName, label, plain)
					}
				}
			})

			// S03 — counts display correctly after results delivered
			t.Run("S03_CountsAfterDeliver", func(t *testing.T) {
				d := buildSmokeDetail(tc, 120, 30)
				if !strings.Contains(d.View(), "RELATED") {
					t.Skipf("%s-S03: right column not visible", tc.shortName)
				}
				d = deliverAll(d, tc.shortName, tc.deliveries)
				plain := stripAnsi(d.View())

				// Find the highest positive count delivered
				hasPositive := false
				for _, del := range tc.deliveries {
					if del.count > 0 {
						hasPositive = true
						countStr := "(" + countToStr(del.count) + ")"
						if !strings.Contains(plain, countStr) {
							t.Errorf("%s-S03: expected %q count in right column; not found\nview:\n%s", tc.shortName, countStr, plain)
						}
					}
				}
				if !hasPositive {
					// All deliveries are zero; just verify (0) appears
					if !strings.Contains(plain, "(0)") {
						t.Errorf("%s-S03: expected '(0)' count; not found\nview:\n%s", tc.shortName, plain)
					}
					return
				}
				// Verify at least one zero-count row if any
				hasZero := false
				for _, del := range tc.deliveries {
					if del.count == 0 {
						hasZero = true
						break
					}
				}
				if hasZero && !strings.Contains(plain, "(0)") {
					t.Errorf("%s-S03: expected '(0)' for zero-count row; not found\nview:\n%s", tc.shortName, plain)
				}
			})

			// S04 — Tab + Enter on first count>0 row emits RelatedNavigateMsg
			t.Run("S04_EnterNavigates", func(t *testing.T) {
				d := buildSmokeDetail(tc, 120, 30)
				if !strings.Contains(d.View(), "RELATED") {
					t.Skipf("%s-S04: right column not visible", tc.shortName)
				}
				d = deliverAll(d, tc.shortName, tc.deliveries)
				d, _ = pressDetailTab(d)

				_, cmd := d.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
				if cmd == nil {
					t.Fatalf("%s-S04: Enter on count>0 row must emit a cmd; got nil", tc.shortName)
				}
				msg := cmd()
				nav, ok := msg.(messages.RelatedNavigateMsg)
				if !ok {
					t.Fatalf("%s-S04: Enter must produce RelatedNavigateMsg, got %T", tc.shortName, msg)
				}
				if nav.TargetType != tc.firstNavTarget {
					t.Errorf("%s-S04: RelatedNavigateMsg.TargetType = %q, want %q", tc.shortName, nav.TargetType, tc.firstNavTarget)
				}
			})

			// S05 — Enter on all-count=0 right column emits no RelatedNavigateMsg
			t.Run("S05_EnterOnAllZeroNoNav", func(t *testing.T) {
				d := buildSmokeDetail(tc, 120, 30)
				if !strings.Contains(d.View(), "RELATED") {
					t.Skipf("%s-S05: right column not visible", tc.shortName)
				}
				d = deliverAll(d, tc.shortName, tc.zeroDeliveries)
				d, _ = pressDetailTab(d)

				_, cmd := d.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
				if cmd != nil {
					msg := cmd()
					if msg != nil {
						if _, isNav := msg.(messages.RelatedNavigateMsg); isNav {
							t.Errorf("%s-S05: Enter on all-count=0 right column must not produce RelatedNavigateMsg", tc.shortName)
						}
					}
				}
			})

			// S06 — checker registration + demo checker (per-type function)
			t.Run("S06_CheckerRegistration", func(t *testing.T) {
				tc.s06(t)
			})
		})
	}
}

// countToStr converts an int count to its string representation (e.g. 1 → "1").
func countToStr(n int) string {
	if n < 0 {
		return "-" + countToStr(-n)
	}
	if n < 10 {
		return string(rune('0' + n))
	}
	return countToStr(n/10) + string(rune('0'+n%10))
}
