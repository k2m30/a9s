package unit_test

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	docdb_types "github.com/aws/aws-sdk-go-v2/service/docdb/types"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	efstypes "github.com/aws/aws-sdk-go-v2/service/efs/types"
	lambdatypes "github.com/aws/aws-sdk-go-v2/service/lambda/types"
	_ "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

func TestRelated_KMS_Role_NilClients(t *testing.T) {
	const keyID = "mrk-aaa111bbb222"
	src := resource.Resource{
		ID: keyID,
		Fields: map[string]string{
			"key_id": keyID,
		},
	}
	checker := kmsCheckerByTarget(t, "role")
	result := checker(context.Background(), nil, src, resource.ResourceCache{})
	if result.State() != domain.RelatedUnknown {
		t.Errorf("Count = %d, want -1 (nil KMS client)", result.Count())
	}
}

func TestRelated_KMS_Role_EmptyKeyID(t *testing.T) {
	src := resource.Resource{
		ID:     "",
		Fields: map[string]string{},
	}
	checker := kmsCheckerByTarget(t, "role")
	result := checker(context.Background(), nil, src, resource.ResourceCache{})
	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0 (empty key ID)", result.Count())
	}
}

func TestRelated_DBC_Subnet_NilClientsW5(t *testing.T) {
	src := resource.Resource{
		ID: "my-docdb-cluster",
		RawStruct: docdb_types.DBCluster{
			DBClusterIdentifier: aws.String("my-docdb-cluster"),
			DBSubnetGroup:       aws.String("my-subnet-group"),
		},
	}
	checker := dbcCheckerByTarget(t, "subnet")
	result := checker(context.Background(), nil, src, resource.ResourceCache{})
	// With no client set no call was made, so nothing failed: the pivot
	// reads unknown, never an AWS error.
	if result.State() != domain.RelatedUnknown || result.Err() != nil {
		t.Errorf("State = %v Err = %v, want unknown with no error (nil clients)", result.State(), result.Err())
	}
}

func TestRelated_DBC_Subnet_WrongRawStruct(t *testing.T) {
	src := resource.Resource{
		ID:        "my-docdb-cluster",
		RawStruct: "not-a-cluster",
	}
	checker := dbcCheckerByTarget(t, "subnet")
	result := checker(context.Background(), nil, src, resource.ResourceCache{})
	// Nothing was read, so the answer is the error rather than a "?" that
	// would stand for three different states.
	if result.State() != domain.RelatedError {
		t.Errorf("State = %v, want RelatedError (RawStruct is not a DBCluster)", result.State())
	}
}

func TestRelated_DBC_Subnet_NoSubnetGroup(t *testing.T) {
	src := resource.Resource{
		ID: "my-docdb-cluster",
		RawStruct: docdb_types.DBCluster{
			DBClusterIdentifier: aws.String("my-docdb-cluster"),
			DBSubnetGroup:       nil,
		},
	}
	checker := dbcCheckerByTarget(t, "subnet")
	result := checker(context.Background(), nil, src, resource.ResourceCache{})
	// A cluster naming no subnet group has no subnets, which is a fact about
	// the cluster rather than a gap in what could be read.
	if result.State() != domain.RelatedResolved || result.Count() != 0 {
		t.Errorf("State = %v, Count = %d; want Resolved 0 (no DBSubnetGroup name)", result.State(), result.Count())
	}
}

func TestRelated_VPC_ENI_FieldMatch(t *testing.T) {
	res := vpcSrcResource()
	cache := resource.ResourceCache{
		"eni": resource.ResourceCacheEntry{Resources: []resource.Resource{
			{
				ID:     "eni-aabbccdd",
				Fields: map[string]string{"vpc_id": vpcTestID},
			},
		}},
	}
	checker := vpcCheckerByTarget(t, "eni")
	result := checker(context.Background(), nil, res, cache)
	if result.Count() != 1 {
		t.Errorf("Count = %d, want 1 (eni vpc_id matches)", result.Count())
	}
	if len(result.ResourceIDs()) != 1 || result.ResourceIDs()[0] != "eni-aabbccdd" {
		t.Errorf("ResourceIDs = %v, want [eni-aabbccdd]", result.ResourceIDs())
	}
}

func TestRelated_VPC_ENI_RawStructMatch(t *testing.T) {
	res := vpcSrcResource()
	eniRes := resource.Resource{
		ID:     "eni-rawstruct001",
		Fields: map[string]string{},
		RawStruct: ec2types.NetworkInterface{
			NetworkInterfaceId: aws.String("eni-rawstruct001"),
			VpcId:              aws.String(vpcTestID),
		},
	}
	cache := resource.ResourceCache{
		"eni": resource.ResourceCacheEntry{Resources: []resource.Resource{eniRes}},
	}
	checker := vpcCheckerByTarget(t, "eni")
	result := checker(context.Background(), nil, res, cache)
	if result.Count() != 1 {
		t.Errorf("Count = %d, want 1 (RawStruct VpcId match)", result.Count())
	}
}

