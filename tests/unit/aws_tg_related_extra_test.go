// aws_tg_related_extra_test.go covers TG related checkers skipped in prior wave:
// checkTGVPC, checkTGCFN, checkTGEC2, checkTGLambda.
//
// checkTGBackup, checkTGDBC, checkTGDBI, checkTGLogs, checkTGDBISnap, checkTGSG,
// checkTGSubnet were removed along with their registrations: each was
// hardcoded to State: RelatedUnknown whenever the TG had an ARN (or, for sg/subnet,
// whenever Fields["vpc_id"] != ""), with no AWS API path to resolve target
// identity from cache alone (DescribeTargetHealth + matching IP addresses
// against instance/DB ENIs, or resolving to the parent ELB's access logs —
// all outside the checker's call budget). See
// qa_demo_pivot_coverage_test.go's knownDisconnectedPivots terminal-state
// comment for the burn-down precedent this deletion follows.
package unit_test

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	elbv2types "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// ---------------------------------------------------------------------------
// checkTGVPC — reads Fields["vpc_id"] (Pattern F)
// ---------------------------------------------------------------------------

func TestRelated_TG_VPC_Found(t *testing.T) {
	res := tgSrcResource() // has Fields["vpc_id"] = "vpc-abc123"
	checker := tgCheckerByTarget(t, "vpc")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != "vpc-abc123" {
		t.Errorf("ResourceIDs = %v, want [vpc-abc123]", result.ResourceIDs)
	}
}

func TestRelated_TG_VPC_Empty(t *testing.T) {
	tgARNVal := tgTestARN
	res := resource.Resource{
		ID:   "my-tg",
		Name: "my-tg",
		Fields: map[string]string{
			"target_group_arn": tgARNVal,
			"vpc_id":           "",
		},
		RawStruct: elbv2types.TargetGroup{
			TargetGroupArn: &tgARNVal,
		},
	}
	checker := tgCheckerByTarget(t, "vpc")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (empty vpc_id)", result.Count)
	}
}

// ---------------------------------------------------------------------------
// checkTGCFN — DescribeTags call extracts aws:cloudformation:stack-name
// ---------------------------------------------------------------------------

func TestRelated_TG_CFN_Found(t *testing.T) {
	res := tgSrcResource()
	clients := &awsclient.ServiceClients{
		ELBv2: newFakeELBv2CRWithCFNTag("my-stack"),
	}

	checker := tgCheckerByTarget(t, "cfn")
	result := checker(context.Background(), clients, res, resource.ResourceCache{})

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != "my-stack" {
		t.Errorf("ResourceIDs = %v, want [my-stack]", result.ResourceIDs)
	}
}

func TestRelated_TG_CFN_NoTag(t *testing.T) {
	// DescribeTags returns no cloudformation tag → Count: 0.
	res := tgSrcResource()
	clients := &awsclient.ServiceClients{
		ELBv2: &fakeELBv2CR{}, // empty DescribeTags response
	}

	checker := tgCheckerByTarget(t, "cfn")
	result := checker(context.Background(), clients, res, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (no CFN tag)", result.Count)
	}
}

func TestRelated_TG_CFN_NilClients(t *testing.T) {
	res := tgSrcResource()
	checker := tgCheckerByTarget(t, "cfn")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})
	if result.State != domain.RelatedUnknown {
		t.Errorf("Count = %d, want -1 (nil clients)", result.Count)
	}
}

// ---------------------------------------------------------------------------
// checkTGEC2 — DescribeTargetHealth, filters i- prefix instance IDs
// ---------------------------------------------------------------------------

func TestRelated_TG_EC2_Found(t *testing.T) {
	tgARNVal := tgTestARN
	res := resource.Resource{
		ID:   "my-tg",
		Name: "my-tg",
		Fields: map[string]string{
			"target_group_arn": tgARNVal,
			"vpc_id":           "vpc-abc123",
		},
		RawStruct: elbv2types.TargetGroup{
			TargetGroupArn: &tgARNVal,
			TargetType:     elbv2types.TargetTypeEnumInstance,
		},
	}
	targets := []elbv2types.TargetHealthDescription{
		{Target: &elbv2types.TargetDescription{Id: aws.String("i-0abc123def456789a")}},
		{Target: &elbv2types.TargetDescription{Id: aws.String("i-0def456abc123789b")}},
	}
	clients := &awsclient.ServiceClients{
		ELBv2: newFakeELBv2CRWithTargetHealth(targets),
	}

	checker := tgCheckerByTarget(t, "ec2")
	result := checker(context.Background(), clients, res, resource.ResourceCache{})

	if result.Count != 2 {
		t.Errorf("Count = %d, want 2", result.Count)
	}
	found := map[string]bool{}
	for _, id := range result.ResourceIDs {
		found[id] = true
	}
	if !found["i-0abc123def456789a"] || !found["i-0def456abc123789b"] {
		t.Errorf("ResourceIDs = %v, want both instance IDs", result.ResourceIDs)
	}
}

func TestRelated_TG_EC2_LambdaTypeSkipped(t *testing.T) {
	// Lambda-type TG → EC2 checker returns 0 without calling the API.
	tgARNVal := tgTestARN
	res := resource.Resource{
		ID:   "lambda-tg",
		Name: "lambda-tg",
		Fields: map[string]string{
			"target_group_arn": tgARNVal,
		},
		RawStruct: elbv2types.TargetGroup{
			TargetGroupArn: &tgARNVal,
			TargetType:     elbv2types.TargetTypeEnumLambda,
		},
	}

	checker := tgCheckerByTarget(t, "ec2")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (lambda-type TG has no EC2 instances)", result.Count)
	}
}

// ---------------------------------------------------------------------------
// checkTGLambda — DescribeTargetHealth on lambda-type TG, extracts function name
// ---------------------------------------------------------------------------

func TestRelated_TG_Lambda_Found(t *testing.T) {
	tgARNVal := tgTestARN
	res := resource.Resource{
		ID:   "lambda-tg",
		Name: "lambda-tg",
		Fields: map[string]string{
			"target_group_arn": tgARNVal,
		},
		RawStruct: elbv2types.TargetGroup{
			TargetGroupArn: &tgARNVal,
			TargetType:     elbv2types.TargetTypeEnumLambda,
		},
	}
	targets := []elbv2types.TargetHealthDescription{
		{Target: &elbv2types.TargetDescription{
			Id: aws.String("arn:aws:lambda:us-east-1:123456789012:function:my-processor"),
		}},
	}
	clients := &awsclient.ServiceClients{
		ELBv2: newFakeELBv2CRWithTargetHealth(targets),
	}

	checker := tgCheckerByTarget(t, "lambda")
	result := checker(context.Background(), clients, res, resource.ResourceCache{})

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != "my-processor" {
		t.Errorf("ResourceIDs = %v, want [my-processor]", result.ResourceIDs)
	}
}

func TestRelated_TG_Lambda_NonLambdaTypeReturnsZero(t *testing.T) {
	res := tgSrcResource() // TargetType = instance
	checker := tgCheckerByTarget(t, "lambda")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (instance-type TG has no lambda targets)", result.Count)
	}
}
