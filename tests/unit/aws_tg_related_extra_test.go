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

func TestRelated_TG_VPC_Found(t *testing.T) {
	res := tgSrcResource()
	checker := tgCheckerByTarget(t, "vpc")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.Count() != 1 {
		t.Errorf("Count = %d, want 1", result.Count())
	}
	if len(result.ResourceIDs()) != 1 || result.ResourceIDs()[0] != "vpc-abc123" {
		t.Errorf("ResourceIDs = %v, want [vpc-abc123]", result.ResourceIDs())
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

	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0 (empty vpc_id)", result.Count())
	}
}

func TestRelated_TG_CFN_Found(t *testing.T) {
	res := tgSrcResource()
	clients := &awsclient.ServiceClients{
		ELBv2: newFakeELBv2CRWithCFNTag("my-stack"),
	}

	checker := tgCheckerByTarget(t, "cfn")
	result := checker(context.Background(), clients, res, resource.ResourceCache{})

	if result.Count() != 1 {
		t.Errorf("Count = %d, want 1", result.Count())
	}
	if len(result.ResourceIDs()) != 1 || result.ResourceIDs()[0] != "my-stack" {
		t.Errorf("ResourceIDs = %v, want [my-stack]", result.ResourceIDs())
	}
}

func TestRelated_TG_CFN_NoTag(t *testing.T) {
	res := tgSrcResource()
	clients := &awsclient.ServiceClients{
		ELBv2: &fakeELBv2CR{},
	}

	checker := tgCheckerByTarget(t, "cfn")
	result := checker(context.Background(), clients, res, resource.ResourceCache{})

	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0 (no CFN tag)", result.Count())
	}
}

func TestRelated_TG_CFN_NilClients(t *testing.T) {
	res := tgSrcResource()
	checker := tgCheckerByTarget(t, "cfn")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})
	if result.State() != domain.RelatedUnknown {
		t.Errorf("Count = %d, want -1 (nil clients)", result.Count())
	}
}

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

	if result.Count() != 2 {
		t.Errorf("Count = %d, want 2", result.Count())
	}
	found := map[string]bool{}
	for _, id := range result.ResourceIDs() {
		found[id] = true
	}
	if !found["i-0abc123def456789a"] || !found["i-0def456abc123789b"] {
		t.Errorf("ResourceIDs = %v, want both instance IDs", result.ResourceIDs())
	}
}

func TestRelated_TG_EC2_LambdaTypeSkipped(t *testing.T) {
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

	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0 (lambda-type TG has no EC2 instances)", result.Count())
	}
}

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

	if result.Count() != 1 {
		t.Errorf("Count = %d, want 1", result.Count())
	}
	if len(result.ResourceIDs()) != 1 || result.ResourceIDs()[0] != "my-processor" {
		t.Errorf("ResourceIDs = %v, want [my-processor]", result.ResourceIDs())
	}
}

func TestRelated_TG_Lambda_NonLambdaTypeReturnsZero(t *testing.T) {
	res := tgSrcResource()
	checker := tgCheckerByTarget(t, "lambda")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0 (instance-type TG has no lambda targets)", result.Count())
	}
}
