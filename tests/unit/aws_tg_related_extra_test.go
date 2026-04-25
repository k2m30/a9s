// aws_tg_related_extra_test.go covers TG related checkers skipped in prior wave:
// checkTGVPC, checkTGBackup, checkTGCFN, checkTGDBC, checkTGDBI, checkTGEC2,
// checkTGLambda, checkTGLogs, checkTGDBISnap, checkTGSG, checkTGSubnet.
package unit_test

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	elbv2types "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/resource"
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
// checkTGBackup — returns 0 when no TG ARN, -1 when TG has ARN (stub)
// The boundary between 0 and -1 is the real logic worth testing.
// ---------------------------------------------------------------------------

func TestRelated_TG_Backup_HasARN(t *testing.T) {
	res := tgSrcResource()
	checker := tgCheckerByTarget(t, "backup")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (has TG ARN → unknown)", result.Count)
	}
}

func TestRelated_TG_Backup_NoARN(t *testing.T) {
	res := resource.Resource{
		ID:     "no-arn-tg",
		Name:   "no-arn-tg",
		Fields: map[string]string{},
	}
	checker := tgCheckerByTarget(t, "backup")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (no TG ARN)", result.Count)
	}
}

// ---------------------------------------------------------------------------
// checkTGDBC and checkTGDBI — same boundary logic as Backup
// ---------------------------------------------------------------------------

func TestRelated_TG_DBC_HasARN(t *testing.T) {
	res := tgSrcResource()
	checker := tgCheckerByTarget(t, "dbc")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})
	if result.Count != -1 {
		t.Errorf("Count = %d, want -1", result.Count)
	}
}

func TestRelated_TG_DBI_HasARN(t *testing.T) {
	res := tgSrcResource()
	checker := tgCheckerByTarget(t, "dbi")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})
	if result.Count != -1 {
		t.Errorf("Count = %d, want -1", result.Count)
	}
}

// ---------------------------------------------------------------------------
// checkTGLogs — returns 0 when no TG ARN, -1 when TG has ARN (stub)
// ---------------------------------------------------------------------------

func TestRelated_TG_Logs_HasARN(t *testing.T) {
	res := tgSrcResource()
	checker := tgCheckerByTarget(t, "logs")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})
	if result.Count != -1 {
		t.Errorf("Count = %d, want -1", result.Count)
	}
}

func TestRelated_TG_Logs_NoARN(t *testing.T) {
	res := resource.Resource{
		ID:     "no-arn-tg",
		Name:   "no-arn-tg",
		Fields: map[string]string{},
	}
	checker := tgCheckerByTarget(t, "logs")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})
	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (no TG ARN)", result.Count)
	}
}

// ---------------------------------------------------------------------------
// checkTGDBISnap — same boundary as Logs
// ---------------------------------------------------------------------------

func TestRelated_TG_DBISnap_HasARN(t *testing.T) {
	res := tgSrcResource()
	checker := tgCheckerByTarget(t, "dbi-snap")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})
	if result.Count != -1 {
		t.Errorf("Count = %d, want -1", result.Count)
	}
}

// ---------------------------------------------------------------------------
// checkTGSG — returns -1 when vpc_id present, 0 otherwise
// ---------------------------------------------------------------------------

func TestRelated_TG_SG_VPCScopedReturnsUnknown(t *testing.T) {
	res := tgSrcResource() // Fields["vpc_id"] = "vpc-abc123"
	checker := tgCheckerByTarget(t, "sg")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (vpc_id present → sg lookup not in scope)", result.Count)
	}
}

func TestRelated_TG_SG_NoVPCReturnsZero(t *testing.T) {
	tgARNVal := tgTestARN
	res := resource.Resource{
		ID:   "lambda-tg",
		Name: "lambda-tg",
		Fields: map[string]string{
			"target_group_arn": tgARNVal,
			"vpc_id":           "",
		},
		RawStruct: elbv2types.TargetGroup{
			TargetGroupArn: &tgARNVal,
			TargetType:     elbv2types.TargetTypeEnumLambda,
		},
	}
	checker := tgCheckerByTarget(t, "sg")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (no vpc_id)", result.Count)
	}
}

// ---------------------------------------------------------------------------
// checkTGSubnet — same vpc_id boundary as SG
// ---------------------------------------------------------------------------

func TestRelated_TG_Subnet_VPCScopedReturnsUnknown(t *testing.T) {
	res := tgSrcResource() // vpc_id present
	checker := tgCheckerByTarget(t, "subnet")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})
	if result.Count != -1 {
		t.Errorf("Count = %d, want -1", result.Count)
	}
}

func TestRelated_TG_Subnet_NoVPCReturnsZero(t *testing.T) {
	tgARNVal := tgTestARN
	res := resource.Resource{
		ID:   "lambda-tg",
		Name: "lambda-tg",
		Fields: map[string]string{
			"target_group_arn": tgARNVal,
			"vpc_id":           "",
		},
		RawStruct: elbv2types.TargetGroup{
			TargetGroupArn: &tgARNVal,
		},
	}
	checker := tgCheckerByTarget(t, "subnet")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})
	if result.Count != 0 {
		t.Errorf("Count = %d, want 0", result.Count)
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
	if result.Count != -1 {
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