func TestRelated_VPC_ENI_NoMatch(t *testing.T) {
	res := vpcSrcResource()
	cache := resource.ResourceCache{
		"eni": resource.ResourceCacheEntry{Resources: []resource.Resource{
			{
				ID:     "eni-other",
				Fields: map[string]string{"vpc_id": "vpc-zzz999"},
			},
		}},
	}
	checker := vpcCheckerByTarget(t, "eni")
	result := checker(context.Background(), nil, res, cache)
	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0 (different VPC)", result.Count())
	}
}

func TestRelated_VPC_ENI_EmptyVPCID(t *testing.T) {
	src := resource.Resource{
		ID:     "",
		Fields: map[string]string{},
		RawStruct: ec2types.Vpc{
			VpcId: aws.String(""),
		},
	}
	checker := vpcCheckerByTarget(t, "eni")
	result := checker(context.Background(), nil, src, resource.ResourceCache{})
	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0 (empty VPC ID)", result.Count())
	}
}

func TestRelated_VPC_TGW_NilClients(t *testing.T) {
	res := vpcSrcResource()
	checker := vpcCheckerByTarget(t, "tgw")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})
	if result.State() != domain.RelatedUnknown {
		t.Errorf("Count = %d, want -1 (nil EC2 client)", result.Count())
	}
}

func TestRelated_VPC_TGW_EmptyVPCID(t *testing.T) {
	src := resource.Resource{
		ID:     "",
		Fields: map[string]string{},
		RawStruct: ec2types.Vpc{
			VpcId: aws.String(""),
		},
	}
	checker := vpcCheckerByTarget(t, "tgw")
	result := checker(context.Background(), nil, src, resource.ResourceCache{})
	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0 (empty VPC ID)", result.Count())
	}
}

func TestRelated_Lambda_SQS_NilClientFromIDField(t *testing.T) {
	src := resource.Resource{
		ID:        "func-from-id-field",
		Name:      "",
		RawStruct: lambdatypes.FunctionConfiguration{},
	}
	checker := lambdaCheckerByTarget(t, "sqs")
	result := checker(context.Background(), nil, src, resource.ResourceCache{})
	if result.State() != domain.RelatedUnknown {
		t.Errorf("Count = %d, want -1 (nil Lambda client, ID-path name extraction)", result.Count())
	}
}

func TestRelated_Lambda_CFN_NilClientWithARN(t *testing.T) {
	src := resource.Resource{
		ID:   "my-tagged-function",
		Name: "my-tagged-function",
		RawStruct: lambdatypes.FunctionConfiguration{
			FunctionName: aws.String("my-tagged-function"),
			FunctionArn:  aws.String("arn:aws:lambda:us-east-1:123456789012:function:my-tagged-function"),
		},
	}
	checker := lambdaCheckerByTarget(t, "cfn")
	result := checker(context.Background(), nil, src, resource.ResourceCache{})
	if result.State() != domain.RelatedUnknown {
		t.Errorf("Count = %d, want -1 (nil Lambda client, has ARN)", result.Count())
	}
}

func TestRelated_Lambda_EBRule_NilClientWithName(t *testing.T) {
	src := resource.Resource{
		ID:   "rule-target-function",
		Name: "rule-target-function",
		RawStruct: lambdatypes.FunctionConfiguration{
			FunctionName: aws.String("rule-target-function"),
			FunctionArn:  aws.String("arn:aws:lambda:us-east-1:123456789012:function:rule-target-function"),
		},
	}
	checker := lambdaCheckerByTarget(t, "eb-rule")
	result := checker(context.Background(), nil, src, resource.ResourceCache{})
	if result.State() != domain.RelatedUnknown {
		t.Errorf("Count = %d, want -1 (nil EventBridge client)", result.Count())
	}
}

func TestRelated_EFS_Lambda_EmptyFSID(t *testing.T) {
	src := resource.Resource{
		ID: "",
		RawStruct: efstypes.FileSystemDescription{
			FileSystemId: aws.String(""),
		},
	}
	checker := efsCheckerByTarget(t, "lambda")
	result := checker(context.Background(), nil, src, resource.ResourceCache{})
	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0 (empty filesystem ID)", result.Count())
	}
}

func TestRelated_EFS_Lambda_NilClients(t *testing.T) {
	src := resource.Resource{
		ID: "fs-abc1234def567890",
		RawStruct: efstypes.FileSystemDescription{
			FileSystemId: aws.String("fs-abc1234def567890"),
		},
	}
	checker := efsCheckerByTarget(t, "lambda")
	result := checker(context.Background(), nil, src, resource.ResourceCache{})
	if result.State() != domain.RelatedUnknown {
		t.Errorf("Count = %d, want -1 (nil EFS client)", result.Count())
	}
}
